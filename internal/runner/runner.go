package runner

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/script"
	"github.com/githiago-f/lazyapi/internal/store"
)

// SafeMethods are run by default: read-only verbs that do not change state.
var SafeMethods = []string{"GET", "HEAD", "OPTIONS"}

// Filter selects operations. Empty fields select everything.
type Filter struct {
	Methods []string       // upper-case; nil = all methods
	Tags    []string       // any of these tags
	Ops     []string       // operationId or "METHOD /path"
	Grep    *regexp.Regexp // matched against "METHOD /path operationId summary"
}

// ParseMethods turns a --methods value into Filter.Methods: "" → SafeMethods,
// "all"/"*" → nil (every method), otherwise the listed methods.
func ParseMethods(v string) []string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "":
		return SafeMethods
	case "all", "*":
		return nil
	}
	var out []string
	for _, m := range strings.Split(v, ",") {
		if m = strings.ToUpper(strings.TrimSpace(m)); m != "" {
			out = append(out, m)
		}
	}
	return out
}

// Options configure a run.
type Options struct {
	Server  string            // URL or index into the spec's servers; "" = first
	Env     map[string]string // environment ({{env.X}})
	Params  map[string]string // path params (Run: only for operations that declare them)
	Query   map[string]string // query params added to every request
	Headers map[string]string // headers added to every request
	Body    *string           // request body override (RunOne)
	// BodyType overrides the media type; Files adds multipart file parts.
	BodyType model.MimeType
	Files    map[string]string
	// EnvName is the named environment in use (reported only).
	EnvName string
	// Vars are the initial run variables ({{var.X}}).
	Vars map[string]string
	// Auth fills credentials by security scheme name.
	Auth map[string]model.AuthSecrets

	Filter  Filter
	Timeout time.Duration // per request
	// HTTPClient sends every request (nil: http.DefaultClient). MaxBody caps
	// response bodies (0: model.DefaultMaxBody).
	HTTPClient *http.Client
	MaxBody    int64

	ScriptPath    string               // overrides every operation's x-lazyapi-tests
	Modules       *script.ModuleLoader // URL imports; nil disables them
	ScriptTimeout time.Duration
	NoContract    bool

	// Flows run after the operations (see RunFlow); FlowTimeout bounds each.
	// SkipOperations runs the flows only.
	Flows          []string
	SkipOperations bool
	FlowTimeout    time.Duration

	DryRun   bool
	Bail     bool // stop scheduling after the first failure or error
	Parallel int  // concurrent operations (default 1); >1 isolates env/vars per operation
	Retries  int  // extra attempts on network errors and 5xx

	// OnResult, when set, receives each result in spec order as soon as it
	// (and every result before it) is complete.
	OnResult func(OpResult)
}

// target is one selected operation.
type target struct {
	ref     model.OpenAPIRef
	op      *openapi3.Operation
	skipped string // non-empty: excluded with this reason
}

// plan lists the operations a run would visit: reads, creates, updates,
// then deletes (spec order within each group). Operations not
// selected by Ops/Tags/Grep are omitted; excluded methods are kept with a
// skip reason so they are visible in reports.
func plan(spec *openapi3.T, specPath string, f Filter) []target {
	var out []target
	for _, item := range store.ListOperations(spec, specPath) {
		method := item.Method.Label()
		ref := model.OpenAPIRef{FilePath: specPath, Path: item.URI, Method: method}
		op := store.FindOperation(spec, ref)
		if op == nil || !f.selects(method, item.URI, op) {
			continue
		}
		t := target{ref: ref, op: op}
		if f.Methods != nil && !slices.Contains(f.Methods, method) {
			t.skipped = fmt.Sprintf("method %s not enabled (use --methods %s or --methods all)", method, method)
		}
		out = append(out, t)
	}
	// Reads first, deletes last, so a run does not remove what later
	// operations read. Spec order is kept within each group.
	slices.SortStableFunc(out, func(a, b target) int { return methodRank(a.ref.Method) - methodRank(b.ref.Method) })
	return out
}

func methodRank(m string) int {
	switch m {
	case "POST":
		return 1
	case "PUT", "PATCH":
		return 2
	case "DELETE":
		return 3
	default:
		return 0
	}
}

