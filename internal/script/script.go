// Package script runs JavaScript/TypeScript test scripts against HTTP
// requests and responses.
//
// Scripts are bundled with esbuild (so they may use ES modules, TypeScript,
// relative imports and https:// imports) and executed in a sandboxed goja
// runtime with no filesystem or process access. The globals available to
// scripts are documented in lazyapi.d.ts (see Types).
package script

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
)

//go:embed prelude.js
var prelude string

//go:embed lazyapi.d.ts
var Types string

// DefaultTimeout bounds script execution when Options.Timeout is zero.
const DefaultTimeout = 30 * time.Second

// Response is an HTTP response exposed to scripts.
type Response struct {
	Status     int
	StatusText string
	Header     http.Header
	Body       string
	Duration   time.Duration
	// Ref identifies the operation that produced the response, when known.
	// It is what expect(response).toMatchSchema() validates against.
	Ref *model.OpenAPIRef
}

// SendOptions are the per-call overrides accepted by lazyapi.send().
type SendOptions struct {
	Params  map[string]string `json:"params"`
	Query   map[string]string `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body"` // string, or any JSON value (encoded)
}

// SendFunc performs a request for lazyapi.send(). target is an operationId or
// "METHOD /path".
type SendFunc func(target string, opts SendOptions) (*Response, error)

// ContractFunc validates a response against the API contract of ref.
type ContractFunc func(ref model.OpenAPIRef, resp *Response) error

// ExecData is the context a script runs in.
type ExecData struct {
	Request  model.Request
	Response *Response // nil when no request has been sent yet

	// Env and Vars are exposed as env.* and vars.*; scripts may mutate them.
	Env  map[string]string
	Vars map[string]string

	Send     SendFunc     // optional: enables lazyapi.send()
	Contract ContractFunc // optional: enables expect(response).toMatchSchema()
}

// Options control how a script is bundled and executed.
type Options struct {
	// Filename is used in error messages; a .ts/.tsx suffix enables TypeScript.
	Filename string
	// ResolveDir is the directory relative imports are resolved against.
	ResolveDir string
	// Root confines local imports; defaults to ResolveDir.
	Root string
	// Modules fetches https:// imports. Nil disables URL imports.
	Modules *ModuleLoader
	// Timeout bounds execution (DefaultTimeout when zero).
	Timeout time.Duration
}

// TestResult is one test() outcome.
type TestResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// LogEntry is a captured console.* call.
type LogEntry struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// Result is the outcome of running a script.
type Result struct {
	Results     []TestResult `json:"results"`
	Logs        []LogEntry   `json:"logs,omitempty"`
	ScriptError string       `json:"error,omitempty"`
	PassCount   int          `json:"passed"`
	FailCount   int          `json:"failed"`
}

// Failed reports whether any test failed or the script errored.
func (r Result) Failed() bool {
	return r.FailCount > 0 || r.ScriptError != ""
}

// Run bundles and executes source, returning the collected test results.
// It never panics; bundling and runtime errors end up in Result.ScriptError.
func Run(source string, data ExecData, opts Options) Result {
	code, err := Bundle(source, opts)
	if err != nil {
		return Result{ScriptError: err.Error(), Results: []TestResult{}}
	}
	return execute(code, data, opts)
}

func execute(code string, data ExecData, opts Options) (result Result) {
	result.Results = []TestResult{}
	b := &bridge{data: data, result: &result}
	vm, stop, err := newRuntime(b, opts)
	if err != nil {
		result.ScriptError = err.Error()
		return finish(result)
	}
	defer stop()
	if _, err := vm.RunScript(scriptName(opts), code); err != nil {
		result.ScriptError = jsError(err)
	}
	return finish(result)
}

