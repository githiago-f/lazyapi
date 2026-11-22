package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/githiago-f/lazyapi/internal/report"
	"github.com/githiago-f/lazyapi/internal/runner"
)

const runUsage = `usage: lazyapi run <spec.yml> [flags]

Selection:
  --methods list      methods to run (default GET,HEAD,OPTIONS; "all" for every method)
  --tag list          only operations with one of these tags
  --op list           only these operations (operationId or "METHOD /path")
  --grep regex        only operations whose "METHOD /path operationId summary" matches

Execution:
  --server url|index  --env file  --param k=v  --query k=v  --header k=v  --timeout 30s
  --parallel N        run N operations concurrently (env/vars are isolated per operation)
  --retries N         retry network errors and 5xx responses N times
  --bail              stop after the first failed or errored operation
  --dry-run           show what would be sent

Checks:
  --script file       test script for every operation (default: each x-lazyapi-tests)
  --no-contract       do not validate responses against the OpenAPI schema
  --script-timeout d  --offline  --reload  --frozen-lockfile  --allow-import hosts

Output:
  --output pretty|json|junit|tap   --output-file path   --bodies failed|all|none

Exit status: 0 all passed, 1 a check failed, 2 usage error, 3 something could not run.`

// RunTests implements `lazyapi run` (and `lazyapi smoke tests`).
func RunTests(args []string) error {
	fs := newFlagSet("run")
	f := registerRunFlags(fs, "test script applied to every operation (overrides x-lazyapi-tests)")
	methods := fs.String("methods", "", "methods to run (default GET,HEAD,OPTIONS; 'all' for every method)")
	tags := fs.String("tag", "", "only operations with one of these tags (comma-separated)")
	ops := fs.String("op", "", "only these operations: operationId or \"METHOD /path\" (comma-separated)")
	grep := fs.String("grep", "", "only operations matching this regular expression")
	parallel := fs.Int("parallel", 1, "operations to run concurrently")
	retries := fs.Int("retries", 0, "retries for network errors and 5xx responses")
	bail := fs.Bool("bail", false, "stop after the first failure")

	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return usageErr("%v\n%s", err, runUsage)
	}
	if len(pos) != 1 {
		return usageErr("%s", runUsage)
	}
	if err := f.validate(); err != nil {
		return err
	}
	if *parallel < 1 || *retries < 0 {
		return usageErr("--parallel must be >= 1 and --retries >= 0")
	}
	re, err := compileGrep(*grep)
	if err != nil {
		return err
	}

	file := pos[0]
	spec, err := loadSpec(file)
	if err != nil {
		return err
	}
	opts, err := f.options(file)
	if err != nil {
		return err
	}
	opts.Filter = runner.Filter{
		Methods: runner.ParseMethods(*methods),
		Tags:    splitList(*tags),
		Ops:     splitList(*ops),
		Grep:    re,
	}
	opts.Parallel, opts.Retries, opts.Bail = *parallel, *retries, *bail
	if f.prettyToStdout() {
		opts.OnResult = func(r runner.OpResult) { report.WriteOp(os.Stdout, r) }
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	rep, lockErr := runner.Run(ctx, spec, file, opts)
	if len(rep.Operations) == 0 && f.prettyToStdout() {
		fmt.Println("No operations matched")
	}
	if err := f.writeReport(rep); err != nil {
		return err
	}
	if lockErr != nil {
		return runtimeErr(lockErr)
	}
	return reportError(rep)
}

// SmokeTests is the historical name of RunTests.
func SmokeTests(args []string) error { return RunTests(args) }
