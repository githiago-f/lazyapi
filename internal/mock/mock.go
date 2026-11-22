// Package mock serves an OpenAPI spec as a fake API: each operation answers
// with its declared examples (or a sample generated from the schema), and
// incoming requests are validated against the spec.
package mock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

// Server is an http.Handler for a spec.
type Server struct {
	spec     *openapi3.T
	specPath string
	routes   []route
	bases    []string // path prefixes taken from the spec's servers

	// Validate rejects requests that do not match the spec with 422.
	Validate bool
	// Log receives one line per request (nil: silent).
	Log io.Writer

	mu sync.Mutex
}

type route struct {
	path   string // template, e.g. /users/{id}
	re     *regexp.Regexp
	names  []string
	params int
}

var paramPattern = regexp.MustCompile(`\{([^}/]+)\}`)

// New builds a mock for spec. specPath is used for operation references.
func New(spec *openapi3.T, specPath string) *Server {
	s := &Server{spec: spec, specPath: specPath, Validate: true}
	for _, p := range spec.Paths.InMatchingOrder() {
		var names []string
		pattern := "^"
		last := 0
		for _, m := range paramPattern.FindAllStringSubmatchIndex(p, -1) {
			pattern += regexp.QuoteMeta(p[last:m[0]]) + `([^/]+)`
			names = append(names, p[m[2]:m[3]])
			last = m[1]
		}
		pattern += regexp.QuoteMeta(p[last:]) + "/?$"
		s.routes = append(s.routes, route{path: p, re: regexp.MustCompile(pattern), names: names, params: len(names)})
	}
	// Literal segments win over parameters: /users/me before /users/{id}.
	sort.SliceStable(s.routes, func(i, j int) bool { return s.routes[i].params < s.routes[j].params })

	for _, srv := range spec.Servers {
		if u, err := url.Parse(srv.URL); err == nil && u.Path != "" && u.Path != "/" && !strings.Contains(u.Path, "{") {
			s.bases = append(s.bases, strings.TrimRight(u.Path, "/"))
		}
	}
	return s
}

// Operations returns the number of operations served.
func (s *Server) Operations() int {
	n := 0
	for _, r := range s.routes {
		n += len(s.spec.Paths.Find(r.path).Operations())
	}
	return n
}

// match finds the route and path parameters for a request path.
func (s *Server) match(p string) (*route, map[string]string) {
	candidates := []string{p}
	for _, b := range s.bases {
		if strings.HasPrefix(p, b+"/") || p == b {
			candidates = append(candidates, strings.TrimPrefix(p, b))
		}
	}
	for _, c := range candidates {
		if c == "" {
			c = "/"
		}
		for i := range s.routes {
			r := &s.routes[i]
			m := r.re.FindStringSubmatch(c)
			if m == nil {
				continue
			}
			params := map[string]string{}
			for j, name := range r.names {
				params[name], _ = url.PathUnescape(m[j+1])
			}
			return r, params
		}
	}
	return nil, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status, opLabel := s.serve(w, r)
	if s.Log != nil {
		s.mu.Lock()
		_, _ = fmt.Fprintf(s.Log, "%s %s → %d %s (%s)\n", r.Method, r.URL.RequestURI(), status, opLabel, time.Since(start).Round(time.Microsecond))
		s.mu.Unlock()
	}
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) (int, string) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	rt, params := s.match(r.URL.Path)
	if rt == nil {
		return writeError(w, http.StatusNotFound, "no operation matches "+r.Method+" "+r.URL.Path, ""), "-"
	}
	pathItem := s.spec.Paths.Find(rt.path)
	op := pathItem.GetOperation(r.Method)
	if op == nil {
		if r.Method == http.MethodOptions {
			allowed := methodsOf(pathItem)
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(allowed, ", "))
			w.Header().Set("Access-Control-Allow-Headers", "*")
			w.WriteHeader(http.StatusNoContent)
			return http.StatusNoContent, "(preflight)"
		}
		w.Header().Set("Allow", strings.Join(methodsOf(pathItem), ", "))
		return writeError(w, http.StatusMethodNotAllowed, r.Method+" is not declared for "+rt.path, ""), "-"
	}
	label := op.OperationID
	if label == "" {
		label = r.Method + " " + rt.path
	}

	ref := model.OpenAPIRef{FilePath: s.specPath, Path: rt.path, Method: r.Method}
	if s.Validate {
		if err := store.ValidateRequest(s.spec, ref, r, params); err != nil {
			return writeError(w, http.StatusUnprocessableEntity, "request does not match the OpenAPI spec", err.Error()), label
		}
	}

	prefer := parsePrefer(r.Header.Get("Prefer"))
	code, resp := chooseResponse(op, prefer["code"])
	if resp == nil {
		w.WriteHeader(code)
		return code, label
	}

	for name, h := range resp.Headers {
		if h == nil || h.Value == nil || strings.EqualFold(name, "Content-Type") {
			continue
		}
		if v := headerExample(h.Value); v != "" {
			w.Header().Set(name, v)
		}
	}

	mediaType, media := chooseMedia(resp.Content, r.Header.Get("Accept"))
	if media == nil {
		w.WriteHeader(code)
		return code, label
	}
	body, err := mediaBody(mediaType, media, prefer["example"])
	if err != nil {
		return writeError(w, http.StatusInternalServerError, "cannot build the example body", err.Error()), label
	}
	w.Header().Set("Content-Type", mediaType)
	w.WriteHeader(code)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
	return code, label
}

