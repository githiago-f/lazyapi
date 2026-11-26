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
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
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

// Check allows req or returns a *DeniedError (asking the user when it can).
func (c *Checker) Check(ctx context.Context, req *http.Request) error {
	if c == nil || c.policy == nil {
		return nil
	}
	p := c.policy
	o := origin(req.URL)
	write := isWrite(req.Method)
	k := key(o, write)
	prompt := Prompt{Origin: o, Method: req.Method, URL: redact(req.URL), Write: write, Loopback: o == Loopback, Tool: c.tool}

	c.mu.Lock()
	defer c.mu.Unlock()
	if answer, ok := c.call[k]; ok {
		if answer == Call {
			return nil
		}
		return &DeniedError{Prompt: prompt, Asked: true} // refused earlier in this call
	}

	// One question at a time: a parallel run asks once per grant.
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session == nil {
		p.session = map[string]bool{}
	}
	if p.allowed(k, o, write) {
		return nil
	}
	var ask Asker
	if p.Ask != nil {
		ask = p.Ask()
	}
	if ask == nil {
		return &DeniedError{Prompt: prompt}
	}
	scope, err := ask(ctx, prompt)
	if err != nil {
		return fmt.Errorf("asking the user to allow %s: %w", o, err)
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
				return fmt.Errorf("saving the permission for %s: %w", o, err)
			}
		}
	default:
		c.call[k] = Deny
		return &DeniedError{Prompt: prompt, Asked: true}
	}
	return nil
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
	if err := g.check.Check(req.Context(), req); err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, err
	}
	return g.base.RoundTrip(req)
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
