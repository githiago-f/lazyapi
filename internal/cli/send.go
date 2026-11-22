package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

const sendUsage = "usage: lazyapi send request <file> <path> <method> [--server url|index] [--env file] " +
	"[--param k=v]... [--query k=v]... [--header k=v]... [--script file] [--timeout 30s] [--save-example] " +
	"[--no-contract] [--offline] [--reload] [--frozen-lockfile] [--allow-import hosts]"

func SendRequest(args []string) error {
	fs := newFlagSet("send request")
	serverFlag := fs.String("server", "", "server URL or index into the spec's servers")
	envFile := fs.String("env", "", "environment file")
	sflags := registerScriptFlags(fs, "test script (overrides x-lazyapi-tests)")
	saveExample := fs.Bool("save-example", false, "save the response as an example in the spec")
	timeout := fs.Duration("timeout", model.DefaultTimeout, "request timeout")
	params, query, headers := kvFlag{}, kvFlag{}, kvFlag{}
	fs.Var(params, "param", "path parameter name=value (repeatable)")
	fs.Var(query, "query", "query parameter name=value (repeatable)")
	fs.Var(headers, "header", "header name=value (repeatable)")

	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, sendUsage)
	}
	if len(pos) != 3 {
		return fmt.Errorf("%s", sendUsage)
	}
	file, path, method := pos[0], pos[1], strings.ToUpper(pos[2])

	spec, err := store.ParseSpec(file)
	if err != nil {
		return fmt.Errorf("error parsing spec: %w", err)
	}

	ref := model.OpenAPIRef{FilePath: file, Path: path, Method: method}
	op := store.FindOperation(spec, ref)
	if op == nil {
		return fmt.Errorf("operation %s %s not found in %s", method, path, file)
	}
	req := store.OperationToRequest(spec, ref)
	req.Timeout = *timeout
	applyOverrides(req.Params, params, false)
	applyOverrides(req.Query, query, false)
	applyOverrides(req.Headers, headers, false)

	if missing := req.MissingPathParams(); len(missing) > 0 {
		return fmt.Errorf("missing value for path parameter(s) %s: pass --param %s=<value> or declare an example in the spec",
			strings.Join(missing, ", "), missing[0])
	}

	if err := selectServer(&req, *serverFlag); err != nil {
		return err
	}
	if req.ServerURL == "" {
		return fmt.Errorf("no server URL available. Specify one with --server <url>")
	}
	fmt.Printf("Server: %s\n", req.ServerURL)

	envMap, err := env.NewStore(*envFile).Load()
	if err != nil {
		return fmt.Errorf("error loading env file: %w", err)
	}
	runner, err := newOpRunner(spec, file, sflags, envMap, *timeout)
	if err != nil {
		return err
	}
	runner.prepare(&req)
	source, scriptOpts, err := runner.scriptFor(op, ref)
	if err != nil {
		return err
	}

	start := time.Now()
	response, body, err := req.Send()
	elapsed := time.Since(start)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	fmt.Printf("Status: %s (%dms)\n", response.Status, elapsed.Milliseconds())
	declared := store.DeclaredStatuses(spec, ref)
	if !store.StatusDeclared(declared, response.StatusCode) {
		fmt.Printf("Warning: status %d is not declared for this operation (declared: %s)\n",
			response.StatusCode, strings.Join(declared, ", "))
	}

	fmt.Println("\n--- Headers ---")
	for name, values := range response.Header {
		for _, v := range values {
			fmt.Printf("  %s: %s\n", name, v)
		}
	}

	fmt.Println("\n--- Body ---")
	fmt.Println(body)

	fmt.Println("\n--- Tests ---")
	passed := runner.check(req, ref, response, body, elapsed, source, scriptOpts, "  ")
	fmt.Printf("\n%d passed, %d failed\n", runner.pass, runner.fail)
	if err := runner.finish(); err != nil {
		return err
	}
	var testFailure error
	if !passed {
		testFailure = fmt.Errorf("request checks failed: %d failed, %d script errors", runner.fail, runner.scriptErrors)
	}

	if *saveExample {
		if err := store.SaveResponseExample(spec, ref, response.StatusCode, response.Header, body); err != nil {
			return fmt.Errorf("error saving example: %w", err)
		}
		if err := store.SaveSpec(file, spec); err != nil {
			return fmt.Errorf("error writing spec: %w", err)
		}
		fmt.Println("\n✓ Example saved to spec")
	}

	return testFailure
}
