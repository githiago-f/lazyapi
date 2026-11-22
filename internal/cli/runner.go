package cli

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/script"
	"github.com/githiago-f/lazyapi/internal/store"
)

// selectServer applies the --server value to req. The value may be a URL or
// an index into the spec's servers list; empty falls back to the first server.
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

// applyOverrides copies name=value overrides into target. When onlyKnown is
// true, names not already present in target are ignored (used by smoke, where
// one --param applies to many operations).
func applyOverrides(target map[string]string, overrides kvFlag, onlyKnown bool) {
	for name, value := range overrides {
		if _, ok := target[name]; ok || !onlyKnown {
			target[name] = value
		}
	}
}

// scriptFlags are the scripting/contract flags shared by send and smoke.
type scriptFlags struct {
	file          *string
	offline       *bool
	reload        *bool
	frozen        *bool
	allowImport   *string
	noContract    *bool
	scriptTimeout *time.Duration
}

func registerScriptFlags(fs *flag.FlagSet, scriptHelp string) *scriptFlags {
	return &scriptFlags{
		file:          fs.String("script", "", scriptHelp),
		offline:       fs.Bool("offline", false, "never download URL imports; use the module cache only"),
		reload:        fs.Bool("reload", false, "re-download URL imports and update "+script.LockFileName),
		frozen:        fs.Bool("frozen-lockfile", false, "fail if a URL import is not pinned in "+script.LockFileName+" (CI)"),
		allowImport:   fs.String("allow-import", "", "comma-separated hosts URL imports may come from (default: any https host)"),
		noContract:    fs.Bool("no-contract", false, "skip validating responses against the OpenAPI schema"),
		scriptTimeout: fs.Duration("script-timeout", script.DefaultTimeout, "maximum run time of a test script"),
	}
}

// opRunner executes requests of one spec and runs their checks.
type opRunner struct {
	spec     *openapi3.T
	specPath string
	flags    *scriptFlags
	modules  *script.ModuleLoader
	ctx      *script.SpecContext

	override     string // --script content
	overridePath string // absolute path of --script
	pass, fail   int
	scriptErrors int
}

func newOpRunner(spec *openapi3.T, specPath string, flags *scriptFlags, env map[string]string, timeout time.Duration) (*opRunner, error) {
	specDir, err := filepath.Abs(filepath.Dir(specPath))
	if err != nil {
		return nil, err
	}
	modules := script.NewModuleLoader(specDir)
	modules.Offline = *flags.offline
	modules.Reload = *flags.reload
	modules.Frozen = *flags.frozen
	modules.AllowedHosts = splitList(*flags.allowImport)

	r := &opRunner{
		spec:     spec,
		specPath: specPath,
		flags:    flags,
		modules:  modules,
		ctx: &script.SpecContext{
			Spec:     spec,
			SpecPath: specPath,
			Env:      env,
			Vars:     map[string]string{},
			Timeout:  timeout,
		},
	}
	if *flags.file != "" {
		data, err := os.ReadFile(*flags.file)
		if err != nil {
			return nil, fmt.Errorf("error reading script file: %w", err)
		}
		r.override = string(data)
		if r.overridePath, err = filepath.Abs(*flags.file); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// prepare finalises a request built from the spec for sending.
func (r *opRunner) prepare(req *model.Request) {
	req.Env = r.ctx.Env
	req.Vars = r.ctx.Vars
	req.Timeout = r.ctx.Timeout
	r.ctx.ServerURL = req.ServerURL
}

// scriptFor returns the script source and options for an operation:
// --script wins, otherwise the operation's x-lazyapi-tests file.
func (r *opRunner) scriptFor(op *openapi3.Operation, ref model.OpenAPIRef) (string, script.Options, error) {
	specDir, _ := filepath.Abs(filepath.Dir(r.specPath))
	opts := script.Options{Modules: r.modules, Timeout: *r.flags.scriptTimeout, Root: specDir}

	if r.overridePath != "" {
		opts.Filename = r.overridePath
		opts.ResolveDir = filepath.Dir(r.overridePath)
		if !withinDir(specDir, r.overridePath) {
			opts.Root = opts.ResolveDir
		}
		return r.override, opts, nil
	}

	source, err := store.LoadOperationTests(r.specPath, op)
	if err != nil || source == "" {
		return "", opts, err
	}
	opOpts, err := script.OperationOptions(r.specPath, op, ref)
	if err != nil {
		return "", opts, err
	}
	opOpts.Modules, opOpts.Timeout = opts.Modules, opts.Timeout
	return source, opOpts, nil
}

// check validates the response against the contract and runs the script,
// printing every result. It returns false if anything failed.
func (r *opRunner) check(req model.Request, ref model.OpenAPIRef, resp *http.Response, body string, elapsed time.Duration,
	source string, opts script.Options, indent string) bool {
	ok := true

	if !*r.flags.noContract && store.ResponseSchemaDeclared(r.spec, ref, resp.StatusCode) {
		if err := store.ValidateResponse(r.spec, ref, resp.StatusCode, resp.Header, []byte(body)); err != nil {
			r.fail++
			ok = false
			fmt.Printf("%s[FAIL] contract: response matches the OpenAPI schema\n%s       %s\n", indent, indent, err)
		} else {
			r.pass++
			fmt.Printf("%s[PASS] contract: response matches the OpenAPI schema\n", indent)
		}
	}

	if source == "" {
		return ok
	}

	data := script.ExecData{
		Request: req,
		Response: &script.Response{
			Status:     resp.StatusCode,
			StatusText: resp.Status,
			Header:     resp.Header,
			Body:       body,
			Duration:   elapsed,
			Ref:        &ref,
		},
	}
	r.ctx.Bind(&data)
	result := script.Run(source, data, opts)

	for _, l := range result.Logs {
		fmt.Printf("%s[%s] %s\n", indent, l.Level, l.Message)
	}
	for _, tr := range result.Results {
		mark := "PASS"
		if !tr.Passed {
			mark = "FAIL"
		}
		fmt.Printf("%s[%s] %s\n", indent, mark, tr.Name)
		if tr.Message != "" {
			fmt.Printf("%s       %s\n", indent, tr.Message)
		}
	}
	if result.ScriptError != "" {
		r.scriptErrors++
		fmt.Printf("%s[ERR]  Script error: %s\n", indent, result.ScriptError)
	}
	r.pass += result.PassCount
	r.fail += result.FailCount
	return ok && !result.Failed()
}

// finish persists newly pinned URL imports.
func (r *opRunner) finish() error {
	return r.modules.SaveLock()
}

func withinDir(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
