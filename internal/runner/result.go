// Package runner executes OpenAPI operations and their checks (declared
// status, response contract, test script) and returns structured results.
// It does no printing: the CLI reporters and the MCP server render Reports.
package runner

import (
	"time"

	"github.com/githiago-f/lazyapi/internal/script"
)

// Outcome is the final state of one operation.
type Outcome string

const (
	Passed  Outcome = "passed"  // every check passed
	Failed  Outcome = "failed"  // the request ran, at least one check failed
	Errored Outcome = "error"   // the request could not be executed (network, config)
	Skipped Outcome = "skipped" // not run (filtered method, missing params, --bail)
	Planned Outcome = "planned" // --dry-run: would have been sent
)

// Check kinds.
const (
	CheckRequest  = "request"  // the request sent matches the spec (params, body)
	CheckStatus   = "status"   // status code declared in the spec
	CheckContract = "contract" // response matches the OpenAPI schema
	CheckTest     = "test"     // a test() in the script
	CheckScript   = "script"   // the script failed to load, compile or run
)

// Check is one assertion about an operation's response.
type Check struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// RequestInfo describes what was sent. Sensitive header values are redacted.
type RequestInfo struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// ResponseInfo describes what came back.
type ResponseInfo struct {
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	DurationMs int64             `json:"durationMs"`
}

// OpResult is the result of one operation.
type OpResult struct {
	OperationID string            `json:"operationId,omitempty"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Summary     string            `json:"summary,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Outcome     Outcome           `json:"outcome"`
	Reason      string            `json:"reason,omitempty"` // why it was skipped or errored
	Attempts    int               `json:"attempts,omitempty"`
	TestsFile   string            `json:"testsFile,omitempty"`
	Request     *RequestInfo      `json:"request,omitempty"`
	Response    *ResponseInfo     `json:"response,omitempty"`
	Checks      []Check           `json:"checks,omitempty"`
	Logs        []script.LogEntry `json:"logs,omitempty"`
}

// Label is "METHOD /path".
func (r OpResult) Label() string { return r.Method + " " + r.Path }

// Summary aggregates a run.
type Summary struct {
	Operations   int   `json:"operations"`
	Passed       int   `json:"passed"`
	Failed       int   `json:"failed"`
	Errors       int   `json:"errors"`
	Skipped      int   `json:"skipped"`
	Planned      int   `json:"planned,omitempty"`
	ChecksPassed int   `json:"checksPassed"`
	ChecksFailed int   `json:"checksFailed"`
	DurationMs   int64 `json:"durationMs"`
}

// Report is the result of a run.
type Report struct {
	Spec       string     `json:"spec"`
	StartedAt  time.Time  `json:"startedAt"`
	Summary    Summary    `json:"summary"`
	Operations []OpResult `json:"operations"`
}

// Exit codes shared by every command.
const (
	ExitOK      = 0 // everything passed
	ExitFailed  = 1 // a check failed
	ExitUsage   = 2 // invalid command line
	ExitErrored = 3 // something could not run (spec, network, config)
)

// ExitCode maps a report to a process exit status: errors win over failures.
func (r Report) ExitCode() int {
	switch {
	case r.Summary.Errors > 0:
		return ExitErrored
	case r.Summary.Failed > 0:
		return ExitFailed
	default:
		return ExitOK
	}
}

// Summarize aggregates results into a Summary.
func Summarize(ops []OpResult, elapsed time.Duration) Summary {
	s := Summary{Operations: len(ops), DurationMs: elapsed.Milliseconds()}
	for _, op := range ops {
		switch op.Outcome {
		case Passed:
			s.Passed++
		case Failed:
			s.Failed++
		case Errored:
			s.Errors++
		case Skipped:
			s.Skipped++
		case Planned:
			s.Planned++
		}
		for _, c := range op.Checks {
			if c.Passed {
				s.ChecksPassed++
			} else {
				s.ChecksFailed++
			}
		}
	}
	return s
}

// outcomeFor derives the outcome of an executed operation from its checks.
func outcomeFor(checks []Check) Outcome {
	for _, c := range checks {
		if !c.Passed {
			return Failed
		}
	}
	return Passed
}