// newRuntime creates a VM with the bridge, the prelude and the module
// object, and arms the timeout. stop must be called when done.
func newRuntime(b *bridge, opts Options) (*goja.Runtime, func(), error) {
	vm := goja.New()
	b.vm = vm
	if err := vm.Set("__lazyapi", b.object()); err != nil {
		return nil, nil, err
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	timer := time.AfterFunc(timeout, func() {
		vm.Interrupt(fmt.Sprintf("script timed out after %s", timeout))
	})
	stop := func() { timer.Stop() }
	if _, err := vm.RunScript("lazyapi:prelude", prelude); err != nil {
		stop()
		return nil, nil, fmt.Errorf("internal prelude error: %s", jsError(err))
	}
	if _, err := vm.RunString("var __lazyapi_module = { exports: {} };"); err != nil {
		stop()
		return nil, nil, err
	}
	return vm, stop, nil
}

func scriptName(opts Options) string {
	if opts.Filename == "" {
		return "script.js"
	}
	return opts.Filename
}

// BeforeResult is the outcome of running a script's before(req) hook.
type BeforeResult struct {
	// Defined is false when the script does not export before().
	Defined bool
	// Request is the request after before() changed it.
	Request model.Request
	Logs    []LogEntry
	// Error is set when the script does not compile or before() threw.
	Error string
}

// Before runs the `export function before(req)` hook of a test script, so it
// can change the request (headers, query, params, body) before it is sent.
//
// The module's top level is evaluated first to reach the export. During that
// evaluation there is no response yet, so test() calls, console output,
// variable writes and lazyapi.send() are ignored, and errors are swallowed
// (top-level code normally reads the response). before() itself runs with
// the full API: it may call lazyapi.send() (e.g. to log in) and set vars.
func Before(source string, data ExecData, opts Options) BeforeResult {
	out := BeforeResult{Request: data.Request}
	code, err := Bundle(source, opts)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	var discard Result
	data.Response = nil
	b := &bridge{data: data, result: &discard, evaluating: true}
	vm, stop, err := newRuntime(b, opts)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	defer stop()

	_, evalErr := vm.RunScript(scriptName(opts), code)
	var intr *goja.InterruptedError
	if errors.As(evalErr, &intr) {
		out.Error = jsError(evalErr)
		return out
	}
	b.evaluating = false

	exports := vm.Get("__lazyapi_module").ToObject(vm).Get("exports")
	if exports == nil || goja.IsUndefined(exports) || goja.IsNull(exports) {
		return out
	}
	fn, ok := goja.AssertFunction(exports.ToObject(vm).Get("before"))
	if !ok {
		return out
	}
	out.Defined = true

	reqObj := vm.ToValue(beforeRequest(data.Request))
	ret, err := fn(goja.Undefined(), reqObj)
	out.Logs = discard.Logs
	if err != nil {
		out.Error = "before(): " + jsError(err)
		return out
	}
	if obj, ok := ret.(*goja.Object); ok && ret.ExportType() != nil {
		reqObj = obj
	}
	updated, err := applyBeforeRequest(data.Request, reqObj.Export())
	if err != nil {
		out.Error = "before(): " + err.Error()
		return out
	}
	out.Request = updated
	return out
}

// beforeRequest is the mutable request object passed to before().
func beforeRequest(r model.Request) map[string]any {
	return map[string]any{
		"method":  r.Method.Label(),
		"url":     r.ResolvedURL(),
		"path":    r.URI,
		"headers": toAnyMap(r.Headers),
		"query":   toAnyMap(r.Query),
		"params":  toAnyMap(r.Params),
		"body":    r.Body.Raw,
	}
}

func toAnyMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// applyBeforeRequest copies headers, query, params and body back from the
// object before() changed. A non-string body is JSON-encoded.
func applyBeforeRequest(r model.Request, v any) (model.Request, error) {
	obj, ok := v.(map[string]any)
	if !ok {
		return r, fmt.Errorf("must change the request in place or return an object, got %T", v)
	}
	var err error
	if r.Headers, err = stringMap(obj["headers"], r.Headers); err != nil {
		return r, fmt.Errorf("headers: %w", err)
	}
	if r.Query, err = stringMap(obj["query"], r.Query); err != nil {
		return r, fmt.Errorf("query: %w", err)
	}
	if r.Params, err = stringMap(obj["params"], r.Params); err != nil {
		return r, fmt.Errorf("params: %w", err)
	}
	switch body := obj["body"].(type) {
	case nil:
		r.Body.Raw = ""
	case string:
		r.Body.Raw = body
	default:
		data, err := json.Marshal(body)
		if err != nil {
			return r, fmt.Errorf("body: %w", err)
		}
		r.Body.Raw = string(data)
		if r.Body.Type == "" {
			r.Body.Type = model.ApplicationJSON
		}
	}
	return r, nil
}

func stringMap(v any, fallback map[string]string) (map[string]string, error) {
	if v == nil {
		return fallback, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected an object, got %T", v)
	}
	out := make(map[string]string, len(m))
	for k, x := range m {
		switch t := x.(type) {
		case string:
			out[k] = t
		case nil:
		default:
			out[k] = fmt.Sprint(t)
		}
	}
	return out, nil
}

func finish(r Result) Result {
	r.PassCount, r.FailCount = 0, 0
	for _, tr := range r.Results {
		if tr.Passed {
			r.PassCount++
		} else {
			r.FailCount++
		}
	}
	return r
}

// jsError renders a goja error as "message (file:line:col)" without the
// full stack trace.
func jsError(err error) string {
	var ex *goja.Exception
	if errors.As(err, &ex) {
		msg := ex.Value().String()
		if obj, ok := ex.Value().(*goja.Object); ok {
			if m := obj.Get("message"); m != nil && !goja.IsUndefined(m) {
				if n := obj.Get("name"); n != nil && !goja.IsUndefined(n) {
					msg = n.String() + ": " + m.String()
				} else {
					msg = m.String()
				}
			}
		}
		if frames := ex.Stack(); len(frames) > 0 {
			pos := frames[0].Position()
			if pos.Filename != "" && pos.Line > 0 {
				msg += fmt.Sprintf(" (%s:%d:%d)", pos.Filename, pos.Line, pos.Column)
			}
		}
		return msg
	}
	var intr *goja.InterruptedError
	if errors.As(err, &intr) {
		return fmt.Sprint(intr.Value())
	}
	return err.Error()
}

// ---------------------------------------------------------------------------
// Native bridge
// ---------------------------------------------------------------------------

type bridge struct {
	vm        *goja.Runtime
	data      ExecData
	result    *Result
	responses []*Response // index = response id seen by JS

	// evaluating is true while Before evaluates the module top level to find
	// the before() export: side effects are suppressed.
	evaluating bool
}

func (b *bridge) object() map[string]any {
	if b.data.Env == nil {
		b.data.Env = map[string]string{}
	}
	if b.data.Vars == nil {
		b.data.Vars = map[string]string{}
	}
	obj := map[string]any{
		"request": b.requestData(),
		"record": func(name string, passed bool, message string) {
			if b.evaluating {
				return
			}
			b.result.Results = append(b.result.Results, TestResult{Name: name, Passed: passed, Message: message})
		},
		"log": func(level, message string) {
			if b.evaluating {
				return
			}
			b.result.Logs = append(b.result.Logs, LogEntry{Level: level, Message: message})
		},
		"envGet": mapGet(b.vm, b.data.Env),
		"envSet": func(k, v string) {
			if !b.evaluating {
				b.data.Env[k] = v
			}
		},
		"envHas": func(k string) bool { _, ok := b.data.Env[k]; return ok },
		"envAll": func() map[string]string { return copyMap(b.data.Env) },
		"varGet": mapGet(b.vm, b.data.Vars),
		"varSet": func(k, v string) {
			if !b.evaluating {
				b.data.Vars[k] = v
			}
		},
		"varHas":           func(k string) bool { _, ok := b.data.Vars[k]; return ok },
		"varAll":           func() map[string]string { return copyMap(b.data.Vars) },
		"send":             b.send,
		"validateContract": b.validateContract,
		"validateSchema":   validateSchema,
	}
	if b.data.Response != nil {
		obj["response"] = b.responseData(b.data.Response)
	} else {
		obj["response"] = nil
	}
	return obj
}

func mapGet(vm *goja.Runtime, m map[string]string) func(string) goja.Value {
	return func(k string) goja.Value {
		if v, ok := m[k]; ok {
			return vm.ToValue(v)
		}
		return goja.Undefined()
	}
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (b *bridge) requestData() map[string]any {
	r := b.data.Request
	return map[string]any{
		"method":  r.Method.Label(),
		"url":     r.ResolvedURL(),
		"path":    r.URI,
		"headers": lowerKeys(r.Headers),
		"query":   copyMap(r.Query),
		"params":  copyMap(r.Params),
		"body":    r.Body.Raw,
	}
}

func lowerKeys(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[strings.ToLower(k)] = v
	}
	return out
}

func (b *bridge) responseData(resp *Response) map[string]any {
	id := len(b.responses)
	b.responses = append(b.responses, resp)
	headers := make(map[string]string, len(resp.Header))
	for k, vs := range resp.Header {
		headers[strings.ToLower(k)] = strings.Join(vs, ", ")
	}
	return map[string]any{
		"id":         id,
		"status":     resp.Status,
		"statusText": resp.StatusText,
		"headers":    headers,
		"body":       resp.Body,
		"time":       resp.Duration.Milliseconds(),
	}
}

func (b *bridge) send(target string, opts map[string]any) (map[string]any, error) {
	if b.evaluating {
		return nil, errors.New("lazyapi.send() at the top level is skipped while preparing before()")
	}
	if b.data.Send == nil {
		return nil, errors.New("lazyapi.send() is not available in this context")
	}
	var so SendOptions
	raw, err := json.Marshal(opts)
	if err != nil {
		return nil, fmt.Errorf("lazyapi.send(): invalid options: %w", err)
	}
	if err := json.Unmarshal(raw, &so); err != nil {
		return nil, fmt.Errorf("lazyapi.send(): invalid options: %w", err)
	}
	resp, err := b.data.Send(target, so)
	if err != nil {
		return nil, fmt.Errorf("lazyapi.send(%q): %w", target, err)
	}
	return b.responseData(resp), nil
}

// validateContract returns "" when the response matches, or the reason.
func (b *bridge) validateContract(id int) string {
	if id < 0 || id >= len(b.responses) {
		return "unknown response"
	}
	resp := b.responses[id]
	if b.data.Contract == nil {
		return "contract validation is not available (no OpenAPI spec in context)"
	}
	if resp.Ref == nil {
		return "response is not linked to an OpenAPI operation"
	}
	if err := b.data.Contract(*resp.Ref, resp); err != nil {
		return err.Error()
	}
	return ""
}

// validateSchema validates a JSON value against a JSON/OpenAPI schema.
// It returns "" on success or the validation error.
func validateSchema(valueJSON, schemaJSON string) string {
	var schema openapi3.Schema
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return "invalid schema: " + err.Error()
	}
	var value any
	if valueJSON != "" {
		if err := json.Unmarshal([]byte(valueJSON), &value); err != nil {
			return "invalid value: " + err.Error()
		}
	}
	if err := schema.VisitJSON(value, openapi3.MultiErrors()); err != nil {
		return err.Error()
	}
	return ""
}

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

// Format renders a result as plain text (one line per test, then logs).
func Format(r Result) string {
	var b strings.Builder
	if r.ScriptError != "" {
		fmt.Fprintf(&b, "Script error: %s\n\n", r.ScriptError)
	}
	if len(r.Results) == 0 && r.ScriptError == "" {
		b.WriteString("No tests defined. Use: test(\"name\", () => expect(response.status).toBe(200))\n")
	}
	for _, tr := range r.Results {
		mark := "PASS"
		if !tr.Passed {
			mark = "FAIL"
		}
		fmt.Fprintf(&b, "  [%s] %s\n", mark, tr.Name)
		if tr.Message != "" {
			fmt.Fprintf(&b, "         %s\n", tr.Message)
		}
	}
	if len(r.Results) > 0 {
		fmt.Fprintf(&b, "\n%d passed, %d failed\n", r.PassCount, r.FailCount)
	}
	if len(r.Logs) > 0 {
		b.WriteString("\nConsole:\n")
		for _, l := range r.Logs {
			fmt.Fprintf(&b, "  [%s] %s\n", l.Level, l.Message)
		}
	}
	return b.String()
}
