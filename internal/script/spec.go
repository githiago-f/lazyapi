package script

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

// SpecContext connects scripts to an OpenAPI spec: lazyapi.send() sends
// other operations of the spec, and toMatchSchema() validates against it.
type SpecContext struct {
	Spec      *openapi3.T
	SpecPath  string
	ServerURL string // empty: first server in the spec
	Env       map[string]string
	Vars      map[string]string
	Timeout   time.Duration

	// Auth fills credentials by security scheme name (lazyapi.config.yml).
	Auth map[string]model.AuthSecrets

	// HTTPClient and MaxBody are applied to every request (nil/0: defaults).
	HTTPClient *http.Client
	MaxBody    int64

	// OnSend, when set, observes every lazyapi.send() call: the operation,
	// the final request and either the response or the error.
	OnSend func(ref model.OpenAPIRef, req model.Request, resp *Response, err error)
}

// ResolveTarget finds an operation by operationId or "METHOD /path".
func ResolveTarget(spec *openapi3.T, specPath, target string) (model.OpenAPIRef, error) {
	target = strings.TrimSpace(target)
	if method, path, ok := strings.Cut(target, " "); ok {
		ref := model.OpenAPIRef{FilePath: specPath, Path: strings.TrimSpace(path), Method: strings.ToUpper(method)}
		if store.FindOperation(spec, ref) == nil {
			return ref, fmt.Errorf("operation %s not found in spec", target)
		}
		return ref, nil
	}
	for _, path := range spec.Paths.InMatchingOrder() {
		for method, op := range spec.Paths.Find(path).Operations() {
			if op.OperationID == target {
				return model.OpenAPIRef{FilePath: specPath, Path: path, Method: method}, nil
			}
		}
	}
	return model.OpenAPIRef{}, fmt.Errorf("no operation with operationId %q (use \"METHOD /path\" or an existing operationId)", target)
}

// Send implements SendFunc. target is an operationId, "METHOD /path", or
// "METHOD https://host/concrete/path?q=1": a URL is mapped to the spec
// operation it belongs to (path params and query come from the URL), so it
// gets the same checks; a URL outside the spec is an error.
func (c *SpecContext) Send(target string, opts SendOptions) (*Response, error) {
	var (
		ref    model.OpenAPIRef
		server string // set for URL targets: send exactly where the URL points
		err    error
	)
	method, rawURL, isPair := strings.Cut(strings.TrimSpace(target), " ")
	rawURL = env.Resolve(strings.TrimSpace(rawURL), env.Context{Env: c.Env, Vars: c.Vars})
	if isPair && strings.Contains(rawURL, "://") {
		ref, server, err = c.resolveURL(method, rawURL, &opts)
	} else {
		ref, err = ResolveTarget(c.Spec, c.SpecPath, target)
	}
	if err != nil {
		return nil, err
	}
	req := store.OperationToRequest(c.Spec, ref)
	if c.ServerURL != "" {
		req.ServerURL = c.ServerURL
	}
	if server != "" {
		req.ServerURL = server
	}
	req.Env, req.Vars, req.Timeout = c.Env, c.Vars, c.Timeout
	req.HTTPClient, req.MaxBody = c.HTTPClient, c.MaxBody
	req.Auth = model.ApplySecrets(req.Auth, c.Auth)
	for k, v := range opts.Params {
		req.Params[k] = v
	}
	for k, v := range opts.Query {
		req.Query[k] = v
	}
	for k, v := range opts.Headers {
		req.Headers[k] = v
	}
	switch body := opts.Body.(type) {
	case nil:
	case string:
		req.Body.Raw = body
	default:
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding body: %w", err)
		}
		req.Body.Raw = string(data)
		if !hasHeader(req.Headers, "Content-Type") {
			req.Headers["Content-Type"] = "application/json"
		}
	}
	if req, err = req.WithDynamic(Dynamic); err != nil {
		c.observe(ref, req, nil, err)
		return nil, err
	}
	if missing := req.MissingPathParams(); len(missing) > 0 {
		err := fmt.Errorf("missing path parameter(s) %s (pass them in options.params)", strings.Join(missing, ", "))
		c.observe(ref, req, nil, err)
		return nil, err
	}
	if req.ServerURL == "" {
		err := fmt.Errorf("no server URL for %s", target)
		c.observe(ref, req, nil, err)
		return nil, err
	}

	start := time.Now()
	httpResp, body, err := req.Send()
	if err != nil {
		c.observe(ref, req, nil, err)
		return nil, err
	}
	resp := &Response{
		Status:     httpResp.StatusCode,
		StatusText: httpResp.Status,
		Header:     httpResp.Header,
		Body:       body,
		Duration:   time.Since(start),
		Ref:        &ref,
	}
	c.observe(ref, req, resp, nil)
	return resp, nil
}