func (f Filter) selects(method, path string, op *openapi3.Operation) bool {
	if len(f.Ops) > 0 && !slices.Contains(f.Ops, method+" "+path) && !slices.Contains(f.Ops, op.OperationID) {
		return false
	}
	if len(f.Tags) > 0 && !slices.ContainsFunc(op.Tags, func(t string) bool { return slices.Contains(f.Tags, t) }) {
		return false
	}
	if f.Grep != nil && !f.Grep.MatchString(strings.Join([]string{method, path, op.OperationID, op.Summary}, " ")) {
		return false
	}
	return true
}

// Run executes every planned operation and returns the report. The returned
// error is only about persisting the module lockfile.
func Run(ctx context.Context, spec *openapi3.T, specPath string, opts Options) (Report, error) {
	start := time.Now()
	var targets []target
	if !opts.SkipOperations {
		targets = plan(spec, specPath, opts.Filter)
	}
	results := make([]OpResult, len(targets))

	sharedEnv := opts.Env
	if sharedEnv == nil {
		sharedEnv = map[string]string{}
	}
	sharedVars := copyMap(opts.Vars)

	parallel := max(1, opts.Parallel)
	em := newEmitter(opts.OnResult)
	var (
		mu      sync.Mutex
		stopped bool
		wg      sync.WaitGroup
		sem     = make(chan struct{}, parallel)
	)

	for i, t := range targets {
		sem <- struct{}{}
		mu.Lock()
		stop := stopped || ctx.Err() != nil
		mu.Unlock()
		if stop {
			<-sem
			results[i] = baseResult(t)
			results[i].Outcome = Skipped
			results[i].Reason = "not run: stopped after an earlier failure (--bail)"
			if ctx.Err() != nil {
				results[i].Reason = "not run: cancelled"
			}
			em.done(i, results[i])
			continue
		}

		wg.Add(1)
		go func(i int, t target) {
			defer wg.Done()
			defer func() { <-sem }()
			envMap, vars := sharedEnv, sharedVars
			if parallel > 1 {
				envMap, vars = copyMap(sharedEnv), copyMap(sharedVars)
			}
			res := execute(ctx, spec, specPath, t, opts, envMap, vars, false)
			results[i] = res
			if opts.Bail && (res.Outcome == Failed || res.Outcome == Errored) {
				mu.Lock()
				stopped = true
				mu.Unlock()
			}
			em.done(i, res)
		}(i, t)

		if parallel == 1 {
			wg.Wait() // keep sequential runs strictly ordered (shared vars)
		}
	}
	wg.Wait()

	// Flows run sequentially after the operations, sharing env and vars.
	for _, flow := range opts.Flows {
		var res OpResult
		switch {
		case stopped || ctx.Err() != nil:
			res = OpResult{Kind: KindFlow, Method: "FLOW", Path: relToSpec(specPath, absPath(flow)), Outcome: Skipped,
				Reason: "not run: stopped after an earlier failure (--bail)"}
		case opts.DryRun:
			res = OpResult{Kind: KindFlow, Method: "FLOW", Path: relToSpec(specPath, absPath(flow)), Outcome: Skipped, Reason: "--dry-run does not run flows"}
		default:
			res = RunFlow(ctx, spec, specPath, flow, opts, sharedEnv, sharedVars)
			if opts.Bail && (res.Outcome == Failed || res.Outcome == Errored) {
				stopped = true
			}
		}
		if opts.OnResult != nil {
			opts.OnResult(res)
		}
		results = append(results, res)
	}

	report := Report{Spec: specPath, Environment: opts.EnvName, StartedAt: start, Operations: results}
	report.Summary = Summarize(results, time.Since(start))
	return report, opts.Modules.SaveLock()
}

// PlannedCount is the number of results Run will produce with these options
// (selected operations, including the ones skipped by method, plus flows).
func PlannedCount(spec *openapi3.T, specPath string, opts Options) int {
	n := len(opts.Flows)
	if !opts.SkipOperations {
		n += len(plan(spec, specPath, opts.Filter))
	}
	return n
}

// RunOne executes a single operation. Unlike Run, overrides apply even if
// the spec does not declare them, and a missing path parameter is an error.
func RunOne(ctx context.Context, spec *openapi3.T, specPath string, ref model.OpenAPIRef, opts Options) (OpResult, error) {
	op := store.FindOperation(spec, ref)
	if op == nil {
		return OpResult{}, fmt.Errorf("operation %s %s not found in %s", strings.ToUpper(ref.Method), ref.Path, specPath)
	}
	ref.Method = strings.ToUpper(ref.Method)
	envMap := opts.Env
	if envMap == nil {
		envMap = map[string]string{}
	}
	res := execute(ctx, spec, specPath, target{ref: ref, op: op}, opts, envMap, copyMap(opts.Vars), true)
	return res, opts.Modules.SaveLock()
}

