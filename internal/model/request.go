package model

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
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
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	reqCtx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	ctx := r.templateContext()
	fullURL := r.ResolvedURL()

	resolvedBody := env.Resolve(r.Body.Raw, ctx)
	var bodyReader io.Reader
	if resolvedBody != "" {
		bodyReader = strings.NewReader(resolvedBody)
	}

	req, err := http.NewRequestWithContext(reqCtx, r.Method.Label(), fullURL, bodyReader)
	if err != nil {
		return nil, "", err
	}

	q := req.URL.Query()
	for name, value := range r.Query {
		// Optional parameters without a value are not sent.
		if resolved := env.Resolve(value, ctx); resolved != "" {
			q.Set(name, resolved)
		}
	}
	for name, value := range r.Headers {
		req.Header.Set(name, env.Resolve(value, ctx))
	}
	if bodyReader != nil && req.Header.Get("Content-Type") == "" && r.Body.Type != "" {
		req.Header.Set("Content-Type", string(r.Body.Type))
	}

	for _, scheme := range r.Auth {
		switch scheme.Type {
		case AuthBasic:
			auth := env.Resolve(scheme.Username, ctx) + ":" + env.Resolve(scheme.Password, ctx)
			encoded := base64.StdEncoding.EncodeToString([]byte(auth))
			req.Header.Set("Authorization", "Basic "+encoded)
		case AuthBearer:
			req.Header.Set("Authorization", "Bearer "+env.Resolve(scheme.Token, ctx))
		case AuthAPIKey:
			keyValue := env.Resolve(scheme.KeyValue, ctx)
			switch scheme.KeyIn {
			case "header":
				req.Header.Set(scheme.KeyName, keyValue)
			case "query":
				q.Set(scheme.KeyName, keyValue)
			}
		case AuthOAuth2:
			if scheme.AccessToken != "" {
				req.Header.Set("Authorization", "Bearer "+env.Resolve(scheme.AccessToken, ctx))
			}
		}
	}
	req.URL.RawQuery = q.Encode()

	client := http.DefaultClient
	if r.HTTPClient != nil {
		client = r.HTTPClient
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = response.Body.Close() }()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, "", err
	}

	return response, string(bodyBytes), nil
}
