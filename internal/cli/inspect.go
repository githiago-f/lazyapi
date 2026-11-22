package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/githiago-f/lazyapi/internal/inspect"
	"github.com/githiago-f/lazyapi/internal/report"
	"github.com/githiago-f/lazyapi/internal/runner"
)

const listUsage = "usage: lazyapi list ops <spec.yml> [--tag t1,t2] [--json]"

// ListOps implements `lazyapi list ops`.
func ListOps(args []string) error {
	fs := newFlagSet("list ops")
	tags := fs.String("tag", "", "only operations with one of these tags")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := parseInterleaved(fs, args)
	if err != nil || len(pos) != 1 {
		return usageErr("%s", listUsage)
	}
	spec, err := loadSpec(pos[0])
	if err != nil {
		return err
	}
	ops := inspect.ListOperations(spec, pos[0], splitList(*tags))
	if *asJSON {
		if ops == nil {
			ops = []inspect.OpSummary{}
		}
		return report.WriteJSON(os.Stdout, ops)
	}
	for _, op := range ops {
		id := op.OperationID
		if id == "" {
			id = "-"
		}
		tests := ""
		if op.TestsFile != "" {
			tests = "  [tests: " + op.TestsFile + "]"
		}
		fmt.Printf("%-7s %-40s %-24s %s%s\n", op.Method, op.Path, id, op.Summary, tests)
	}
	return nil
}

const describeUsage = `usage: lazyapi describe <spec.yml> <operationId | "METHOD /path"> [--json]`

// Describe implements `lazyapi describe`.
func Describe(args []string) error {
	fs := newFlagSet("describe")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := parseInterleaved(fs, args)
	if err != nil || len(pos) < 2 {
		return usageErr("%s", describeUsage)
	}
	spec, err := loadSpec(pos[0])
	if err != nil {
		return err
	}
	d, err := inspect.Describe(spec, pos[0], strings.Join(pos[1:], " "))
	if err != nil {
		return runtimeErr(err)
	}
	if *asJSON {
		return report.WriteJSON(os.Stdout, d)
	}
	printDetail(d)
	return nil
}

func printDetail(d inspect.OpDetail) {
	fmt.Printf("%s %s", d.Method, d.Path)
	if d.OperationID != "" {
		fmt.Printf("  (%s)", d.OperationID)
	}
	fmt.Println()
	if d.Summary != "" {
		fmt.Println(d.Summary)
	}
	if d.Description != "" {
		fmt.Println(d.Description)
	}
	if len(d.Tags) > 0 {
		fmt.Println("Tags:", strings.Join(d.Tags, ", "))
	}
	if len(d.Security) > 0 {
		fmt.Println("Security:", strings.Join(d.Security, ", "))
	}
	if len(d.Parameters) > 0 {
		fmt.Println("\nParameters:")
		for _, p := range d.Parameters {
			req := ""
			if p.Required {
				req = " (required)"
			}
			ex := ""
			if p.Example != "" {
				ex = "  example: " + p.Example
			}
			fmt.Printf("  %-8s %s%s  %s%s\n", p.In, p.Name, req, string(p.Schema), ex)
		}
	}
	if d.RequestBody != nil {
		fmt.Println("\nRequest body:")
		for _, c := range d.RequestBody.Content {
			fmt.Printf("  %s  %s\n", c.MediaType, string(c.Schema))
		}
	}
	fmt.Println("\nResponses:")
	for _, r := range d.Responses {
		fmt.Printf("  %s  %s\n", r.Status, r.Description)
		for _, c := range r.Content {
			fmt.Printf("       %s  %s\n", c.MediaType, string(c.Schema))
		}
	}
	if len(d.Schemas) > 0 {
		fmt.Println("\nSchemas:")
		for name, s := range d.Schemas {
			fmt.Printf("  %s: %s\n", name, string(s))
		}
	}
	if d.TestsFile != "" {
		fmt.Printf("\nTests: %s\n", d.TestsFile)
	}
	fmt.Println("\nUse --json for the machine-readable form.")
}

const validateUsage = "usage: lazyapi validate spec <spec.yml> [--json]"

// ValidateSpec implements `lazyapi validate spec`.
func ValidateSpec(args []string) error {
	fs := newFlagSet("validate spec")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := parseInterleaved(fs, args)
	if err != nil || len(pos) != 1 {
		return usageErr("%s", validateUsage)
	}
	v := inspect.Validate(pos[0])
	if *asJSON {
		if err := report.WriteJSON(os.Stdout, v); err != nil {
			return runtimeErr(err)
		}
	} else {
		for _, i := range v.Issues {
			where := ""
			if i.Operation != "" {
				where = i.Operation + ": "
			}
			fmt.Printf("%-7s %s%s\n", strings.ToUpper(i.Severity), where, i.Message)
		}
		if v.Valid {
			fmt.Printf("✓ %s is valid (%d warnings)\n", v.Spec, len(v.Issues))
		}
	}
	if !v.Valid {
		return &ExitError{Code: runner.ExitFailed, Err: fmt.Errorf("%s has errors", v.Spec)}
	}
	return nil
}