func baseResult(t target) OpResult {
	return OpResult{
		OperationID: t.op.OperationID,
		Method:      t.ref.Method,
		Path:        t.ref.Path,
		Summary:     t.op.Summary,
		Tags:        t.op.Tags,
	}
}

// buildRequest turns an operation plus the run options into the request to
// send. A non-empty outcome means it cannot be sent (with the reason).
func buildRequest(spec *openapi3.T, t target, opts Options, envMap, vars map[string]string, single bool) (model.Request, Outcome, string) {
	req := store.OperationToRequest(spec, t.ref)
	for k, v := range opts.Params {
		if _, declared := req.Params[k]; declared || single {
			req.Params[k] = v
		}
	}
	for k, v := range opts.Query {
		req.Query[k] = v
	}
	for k, v := range opts.Headers {
		req.Headers[k] = v
	}
	if opts.Body != nil {
		req.Body.Raw = *opts.Body
	}
	if opts.BodyType != "" {
		req.Body.Type = opts.BodyType
	}
	if len(opts.Files) > 0 {
		req.Body.Files = opts.Files
	}
	req.Env, req.Vars, req.Timeout = envMap, vars, opts.Timeout
	req.HTTPClient, req.MaxBody = opts.HTTPClient, opts.MaxBody
	req.Auth = model.ApplySecrets(req.Auth, opts.Auth)

	if missing := req.MissingPathParams(); len(missing) > 0 {
		reason := fmt.Sprintf("no value for path parameter(s) %s (pass --param %s=<value> or declare an example in the spec)",
			strings.Join(missing, ", "), missing[0])
		if single {
			return req, Errored, reason
		}
		return req, Skipped, reason
	}
	if err := selectServer(&req, opts.Server); err != nil {
		return req, Errored, err.Error()
	}
	if req.ServerURL == "" {
		return req, Errored, "no server URL: the spec declares no servers; pass --server <url>"
	}
	return req, "", ""
}

// Prepare builds the request RunOne would send, after the operation's
// before(req) hook, without sending it (e.g. to export it as cURL).
func Prepare(ctx context.Context, spec *openapi3.T, specPath string, ref model.OpenAPIRef, opts Options) (model.Request, error) {
	ref.Method = strings.ToUpper(ref.Method)
	op := store.FindOperation(spec, ref)
	if op == nil {
		return model.Request{}, fmt.Errorf("operation %s %s not found in %s", ref.Method, ref.Path, specPath)
	}
	envMap := opts.Env
	if envMap == nil {
		envMap = map[string]string{}
	}
	t := target{ref: ref, op: op}
	req, outcome, reason := buildRequest(spec, t, opts, envMap, copyMap(opts.Vars), true)
	if outcome != "" {
		return req, errors.New(reason)
	}
	source, sopts, err := loadScript(spec, specPath, t, opts)
	if err != nil || source == "" {
		return req, err
	}
	data := script.ExecData{Request: req, Env: req.Env, Vars: req.Vars}
	in := ExchangeInput{Spec: spec, SpecPath: specPath, Ref: &ref, Auth: opts.Auth}
	in.bind(&data, req)
	br := script.Before(source, data, sopts)
	if br.Error != "" {
		return req, errors.New(br.Error)
	}
	if br.Defined {
		req = br.Request
	}
	return req, opts.Modules.SaveLock()
}

// execute runs one operation end to end.
func execute(ctx context.Context, spec *openapi3.T, specPath string, t target, opts Options,
	envMap, vars map[string]string, single bool) OpResult {
	res := baseResult(t)
	if t.skipped != "" {
		res.Outcome, res.Reason = Skipped, t.skipped
		return res
	}

	req, outcome, reason := buildRequest(spec, t, opts, envMap, vars, single)
	if outcome != "" {
		res.Outcome, res.Reason = outcome, reason
		return res
	}

	source, sopts, err := loadScript(spec, specPath, t, opts)
	if err != nil {
		res.Outcome, res.Reason = Errored, err.Error()
		return res
	}
	if source != "" {
		res.TestsFile = relToSpec(specPath, sopts.Filename)
	}

	if opts.DryRun {
		res.Request = requestInfo(req)
		res.Outcome = Planned
		return res
	}

	ref := t.ref
	return exchange(ctx, res, ExchangeInput{
		Spec: spec, SpecPath: specPath, Ref: &ref, Request: req,
		Script: source, ScriptOptions: sopts,
		NoContract: opts.NoContract, Retries: opts.Retries, Auth: opts.Auth,
	})
}

