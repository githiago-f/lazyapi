// Package permit decides which servers an agent may send requests to.
//
// Every request the MCP server sends (operations, lazyapi.send in scripts
// and flows, OAuth token calls, redirects) goes through a Policy. A request
// to an origin (scheme://host:port) nobody allowed is held while the user
// is asked; without a way to ask, it is refused with a message telling the
// agent how the user can allow it. Grants:
//
//   - loopback (localhost, 127.0.0.0/8, ::1): asked once, then remembered
//     for every local port;
//   - other origins: for the tool call, the session or always; reads
//     (GET, HEAD, OPTIONS) and writes are granted separately, so allowing
//     a GET never allows a DELETE.
//
// Remembered grants live in the user's config directory, never in the
// project: a repository cannot approve hosts for whoever opens it.
package permit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Loopback is the grant name for every local server.
const Loopback = "localhost"

// Scope is how long an answer lasts.
type Scope string

const (
	Deny    Scope = "deny"
	Call    Scope = "call"    // this tool call
	Session Scope = "session" // until the MCP server stops
	Always  Scope = "always"  // remembered in the user's config
)

// Prompt is what the user is asked about.
type Prompt struct {
	Origin   string // scheme://host:port, or Loopback
	Method   string
	URL      string
	Write    bool   // the method changes data
	Loopback bool   // a local server: one answer covers every port
	Tool     string // the MCP tool that sends it
}

// Asker asks the user; it returns Deny when they refuse or cannot answer.
type Asker func(ctx context.Context, p Prompt) (Scope, error)

// DeniedError is returned for a request that is not allowed.
type DeniedError struct {
	Prompt Prompt
	Asked  bool // the user was asked and refused
}

func (e *DeniedError) Error() string {
	p := e.Prompt
	what := "requests"
	if p.Write {
		what = "requests that change data (" + p.Method + ")"
	}
	if e.Asked {
		return fmt.Sprintf("lazyapi blocked %s %s: the user did not allow %s to %s. Do not retry it unless the user asks you to.", p.Method, p.URL, what, p.Origin)
	}
	cmd := "lazyapi allow " + p.Origin
	if p.Write && !p.Loopback {
		cmd += " --writes"
	}
	return fmt.Sprintf("lazyapi blocked %s %s: %s to %s are not allowed yet, and this MCP client cannot ask the user. "+
		"Ask the user whether lazyapi may call %s; they allow it by running `%s` in a terminal "+
		"(or by adding %s %s to the lazyapi mcp command). Do not run it yourself.",
		p.Method, p.URL, what, p.Origin, p.Origin, cmd, flagFor(p), p.Origin)
}

// Policy holds the grants of one MCP server.
type Policy struct {
	// Store keeps the "always" grants (nil: nothing is remembered).
	Store *Store
	// Allow are origins (or Loopback) allowed from the command line for
	// reads; AllowWrites for reads and writes (Loopback is always both).
	Allow       []string
	AllowWrites []string
	// Ask asks the user (nil: refuse what is not allowed).
	Ask func() Asker
	// AskTimeout bounds one question (0: DefaultAskTimeout). It does not
	// count against the request's own timeout.
	AskTimeout time.Duration

	mu      sync.Mutex
	session map[string]bool // grant key → allowed
}

// Normalize returns the origin of a URL ("https://api.example.com:443"),
// or Loopback for a local server. It accepts a bare host ("localhost:4010").
func Normalize(raw string) (string, error) {
	if strings.EqualFold(raw, Loopback) {
		return Loopback, nil
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("%q is not an origin (e.g. https://api.example.com or localhost)", raw)
	}
	return origin(u), nil
}

