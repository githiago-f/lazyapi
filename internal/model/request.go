package model

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"sort"
	"strings"
	"time"

	"github.com/githiago-f/lazyapi/internal/env"
)

type OpenAPIRef struct {
	FilePath string
	Path     string
	Method   string
}

type Request struct {
	FileName string

	About About `yaml:"about"`

	URI    string `yaml:"uri"`
	Method Method `yaml:"method"`

	Body    Body              `yaml:"body"`
	Headers map[string]string `yaml:"headers"`
	Params  map[string]string `yaml:"pathParams"`
	Query   map[string]string `yaml:"query"`

	Tags  []string     `yaml:"tags,omitempty"`
	Auth  []AuthScheme `yaml:"auth,omitempty"`
	Tests string       `yaml:"tests,omitempty"`

	ServerURL string   `yaml:"serverUrl,omitempty"`
	Servers   []string `yaml:"-"`

	OpenAPIRef *OpenAPIRef `yaml:"-"`
	DraftPath  string      `yaml:"-"`

	Env  map[string]string `yaml:"-"`
	Vars map[string]string `yaml:"-"`

	HTTPClient *http.Client `yaml:"-"`

	// Timeout bounds the whole request (connect + headers + body). Zero means
	// DefaultTimeout. Ignored when HTTPClient is set and has its own timeout.
	Timeout time.Duration `yaml:"-"`

	// MaxBody caps the response body read (DefaultMaxBody when zero).
	MaxBody int64 `yaml:"-"`
}

// DefaultTimeout is applied to requests that do not set Request.Timeout.
const DefaultTimeout = 30 * time.Second

func (r *Request) templateContext() env.Context {
	return env.Context{Env: r.Env, Vars: r.Vars}
}

// ResolvedURL returns server + URI with templates resolved and path
// parameters substituted (query string not included).
func (r *Request) ResolvedURL() string {
	ctx := r.templateContext()
	fullURL := env.Resolve(r.ServerURL, ctx) + env.Resolve(r.URI, ctx)

	names := make([]string, 0, len(r.Params))
	for name := range r.Params {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fullURL = strings.ReplaceAll(fullURL, "{"+name+"}", env.Resolve(r.Params[name], ctx))
	}
	return fullURL
}

// MissingPathParams lists (sorted) path parameters that have no value.
func (r *Request) MissingPathParams() []string {
	var missing []string
	for name, value := range r.Params {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// Send performs the request with a background context. See SendContext.
func (r *Request) Send() (*http.Response, string, error) {
	return r.SendContext(context.Background())
}

// SendContext resolves templates, performs the HTTP request and returns the
// response together with its fully-read body. The request is bounded by
// r.Timeout (or DefaultTimeout) and can be cancelled through ctx.
func (r *Request) SendContext(parent context.Context) (*http.Response, string, error) {
	resp, body, _, err := r.SendTimed(parent)
	return resp, body, err
}

// SendTimed is SendContext plus a breakdown of where the time went.
func (r *Request) SendTimed(parent context.Context) (*http.Response, string, Timing, error) {
	var timing Timing
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	reqCtx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	var dnsStart, connStart, tlsStart, wrote time.Time
	trace := &httptrace.ClientTrace{
		DNSStart:          func(httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone:           func(httptrace.DNSDoneInfo) { timing.DNS = time.Since(dnsStart) },
		ConnectStart:      func(string, string) { connStart = time.Now() },
		ConnectDone:       func(string, string, error) { timing.Connect = time.Since(connStart) },
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone:  func(tls.ConnectionState, error) { timing.TLS = time.Since(tlsStart) },
		GotConn:           func(info httptrace.GotConnInfo) { timing.Reused = info.Reused },
		WroteRequest:      func(httptrace.WroteRequestInfo) { wrote = time.Now() },
		GotFirstResponseByte: func() {
			if !wrote.IsZero() {
				timing.TTFB = time.Since(wrote)
			}
		},
	}
	start := time.Now()
	req, err := r.HTTPRequest(httptrace.WithClientTrace(reqCtx, trace))
	if err != nil {
		return nil, "", timing, err
	}

	response, err := r.client().Do(req)
	if err != nil {
		return nil, "", timing, err
	}
	defer func() { _ = response.Body.Close() }()

	limit := r.MaxBody
	if limit <= 0 {
		limit = DefaultMaxBody
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	timing.Total = time.Since(start)
	if err != nil {
		return nil, "", timing, err
	}
	if int64(len(bodyBytes)) > limit {
		return nil, "", timing, fmt.Errorf("response body is larger than %d bytes (raise it with --max-body)", limit)
	}
	return response, string(bodyBytes), timing, nil
}

// HTTPRequest builds the *http.Request that SendContext would send:
// templates resolved, path params substituted, query, headers, body and
// auth applied. Optional query parameters without a value are omitted.
func (r *Request) HTTPRequest(ctx context.Context) (*http.Request, error) {
	tctx := r.templateContext()
	bodyReader, contentType, err := r.Body.encode(env.Resolve(r.Body.Raw, tctx))
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, r.Method.Label(), r.ResolvedURL(), bodyReader)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	for name, value := range r.Query {
		if resolved := env.Resolve(value, tctx); resolved != "" {
			q.Set(name, resolved)
		}
	}
	for name, value := range r.Headers {
		req.Header.Set(name, env.Resolve(value, tctx))
	}
	switch {
	case bodyReader == nil || contentType == "":
	case strings.HasPrefix(contentType, string(MultipartForm)):
		req.Header.Set("Content-Type", contentType) // carries the boundary
	case req.Header.Get("Content-Type") == "":
		req.Header.Set("Content-Type", contentType)
	}

	// Credentials are only applied when they have a value, so an empty scheme
	// does not clobber an Authorization header set explicitly (or by before()).
	for _, scheme := range r.Auth {
		switch scheme.Type {
		case AuthBasic:
			user, pass := env.Resolve(scheme.Username, tctx), env.Resolve(scheme.Password, tctx)
			if user != "" || pass != "" {
				encoded := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
				req.Header.Set("Authorization", "Basic "+encoded)
			}
		case AuthBearer:
			if token := env.Resolve(scheme.Token, tctx); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		case AuthAPIKey:
			keyValue := env.Resolve(scheme.KeyValue, tctx)
			if keyValue == "" {
				continue
			}
			switch scheme.KeyIn {
			case "header":
				req.Header.Set(scheme.KeyName, keyValue)
			case "query":
				q.Set(scheme.KeyName, keyValue)
			case "cookie":
				req.AddCookie(&http.Cookie{Name: scheme.KeyName, Value: keyValue})
			}
		case AuthOAuth2:
			token := env.Resolve(scheme.AccessToken, tctx)
			if token == "" {
				var err error
				if token, err = oauthToken(ctx, r.client(), scheme, tctx); err != nil {
					return nil, err
				}
			}
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		}
	}
	req.URL.RawQuery = q.Encode()
	return req, nil
}

func (r *Request) client() *http.Client {
	if r.HTTPClient != nil {
		return r.HTTPClient
	}
	return http.DefaultClient
}