// ExchangeInput is a fully built request plus what to check it against.
type ExchangeInput struct {
	Spec     *openapi3.T // nil: no status/contract checks, no lazyapi.send
	SpecPath string
	Ref      *model.OpenAPIRef // nil for requests that are not in the spec (drafts)
	Request  model.Request     // Env/Vars/Timeout/ServerURL already set

	Script        string // test script source; "" for none
	ScriptOptions script.Options
	NoContract    bool
	Retries       int
	// Auth fills credentials into lazyapi.send() calls made by the script.
	Auth map[string]model.AuthSecrets
}

// bind connects a script to the spec (lazyapi.send, toMatchSchema).
func (in ExchangeInput) bind(data *script.ExecData, req model.Request) {
	if in.Spec == nil {
		return
	}
	sctx := &script.SpecContext{Spec: in.Spec, SpecPath: in.SpecPath, ServerURL: req.ServerURL,
		Env: req.Env, Vars: req.Vars, Timeout: req.Timeout, Auth: in.Auth,
		HTTPClient: req.HTTPClient, MaxBody: req.MaxBody}
	sctx.Bind(data)
}

// Exchange sends an already-built request — e.g. one edited in the TUI — and
// runs the same checks as Run: declared status, contract and script.
func Exchange(ctx context.Context, in ExchangeInput) OpResult {
	res := OpResult{Method: in.Request.Method.Label(), Path: in.Request.URI, Summary: in.Request.About.Summary}
	if in.Spec != nil && in.Ref != nil {
		if op := store.FindOperation(in.Spec, *in.Ref); op != nil {
			res.OperationID, res.Tags = op.OperationID, op.Tags
		}
	}
	if missing := in.Request.MissingPathParams(); len(missing) > 0 {
		res.Outcome = Errored
		res.Reason = "no value for path parameter(s) " + strings.Join(missing, ", ")
		return res
	}
	if in.Request.ServerURL == "" {
		res.Outcome, res.Reason = Errored, "no server URL"
		return res
	}
	return exchange(ctx, res, in)
}

