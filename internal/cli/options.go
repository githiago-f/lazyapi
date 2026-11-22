package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/project"
	"github.com/githiago-f/lazyapi/internal/report"
	"github.com/githiago-f/lazyapi/internal/runner"
	"github.com/githiago-f/lazyapi/internal/script"
	"github.com/githiago-f/lazyapi/internal/store"
)

// ExitError carries the process exit status for an error.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// ExitCode returns the exit status for err (0 for nil, 1 by default).
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return runner.ExitFailed
}

func usageErr(format string, args ...any) error {
	return &ExitError{Code: runner.ExitUsage, Err: fmt.Errorf(format, args...)}
}

func runtimeErr(err error) error {
	return &ExitError{Code: runner.ExitErrored, Err: err}
}

// runFlags are shared by `run`/`smoke` and `send`.
type runFlags struct {
	server, envFile, envName, output, outputFile, bodies *string
	params, query, headers                               kvFlag
	timeout                                              *time.Duration

	script, allowImport     *string
	offline, reload, frozen *bool
	noContract, dryRun      *bool
	scriptTimeout           *time.Duration

	http    model.HTTPOptions
	maxBody *string
}

func registerRunFlags(fs *flag.FlagSet, scriptHelp string) *runFlags {
	f := &runFlags{params: kvFlag{}, query: kvFlag{}, headers: kvFlag{}}
	f.server = fs.String("server", "", "server URL or index into the spec's servers (default: first)")
	f.envFile = fs.String("env", "", "environment file ({{env.NAME}})")
	f.envName = fs.String("env-name", "", "environment from "+project.FileName+" (default: its 'default', or $LAZYAPI_ENV)")
	f.output = fs.String("output", report.Pretty, "output format: "+strings.Join(report.Formats, ", "))
	f.outputFile = fs.String("output-file", "", "write the --output report to this file (stdout keeps the pretty log)")
	f.bodies = fs.String("bodies", string(report.BodiesFailed), "bodies in JSON output: failed, all, none")
	f.timeout = fs.Duration("timeout", model.DefaultTimeout, "per-request timeout")
	fs.Var(f.params, "param", "path parameter name=value (repeatable)")
	fs.Var(f.query, "query", "query parameter name=value (repeatable)")
	fs.Var(f.headers, "header", "header name=value (repeatable)")

	f.script = fs.String("script", "", scriptHelp)
	f.offline = fs.Bool("offline", false, "never download URL imports; use the module cache only")
	f.reload = fs.Bool("reload", false, "re-download URL imports and update "+script.LockFileName)
	f.frozen = fs.Bool("frozen-lockfile", false, "fail if a URL import is not pinned in "+script.LockFileName+" (CI)")
	f.allowImport = fs.String("allow-import", "", "comma-separated hosts URL imports may come from (default: any https host)")
	f.noContract = fs.Bool("no-contract", false, "skip validating responses against the OpenAPI schema")
	f.dryRun = fs.Bool("dry-run", false, "print what would be sent without sending")
	f.scriptTimeout = fs.Duration("script-timeout", script.DefaultTimeout, "maximum run time of a test script")

	fs.BoolVar(&f.http.Insecure, "insecure", false, "skip TLS certificate verification")
	fs.StringVar(&f.http.CACert, "ca-cert", "", "PEM file with extra trusted CA certificates")
	fs.StringVar(&f.http.ClientCert, "client-cert", "", "PEM client certificate (mutual TLS)")
	fs.StringVar(&f.http.ClientKey, "client-key", "", "PEM client key (mutual TLS)")
	fs.StringVar(&f.http.Proxy, "proxy", "", "proxy URL (http, https or socks5); default: $HTTPS_PROXY/$HTTP_PROXY")
	fs.BoolVar(&f.http.NoRedirects, "no-redirects", false, "do not follow redirects (3xx responses are checked as-is)")
	fs.IntVar(&f.http.MaxRedirects, "max-redirects", 0, "maximum redirects to follow (default 10)")
	f.maxBody = fs.String("max-body", "", "maximum response body size, e.g. 10MB (default 50MB)")
	return f
}

func (f *runFlags) validate() error {
	if !slices.Contains(report.Formats, *f.output) {
		return usageErr("unknown --output %q (use %s)", *f.output, strings.Join(report.Formats, ", "))
	}
	switch report.Bodies(*f.bodies) {
	case report.BodiesFailed, report.BodiesAll, report.BodiesNone:
	default:
		return usageErr("unknown --bodies %q (use failed, all or none)", *f.bodies)
	}
	return nil
}

