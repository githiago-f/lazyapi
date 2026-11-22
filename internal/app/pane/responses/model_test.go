package responses

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/runner"
	"github.com/githiago-f/lazyapi/internal/script"
)

func press(s string) tea.KeyPressMsg {
	if len(s) == 1 {
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	panic(s)
}

func sample(outcome runner.Outcome) runner.OpResult {
	return runner.OpResult{
		Method: "GET", Path: "/users/1", Outcome: outcome,
		Request: &runner.RequestInfo{Method: "GET", URL: "https://api/users/1", Headers: map[string]string{"Authorization": "<redacted>"}},
		Response: &runner.ResponseInfo{
			Status: 200, StatusText: "200 OK", DurationMs: 12,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    `{"id":1,"html":"<b>x</b>","name":"ana"}`,
			Timing:  &runner.TimingInfo{DNSMs: 1, ConnectMs: 2, TLSMs: 3, TTFBMs: 5, TotalMs: 12},
			Cookies: []runner.CookieInfo{{Name: "session", Value: "abc", Path: "/", HTTPOnly: true}},
		},
		Checks: []runner.Check{
			{Kind: runner.CheckStatus, Name: "status 200 is declared", Passed: true},
			{Kind: runner.CheckContract, Name: "response matches the OpenAPI schema", Passed: outcome == runner.Passed,
				Message: "id: value must be a string"},
		},
		Logs: []script.LogEntry{{Level: "warn", Message: "slow"}},
	}
}

func view(m Model) string { return ansi.Strip(m.View()) }

func TestPrettyKeepsOrderAndDoesNotEscape(t *testing.T) {
	got := Pretty("application/json", `{"b":1,"a":"<x>"}`)
	if got != "{\n  \"b\": 1,\n  \"a\": \"<x>\"\n}" {
		t.Errorf("Pretty = %q", got)
	}
	if Pretty("text/plain", "  hi ") != "  hi " {
		t.Error("text must be untouched")
	}
	if got := Pretty("", `[1,2]`); got != "[\n  1,\n  2\n]" {
		t.Errorf("sniffed JSON = %q", got)
	}
	if !strings.Contains(Highlight("application/json", `{"a":1}`), "\x1b[") {
		t.Error("JSON should be highlighted")
	}
	if Highlight("text/plain", "hi") != "hi" {
		t.Error("plain text should not be highlighted")
	}
	if HumanSize(10) != "10 B" || HumanSize(2048) != "2.0 KB" {
		t.Error("HumanSize")
	}
}

func TestModelTabsAndStatus(t *testing.T) {
	m := New("nothing yet")
	m.SetSize(80, 20)
	if !strings.Contains(view(m), "nothing yet") {
		t.Errorf("placeholder: %s", view(m))
	}

	m.SetResult(sample(runner.Passed))
	m.Focus()
	out := view(m)
	for _, want := range []string{"200 OK", "12 ms", "✓ 2 checks", `"html": "<b>x</b>"`} {
		if !strings.Contains(out, want) {
			t.Errorf("body view missing %q:\n%s", want, out)
		}
	}

	m, _ = m.Update(press("]"))
	if out := view(m); !strings.Contains(out, "Authorization: <redacted>") || !strings.Contains(out, "Content-Type: application/json") {
		t.Errorf("headers tab:\n%s", out)
	}
	m, _ = m.Update(press("6"))
	if out := view(m); !strings.Contains(out, "[warn] slow") {
		t.Errorf("console tab:\n%s", out)
	}
	m, _ = m.Update(press("1"))
	m, _ = m.Update(press("r"))
	if out := view(m); !strings.Contains(out, `{"id":1,"html"`) || !strings.Contains(out, "Body (raw)") {
		t.Errorf("raw body:\n%s", out)
	}
}

func TestModelFailureJumpsToChecks(t *testing.T) {
	m := New("")
	m.SetSize(80, 20)
	m.SetResult(sample(runner.Failed))
	out := view(m)
	if !strings.Contains(out, "✗ 1/2 checks failed") || !strings.Contains(out, "id: value must be a string") {
		t.Errorf("failed result should open the Checks tab:\n%s", out)
	}

	m.SetResult(runner.OpResult{Outcome: runner.Errored, Reason: "request failed: connection refused"})
	if out := view(m); !strings.Contains(out, "ERROR") || !strings.Contains(out, "connection refused") {
		t.Errorf("error result:\n%s", out)
	}
}

func TestModelSearch(t *testing.T) {
	m := New("")
	m.SetSize(80, 20)
	m.SetResult(sample(runner.Passed))
	m.Focus()

	m, _ = m.Update(press("/"))
	if !m.Searching() {
		t.Fatal("/ should open the search input")
	}
	for _, r := range "ana" {
		m, _ = m.Update(press(string(r)))
	}
	m, _ = m.Update(press("enter"))
	if m.Searching() || m.hits != 1 || !strings.Contains(view(m), "/ana — 1 match") {
		t.Errorf("search: hits=%d\n%s", m.hits, view(m))
	}

	// Unfocused views ignore keys.
	m.Blur()
	before := m.tab
	m, _ = m.Update(press("]"))
	if m.tab != before {
		t.Error("unfocused view must ignore keys")
	}
}

func TestModelCookiesAndTimeline(t *testing.T) {
	m := New("")
	m.SetSize(100, 20)
	m.SetResult(sample(runner.Passed))
	m.Focus()
	m, _ = m.Update(press("3"))
	if out := view(m); !strings.Contains(out, "session= abc") || !strings.Contains(out, "HttpOnly") || !strings.Contains(out, "Cookies 1") {
		t.Errorf("cookies tab:\n%s", out)
	}
	m, _ = m.Update(press("4"))
	out := view(m)
	for _, want := range []string{"DNS lookup", "TLS handshake", "3.00 ms", "Waiting (TTFB)", "Total", "12.00 ms", "█"} {
		if !strings.Contains(out, want) {
			t.Errorf("timeline missing %q:\n%s", want, out)
		}
	}
}

func TestModelJQFilter(t *testing.T) {
	m := New("")
	m.SetSize(100, 20)
	m.SetResult(sample(runner.Passed))
	m.Focus()

	m, _ = m.Update(press("f"))
	if !m.Searching() {
		t.Fatal("f should open the jq input (and capture keys)")
	}
	for _, r := range ".name" {
		m, _ = m.Update(press(string(r)))
	}
	m, _ = m.Update(press("enter"))
	out := view(m)
	if m.Filter() != ".name" || !strings.Contains(out, `"ana"`) || strings.Contains(out, `"id"`) {
		t.Errorf("filtered body:\n%s", out)
	}

	// The filter is kept for the next response of the same request.
	m.SetResult(sample(runner.Passed))
	if !strings.Contains(view(m), "jq .name") {
		t.Error("filter should stay applied")
	}

	m.SetFilter(".[")
	if out := view(m); !strings.Contains(out, "jq:") || !strings.Contains(out, "error") {
		t.Errorf("invalid filter must show the error:\n%s", out)
	}
	m.SetFilter("")
	if strings.Contains(view(m), "jq ") {
		t.Error("empty filter clears it")
	}
}

func TestRawOffsets(t *testing.T) {
	red := "\x1b[31m"
	reset := "\x1b[0m"
	raw := `{"` + red + `name` + reset + `": "` + red + `ana` + reset + `", "x": "anA"}`
	plain := ansi.Strip(raw)
	re := regexp.MustCompile("(?i)ana")
	got := rawOffsets(raw, re.FindAllStringIndex(plain, -1))
	if len(got) != 2 {
		t.Fatalf("matches = %v", got)
	}
	for _, m := range got {
		if s := ansi.Strip(raw[m[0]:m[1]]); !strings.EqualFold(s, "ana") {
			t.Errorf("raw[%d:%d] = %q, want ana", m[0], m[1], raw[m[0]:m[1]])
		}
	}
	// A query never matches inside an escape sequence.
	if got := rawOffsets(raw, regexp.MustCompile("31").FindAllStringIndex(plain, -1)); len(got) != 0 {
		t.Errorf("matched inside an escape code: %v", got)
	}
	// Adjacent matches and a match at the very end.
	raw2 := red + "abab" + reset
	got = rawOffsets(raw2, regexp.MustCompile("ab").FindAllStringIndex(ansi.Strip(raw2), -1))
	if len(got) != 2 || raw2[got[0][0]:got[0][1]] != "ab" || raw2[got[1][0]:got[1][1]] != "ab" || got[0][1] != got[1][0] {
		t.Errorf("adjacent = %v", got)
	}
}

func TestModelSearchKeepsColors(t *testing.T) {
	m := New("")
	m.SetSize(80, 20)
	m.SetResult(sample(runner.Passed))
	m.Focus()
	m, _ = m.Update(press("/"))
	for _, r := range "ana" {
		m, _ = m.Update(press(string(r)))
	}
	m, _ = m.Update(press("enter"))
	if m.hits != 1 {
		t.Fatalf("hits = %d", m.hits)
	}
	if !strings.Contains(m.vp.GetContent(), "\x1b[") {
		t.Error("the body must stay syntax-highlighted while searching")
	}
}
