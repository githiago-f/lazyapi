package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/importer"
	"github.com/githiago-f/lazyapi/internal/inspect"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/project"
	"github.com/githiago-f/lazyapi/internal/report"
	"github.com/githiago-f/lazyapi/internal/runner"
	"github.com/githiago-f/lazyapi/internal/script"
	"github.com/githiago-f/lazyapi/internal/store"
)

// Tools implements the LazyAPI tool set. Every path argument is resolved
// relative to Root and must stay inside it.
type Tools struct {
	Root string
	// FindSpecs lists OpenAPI files under Root (injected for testability).
	FindSpecs func(root string) ([]string, error)
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	call        func(t *Tools, ctx context.Context, args json.RawMessage) (any, error)
}

func obj(required []string, props map[string]any) map[string]any {
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
func strMap(desc string) map[string]any {
	return map[string]any{"type": "object", "description": desc, "additionalProperties": map[string]any{"type": "string"}}
}
func strList(desc string) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": map[string]any{"type": "string"}}
}

var (
	specArg = str("Path to the OpenAPI spec, relative to the server root")
	opArg   = str(`operationId, or "METHOD /path" as written in the spec (e.g. "GET /users/{id}")`)
)

var toolList = []tool{
	{
		Name:        "list_specs",
		Description: "List OpenAPI spec files under the server root.",
		InputSchema: obj(nil, map[string]any{}),
		call:        (*Tools).listSpecs,
	},
	{
		Name:        "list_operations",
		Description: "List the operations of a spec: method, path, operationId, summary, tags and test script file.",
		InputSchema: obj([]string{"spec"}, map[string]any{"spec": specArg, "tags": strList("Only operations with one of these tags")}),
		call:        (*Tools).listOperations,
	},
	{
		Name:        "describe_operation",
		Description: "Everything needed to call and test one operation: parameters (with examples), request body and response schemas, referenced component schemas, security and the current test script.",
		InputSchema: obj([]string{"spec", "operation"}, map[string]any{"spec": specArg, "operation": opArg}),
		call:        (*Tools).describeOperation,
	},
	{
		Name: "send_request",
		Description: "Send one operation and run its checks: declared status, response schema (contract) and the operation's test script. " +
			"Returns the request, the response (status, headers, body) and every check with its failure message.",
		InputSchema: obj([]string{"spec", "operation"}, map[string]any{
			"spec":        specArg,
			"operation":   opArg,
			"server":      str("Server URL, or index into the spec's servers (default: first)"),
			"params":      strMap("Path parameters"),
			"query":       strMap("Query parameters"),
			"headers":     strMap("Request headers"),
			"body":        map[string]any{"description": "Request body: a string is sent as-is, anything else is JSON-encoded"},
			"env_file":    str("Environment file for {{env.NAME}} templates, relative to the root"),
			"env_name":    str("Named environment from lazyapi.config.yml (server, env file, vars, credentials, TLS/proxy)"),
			"insecure":    boolean("Skip TLS certificate verification (self-signed development servers)"),
			"no_contract": boolean("Skip response schema validation"),
			"dry_run":     boolean("Resolve the request without sending it"),
		}),
		call: (*Tools).sendRequest,
	},
	{
		Name: "run_tests",
		Description: "Run many operations with their checks and return a report with a summary and per-operation results. " +
			"Only GET, HEAD and OPTIONS run unless methods is set (use [\"all\"] to include writes). Bodies are included for operations that did not pass.",
		InputSchema: obj([]string{"spec"}, map[string]any{
			"spec":          specArg,
			"server":        str("Server URL, or index into the spec's servers"),
			"methods":       strList(`Methods to run, or ["all"]. Default: GET, HEAD, OPTIONS`),
			"tags":          strList("Only operations with one of these tags"),
			"operations":    strList(`Only these operations (operationId or "METHOD /path")`),
			"grep":          str("Regular expression over \"METHOD /path operationId summary\""),
			"params":        strMap("Path parameters for every operation that declares them"),
			"headers":       strMap("Headers added to every request"),
			"env_file":      str("Environment file, relative to the root"),
			"no_contract":   boolean("Skip response schema validation"),
			"bail":          boolean("Stop after the first failure"),
			"dry_run":       boolean("List what would be sent without sending"),
			"include_flows": boolean("Also run every flows/*.js next to the spec, after the operations"),
		}),
		call: (*Tools).runTests,
	},
	{
		Name: "run_flow",
		Description: "Run a flow script (flows/<name>.js): several operations in order via lazyapi.send(), sharing vars. " +
			"Every request is checked (request, status, contract) and test() calls are reported. Returns steps and checks.",
		InputSchema: obj([]string{"spec", "flow"}, map[string]any{
			"spec":        specArg,
			"flow":        str("Flow file relative to the root (e.g. api/flows/checkout.js), or a name under the spec's flows/ directory"),
			"server":      str("Server URL, or index into the spec's servers"),
			"env_file":    str("Environment file, relative to the root"),
			"env_name":    str("Named environment from lazyapi.config.yml"),
			"no_contract": boolean("Skip request/response schema checks for each step"),
		}),
		call: (*Tools).runFlow,
	},
	{
		Name: "write_flow",
		Description: "Create or replace flows/<name>.js next to the spec. A flow calls lazyapi.send(operationId, {params, query, headers, body}) " +
			"in order and asserts with test()/expect(); use vars.set/get to pass ids between steps. Returns the path and any compile error.",
		InputSchema: obj([]string{"spec", "name", "source"}, map[string]any{
			"spec":   specArg,
			"name":   str("Flow name, e.g. user-lifecycle (letters, digits, - and _)"),
			"source": str("Flow script source"),
		}),
		call: (*Tools).writeFlow,
	},
	{
		Name: "import_curl",
		Description: "Add the operation described by a curl command to a spec (created if missing): path parameters, " +
			"query/header parameters, body example and inferred schema, security scheme. Credentials are not copied.",
		InputSchema: obj([]string{"spec", "command"}, map[string]any{
			"spec":    specArg,
			"command": str("The curl command line"),
		}),
		call: (*Tools).importCurl,
	},
	{
		Name:        "export_curl",
		Description: "Build the exact request for an operation (examples, overrides, environment credentials, before() hook) and return it as a curl command, without sending it.",
		InputSchema: obj([]string{"spec", "operation"}, map[string]any{
			"spec": specArg, "operation": opArg,
			"server":   str("Server URL, or index into the spec's servers"),
			"params":   strMap("Path parameters"),
			"query":    strMap("Query parameters"),
			"headers":  strMap("Request headers"),
			"body":     map[string]any{"description": "Request body: a string is sent as-is, anything else is JSON-encoded"},
			"env_file": str("Environment file, relative to the root"),
			"env_name": str("Named environment from lazyapi.config.yml"),
		}),
		call: (*Tools).exportCurl,
	},
	{
		Name:        "read_test",
		Description: "Read an operation's test script and where it is (or would be) stored.",
		InputSchema: obj([]string{"spec", "operation"}, map[string]any{"spec": specArg, "operation": opArg}),
		call:        (*Tools).readTest,
	},
	{
		Name: "write_test",
		Description: "Create or replace an operation's JavaScript/TypeScript test script. It is saved next to the spec and linked with x-lazyapi-tests. " +
			"Returns the file path and any compile error. Call script_api for the available globals. An empty source unlinks the script.",
		InputSchema: obj([]string{"spec", "operation", "source"}, map[string]any{
			"spec":      specArg,
			"operation": opArg,
			"source":    str(`Script source, e.g. test("ok", () => expect(response).toHaveStatus(200));`),
		}),
		call: (*Tools).writeTest,
	},
	{
		Name:        "validate_spec",
		Description: "Validate a spec against OpenAPI and LazyAPI conventions (operationIds, declared responses, test scripts that exist and compile).",
		InputSchema: obj([]string{"spec"}, map[string]any{"spec": specArg}),
		call:        (*Tools).validateSpec,
	},
	{
		Name:        "script_api",
		Description: "TypeScript definitions of the test script API (request, response, env, vars, lazyapi.send, test, expect matchers).",
		InputSchema: obj(nil, map[string]any{}),
		call:        func(*Tools, context.Context, json.RawMessage) (any, error) { return script.Types, nil },
	},
}

