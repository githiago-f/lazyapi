package runner

import (
	"context"
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

	Filter  Filter
	Timeout time.Duration // per request

	ScriptPath    string               // overrides every operation's x-lazyapi-tests
	Modules       *script.ModuleLoader // URL imports; nil disables them
	ScriptTimeout time.Duration
	NoContract    bool

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

// plan lists the operations a run would visit, in spec order. Operations not
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
	return out
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
	targets := plan(spec, specPath, opts.Filter)
	results := make([]OpResult, len(targets))

	sharedEnv := opts.Env
	if sharedEnv == nil {
		sharedEnv = map[string]string{}
	}
	sharedVars := map[string]string{}

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

	report := Report{Spec: specPath, StartedAt: start, Operations: results}
	report.Summary = Summarize(results, time.Since(start))
	return report, opts.Modules.SaveLock()
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
	res := execute(ctx, spec, specPath, target{ref: ref, op: op}, opts, envMap, map[string]string{}, true)
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

// execute runs one operation end to end.
func execute(ctx context.Context, spec *openapi3.T, specPath string, t target, opts Options,
	envMap, vars map[string]string, single bool) OpResult {
	res := baseResult(t)
	if t.skipped != "" {
		res.Outcome, res.Reason = Skipped, t.skipped
		return res
	}

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
	req.Env, req.Vars, req.Timeout = envMap, vars, opts.Timeout

	if missing := req.MissingPathParams(); len(missing) > 0 {
		res.Reason = fmt.Sprintf("no value for path parameter(s) %s (pass --param %s=<value> or declare an example in the spec)",
			strings.Join(missing, ", "), missing[0])
		res.Outcome = Skipped
		if single {
			res.Outcome = Errored
		}
		return res
	}
	if err := selectServer(&req, opts.Server); err != nil {
		res.Outcome, res.Reason = Errored, err.Error()
		return res
	}
	if req.ServerURL == "" {
		res.Outcome, res.Reason = Errored, "no server URL: the spec declares no servers; pass --server <url>"
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
	res.Request = requestInfo(req)

	if opts.DryRun {
		res.Outcome = Planned
		return res
	}

	resp, body, elapsed, attempts, err := sendWithRetries(ctx, &req, opts.Retries)
	res.Attempts = attempts
	if err != nil {
		res.Outcome, res.Reason = Errored, "request failed: "+err.Error()
		return res
	}
	res.Response = &ResponseInfo{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		Headers:    flattenHeader(resp.Header),
		Body:       body,
		DurationMs: elapsed.Milliseconds(),
	}

	declared := store.DeclaredStatuses(spec, t.ref)
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

	if !opts.NoContract && store.ResponseSchemaDeclared(spec, t.ref, resp.StatusCode) {
		c := Check{Kind: CheckContract, Name: "response matches the OpenAPI schema", Passed: true}
		if err := store.ValidateResponse(spec, t.ref, resp.StatusCode, resp.Header, []byte(body)); err != nil {
			c.Passed, c.Message = false, err.Error()
		}
		res.Checks = append(res.Checks, c)
	}

	if source != "" {
		ref := t.ref
		data := script.ExecData{
			Request: req,
			Response: &script.Response{Status: resp.StatusCode, StatusText: resp.Status, Header: resp.Header,
				Body: body, Duration: elapsed, Ref: &ref},
			Env:  envMap,
			Vars: vars,
		}
		sctx := &script.SpecContext{Spec: spec, SpecPath: specPath, ServerURL: req.ServerURL,
			Env: envMap, Vars: vars, Timeout: opts.Timeout}
		sctx.Bind(&data)
		sr := script.Run(source, data, sopts)
		for _, tr := range sr.Results {
			res.Checks = append(res.Checks, Check{Kind: CheckTest, Name: tr.Name, Passed: tr.Passed, Message: tr.Message})
		}
		if sr.ScriptError != "" {
			res.Checks = append(res.Checks, Check{Kind: CheckScript, Name: "script ran without errors", Message: sr.ScriptError})
		}
		res.Logs = sr.Logs
	}

	res.Outcome = outcomeFor(res.Checks)
	return res
}

func sendWithRetries(ctx context.Context, req *model.Request, retries int) (*http.Response, string, time.Duration, int, error) {
	var (
		resp    *http.Response
		body    string
		err     error
		elapsed time.Duration
	)
	for attempt := 1; ; attempt++ {
		start := time.Now()
		resp, body, err = req.SendContext(ctx)
		elapsed = time.Since(start)
		retryable := err != nil || resp.StatusCode >= 500
		if !retryable || attempt > retries || ctx.Err() != nil {
			return resp, body, elapsed, attempt, err
		}
		select {
		case <-ctx.Done():
			return resp, body, elapsed, attempt, err
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