func origin(u *url.URL) string {
	if IsLoopback(u.Hostname()) {
		return Loopback
	}
	port := u.Port()
	if port == "" {
		port = "443"
		if u.Scheme == "http" {
			port = "80"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}

// IsLoopback reports whether host is this machine.
func IsLoopback(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isWrite(method string) bool {
	switch strings.ToUpper(method) {
	case "", http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// key names a grant: the origin and, for remote ones, reads or writes.
func key(origin string, write bool) string {
	if origin == Loopback {
		return Loopback
	}
	if write {
		return origin + " write"
	}
	return origin + " read"
}

// Calls returns the checker of one tool call: its "this call" answers
// (and refusals) last as long as it.
func (p *Policy) Calls(tool string) *Checker {
	return &Checker{policy: p, tool: tool, call: map[string]Scope{}}
}

// Checker checks the requests of one tool call.
type Checker struct {
	policy *Policy
	tool   string
	mu     sync.Mutex
	call   map[string]Scope
}

func (p *Policy) allowed(k, origin string, write bool) bool {
	for _, a := range p.AllowWrites {
		if o, err := Normalize(a); err == nil && o == origin {
			return true
		}
	}
	for _, a := range p.Allow {
		if o, err := Normalize(a); err == nil && o == origin && (!write || o == Loopback) {
			return true
		}
	}
	if p.session[k] {
		return true
	}
	if p.Store != nil {
		return p.Store.Allowed(origin, write)
	}
	return false
}

// DefaultAskTimeout is how long a question waits for the user's answer.
const DefaultAskTimeout = 5 * time.Minute

// Check allows req or returns a *DeniedError (asking the user when it can).
// The question ignores ctx's deadline (it has its own, Policy.AskTimeout)
// but ends when ctx is cancelled.
func (c *Checker) Check(ctx context.Context, req *http.Request) error {
	_, err := c.check(ctx, req)
	return err
}

// check is Check; paused reports that it waited, for the user or for
// another request's question, so the caller can give that time back.
func (c *Checker) check(ctx context.Context, req *http.Request) (paused bool, err error) {
	if c == nil || c.policy == nil {
		return false, nil
	}
	p := c.policy
	o := origin(req.URL)
	write := isWrite(req.Method)
	k := key(o, write)
	prompt := Prompt{Origin: o, Method: req.Method, URL: redact(req.URL), Write: write, Loopback: o == Loopback, Tool: c.tool}

	if !c.mu.TryLock() {
		paused = true
		c.mu.Lock()
	}
	defer c.mu.Unlock()
	if answer, ok := c.call[k]; ok {
		if answer == Call {
			return paused, nil
		}
		return paused, &DeniedError{Prompt: prompt, Asked: true} // refused earlier in this call
	}

	// One question at a time: a parallel run asks once per grant.
	if !p.mu.TryLock() {
		paused = true
		p.mu.Lock()
	}
	defer p.mu.Unlock()
	if p.session == nil {
		p.session = map[string]bool{}
	}
	if p.allowed(k, o, write) {
		return paused, nil
	}
	var ask Asker
	if p.Ask != nil {
		ask = p.Ask()
	}
	if ask == nil {
		return paused, &DeniedError{Prompt: prompt}
	}
	paused = true
	timeout := p.AskTimeout
	if timeout <= 0 {
		timeout = DefaultAskTimeout
	}
	askCtx, release := detach(ctx)
	defer release()
	askCtx, cancel := context.WithTimeout(askCtx, timeout)
	defer cancel()
	scope, err := ask(askCtx, prompt)
	if err != nil {
		return paused, fmt.Errorf("asking the user to allow %s: %w", o, err)
	}
	if o == Loopback && scope != Deny {
		scope = Always // local servers are asked once
	}
	switch scope {
	case Call:
		c.call[k] = Call
	case Session:
		p.session[k] = true
	case Always:
		p.session[k] = true
		if p.Store != nil {
			if err := p.Store.Grant(o, write || o == Loopback); err != nil {
				return paused, fmt.Errorf("saving the permission for %s: %w", o, err)
			}
		}
	default:
		c.call[k] = Deny
		return paused, &DeniedError{Prompt: prompt, Asked: true}
	}
	return paused, nil
}

// redact drops the query and credentials from a URL shown to the user.
func redact(u *url.URL) string {
	c := *u
	c.User = nil
	c.RawQuery = ""
	c.Fragment = ""
	return c.String()
}

// Client returns a copy of client whose every request (redirects included)
// passes c.Check first. A nil client stands for http.DefaultClient.
func (c *Checker) Client(client *http.Client) *http.Client {
	var out http.Client
	if client != nil {
		out = *client
	}
	base := out.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	out.Transport = guard{base: base, check: c}
	return &out
}

type guard struct {
	base  http.RoundTripper
	check *Checker
}

func (g guard) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	paused, err := g.check.check(req.Context(), req)
	if err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, err
	}
	if !paused {
		return g.base.RoundTrip(req)
	}
	// The user's thinking time is not the server's: move the request's
	// deadline (Request.Timeout, http.Client.Timeout) by the time it waited.
	// http.Client also cancels through the deprecated Request.Cancel
	// channel for transports it does not know; the moved deadline
	// replaces both.
	ctx, release := extend(req.Context(), time.Since(start))
	resumed := req.WithContext(ctx)
	resumed.Cancel = nil //nolint:staticcheck // the field http.Client sets
	resp, err := g.base.RoundTrip(resumed)
	if err != nil {
		release()
		return nil, err
	}
	resp.Body = releaseOnClose{ReadCloser: resp.Body, release: release}
	return resp, nil
}

type releaseOnClose struct {
	io.ReadCloser
	release context.CancelFunc
}

func (r releaseOnClose) Close() error {
	err := r.ReadCloser.Close()
	r.release()
	return err
}

// detach returns a context without ctx's deadline that is still cancelled
// when ctx is cancelled for another reason (the tool call ended).
func detach(ctx context.Context) (context.Context, context.CancelFunc) {
	out, cancel := context.WithCancelCause(context.WithoutCancel(ctx))
	stop := context.AfterFunc(ctx, func() {
		if !expired(ctx) {
			cancel(context.Cause(ctx))
		}
	})
	return out, func() {
		stop()
		cancel(context.Canceled)
	}
}

// expired reports whether ctx ended by reaching its deadline. http.Client
// cancels a request at its Timeout with a plain cancel for transports it
// does not know (like guard), so a cancel at or after the deadline counts.
func expired(ctx context.Context) bool {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return true
	}
	d, ok := ctx.Deadline()
	return ok && !time.Now().Before(d)
}

// extend returns ctx with its deadline (if any) moved by d.
func extend(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	out, release := detach(ctx)
	deadline, ok := ctx.Deadline()
	if !ok {
		return out, release
	}
	out, cancel := context.WithDeadline(out, deadline.Add(d))
	return out, func() {
		cancel()
		release()
	}
}

// IsDenied reports whether err (possibly wrapped, e.g. by http.Client) is
// a refusal.
func IsDenied(err error) bool {
	var d *DeniedError
	return errors.As(err, &d)
}

func flagFor(p Prompt) string {
	if p.Write && !p.Loopback {
		return "--allow-writes"
	}
	return "--allow"
}