func exchange(ctx context.Context, res OpResult, in ExchangeInput) OpResult {
	req := in.Request
	if in.Script != "" && in.ScriptOptions.Filename != "" && in.SpecPath != "" {
		res.TestsFile = relToSpec(in.SpecPath, in.ScriptOptions.Filename)
	}

	// before(req) may change the request (auth, signatures, bodies) first.
	if in.Script != "" && strings.Contains(in.Script, "before") {
		data := script.ExecData{Request: req, Env: req.Env, Vars: req.Vars}
		in.bind(&data, req)
		br := script.Before(in.Script, data, in.ScriptOptions)
		res.Logs = append(res.Logs, br.Logs...)
		if br.Error != "" {
			res.Request = requestInfo(req)
			res.Checks = append(res.Checks, Check{Kind: CheckScript, Name: "before() ran without errors", Message: br.Error})
			res.Outcome = Failed
			res.Reason = "not sent: before() failed"
			return res
		}
		if br.Defined {
			req = br.Request
		}
	}
	res.Request = requestInfo(req)

	inSpec := in.Spec != nil && in.Ref != nil && store.FindOperation(in.Spec, *in.Ref) != nil
	if inSpec && !in.NoContract {
		res.Checks = append(res.Checks, requestCheck(ctx, in.Spec, *in.Ref, req))
	}

	resp, body, timing, attempts, err := sendWithRetries(ctx, &req, in.Retries)
	elapsed := timing.Total
	res.Attempts = attempts
	if err != nil {
		res.Outcome, res.Reason = Errored, "request failed: "+err.Error()
		if ctx.Err() == context.Canceled {
			res.Reason = "request cancelled"
		}
		return res
	}
	res.Response = &ResponseInfo{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		Headers:    flattenHeader(resp.Header),
		Body:       body,
		DurationMs: elapsed.Milliseconds(),
		Timing:     newTimingInfo(timing),
		Cookies:    cookiesOf(resp),
	}

	if inSpec {
		ref := *in.Ref
		declared := store.DeclaredStatuses(in.Spec, ref)
		statusCheck := Check{Kind: CheckStatus, Name: fmt.Sprintf("status %d is declared", resp.StatusCode),
			Passed: store.StatusDeclared(declared, resp.StatusCode)}
		if !statusCheck.Passed {
			if len(declared) == 0 {
				statusCheck.Message = "the operation declares no responses, so only statuses below 400 are accepted"
			} else {
				statusCheck.Message = "declared: " + strings.Join(declared, ", ")
			}
		}
		res.Checks = append(res.Checks, statusCheck)

		if !in.NoContract && store.ResponseSchemaDeclared(in.Spec, ref, resp.StatusCode) {
			c := Check{Kind: CheckContract, Name: "response matches the OpenAPI schema", Passed: true}
			if err := store.ValidateResponse(in.Spec, ref, resp.StatusCode, resp.Header, []byte(body)); err != nil {
				c.Passed, c.Message = false, err.Error()
			}
			res.Checks = append(res.Checks, c)
		}
	}

	if in.Script != "" {
		data := script.ExecData{
			Request: req,
			Response: &script.Response{Status: resp.StatusCode, StatusText: resp.Status, Header: resp.Header,
				Body: body, Duration: elapsed, Ref: in.Ref},
			Env:  req.Env,
			Vars: req.Vars,
		}
		in.bind(&data, req)
		sr := script.Run(in.Script, data, in.ScriptOptions)
		for _, tr := range sr.Results {
			res.Checks = append(res.Checks, Check{Kind: CheckTest, Name: tr.Name, Passed: tr.Passed, Message: tr.Message})
		}
		if sr.ScriptError != "" {
			res.Checks = append(res.Checks, Check{Kind: CheckScript, Name: "script ran without errors", Message: sr.ScriptError})
		}
		res.Logs = append(res.Logs, sr.Logs...)
	}

	res.Outcome = outcomeFor(res.Checks)
	return res
}

// requestCheck validates the request that is about to be sent against the
// operation (parameters and body). It does not stop the request: the server's
// answer to an invalid request is still useful.
func requestCheck(ctx context.Context, spec *openapi3.T, ref model.OpenAPIRef, req model.Request) Check {
	c := Check{Kind: CheckRequest, Name: "request matches the OpenAPI spec", Passed: true}
	httpReq, err := req.HTTPRequest(ctx)
	if err != nil {
		c.Passed, c.Message = false, err.Error()
		return c
	}
	params := make(map[string]string, len(req.Params))
	for k, v := range req.Params {
		params[k] = env.Resolve(v, env.Context{Env: req.Env, Vars: req.Vars})
	}
	if err := store.ValidateRequest(spec, ref, httpReq, params); err != nil {
		c.Passed, c.Message = false, err.Error()
	}
	return c
}

func sendWithRetries(ctx context.Context, req *model.Request, retries int) (*http.Response, string, model.Timing, int, error) {
	var (
		resp   *http.Response
		body   string
		err    error
		timing model.Timing
	)
	for attempt := 1; ; attempt++ {
		resp, body, timing, err = req.SendTimed(ctx)
		retryable := err != nil || resp.StatusCode >= 500
		if !retryable || attempt > retries || ctx.Err() != nil {
			return resp, body, timing, attempt, err
		}
		select {
		case <-ctx.Done():
			return resp, body, timing, attempt, err
		case <-time.After(time.Duration(attempt) * 250 * time.Millisecond):
		}
	}
}

func loadScript(spec *openapi3.T, specPath string, t target, opts Options) (string, script.Options, error) {
	specDir, err := filepath.Abs(filepath.Dir(specPath))
	if err != nil {
		return "", script.Options{}, err
	}
	base := script.Options{Modules: opts.Modules, Timeout: opts.ScriptTimeout, Root: specDir}

	if opts.ScriptPath != "" {
		abs, err := filepath.Abs(opts.ScriptPath)
		if err != nil {
			return "", base, err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return "", base, fmt.Errorf("reading script %s: %w", opts.ScriptPath, err)
		}
		base.Filename, base.ResolveDir = abs, filepath.Dir(abs)
		if rel, err := filepath.Rel(specDir, abs); err != nil || strings.HasPrefix(rel, "..") {
			base.Root = base.ResolveDir
		}
		return string(data), base, nil
	}

	source, err := store.LoadOperationTests(specPath, t.op)
	if err != nil || source == "" {
		return "", base, err
	}
	o, err := script.OperationOptions(specPath, t.op, t.ref)
	if err != nil {
		return "", base, err
	}
	o.Modules, o.Timeout = base.Modules, base.Timeout
	return source, o, nil
}

