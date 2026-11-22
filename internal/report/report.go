// Package report renders runner results as human-readable text, JSON,
// JUnit XML or TAP.
package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/githiago-f/lazyapi/internal/runner"
)

// Formats accepted by --output.
const (
	Pretty = "pretty"
	JSON   = "json"
	JUnit  = "junit"
	TAP    = "tap"
)

// Formats lists every supported output format.
var Formats = []string{Pretty, JSON, JUnit, TAP}

// Bodies selects which HTTP bodies are included in JSON output.
type Bodies string

const (
	BodiesFailed Bodies = "failed" // only for operations that did not pass (default)
	BodiesAll    Bodies = "all"
	BodiesNone   Bodies = "none"
)

// MaxBody caps bodies embedded in JSON output.
const MaxBody = 64 << 10

// Write renders a full report in the given format.
func Write(w io.Writer, format string, r runner.Report, bodies Bodies) error {
	switch format {
	case Pretty:
		for _, op := range r.Operations {
			WriteOp(w, op)
		}
		WriteSummary(w, r)
		return nil
	case JSON:
		return writeJSON(w, withBodies(r, bodies))
	case JUnit:
		return writeJUnit(w, r)
	case TAP:
		return writeTAP(w, r)
	default:
		return fmt.Errorf("unknown output format %q (use %s)", format, strings.Join(Formats, ", "))
	}
}

// WriteJSON renders any value as indented JSON.
func WriteJSON(w io.Writer, v any) error { return writeJSON(w, v) }

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// FilterBodies applies the body policy to one result.
func FilterBodies(op runner.OpResult, bodies Bodies) runner.OpResult {
	keep := bodies == BodiesAll || (bodies != BodiesNone && op.Outcome != runner.Passed)
	if op.Request != nil {
		req := *op.Request
		if keep {
			req.Body = truncate(req.Body)
		} else {
			req.Body = ""
		}
		op.Request = &req
	}
	if op.Response != nil {
		resp := *op.Response
		if keep {
			resp.Body = truncate(resp.Body)
		} else {
			resp.Body = ""
		}
		op.Response = &resp
	}
	if len(op.Steps) > 0 {
		steps := make([]runner.Step, len(op.Steps))
		for i, st := range op.Steps {
			if st.Request != nil {
				r := *st.Request
				r.Body = bodyPolicy(r.Body, keep)
				st.Request = &r
			}
			if st.Response != nil {
				r := *st.Response
				r.Body = bodyPolicy(r.Body, keep)
				st.Response = &r
			}
			steps[i] = st
		}
		op.Steps = steps
	}
	return op
}

func bodyPolicy(body string, keep bool) string {
	if keep {
		return truncate(body)
	}
	return ""
}

func withBodies(r runner.Report, bodies Bodies) runner.Report {
	ops := make([]runner.OpResult, len(r.Operations))
	for i, op := range r.Operations {
		ops[i] = FilterBodies(op, bodies)
	}
	r.Operations = ops
	return r
}

func truncate(s string) string {
	if len(s) <= MaxBody {
		return s
	}
	return s[:MaxBody] + fmt.Sprintf("\n…[truncated %d bytes]", len(s)-MaxBody)
}

// ---------------------------------------------------------------- pretty

// WriteOp prints one operation in the human-readable format.
func WriteOp(w io.Writer, op runner.OpResult) {
	_, _ = io.WriteString(w, FormatOp(op))
}

// FormatOp renders one operation in the human-readable format.
func FormatOp(op runner.OpResult) string {
	var b strings.Builder
	switch op.Outcome {
	case runner.Skipped:
		fmt.Fprintf(&b, "SKIP  %s — %s\n", op.Label(), op.Reason)
		return b.String()
	case runner.Errored:
		fmt.Fprintf(&b, "ERR   %s — %s\n\n", op.Label(), op.Reason)
		return b.String()
	case runner.Planned:
		fmt.Fprintf(&b, "DRY   %s %s\n", op.Method, op.Request.URL)
		return b.String()
	}

	status := ""
	switch {
	case op.Response != nil:
		status = fmt.Sprintf(" — %s (%dms)", op.Response.StatusText, op.Response.DurationMs)
	case op.Kind == runner.KindFlow:
		status = fmt.Sprintf(" — %d requests", len(op.Steps))
	}
	attempts := ""
	if op.Attempts > 1 {
		attempts = fmt.Sprintf(" [%d attempts]", op.Attempts)
	}
	fmt.Fprintf(&b, "%s %s%s%s\n", strings.ToUpper(string(op.Outcome[:1]))+string(op.Outcome[1:]), op.Label(), status, attempts)
	for i, st := range op.Steps {
		if st.Error != "" {
			fmt.Fprintf(&b, "  → #%d %s %s — %s\n", i+1, st.Method, st.Path, st.Error)
			continue
		}
		fmt.Fprintf(&b, "  → #%d %s %s — %d (%dms)\n", i+1, st.Method, st.Path, st.Status, st.DurationMs)
	}
	for _, l := range op.Logs {
		fmt.Fprintf(&b, "  [%s] %s\n", l.Level, l.Message)
	}
	for _, c := range op.Checks {
		mark := "PASS"
		if !c.Passed {
			mark = "FAIL"
		}
		name := c.Name
		if c.Kind != runner.CheckTest && op.Kind != runner.KindFlow {
			name = c.Kind + ": " + name
		}
		fmt.Fprintf(&b, "  [%s] %s\n", mark, name)
		if c.Message != "" {
			fmt.Fprintf(&b, "         %s\n", c.Message)
		}
	}
	if op.TestsFile == "" {
		fmt.Fprintln(&b, "  (no test script — add one with x-lazyapi-tests or --script)")
	}
	fmt.Fprintln(&b)
	return b.String()
}