// Definitions returns the tool list for tools/list.
func (t *Tools) Definitions() []tool { return toolList }

// Has reports whether a tool exists.
func (t *Tools) Has(name string) bool {
	for _, tl := range toolList {
		if tl.Name == name {
			return true
		}
	}
	return false
}

// Call runs a tool.
func (t *Tools) Call(ctx context.Context, name string, args json.RawMessage) (any, error) {
	for _, tl := range toolList {
		if tl.Name == name {
			if len(args) == 0 || string(args) == "null" {
				args = json.RawMessage("{}")
			}
			return tl.call(t, ctx, args)
		}
	}
	return nil, fmt.Errorf("unknown tool %q", name)
}

func decode(args json.RawMessage, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(args)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

// path resolves p inside Root.
func (t *Tools) path(p string) (string, error) {
	if p == "" {
		return "", errors.New("path is required")
	}
	root, err := filepath.Abs(t.Root)
	if err != nil {
		return "", err
	}
	full := p
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, p)
	}
	full = filepath.Clean(full)
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the server root %s", p, root)
	}
	return full, nil
}

func (t *Tools) rel(p string) string {
	root, _ := filepath.Abs(t.Root)
	if r, err := filepath.Rel(root, p); err == nil {
		return filepath.ToSlash(r)
	}
	return p
}