// options builds runner options for a spec. A named environment from
// lazyapi.config.yml supplies the server, env file, vars and credentials;
// explicit --server / --env win over it.
func (f *runFlags) options(specPath string) (runner.Options, error) {
	selected, err := project.Resolve(specPath, *f.envName, *f.server, *f.envFile)
	if err != nil {
		return runner.Options{}, usageErr("%v", err)
	}
	server, envFile := selected.Server, selected.EnvFile

	client, err := selected.HTTP.Merge(f.http).Client()
	if err != nil {
		return runner.Options{}, usageErr("%v", err)
	}
	maxBody, err := parseSize(*f.maxBody)
	if err != nil {
		return runner.Options{}, usageErr("--max-body: %v", err)
	}

	envMap, err := env.NewStore(envFile).Load()
	if err != nil {
		return runner.Options{}, runtimeErr(fmt.Errorf("error loading env file: %w", err))
	}
	specDir, err := filepath.Abs(filepath.Dir(specPath))
	if err != nil {
		return runner.Options{}, runtimeErr(err)
	}
	modules := script.NewModuleLoader(specDir)
	modules.Offline = *f.offline
	modules.Reload = *f.reload
	modules.Frozen = *f.frozen
	modules.AllowedHosts = splitList(*f.allowImport)

	return runner.Options{
		HTTPClient:    client,
		MaxBody:       maxBody,
		EnvName:       selected.Name,
		Vars:          selected.Vars,
		Auth:          selected.Auth,
		Server:        server,
		Env:           envMap,
		Params:        f.params,
		Query:         f.query,
		Headers:       f.headers,
		Timeout:       *f.timeout,
		ScriptPath:    *f.script,
		Modules:       modules,
		ScriptTimeout: *f.scriptTimeout,
		NoContract:    *f.noContract,
		DryRun:        *f.dryRun,
	}, nil
}

// prettyToStdout reports whether progress should be printed as it happens.
func (f *runFlags) prettyToStdout() bool {
	return *f.output == report.Pretty || *f.outputFile != ""
}

// writeReport renders the final report to stdout or --output-file.
func (f *runFlags) writeReport(r runner.Report) error {
	if *f.outputFile == "" {
		if *f.output == report.Pretty {
			report.WriteSummary(os.Stdout, r)
			return nil
		}
		return report.Write(os.Stdout, *f.output, r, report.Bodies(*f.bodies))
	}
	report.WriteSummary(os.Stdout, r)
	file, err := os.Create(*f.outputFile)
	if err != nil {
		return runtimeErr(err)
	}
	if err := report.Write(file, *f.output, r, report.Bodies(*f.bodies)); err != nil {
		_ = file.Close()
		return runtimeErr(err)
	}
	return file.Close()
}

// loadSpec parses a spec, mapping failures to the runtime exit code.
func loadSpec(path string) (*openapi3.T, error) {
	spec, err := store.ParseSpec(path)
	if err != nil {
		return nil, runtimeErr(fmt.Errorf("error parsing spec: %w", err))
	}
	return spec, nil
}

// compileGrep parses --grep.
func compileGrep(v string) (*regexp.Regexp, error) {
	if v == "" {
		return nil, nil
	}
	re, err := regexp.Compile(v)
	if err != nil {
		return nil, usageErr("invalid --grep: %v", err)
	}
	return re, nil
}

// reportError turns a non-passing report into an ExitError.
func reportError(r runner.Report) error {
	code := r.ExitCode()
	if code == runner.ExitOK {
		return nil
	}
	s := r.Summary
	return &ExitError{Code: code, Err: fmt.Errorf("%d failed, %d errors (%d checks failed)", s.Failed, s.Errors, s.ChecksFailed)}
}

// parseSize reads sizes like "512", "10KB", "50MB", "1GB" ("" means 0).
func parseSize(v string) (int64, error) {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v == "" {
		return 0, nil
	}
	mult := int64(1)
	for _, u := range []struct {
		suffix string
		mult   int64
	}{{"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10}, {"B", 1}} {
		if strings.HasSuffix(v, u.suffix) {
			v, mult = strings.TrimSpace(strings.TrimSuffix(v, u.suffix)), u.mult
			break
		}
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid size %q (use e.g. 10MB)", v)
	}
	return n * mult, nil
}