func (c *SpecContext) observe(ref model.OpenAPIRef, req model.Request, resp *Response, err error) {
	if c.OnSend != nil {
		c.OnSend(ref, req, resp, err)
	}
}

// Contract implements ContractFunc.
func (c *SpecContext) Contract(ref model.OpenAPIRef, resp *Response) error {
	return store.ValidateResponse(c.Spec, ref, resp.Status, resp.Header, []byte(resp.Body))
}

// OperationOptions returns the Options for running an operation's script:
// relative imports resolve from the script's directory and may not leave the
// spec's directory.
func OperationOptions(specPath string, op *openapi3.Operation, ref model.OpenAPIRef) (Options, error) {
	specDir, err := filepath.Abs(filepath.Dir(specPath))
	if err != nil {
		return Options{}, err
	}
	path, err := store.OperationTestsPath(specPath, op, ref)
	if err != nil {
		return Options{}, err
	}
	return Options{Filename: path, ResolveDir: filepath.Dir(path), Root: specDir}, nil
}

// Bind fills the spec-dependent hooks of data.
func (c *SpecContext) Bind(data *ExecData) {
	data.Send = c.Send
	data.Contract = c.Contract
	if data.Env == nil {
		data.Env = c.Env
	}
	if data.Vars == nil {
		data.Vars = c.Vars
	}
}

func hasHeader(h map[string]string, name string) bool {
	for k := range h {
		if strings.EqualFold(k, name) {
			return true
		}
	}
	return false
}

// resolveURL maps an absolute URL to an operation. The URL must be on the
// context's server or one of the spec's servers; its path (without the
// server's base path) must match an operation. Path params and query
// parameters from the URL are added to opts (explicit options win).
func (c *SpecContext) resolveURL(method, rawURL string, opts *SendOptions) (model.OpenAPIRef, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return model.OpenAPIRef{}, "", fmt.Errorf("invalid URL %q", rawURL)
	}
	servers := []string{c.ServerURL}
	for _, s := range c.Spec.Servers {
		servers = append(servers, s.URL)
	}
	for _, s := range servers {
		su, err := url.Parse(env.Resolve(s, env.Context{Env: c.Env, Vars: c.Vars}))
		if s == "" || err != nil || !strings.EqualFold(su.Host, u.Host) {
			continue
		}
		base := strings.TrimRight(su.Path, "/")
		if !strings.HasPrefix(u.Path, base) {
			continue
		}
		ref, params, ok := store.MatchPath(c.Spec, method, strings.TrimPrefix(u.Path, base))
		if !ok {
			continue
		}
		ref.FilePath = c.SpecPath
		opts.Params = mergeMissing(opts.Params, params)
		q := map[string]string{}
		for k, vs := range u.Query() {
			if len(vs) > 0 {
				q[k] = vs[len(vs)-1]
			}
		}
		opts.Query = mergeMissing(opts.Query, q)
		return ref, u.Scheme + "://" + u.Host + base, nil
	}
	return model.OpenAPIRef{}, "", fmt.Errorf("%s %s is not an operation of the spec on one of its servers; add it (lazyapi import curl or lazyapi add request) so it is checked like the others", strings.ToUpper(method), u.Redacted())
}

// mergeMissing adds the entries of extra that dst does not have.
func mergeMissing(dst, extra map[string]string) map[string]string {
	if dst == nil {
		dst = map[string]string{}
	}
	for k, v := range extra {
		if _, ok := dst[k]; !ok {
			dst[k] = v
		}
	}
	return dst
}
