package permit

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func req(method, url string) *http.Request {
	r, _ := http.NewRequest(method, url, nil)
	return r
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"https://API.example.com/v1?x=1": "https://api.example.com:443",
		"http://example.com":             "http://example.com:80",
		"api.example.com:8443":           "https://api.example.com:8443",
		"http://127.0.0.1:4010":          Loopback,
		"http://[::1]:9":                 Loopback,
		"http://app.localhost:3000":      Loopback,
		"localhost":                      Loopback,
	}
	for in, want := range cases {
		if got, err := Normalize(in); err != nil || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := Normalize("http://"); err == nil {
		t.Error("an empty host is not an origin")
	}
}

// asker records the questions and answers with the scripted scopes.
type asker struct {
	answers []Scope
	asked   []Prompt
}

func (a *asker) ask(_ context.Context, p Prompt) (Scope, error) {
	a.asked = append(a.asked, p)
	if len(a.answers) == 0 {
		return Deny, nil
	}
	s := a.answers[0]
	a.answers = a.answers[1:]
	return s, nil
}

func policy(t *testing.T, a *asker) *Policy {
	t.Helper()
	p := &Policy{Store: &Store{Path: filepath.Join(t.TempDir(), "permissions.json")}}
	if a != nil {
		p.Ask = func() Asker { return a.ask }
	}
	return p
}

func TestNoAskerRefusesWithInstructions(t *testing.T) {
	p := policy(t, nil)
	err := p.Calls("send_request").Check(context.Background(), req("DELETE", "https://api.example.com/users/1?token=s3cret"))
	if !IsDenied(err) {
		t.Fatalf("not allowed: %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"lazyapi allow https://api.example.com:443 --writes", "--allow-writes", "Do not run it yourself"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q: %s", want, msg)
		}
	}
	if strings.Contains(msg, "s3cret") {
		t.Errorf("the query (may hold secrets) is shown: %s", msg)
	}
}

func TestScopes(t *testing.T) {
	ctx := context.Background()
	a := &asker{answers: []Scope{Call, Session, Always}}
	p := policy(t, a)

	// "this call": allowed for the rest of the call, asked again in the next.
	c := p.Calls("run_tests")
	for range 3 {
		if err := c.Check(ctx, req("GET", "https://a.example.com/x")); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.asked) != 1 {
		t.Fatalf("asked %d times in one call", len(a.asked))
	}
	if err := p.Calls("run_tests").Check(ctx, req("GET", "https://a.example.com/y")); err != nil {
		t.Fatal(err)
	}
	if len(a.asked) != 2 {
		t.Fatal("a new call asks again after a one-call answer")
	}

	// The session answer covers later calls, not writes.
	for range 2 {
		if err := p.Calls("send_request").Check(ctx, req("GET", "https://a.example.com/z")); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.asked) != 2 {
		t.Fatal("a session answer lasts across calls")
	}
	if err := p.Calls("send_request").Check(ctx, req("POST", "https://a.example.com/z")); err != nil {
		t.Fatal(err)
	}
	if len(a.asked) != 3 || !a.asked[2].Write {
		t.Fatalf("a write is asked separately: %+v", a.asked)
	}

	// "always" is remembered (for writes too here) and survives a restart.
	fresh := &Policy{Store: p.Store}
	if err := fresh.Calls("send_request").Check(ctx, req("PUT", "https://a.example.com/z")); err != nil {
		t.Errorf("remembered grant: %v", err)
	}
	if info, err := os.Stat(p.Store.Path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("store file mode: %v %v", info, err)
	}
}

func TestRefusalIsNotAskedAgainInTheCall(t *testing.T) {
	a := &asker{answers: []Scope{Deny}}
	p := policy(t, a)
	c := p.Calls("run_tests")
	for range 5 {
		if err := c.Check(context.Background(), req("GET", "https://b.example.com/")); !IsDenied(err) || !strings.Contains(err.Error(), "did not allow") {
			t.Fatalf("refused: %v", err)
		}
	}
	if len(a.asked) != 1 {
		t.Errorf("asked %d times", len(a.asked))
	}
}