// serverIndex reports whether v is a numeric --server index.
func serverIndex(v string) (int, bool) {
	idx, err := strconv.Atoi(v)
	return idx, err == nil && idx >= 0
}

// selectServer applies a --server value: a URL, or an index into the spec's
// servers. Empty keeps the first server.
func selectServer(req *model.Request, value string) error {
	if value == "" {
		if len(req.Servers) > 0 {
			req.ServerURL = req.Servers[0]
		}
		return nil
	}
	if idx, err := strconv.Atoi(value); err == nil {
		if idx < 0 || idx >= len(req.Servers) {
			return fmt.Errorf("server index %d out of range (spec declares %d servers)", idx, len(req.Servers))
		}
		req.ServerURL = req.Servers[idx]
		return nil
	}
	req.ServerURL = value
	return nil
}

var sensitiveHeader = regexp.MustCompile(`(?i)(authorization|cookie|token|secret|password|api[-_]?key)`)

func requestInfo(req model.Request) *RequestInfo {
	ctx := env.Context{Env: req.Env, Vars: req.Vars}
	info := &RequestInfo{Method: req.Method.Label(), URL: req.ResolvedURL(), Body: env.Resolve(req.Body.Raw, ctx)}

	names := make([]string, 0, len(req.Query))
	for k := range req.Query {
		names = append(names, k)
	}
	sort.Strings(names)
	var q []string
	for _, k := range names {
		if v := env.Resolve(req.Query[k], ctx); v != "" {
			q = append(q, k+"="+v)
		}
	}
	if len(q) > 0 {
		info.URL += "?" + strings.Join(q, "&")
	}

	for k, v := range req.Headers {
		if info.Headers == nil {
			info.Headers = map[string]string{}
		}
		if sensitiveHeader.MatchString(k) {
			info.Headers[k] = "<redacted>"
		} else {
			info.Headers[k] = env.Resolve(v, ctx)
		}
	}
	return info
}

func flattenHeader(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, vs := range h {
		out[k] = strings.Join(vs, ", ")
	}
	return out
}

func relToSpec(specPath, file string) string {
	specDir, err := filepath.Abs(filepath.Dir(specPath))
	if err != nil {
		return file
	}
	if rel, err := filepath.Rel(specDir, file); err == nil {
		return filepath.ToSlash(rel)
	}
	return file
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// emitter delivers results to a callback in index order.
type emitter struct {
	mu    sync.Mutex
	fn    func(OpResult)
	ready map[int]OpResult
	next  int
}

func newEmitter(fn func(OpResult)) *emitter {
	return &emitter{fn: fn, ready: map[int]OpResult{}}
}

func (e *emitter) done(i int, r OpResult) {
	if e.fn == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ready[i] = r
	for {
		r, ok := e.ready[e.next]
		if !ok {
			return
		}
		delete(e.ready, e.next)
		e.next++
		e.fn(r)
	}
}

func newTimingInfo(t model.Timing) *TimingInfo {
	ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
	return &TimingInfo{DNSMs: ms(t.DNS), ConnectMs: ms(t.Connect), TLSMs: ms(t.TLS), TTFBMs: ms(t.TTFB),
		TotalMs: ms(t.Total), Reused: t.Reused}
}

func cookiesOf(resp *http.Response) []CookieInfo {
	var out []CookieInfo
	for _, c := range resp.Cookies() {
		ci := CookieInfo{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path,
			HTTPOnly: c.HttpOnly, Secure: c.Secure, SameSite: sameSite(c.SameSite)}
		if !c.Expires.IsZero() {
			ci.Expires = c.Expires.UTC().Format(time.RFC3339)
		}
		if c.MaxAge > 0 {
			ci.MaxAge = c.MaxAge
		}
		out = append(out, ci)
	}
	return out
}

func sameSite(s http.SameSite) string {
	switch s {
	case http.SameSiteLaxMode:
		return "Lax"
	case http.SameSiteStrictMode:
		return "Strict"
	case http.SameSiteNoneMode:
		return "None"
	}
	return ""
}