type specArgs struct {
	Spec string `json:"spec"`
}

// ---------------------------------------------------------------- tools

func (t *Tools) listSpecs(_ context.Context, args json.RawMessage) (any, error) {
	if err := decode(args, &struct{}{}); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(t.Root)
	if err != nil {
		return nil, err
	}
	files, err := t.FindSpecs(root)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, t.rel(f))
	}
	return map[string]any{"specs": out}, nil
}

func (t *Tools) listOperations(_ context.Context, args json.RawMessage) (any, error) {
	var a struct {
		Spec string   `json:"spec"`
		Tags []string `json:"tags"`
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	specPath, err := t.path(a.Spec)
	if err != nil {
		return nil, err
	}
	spec, err := store.ParseSpec(specPath)
	if err != nil {
		return nil, err
	}
	ops := inspect.ListOperations(spec, specPath, a.Tags)
	if ops == nil {
		ops = []inspect.OpSummary{}
	}
	return map[string]any{"operations": ops}, nil
}

func (t *Tools) describeOperation(_ context.Context, args json.RawMessage) (any, error) {
	var a struct {
		Spec      string `json:"spec"`
		Operation string `json:"operation"`
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	specPath, err := t.path(a.Spec)
	if err != nil {
		return nil, err
	}
	spec, err := store.ParseSpec(specPath)
	if err != nil {
		return nil, err
	}
	return inspect.Describe(spec, specPath, a.Operation)
}

type runArgs struct {
	Spec       string            `json:"spec"`
	Server     string            `json:"server"`
	Params     map[string]string `json:"params"`
	Query      map[string]string `json:"query"`
	Headers    map[string]string `json:"headers"`
	EnvFile    string            `json:"env_file"`
	EnvName    string            `json:"env_name"`
	Insecure   bool              `json:"insecure"`
	NoContract bool              `json:"no_contract"`
	DryRun     bool              `json:"dry_run"`
}

func (t *Tools) options(a runArgs) (string, runner.Options, error) {
	specPath, err := t.path(a.Spec)
	if err != nil {
		return "", runner.Options{}, err
	}
	envPath := ""
	if a.EnvFile != "" {
		if envPath, err = t.path(a.EnvFile); err != nil {
			return "", runner.Options{}, err
		}
	}
	sel, err := project.Resolve(specPath, a.EnvName, a.Server, envPath)
	if err != nil {
		return "", runner.Options{}, err
	}
	envMap, err := env.NewStore(sel.EnvFile).Load()
	if err != nil {
		return "", runner.Options{}, fmt.Errorf("loading env file: %w", err)
	}
	client, err := sel.HTTP.Merge(model.HTTPOptions{Insecure: a.Insecure}).Client()
	if err != nil {
		return "", runner.Options{}, err
	}
	return specPath, runner.Options{
		HTTPClient:    client,
		EnvName:       sel.Name,
		Vars:          sel.Vars,
		Auth:          sel.Auth,
		Server:        sel.Server,
		Env:           envMap,
		Params:        a.Params,
		Query:         a.Query,
		Headers:       a.Headers,
		Timeout:       model.DefaultTimeout,
		Modules:       script.NewModuleLoader(filepath.Dir(specPath)),
		ScriptTimeout: script.DefaultTimeout,
		NoContract:    a.NoContract,
		DryRun:        a.DryRun,
	}, nil
}

func (t *Tools) sendRequest(ctx context.Context, args json.RawMessage) (any, error) {
	var a struct {
		runArgs
		Operation string `json:"operation"`
		Body      any    `json:"body"`
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	specPath, opts, err := t.options(a.runArgs)
	if err != nil {
		return nil, err
	}
	spec, err := store.ParseSpec(specPath)
	if err != nil {
		return nil, err
	}
	ref, err := script.ResolveTarget(spec, specPath, a.Operation)
	if err != nil {
		return nil, err
	}
	if a.Body != nil {
		var raw string
		if s, ok := a.Body.(string); ok {
			raw = s
		} else {
			data, err := json.Marshal(a.Body)
			if err != nil {
				return nil, err
			}
			raw = string(data)
			if opts.Headers == nil {
				opts.Headers = map[string]string{}
			}
			if _, ok := opts.Headers["Content-Type"]; !ok {
				opts.Headers["Content-Type"] = "application/json"
			}
		}
		opts.Body = &raw
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	res, lockErr := runner.RunOne(ctx, spec, specPath, ref, opts)
	if lockErr != nil {
		return nil, lockErr
	}
	return report.FilterBodies(res, report.BodiesAll), nil
}

func (t *Tools) runTests(ctx context.Context, args json.RawMessage) (any, error) {
	var a struct {
		runArgs
		Methods      []string `json:"methods"`
		Tags         []string `json:"tags"`
		Operations   []string `json:"operations"`
		Grep         string   `json:"grep"`
		Bail         bool     `json:"bail"`
		IncludeFlows bool     `json:"include_flows"`
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	specPath, opts, err := t.options(a.runArgs)
	if err != nil {
		return nil, err
	}
	spec, err := store.ParseSpec(specPath)
	if err != nil {
		return nil, err
	}
	opts.Filter = runner.Filter{Methods: runner.ParseMethods(strings.Join(a.Methods, ",")), Tags: a.Tags, Ops: a.Operations}
	if a.Grep != "" {
		if opts.Filter.Grep, err = regexp.Compile(a.Grep); err != nil {
			return nil, fmt.Errorf("invalid grep: %w", err)
		}
	}
	opts.Bail = a.Bail
	if a.IncludeFlows {
		if opts.Flows, err = runner.FindFlows(specPath); err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	rep, lockErr := runner.Run(ctx, spec, specPath, opts)
	if lockErr != nil {
		return nil, lockErr
	}
	rep.Spec = t.rel(specPath)
	ops := make([]runner.OpResult, len(rep.Operations))
	for i, op := range rep.Operations {
		ops[i] = report.FilterBodies(op, report.BodiesFailed)
	}
	rep.Operations = ops
	return rep, nil
}

type opArgs struct {
	Spec      string `json:"spec"`
	Operation string `json:"operation"`
}

func (t *Tools) locate(a opArgs) (specPath, scriptPath string, ref model.OpenAPIRef, err error) {
	if specPath, err = t.path(a.Spec); err != nil {
		return
	}
	spec, err := store.ParseSpec(specPath)
	if err != nil {
		return
	}
	if ref, err = script.ResolveTarget(spec, specPath, a.Operation); err != nil {
		return
	}
	scriptPath, err = store.OperationTestsPath(specPath, store.FindOperation(spec, ref), ref)
	return
}

func (t *Tools) readTest(_ context.Context, args json.RawMessage) (any, error) {
	var a opArgs
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	_, scriptPath, _, err := t.locate(a)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(scriptPath)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return map[string]any{"path": t.rel(scriptPath), "exists": exists, "source": string(data)}, nil
}

func (t *Tools) writeTest(_ context.Context, args json.RawMessage) (any, error) {
	var a struct {
		opArgs
		Source string `json:"source"`
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	specPath, _, ref, err := t.locate(a.opArgs)
	if err != nil {
		return nil, err
	}
	spec, err := store.ParseSpec(specPath)
	if err != nil {
		return nil, err
	}
	if err := store.SaveOperationTests(spec, specPath, ref, a.Source); err != nil {
		return nil, err
	}
	if err := store.SaveSpec(specPath, spec); err != nil {
		return nil, err
	}
	if strings.TrimSpace(a.Source) == "" {
		return map[string]any{"unlinked": true}, nil
	}

	op := store.FindOperation(spec, ref)
	scriptPath, _ := store.OperationTestsPath(specPath, op, ref)
	out := map[string]any{"path": t.rel(scriptPath), "saved": true}
	opts, err := script.OperationOptions(specPath, op, ref)
	if err == nil {
		opts.Modules = script.NewModuleLoader(filepath.Dir(specPath))
		_, err = script.Bundle(a.Source, opts)
		if saveErr := opts.Modules.SaveLock(); err == nil {
			err = saveErr
		}
	}
	if err != nil {
		out["compileError"] = err.Error()
	}
	return out, nil
}

func (t *Tools) validateSpec(_ context.Context, args json.RawMessage) (any, error) {
	var a specArgs
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	specPath, err := t.path(a.Spec)
	if err != nil {
		return nil, err
	}
	v := inspect.Validate(specPath)
	v.Spec = t.rel(specPath)
	return v, nil
}

var flowName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func (t *Tools) runFlow(ctx context.Context, args json.RawMessage) (any, error) {
	var a struct {
		runArgs
		Flow string `json:"flow"`
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	specPath, opts, err := t.options(a.runArgs)
	if err != nil {
		return nil, err
	}
	flowPath := a.Flow
	if flowName.MatchString(strings.TrimSuffix(flowPath, ".js")) {
		flowPath = filepath.Join(filepath.Dir(specPath), runner.FlowsDir, strings.TrimSuffix(flowPath, ".js")+".js")
	} else if flowPath, err = t.path(a.Flow); err != nil {
		return nil, err
	}
	spec, err := store.ParseSpec(specPath)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, runner.DefaultFlowTimeout)
	defer cancel()
	vars := map[string]string{}
	for k, v := range opts.Vars {
		vars[k] = v
	}
	res := runner.RunFlow(ctx, spec, specPath, flowPath, opts, opts.Env, vars)
	if err := opts.Modules.SaveLock(); err != nil {
		return nil, err
	}
	return res, nil
}

func (t *Tools) writeFlow(_ context.Context, args json.RawMessage) (any, error) {
	var a struct {
		Spec   string `json:"spec"`
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	name := strings.TrimSuffix(a.Name, ".js")
	if !flowName.MatchString(name) {
		return nil, fmt.Errorf("invalid flow name %q: use letters, digits, - and _", a.Name)
	}
	specPath, err := t.path(a.Spec)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(filepath.Dir(specPath), runner.FlowsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name+".js")
	if err := os.WriteFile(path, []byte(a.Source), 0o644); err != nil {
		return nil, err
	}
	out := map[string]any{"path": t.rel(path), "saved": true}
	specDir, _ := filepath.Abs(filepath.Dir(specPath))
	modules := script.NewModuleLoader(specDir)
	if _, err := script.Bundle(a.Source, script.Options{Filename: path, ResolveDir: dir, Root: specDir, Modules: modules}); err != nil {
		out["compileError"] = err.Error()
	} else if err := modules.SaveLock(); err != nil {
		out["compileError"] = err.Error()
	}
	return out, nil
}

func (t *Tools) importCurl(_ context.Context, args json.RawMessage) (any, error) {
	var a struct {
		Spec    string `json:"spec"`
		Command string `json:"command"`
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	specPath, err := t.path(a.Spec)
	if err != nil {
		return nil, err
	}
	r, err := importer.ParseCurl(a.Command)
	if err != nil {
		return nil, err
	}
	spec := importer.NewSpec("")
	if _, statErr := os.Stat(specPath); statErr == nil {
		if spec, err = store.ParseSpec(specPath); err != nil {
			return nil, err
		}
	}
	res, err := importer.Apply(spec, specPath, []importer.Request{r})
	if err != nil {
		return nil, err
	}
	if len(res.Added) > 0 {
		if err := store.SaveSpec(specPath, spec); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (t *Tools) exportCurl(ctx context.Context, args json.RawMessage) (any, error) {
	var a struct {
		runArgs
		Operation string `json:"operation"`
		Body      any    `json:"body"`
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	specPath, opts, err := t.options(a.runArgs)
	if err != nil {
		return nil, err
	}
	spec, err := store.ParseSpec(specPath)
	if err != nil {
		return nil, err
	}
	ref, err := script.ResolveTarget(spec, specPath, a.Operation)
	if err != nil {
		return nil, err
	}
	if a.Body != nil {
		raw, ok := a.Body.(string)
		if !ok {
			data, err := json.Marshal(a.Body)
			if err != nil {
				return nil, err
			}
			raw = string(data)
		}
		opts.Body = &raw
	}
	req, err := runner.Prepare(ctx, spec, specPath, ref, opts)
	if err != nil {
		return nil, err
	}
	cmd, err := req.Curl(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"curl": cmd}, nil
}
