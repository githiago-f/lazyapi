package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/script"
	"github.com/githiago-f/lazyapi/internal/store"
)

// FlowsDir is where flows live, next to the spec.
const FlowsDir = "flows"

// DefaultFlowTimeout bounds a whole flow (all its requests and assertions).
const DefaultFlowTimeout = 5 * time.Minute

// FindFlows lists flows/*.js|*.ts next to the spec, sorted.
func FindFlows(specPath string) ([]string, error) {
	dir := filepath.Join(filepath.Dir(specPath), FlowsDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".js", ".mjs", ".ts", ".mts":
			if !e.IsDir() {
				out = append(out, filepath.Join(dir, e.Name()))
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// RunFlow executes a flow script: a sequence of lazyapi.send() calls with
// test() assertions. Every call is checked like an operation in Run
// (request, declared status, contract) unless opts.NoContract.
func RunFlow(ctx context.Context, spec *openapi3.T, specPath, flowPath string, opts Options, envMap, vars map[string]string) OpResult {
	res := OpResult{Kind: KindFlow, Method: "FLOW", Path: relToSpec(specPath, absPath(flowPath))}
	source, err := os.ReadFile(flowPath)
	if err != nil {
		res.Outcome, res.Reason = Errored, err.Error()
		return res
	}
	specDir, _ := filepath.Abs(filepath.Dir(specPath))
	abs := absPath(flowPath)
	sopts := script.Options{
		Filename: abs, ResolveDir: filepath.Dir(abs), Root: specDir,
		Modules: opts.Modules, Timeout: opts.FlowTimeout,
	}
	if sopts.Timeout <= 0 {
		sopts.Timeout = DefaultFlowTimeout
	}
	if rel, err := filepath.Rel(specDir, abs); err != nil || strings.HasPrefix(rel, "..") {
		sopts.Root = sopts.ResolveDir
	}

	server := opts.Server
	if server == "" && len(spec.Servers) > 0 {
		server = spec.Servers[0].URL
	} else if idx, ok := serverIndex(server); ok {
		if idx >= len(spec.Servers) {
			res.Outcome, res.Reason = Errored, fmt.Sprintf("server index %d out of range", idx)
			return res
		}
		server = spec.Servers[idx].URL
	}

	sctx := &script.SpecContext{
		Spec: spec, SpecPath: specPath, ServerURL: server,
		Env: envMap, Vars: vars, Timeout: opts.Timeout, Auth: opts.Auth,
		HTTPClient: opts.HTTPClient, MaxBody: opts.MaxBody,
	}
	sctx.OnSend = func(ref model.OpenAPIRef, req model.Request, resp *script.Response, sendErr error) {
		n := len(res.Steps) + 1
		step := Step{Method: ref.Method, Path: ref.Path, URL: req.ResolvedURL()}
		if op := store.FindOperation(spec, ref); op != nil {
			step.OperationID = op.OperationID
		}
		label := fmt.Sprintf("#%d %s %s", n, ref.Method, ref.Path)
		if sendErr != nil {
			step.Error = sendErr.Error()
			res.Steps = append(res.Steps, step)
			return // the script sees the error and decides; test() reports it
		}
		step.Status, step.DurationMs = resp.Status, resp.Duration.Milliseconds()
		res.Steps = append(res.Steps, step)
		if opts.NoContract {
			return
		}
		for _, c := range stepChecks(ctx, spec, ref, req, resp) {
			c.Name = label + " · " + c.Name
			res.Checks = append(res.Checks, c)
		}
	}

	data := script.ExecData{Request: model.Request{Env: envMap, Vars: vars}, Env: envMap, Vars: vars}
	sctx.Bind(&data)
	sr := script.Run(string(source), data, sopts)
	for _, tr := range sr.Results {
		res.Checks = append(res.Checks, Check{Kind: CheckTest, Name: tr.Name, Passed: tr.Passed, Message: tr.Message})
	}
	if sr.ScriptError != "" {
		res.Checks = append(res.Checks, Check{Kind: CheckScript, Name: "flow ran without errors", Message: sr.ScriptError})
	}
	res.Logs = sr.Logs
	res.TestsFile = res.Path
	if len(res.Checks) == 0 {
		res.Checks = append(res.Checks, Check{Kind: CheckScript, Name: "flow made assertions", Message: "the flow sent no requests and ran no test()"})
	}
	res.Outcome = outcomeFor(res.Checks)
	return res
}

// stepChecks are the automatic checks for one request made by a flow.
func stepChecks(ctx context.Context, spec *openapi3.T, ref model.OpenAPIRef, req model.Request, resp *script.Response) []Check {
	checks := []Check{requestCheck(ctx, spec, ref, req)}
	declared := store.DeclaredStatuses(spec, ref)
	status := Check{Kind: CheckStatus, Name: fmt.Sprintf("status %d is declared", resp.Status),
		Passed: store.StatusDeclared(declared, resp.Status)}
	if !status.Passed {
		status.Message = "declared: " + strings.Join(declared, ", ")
	}
	checks = append(checks, status)
	if store.ResponseSchemaDeclared(spec, ref, resp.Status) {
		c := Check{Kind: CheckContract, Name: "response matches the OpenAPI schema", Passed: true}
		if err := store.ValidateResponse(spec, ref, resp.Status, resp.Header, []byte(resp.Body)); err != nil {
			c.Passed, c.Message = false, err.Error()
		}
		checks = append(checks, c)
	}
	return checks
}

func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}
