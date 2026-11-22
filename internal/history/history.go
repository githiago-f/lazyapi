// Package history keeps the requests sent from the TUI, per spec, so they
// can be replayed and their responses compared.
//
// Entries are JSON lines under the user cache dir
// (~/.cache/lazyapi/history/<spec>.jsonl, mode 0600). Credentials are never
// written: the environment and auth secrets are dropped, sensitive headers
// and query parameters are redacted and bodies are kept as templates
// ({{env.X}}), not resolved.
package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/runner"
)

const (
	// Keep is how many entries are kept per spec.
	Keep = 200
	// MaxBody caps the response body stored per entry.
	MaxBody = 256 << 10
	// Redacted replaces credential values.
	Redacted = "<redacted>"
)

// Request is what is needed to send a request again.
type Request struct {
	Method    string            `json:"method"`
	URI       string            `json:"uri"`
	ServerURL string            `json:"serverUrl,omitempty"`
	Params    map[string]string `json:"params,omitempty"`
	Query     map[string]string `json:"query,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	BodyType  string            `json:"bodyType,omitempty"`
	Body      string            `json:"body,omitempty"`
}

// Entry is one sent request and what came back.
type Entry struct {
	Time    time.Time         `json:"time"`
	Spec    string            `json:"spec,omitempty"`
	Ref     *model.OpenAPIRef `json:"ref,omitempty"`
	Env     string            `json:"env,omitempty"` // environment name (not its values)
	Request Request           `json:"request"`
	Result  runner.OpResult   `json:"result"`
	// Truncated is set when the stored body was cut at MaxBody.
	Truncated bool `json:"truncated,omitempty"`
}

// Label is "METHOD /path".
func (e Entry) Label() string { return e.Request.Method + " " + e.Request.URI }

// SameOperation reports whether two entries are the same operation.
func (e Entry) SameOperation(o Entry) bool {
	return strings.EqualFold(e.Request.Method, o.Request.Method) && e.Request.URI == o.Request.URI && e.Spec == o.Spec
}

var mu sync.Mutex

// Dir is where history files live: $LAZYAPI_HISTORY_DIR, or the user cache
// dir. An empty result disables history.
func Dir() string {
	if os.Getenv("LAZYAPI_NO_HISTORY") != "" {
		return ""
	}
	if d := os.Getenv("LAZYAPI_HISTORY_DIR"); d != "" {
		return d
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "lazyapi", "history")
}

// File is the history file for a spec ("" spec: requests without one).
func File(spec string) string {
	dir := Dir()
	if dir == "" {
		return ""
	}
	name := "scratch"
	if spec != "" {
		if abs, err := filepath.Abs(spec); err == nil {
			spec = abs
		}
		name = sanitize(spec)
	}
	return filepath.Join(dir, name+".jsonl")
}

func sanitize(p string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	s := strings.Trim(r.Replace(p), "_")
	if len(s) > 180 {
		s = s[len(s)-180:]
	}
	return s
}

// New builds a redacted entry from a request and its result.
func New(req model.Request, res runner.OpResult, envName string) Entry {
	e := Entry{
		Time:    time.Now(),
		Spec:    req.FileName,
		Env:     envName,
		Request: requestOf(req),
		Result:  res,
	}
	if req.OpenAPIRef != nil {
		ref := *req.OpenAPIRef
		e.Ref = &ref
	}
	e.Result.Logs = nil // console output may print secrets
	if r := e.Result.Request; r != nil {
		cp := *r
		cp.URL = redactURL(cp.URL)
		cp.Body = req.Body.Raw // the template, not the resolved value
		cp.Headers = redactHeaders(cp.Headers)
		e.Result.Request = &cp
	}
	if r := e.Result.Response; r != nil {
		cp := *r
		cp.Headers = redactHeaders(cp.Headers)
		cp.Cookies = nil
		if len(cp.Body) > MaxBody {
			cp.Body = cp.Body[:MaxBody]
			e.Truncated = true
		}
		e.Result.Response = &cp
	}
	return e
}

func requestOf(req model.Request) Request {
	r := Request{
		Method:    req.Method.Label(),
		URI:       req.URI,
		ServerURL: req.ServerURL,
		Params:    copyMap(req.Params),
		Query:     map[string]string{},
		Headers:   redactHeaders(req.Headers),
		BodyType:  string(req.Body.Type),
		Body:      req.Body.Raw,
	}
	for k, v := range req.Query {
		if runner.IsSensitive(k) && !isTemplate(v) {
			v = Redacted
		}
		r.Query[k] = v
	}
	return r
}

// isTemplate reports whether v is only a template reference ({{env.X}}),
// which is safe to keep: the secret lives in the environment.
func isTemplate(v string) bool {
	v = strings.TrimSpace(v)
	return strings.HasPrefix(v, "{{") && strings.HasSuffix(v, "}}") && strings.Count(v, "{{") == 1
}

func redactHeaders(h map[string]string) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		if runner.IsSensitive(k) && !isTemplate(v) {
			v = Redacted
		}
		out[k] = v
	}
	return out
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.RawQuery == "" {
		return raw
	}
	q := u.Query()
	changed := false
	for k := range q {
		if runner.IsSensitive(k) {
			q.Set(k, Redacted)
			changed = true
		}
	}
	if u.User != nil {
		u.User = nil
		changed = true
	}
	if !changed {
		return raw
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func copyMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Append stores an entry, keeping the newest Keep entries of its spec.
func Append(e Entry) error {
	path := File(e.Spec)
	if path == "" {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(line, '\n'))
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return werr
	}
	entries, err := read(path)
	if err != nil || len(entries) <= Keep+Keep/4 {
		return err
	}
	return write(path, entries[len(entries)-Keep:])
}

// Load returns a spec's entries, newest first.
func Load(spec string) ([]Entry, error) {
	path := File(spec)
	if path == "" {
		return nil, nil
	}
	mu.Lock()
	defer mu.Unlock()
	entries, err := read(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries, err
}

// Clear removes a spec's history.
func Clear(spec string) error {
	path := File(spec)
	if path == "" {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func read(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 4*MaxBody)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			out = append(out, e) // a corrupt line is skipped, not fatal
		}
	}
	return out, sc.Err()
}

func write(path string, entries []Entry) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, e := range entries {
		line, err := json.Marshal(e)
		if err != nil {
			continue
		}
		_, _ = w.Write(append(line, '\n'))
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Replay turns an entry back into a request. base is the operation as
// loaded from the spec (auth schemes, tests, servers), or a zero Request;
// redacted values are taken from base instead of the entry.
func (e Entry) Replay(base model.Request) model.Request {
	req := base
	r := e.Request
	if m, ok := model.ParseMethod(r.Method); ok {
		req.Method = m
	}
	req.URI = r.URI
	if r.ServerURL != "" {
		req.ServerURL = r.ServerURL
	}
	req.Params = merge(base.Params, r.Params)
	req.Query = merge(base.Query, r.Query)
	req.Headers = merge(base.Headers, r.Headers)
	if r.BodyType != "" {
		req.Body.Type = model.MimeType(r.BodyType)
	}
	req.Body.Raw = r.Body
	if e.Ref != nil && req.OpenAPIRef == nil {
		ref := *e.Ref
		req.OpenAPIRef = &ref
	}
	if req.FileName == "" {
		req.FileName = e.Spec
	}
	return req
}

func merge(base, over map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		if v == Redacted {
			continue // keep the base value (or none)
		}
		out[k] = v
	}
	return out
}
