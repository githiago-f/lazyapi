package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

const smokeUsage = "usage: lazyapi smoke tests <file> [--server url|index] [--env file] [--script file] " +
	"[--methods GET,HEAD|all] [--tag t1,t2] [--op id|\"METHOD /path\"] [--param k=v]... [--timeout 30s] [--dry-run]"

// defaultSmokeMethods are the methods smoke runs unless --methods says
// otherwise: safe, read-only verbs that do not mutate server state.
var defaultSmokeMethods = []string{"GET", "HEAD", "OPTIONS"}

type smokeFilter struct {
	methods []string // upper-case; nil means all
	tags    []string
	ops     []string
}

func newSmokeFilter(methods, tags, ops string) smokeFilter {
	f := smokeFilter{tags: splitList(tags), ops: splitList(ops)}
	switch strings.ToLower(strings.TrimSpace(methods)) {
	case "":
		f.methods = defaultSmokeMethods
	case "all", "*":
		f.methods = nil
	default:
		for _, m := range splitList(methods) {
			f.methods = append(f.methods, strings.ToUpper(m))
		}
	}
	return f
}

// check reports whether an operation should run. Operations not selected by
// --op/--tag are dropped silently (reason ""); excluded methods carry a
// reason so they are reported as skipped.
func (f smokeFilter) check(method, path string, op *openapi3.Operation) (run bool, reason string) {
	if len(f.ops) > 0 {
		key := method + " " + path
		if !slices.Contains(f.ops, key) && (op == nil || !slices.Contains(f.ops, op.OperationID)) {
			return false, ""
		}
	}
	if len(f.tags) > 0 {
		if op == nil || !slices.ContainsFunc(op.Tags, func(t string) bool { return slices.Contains(f.tags, t) }) {
			return false, ""
		}
	}
	if f.methods != nil && !slices.Contains(f.methods, method) {
		return false, fmt.Sprintf("method %s not enabled (use --methods %s or --methods all)", method, method)
	}
	return true, ""
}

func SmokeTests(args []string) error {
	fs := newFlagSet("smoke tests")
	serverFlag := fs.String("server", "", "server URL or index into the spec's servers")
	envFile := fs.String("env", "", "environment file")
	scriptFile := fs.String("script", "", "test script applied to every operation (overrides x-lazyapi-tests)")
	methods := fs.String("methods", "", "comma-separated methods to run (default GET,HEAD,OPTIONS; 'all' for every method)")
	tags := fs.String("tag", "", "only run operations with one of these tags (comma-separated)")
	ops := fs.String("op", "", "only run these operations: operationId or \"METHOD /path\" (comma-separated)")
	dryRun := fs.Bool("dry-run", false, "print what would be sent without sending")
	timeout := fs.Duration("timeout", model.DefaultTimeout, "per-request timeout")
	params := kvFlag{}
	fs.Var(params, "param", "path parameter name=value applied to every operation that has it (repeatable)")

	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, smokeUsage)
	}
	if len(pos) != 1 {
		return fmt.Errorf("%s", smokeUsage)
	}
	file := pos[0]
	filter := newSmokeFilter(*methods, *tags, *ops)

	spec, err := store.ParseSpec(file)
	if err != nil {
		return fmt.Errorf("error parsing spec: %w", err)
	}

	envMap, err := env.NewStore(*envFile).Load()
	if err != nil {
		return fmt.Errorf("error loading env file: %w", err)
	}

	sharedScript, err := readScriptFile(*scriptFile)
	if err != nil {
		return err
	}

	items := store.ListOperations(spec, file)
	if len(items) == 0 {
		fmt.Println("No operations found in spec")
		return nil
	}

	var totalPass, totalFail, totalErrors, totalSkipped int

	for _, item := range items {
		method := item.Method.Label()
		ref := model.OpenAPIRef{FilePath: file, Path: item.URI, Method: method}
		op := store.FindOperation(spec, ref)
		label := method + " " + item.URI

		if run, reason := filter.check(method, item.URI, op); !run {
			if reason != "" {
				totalSkipped++
				fmt.Printf("SKIP  %s — %s\n", label, reason)
			}
			continue
		}

		req := store.OperationToRequest(spec, ref)
		req.Timeout = *timeout
		req.Env = envMap
		applyOverrides(req.Params, params, true)

		if missing := req.MissingPathParams(); len(missing) > 0 {
			totalSkipped++
			fmt.Printf("SKIP  %s — no value for path parameter(s) %s (use --param or declare an example)\n",
				label, strings.Join(missing, ", "))
			continue
		}

		if err := selectServer(&req, *serverFlag); err != nil {
			return err
		}
		if req.ServerURL == "" {
			totalSkipped++
			fmt.Printf("SKIP  %s — no server URL\n", label)
			continue
		}

		testScript, err := operationScript(sharedScript, file, op)
		if err != nil {
			totalErrors++
			fmt.Printf("ERR   %s — %s\n\n", label, err)
			continue
		}

		if *dryRun {
			fmt.Printf("DRY   %s %s\n", method, req.ResolvedURL())
			continue
		}

		response, body, err := req.Send()
		if err != nil {
			totalErrors++
			fmt.Printf("ERR   %s — %s\n\n", label, err)
			continue
		}
		fmt.Printf("%s — Status %d\n", label, response.StatusCode)

		declared := store.DeclaredStatuses(spec, ref)
		if !store.StatusDeclared(declared, response.StatusCode) {
			totalErrors++
			if len(declared) == 0 {
				fmt.Printf("  [ERR]  unexpected HTTP status %d\n", response.StatusCode)
			} else {
				fmt.Printf("  [ERR]  status %d not declared in spec (declared: %s)\n",
					response.StatusCode, strings.Join(declared, ", "))
			}
		}

		if testScript == "" {
			fmt.Println("  (no test script — use --script or x-lazyapi-tests)")
			fmt.Println()
			continue
		}

		result := runTests(testScript, req, envMap, response, body, "  ")
		totalPass += result.PassCount
		totalFail += result.FailCount
		if result.ScriptError != "" {
			totalErrors++
		}
		fmt.Printf("  %d passed, %d failed\n\n", result.PassCount, result.FailCount)
	}

	fmt.Println("========================================")
	fmt.Printf("Total: %d passed, %d failed, %d errors, %d skipped\n", totalPass, totalFail, totalErrors, totalSkipped)
	if totalFail > 0 || totalErrors > 0 {
		return fmt.Errorf("smoke tests failed: %d failed, %d errors", totalFail, totalErrors)
	}

	return nil
}
