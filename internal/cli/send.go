package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/report"
	"github.com/githiago-f/lazyapi/internal/runner"
	"github.com/githiago-f/lazyapi/internal/script"
	"github.com/githiago-f/lazyapi/internal/store"
)

const sendUsage = `usage: lazyapi send request <spec.yml> (<path> <method> | --op <operationId>) [flags]

  --op id             operation to send (instead of <path> <method>)
  --param k=v  --query k=v  --header k=v   (repeatable)
  --body value|@file  request body; @file reads it from a file, @- from stdin
  --server url|index  --env file  --timeout 30s
  --script file       test script (default: the operation's x-lazyapi-tests)
  --no-contract  --script-timeout d  --offline  --reload  --frozen-lockfile  --allow-import hosts
  --output pretty|json|junit|tap  --output-file path  --bodies failed|all|none (default all)
  --save-example      save the response as an example in the spec
  --dry-run           show the request without sending it

Exit status: 0 passed, 1 a check failed, 2 usage error, 3 the request could not be sent.`

func SendRequest(args []string) error {
	fs := newFlagSet("send request")
	f := registerRunFlags(fs, "test script (overrides x-lazyapi-tests)")
	opFlag := fs.String("op", "", "operationId (or \"METHOD /path\") to send")
	body := fs.String("body", "", "request body, @file or @- for stdin")
	saveExample := fs.Bool("save-example", false, "save the response as an example in the spec")

	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return usageErr("%v\n%s", err, sendUsage)
	}
	if !flagSet(fs, "bodies") {
		*f.bodies = string(report.BodiesAll) // a single request: show what came back
	}
	if err := f.validate(); err != nil {
		return err
	}

	var file, target string
	switch {
	case len(pos) == 1 && *opFlag != "":
		file, target = pos[0], *opFlag
	case len(pos) == 3 && *opFlag == "":
		file, target = pos[0], strings.ToUpper(pos[2])+" "+pos[1]
	default:
		return usageErr("%s", sendUsage)
	}

	spec, err := loadSpec(file)
	if err != nil {
		return err
	}
	ref, err := script.ResolveTarget(spec, file, target)
	if err != nil {
		return runtimeErr(err)
	}
	opts, err := f.options(file)
	if err != nil {
		return err
	}
	if flagSet(fs, "body") {
		raw, err := readBody(*body)
		if err != nil {
			return usageErr("--body: %v", err)
		}
		opts.Body = &raw
	}

	res, lockErr := runner.RunOne(context.Background(), spec, file, ref, opts)

	if f.prettyToStdout() {
		printSendResult(res)
	}
	if err := writeSendReport(f, file, res); err != nil {
		return err
	}
	if lockErr != nil {
		return runtimeErr(lockErr)
	}

	if *saveExample && res.Response != nil {
		if err := saveResponseExample(file, ref, res.Response); err != nil {
			return runtimeErr(err)
		}
		if f.prettyToStdout() {
			fmt.Println("\n✓ Example saved to spec")
		}
	}

	switch res.Outcome {
	case runner.Errored:
		return &ExitError{Code: runner.ExitErrored, Err: fmt.Errorf("%s: %s", res.Label(), res.Reason)}
	case runner.Failed:
		failed := 0
		for _, c := range res.Checks {
			if !c.Passed {
				failed++
			}
		}
		return &ExitError{Code: runner.ExitFailed, Err: fmt.Errorf("request checks failed: %d failed", failed)}
	}
	return nil
}

// writeSendReport emits the machine-readable output of a single request.
// JSON on stdout is just the operation result; other cases use a report.
func writeSendReport(f *runFlags, file string, res runner.OpResult) error {
	if *f.output == report.Pretty {
		return nil
	}
	if *f.output == report.JSON && *f.outputFile == "" {
		if err := report.WriteJSON(os.Stdout, report.FilterBodies(res, report.Bodies(*f.bodies))); err != nil {
			return runtimeErr(err)
		}
		return nil
	}
	rep := runner.Report{Spec: file, Operations: []runner.OpResult{res}}
	rep.Summary = runner.Summarize(rep.Operations, 0)
	if res.Response != nil {
		rep.Summary.DurationMs = res.Response.DurationMs
	}
	if *f.outputFile != "" {
		return f.writeReport(rep)
	}
	if err := report.Write(os.Stdout, *f.output, rep, report.Bodies(*f.bodies)); err != nil {
		return runtimeErr(err)
	}
	return nil
}

func flagSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(fl *flag.Flag) { found = found || fl.Name == name })
	return found
}

func readBody(v string) (string, error) {
	switch {
	case v == "@-":
		data, err := io.ReadAll(os.Stdin)
		return string(data), err
	case strings.HasPrefix(v, "@"):
		data, err := os.ReadFile(v[1:])
		return string(data), err
	default:
		return v, nil
	}
}

func printSendResult(res runner.OpResult) {
	if res.Request != nil {
		fmt.Printf("%s %s\n", res.Request.Method, res.Request.URL)
	}
	switch res.Outcome {
	case runner.Errored, runner.Skipped:
		fmt.Printf("Error: %s\n", res.Reason)
		return
	case runner.Planned:
		fmt.Println("(dry run — not sent)")
		return
	}
	r := res.Response
	fmt.Printf("Status: %s (%dms)\n", r.StatusText, r.DurationMs)
	fmt.Println("\n--- Headers ---")
	names := make([]string, 0, len(r.Headers))
	for k := range r.Headers {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Printf("  %s: %s\n", k, r.Headers[k])
	}
	fmt.Println("\n--- Body ---")
	fmt.Println(r.Body)
	fmt.Println("\n--- Checks ---")
	for _, l := range res.Logs {
		fmt.Printf("  [%s] %s\n", l.Level, l.Message)
	}
	passed, failed := 0, 0
	for _, c := range res.Checks {
		mark := "PASS"
		if c.Passed {
			passed++
		} else {
			failed++
			mark = "FAIL"
		}
		name := c.Name
		if c.Kind != runner.CheckTest {
			name = c.Kind + ": " + name
		}
		fmt.Printf("  [%s] %s\n", mark, name)
		if c.Message != "" {
			fmt.Printf("         %s\n", c.Message)
		}
	}
	fmt.Printf("\n%d passed, %d failed\n", passed, failed)
}

func saveResponseExample(file string, ref model.OpenAPIRef, r *runner.ResponseInfo) error {
	spec, err := store.ParseSpec(file)
	if err != nil {
		return err
	}
	header := http.Header{}
	for k, v := range r.Headers {
		header.Set(k, v)
	}
	if err := store.SaveResponseExample(spec, ref, r.Status, header, r.Body); err != nil {
		return fmt.Errorf("error saving example: %w", err)
	}
	return store.SaveSpec(file, spec)
}