func TestLoopbackIsAskedOnce(t *testing.T) {
	a := &asker{answers: []Scope{Call}} // any yes becomes "always" for local servers
	p := policy(t, a)
	for _, u := range []string{"http://localhost:4010/a", "http://127.0.0.1:8080/b", "http://[::1]:3000/c"} {
		for _, m := range []string{"GET", "DELETE"} {
			if err := p.Calls("send_request").Check(context.Background(), req(m, u)); err != nil {
				t.Fatalf("%s %s: %v", m, u, err)
			}
		}
	}
	if len(a.asked) != 1 || !a.asked[0].Loopback {
		t.Errorf("asked %+v", a.asked)
	}
	if !p.Store.Allowed(Loopback, true) {
		t.Error("the local grant is remembered")
	}
}

func TestCommandLineAllows(t *testing.T) {
	p := policy(t, nil)
	p.Allow = []string{"https://ro.example.com"}
	p.AllowWrites = []string{"rw.example.com"}
	ctx := context.Background()
	if err := p.Calls("").Check(ctx, req("GET", "https://ro.example.com/x")); err != nil {
		t.Error(err)
	}
	if err := p.Calls("").Check(ctx, req("POST", "https://ro.example.com/x")); !IsDenied(err) {
		t.Error("--allow is reads only")
	}
	if err := p.Calls("").Check(ctx, req("DELETE", "https://rw.example.com/x")); err != nil {
		t.Error(err)
	}
	if err := p.Calls("").Check(ctx, req("GET", "http://ro.example.com/x")); !IsDenied(err) {
		t.Error("another scheme is another origin")
	}
}

// The guarded client checks redirects too: an allowed server cannot send
// the agent to one that is not.
func TestClientGuardsRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer target.Close()
	targetURL := strings.Replace(target.URL, "127.0.0.1", "example.invalid", 1)
	start := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetURL, http.StatusFound)
	}))
	defer start.Close()

	p := policy(t, nil)
	p.Allow = []string{Loopback}
	client := p.Calls("send_request").Client(nil)
	_, err := client.Get(start.URL)
	if !IsDenied(err) || !strings.Contains(err.Error(), "example.invalid") {
		t.Fatalf("redirect to an origin nobody allowed: %v", err)
	}
}

// slowAsker answers after delay, or gives up when ctx ends.
func slowAsker(delay time.Duration, answer Scope) Asker {
	return func(ctx context.Context, _ Prompt) (Scope, error) {
		select {
		case <-time.After(delay):
			return answer, nil
		case <-ctx.Done():
			return Deny, ctx.Err()
		}
	}
}

// The user's thinking time does not count against the request's timeout
// (http.Client.Timeout or a context deadline, like Request.Timeout).
func TestQuestionDoesNotUseTheRequestTimeout(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer api.Close()

	p := &Policy{Ask: func() Asker { return slowAsker(300*time.Millisecond, Call) }}
	client := p.Calls("t").Client(&http.Client{Timeout: 100 * time.Millisecond})
	resp, err := client.Get(api.URL)
	if err != nil {
		t.Fatalf("Client.Timeout: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || string(body) != "ok" {
		t.Fatalf("body %q, %v", body, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, http.MethodGet, api.URL, nil)
	resp, err = p.Calls("t").Client(nil).Do(r)
	if err != nil {
		t.Fatalf("context deadline: %v", err)
	}
	_ = resp.Body.Close()
}

// A question still ends with the tool call, and has its own limit.
func TestQuestionEnds(t *testing.T) {
	p := &Policy{Ask: func() Asker { return slowAsker(time.Hour, Call) }}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	if err := p.Calls("t").Check(ctx, req("GET", "https://api.example.com/")); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled call: %v", err)
	}

	p.AskTimeout = 50 * time.Millisecond
	if err := p.Calls("t").Check(context.Background(), req("GET", "https://api.example.com/")); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("AskTimeout: %v", err)
	}
}

// The moved deadline still applies to a slow server.
func TestPausedRequestStillTimesOut(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer api.Close()
	p := &Policy{Ask: func() Asker { return slowAsker(100*time.Millisecond, Call) }}
	start := time.Now()
	_, err := p.Calls("t").Client(&http.Client{Timeout: 200 * time.Millisecond}).Get(api.URL)
	if err == nil || time.Since(start) > time.Second {
		t.Errorf("err %v after %v", err, time.Since(start))
	}
}