// WriteSummary prints the totals line.
func WriteSummary(w io.Writer, r runner.Report) {
	var b strings.Builder
	s := r.Summary
	fmt.Fprintln(&b, "========================================")
	fmt.Fprintf(&b, "Operations: %d passed, %d failed, %d errors, %d skipped", s.Passed, s.Failed, s.Errors, s.Skipped)
	if s.Planned > 0 {
		fmt.Fprintf(&b, ", %d planned", s.Planned)
	}
	fmt.Fprintf(&b, " (%dms)\n", s.DurationMs)
	fmt.Fprintf(&b, "Checks:     %d passed, %d failed\n", s.ChecksPassed, s.ChecksFailed)
	_, _ = io.WriteString(w, b.String())
}

// ---------------------------------------------------------------- junit

type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Errors   int          `xml:"errors,attr"`
	Skipped  int          `xml:"skipped,attr"`
	Time     string       `xml:"time,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Errors   int         `xml:"errors,attr"`
	Skipped  int         `xml:"skipped,attr"`
	Time     string      `xml:"time,attr"`
	Cases    []junitCase `xml:"testcase"`
	Output   string      `xml:"system-out,omitempty"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitMessage `xml:"failure,omitempty"`
	Error     *junitMessage `xml:"error,omitempty"`
	Skipped   *junitMessage `xml:"skipped,omitempty"`
}

type junitMessage struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

func seconds(ms int64) string { return fmt.Sprintf("%.3f", float64(ms)/1000) }

// writeJUnit emits one <testsuite> per operation and one <testcase> per check.
func writeJUnit(w io.Writer, r runner.Report) error {
	root := junitSuites{Name: "lazyapi " + r.Spec, Time: seconds(r.Summary.DurationMs)}
	for _, op := range r.Operations {
		suite := junitSuite{Name: op.Label()}
		var ms int64
		if op.Response != nil {
			ms = op.Response.DurationMs
		}
		suite.Time = seconds(ms)
		class := op.OperationID
		if class == "" {
			class = op.Label()
		}

		switch op.Outcome {
		case runner.Skipped, runner.Planned:
			suite.Cases = append(suite.Cases, junitCase{Name: "request", Classname: class, Time: "0",
				Skipped: &junitMessage{Message: op.Reason}})
			suite.Skipped++
		case runner.Errored:
			suite.Cases = append(suite.Cases, junitCase{Name: "request", Classname: class, Time: "0",
				Error: &junitMessage{Message: op.Reason, Body: op.Reason}})
			suite.Errors++
		}
		for _, c := range op.Checks {
			tc := junitCase{Name: c.Kind + ": " + c.Name, Classname: class, Time: "0"}
			if !c.Passed {
				tc.Failure = &junitMessage{Message: c.Message, Body: c.Message}
				suite.Failures++
			}
			suite.Cases = append(suite.Cases, tc)
		}
		suite.Tests = len(suite.Cases)
		var out []string
		for _, l := range op.Logs {
			out = append(out, "["+l.Level+"] "+l.Message)
		}
		suite.Output = strings.Join(out, "\n")

		root.Tests += suite.Tests
		root.Failures += suite.Failures
		root.Errors += suite.Errors
		root.Skipped += suite.Skipped
		root.Suites = append(root.Suites, suite)
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(root); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// ---------------------------------------------------------------- tap

// writeTAP emits TAP version 14, one test point per check.
func writeTAP(w io.Writer, r runner.Report) error {
	var lines []string
	n := 0
	point := func(ok bool, desc, directive string, diag map[string]string) {
		n++
		status := "ok"
		if !ok {
			status = "not ok"
		}
		line := fmt.Sprintf("%s %d - %s", status, n, tapEscape(desc))
		if directive != "" {
			line += " # " + directive
		}
		lines = append(lines, line)
		if len(diag) > 0 {
			lines = append(lines, "  ---")
			keys := make([]string, 0, len(diag))
			for k := range diag {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				v, _ := json.Marshal(diag[k])
				lines = append(lines, fmt.Sprintf("  %s: %s", k, v))
			}
			lines = append(lines, "  ...")
		}
	}

	for _, op := range r.Operations {
		switch op.Outcome {
		case runner.Skipped, runner.Planned:
			point(true, op.Label(), "SKIP "+op.Reason, nil)
			continue
		case runner.Errored:
			point(false, op.Label()+" › request", "", map[string]string{"message": op.Reason})
			continue
		}
		for _, c := range op.Checks {
			var diag map[string]string
			if !c.Passed {
				diag = map[string]string{"message": c.Message, "kind": c.Kind}
			}
			point(c.Passed, op.Label()+" › "+c.Name, "", diag)
		}
	}
	out := "TAP version 14\n" + strings.Join(lines, "\n")
	if len(lines) > 0 {
		out += "\n"
	}
	out += fmt.Sprintf("1..%d\n", n)
	_, err := io.WriteString(w, out)
	return err
}

func tapEscape(s string) string {
	return strings.NewReplacer("#", `\#`, "\n", " ").Replace(s)
}