func methodsOf(p *openapi3.PathItem) []string {
	var out []string
	for m := range p.Operations() {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// parsePrefer reads RFC 7240 preferences such as "code=404, example=empty".
func parsePrefer(v string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' }) {
		k, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok {
			out[strings.ToLower(k)] = strings.Trim(strings.TrimSpace(val), `"`)
		}
	}
	return out
}

// chooseResponse picks the preferred status, else the lowest 2xx, else the
// lowest declared code; "default" answers as 200 (or the preferred code).
func chooseResponse(op *openapi3.Operation, preferred string) (int, *openapi3.Response) {
	if op.Responses == nil || op.Responses.Len() == 0 {
		return http.StatusOK, nil
	}
	responses := op.Responses.Map()
	if preferred != "" {
		if r, ok := responses[preferred]; ok && r.Value != nil {
			code, _ := strconv.Atoi(preferred)
			return code, r.Value
		}
		if code, err := strconv.Atoi(preferred); err == nil {
			if r, ok := responses[preferred[:1]+"XX"]; ok && r.Value != nil {
				return code, r.Value
			}
			if r, ok := responses["default"]; ok && r.Value != nil {
				return code, r.Value
			}
		}
	}
	var codes []int
	for k := range responses {
		if c, err := strconv.Atoi(k); err == nil {
			codes = append(codes, c)
		}
	}
	slices.Sort(codes)
	for _, c := range codes {
		if c >= 200 && c < 300 {
			return c, responses[strconv.Itoa(c)].Value
		}
	}
	if len(codes) > 0 {
		return codes[0], responses[strconv.Itoa(codes[0])].Value
	}
	return http.StatusOK, responses["default"].Value
}

// chooseMedia honours Accept (exact or type/* or */*), preferring JSON.
func chooseMedia(content openapi3.Content, accept string) (string, *openapi3.MediaType) {
	if len(content) == 0 {
		return "", nil
	}
	types := make([]string, 0, len(content))
	for mt := range content {
		types = append(types, mt)
	}
	sort.Slice(types, func(i, j int) bool {
		ji, jj := strings.Contains(types[i], "json"), strings.Contains(types[j], "json")
		if ji != jj {
			return ji
		}
		return types[i] < types[j]
	})
	for _, a := range strings.Split(accept, ",") {
		a = strings.TrimSpace(strings.Split(a, ";")[0])
		if a == "" || a == "*/*" {
			continue
		}
		for _, mt := range types {
			if mt == a || (strings.HasSuffix(a, "/*") && strings.HasPrefix(mt, strings.TrimSuffix(a, "*"))) {
				return mt, content[mt]
			}
		}
	}
	return types[0], content[types[0]]
}

// mediaBody renders the example body: a named example (Prefer example=),
// the media example, the first named example, or a schema sample.
func mediaBody(mediaType string, m *openapi3.MediaType, preferred string) ([]byte, error) {
	var value any
	switch {
	case preferred != "" && m.Examples[preferred] != nil && m.Examples[preferred].Value != nil:
		value = m.Examples[preferred].Value.Value
	case m.Example != nil:
		value = m.Example
	case len(m.Examples) > 0:
		names := make([]string, 0, len(m.Examples))
		for n := range m.Examples {
			names = append(names, n)
		}
		sort.Strings(names)
		if e := m.Examples[names[0]]; e != nil && e.Value != nil {
			value = e.Value.Value
		}
	default:
		value = store.SchemaSample(m.Schema)
	}
	if s, ok := value.(string); ok && !strings.Contains(mediaType, "json") {
		return []byte(s), nil
	}
	return json.MarshalIndent(value, "", "  ")
}

func headerExample(h *openapi3.Header) string {
	switch {
	case h.Example != nil:
		return fmt.Sprint(h.Example)
	case h.Schema != nil:
		if v := store.SchemaSample(h.Schema); v != nil {
			return fmt.Sprint(v)
		}
	}
	return ""
}

func writeError(w http.ResponseWriter, code int, msg, details string) int {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	body := map[string]any{"error": msg, "mock": "lazyapi"}
	if details != "" {
		body["details"] = details
	}
	_ = json.NewEncoder(w).Encode(body)
	return code
}
