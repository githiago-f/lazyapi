// Package importer turns requests captured elsewhere (curl commands, Postman
// collections, HAR files) into operations of an OpenAPI spec, with examples,
// inferred schemas, security schemes and test scripts.
package importer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

// KV is an ordered name/value pair (headers, query, form fields).
type KV struct {
	Key   string
	Value string
	File  bool // multipart file field
}

// Response is an observed or saved response, imported as an example.
type Response struct {
	Status      int
	ContentType string
	Body        string
}

// Request is one captured request, independent of where it came from.
type Request struct {
	Name        string
	Description string
	Tags        []string
	Method      string
	URL         string // absolute URL
	Headers     []KV
	Body        string
	BodyType    string // media type; "" means from Content-Type / sniffing
	Form        []KV   // form-urlencoded or multipart fields (BodyType says which)
	Tests       string // JS test script (Postman scripts are wrapped by the caller)
	Responses   []Response
}

// Result summarises an import.
type Result struct {
	Added   []string `json:"added"`
	Skipped []string `json:"skipped,omitempty"`
	Notes   []string `json:"notes,omitempty"`
	Files   []string `json:"files,omitempty"` // test scripts and other files written
}

func (r *Result) note(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	for _, n := range r.Notes {
		if n == msg {
			return
		}
	}
	r.Notes = append(r.Notes, msg)
}

// skipHeaders are not imported as parameters (transport or tooling noise).
var skipHeaders = map[string]bool{
	"accept": true, "accept-encoding": true, "accept-language": true, "authorization": true,
	"cache-control": true, "connection": true, "content-length": true, "content-type": true,
	"cookie": true, "host": true, "origin": true, "postman-token": true, "pragma": true,
	"referer": true, "user-agent": true, "sec-fetch-dest": true, "sec-fetch-mode": true,
	"sec-fetch-site": true, "sec-ch-ua": true, "sec-ch-ua-mobile": true, "sec-ch-ua-platform": true,
	"upgrade-insecure-requests": true, "dnt": true,
}

var apiKeyHeader = regexp.MustCompile(`(?i)^(x-)?(api[-_]?key|auth[-_]?token|access[-_]?token)$`)

// Apply adds the requests to spec. Operations that already exist (same
// method and path) are skipped. Test scripts are written next to specPath
// (not at all when specPath is "", a dry run). The caller saves the spec.
func Apply(spec *openapi3.T, specPath string, reqs []Request) (Result, error) {
	var res Result
	if spec.Paths == nil {
		spec.Paths = openapi3.NewPaths()
	}
	for _, r := range reqs {
		label, err := applyOne(spec, specPath, r, &res)
		if err != nil {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: %v", r.Name, err))
			continue
		}
		if label != "" {
			res.Added = append(res.Added, label)
		}
	}
	return res, nil
}

func applyOne(spec *openapi3.T, specPath string, r Request, res *Result) (string, error) {
	method := strings.ToUpper(r.Method)
	if method == "" {
		method = http.MethodGet
	}
	u, err := url.Parse(r.URL)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("not an absolute URL: %q", r.URL)
	}

	base := ensureServer(spec, u)
	rawPath := strings.TrimPrefix(u.EscapedPath(), base)
	if rawPath == "" {
		rawPath = "/"
	}
	path, pathParams := templatePath(rawPath)
	label := method + " " + path

	if item := spec.Paths.Find(path); item != nil && item.GetOperation(method) != nil {
		res.Skipped = append(res.Skipped, label+": already in the spec")
		return "", nil
	}

	op := openapi3.NewOperation()
	op.OperationID = uniqueOperationID(spec, operationID(r.Name, method, path))
	op.Summary = r.Name
	op.Description = r.Description
	op.Tags = r.Tags

	for _, p := range pathParams {
		op.AddParameter(param("path", p.Key, p.Value, true))
	}
	seenQuery := map[string]bool{}
	for k, vs := range u.Query() {
		if !seenQuery[k] && len(vs) > 0 {
			seenQuery[k] = true
			op.AddParameter(param("query", k, vs[0], false))
		}
	}
	sortParams(op)

	contentType := ""
	for _, h := range r.Headers {
		name := strings.ToLower(h.Key)
		switch {
		case name == "content-type":
			contentType = h.Value
		case name == "authorization":
			addAuth(spec, op, h.Value, res)
		case apiKeyHeader.MatchString(h.Key):
			addSecurity(spec, op, "apiKeyAuth", &openapi3.SecurityScheme{Type: "apiKey", In: "header", Name: h.Key})
			res.note("%s: the %s value was not copied into the spec; set it in lazyapi.config.yml under auth.apiKeyAuth.value", label, h.Key)
		case !skipHeaders[name]:
			op.AddParameter(param("header", h.Key, h.Value, false))
		}
	}

	if body := requestBody(r, contentType); body != nil {
		op.RequestBody = &openapi3.RequestBodyRef{Value: body}
	}

	op.Responses = openapi3.NewResponses()
	op.Responses.Delete("default")
	for _, resp := range r.Responses {
		addResponse(op, resp)
	}
	if op.Responses.Len() == 0 {
		desc := "OK"
		op.Responses.Set("200", &openapi3.ResponseRef{Value: &openapi3.Response{Description: &desc}})
	}

	spec.AddOperation(path, method, op)

	if strings.TrimSpace(r.Tests) != "" && specPath == "" {
		res.Files = append(res.Files, "(dry run) tests/"+store.TestScriptName(op, model.OpenAPIRef{Path: path, Method: method}))
	} else if strings.TrimSpace(r.Tests) != "" {
		ref := model.OpenAPIRef{FilePath: specPath, Path: path, Method: method}
		if err := store.SaveOperationTests(spec, specPath, ref, r.Tests); err != nil {
			res.note("%s: tests not written: %v", label, err)
		} else if p, err := store.OperationTestsPath(specPath, op, ref); err == nil {
			res.Files = append(res.Files, p)
		}
	}
	return label, nil
}

// ensureServer returns the base path of the spec server matching u, adding
// a server for u's origin when none matches.
func ensureServer(spec *openapi3.T, u *url.URL) string {
	origin := u.Scheme + "://" + u.Host
	best := ""
	found := false
	for _, s := range spec.Servers {
		su, err := url.Parse(s.URL)
		if err != nil || su.Scheme+"://"+su.Host != origin {
			continue
		}
		base := strings.TrimRight(su.EscapedPath(), "/")
		if strings.HasPrefix(u.EscapedPath(), base) && len(base) >= len(best) {
			best, found = base, true
		}
	}
	if !found {
		spec.Servers = append(spec.Servers, &openapi3.Server{URL: origin})
	}
	return best
}

var (
	numeric = regexp.MustCompile(`^\d+$`)
	uuidish = regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$|^[0-9a-fA-F]{24,}$`)
	braces  = regexp.MustCompile(`^\{\{?([A-Za-z0-9_.-]+)\}?\}$`)
)

// templatePath turns concrete segments into parameters: ":id" and "{{id}}"
// (Postman), numbers and UUID-like values. Returns the template and the
// parameter names with their observed values.
func templatePath(p string) (string, []KV) {
	segments := strings.Split(p, "/")
	var params []KV
	used := map[string]bool{}
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		raw, _ := url.PathUnescape(seg)
		name, value := "", raw
		switch {
		case strings.HasPrefix(raw, ":") && len(raw) > 1:
			name, value = raw[1:], ""
		case braces.MatchString(raw):
			name, value = braces.FindStringSubmatch(raw)[1], ""
		case numeric.MatchString(raw) || uuidish.MatchString(raw):
			name = "id"
			if prev := previousLiteral(segments[:i]); prev != "" && used["id"] {
				name = singular(prev) + "Id"
			}
		default:
			continue
		}
		name = sanitizeName(name)
		for used[name] {
			name += "2"
		}
		used[name] = true
		segments[i] = "{" + name + "}"
		params = append(params, KV{Key: name, Value: value})
	}
	return strings.Join(segments, "/"), params
}

func previousLiteral(segs []string) string {
	for i := len(segs) - 1; i >= 0; i-- {
		if s := segs[i]; s != "" && !strings.HasPrefix(s, "{") {
			return s
		}
	}
	return ""
}

func singular(s string) string {
	s = sanitizeName(s)
	switch {
	case strings.HasSuffix(s, "ies") && len(s) > 3:
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") && len(s) > 1:
		return s[:len(s)-1]
	}
	return s
}

var nonIdent = regexp.MustCompile(`[^A-Za-z0-9_]+`)

func sanitizeName(s string) string {
	parts := nonIdent.Split(s, -1)
	out := ""
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i > 0 && out != "" {
			p = strings.ToUpper(p[:1]) + p[1:]
		}
		out += p
	}
	if out == "" {
		return "param"
	}
	return out
}

// operationID builds a camelCase id from the request name, or from the
// method and path ("GET /users/{id}" → getUsersById).
func operationID(name, method, path string) string {
	words := splitWords(name)
	if len(words) == 0 {
		words = []string{strings.ToLower(method)}
		for _, seg := range strings.Split(path, "/") {
			switch {
			case seg == "":
			case strings.HasPrefix(seg, "{"):
				words = append(words, "by", strings.Trim(seg, "{}"))
			default:
				words = append(words, splitWords(seg)...)
			}
		}
	}
	id := ""
	for i, w := range words {
		if w == strings.ToUpper(w) {
			w = strings.ToLower(w) // GET, API → get, api
		}
		if i == 0 {
			w = strings.ToLower(w[:1]) + w[1:]
		} else {
			w = strings.ToUpper(w[:1]) + w[1:]
		}
		id += w
	}
	if id == "" || unicode.IsDigit(rune(id[0])) {
		id = "op" + id
	}
	return id
}

func splitWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

func uniqueOperationID(spec *openapi3.T, id string) string {
	taken := map[string]bool{}
	for _, p := range spec.Paths.InMatchingOrder() {
		for _, op := range spec.Paths.Find(p).Operations() {
			taken[op.OperationID] = true
		}
	}
	candidate := id
	for n := 2; taken[candidate]; n++ {
		candidate = id + strconv.Itoa(n)
	}
	return candidate
}

func param(in, name, example string, required bool) *openapi3.Parameter {
	p := &openapi3.Parameter{Name: name, In: in, Required: required}
	var ex any = example
	schema := &openapi3.Schema{Type: &openapi3.Types{"string"}}
	if n, err := strconv.ParseInt(example, 10, 64); err == nil && in != "header" {
		schema = &openapi3.Schema{Type: &openapi3.Types{"integer"}}
		ex = n
	}
	if example != "" && !strings.Contains(example, "{{") {
		p.Example = ex
	}
	p.Schema = &openapi3.SchemaRef{Value: schema}
	return p
}

func sortParams(op *openapi3.Operation) {
	order := map[string]int{"path": 0, "query": 1, "header": 2, "cookie": 3}
	sort.SliceStable(op.Parameters, func(i, j int) bool {
		return order[op.Parameters[i].Value.In] < order[op.Parameters[j].Value.In]
	})
}

func addAuth(spec *openapi3.T, op *openapi3.Operation, value string, res *Result) {
	scheme, _, _ := strings.Cut(strings.TrimSpace(value), " ")
	switch strings.ToLower(scheme) {
	case "bearer":
		addSecurity(spec, op, "bearerAuth", &openapi3.SecurityScheme{Type: "http", Scheme: "bearer"})
		res.note("bearer tokens were not copied into the spec; set one in lazyapi.config.yml under auth.bearerAuth.token")
	case "basic":
		addSecurity(spec, op, "basicAuth", &openapi3.SecurityScheme{Type: "http", Scheme: "basic"})
		res.note("basic credentials were not copied into the spec; set them in lazyapi.config.yml under auth.basicAuth")
	default:
		addSecurity(spec, op, "apiKeyAuth", &openapi3.SecurityScheme{Type: "apiKey", In: "header", Name: "Authorization"})
		res.note("the Authorization value was not copied into the spec; set it in lazyapi.config.yml under auth.apiKeyAuth.value")
	}
}

func addSecurity(spec *openapi3.T, op *openapi3.Operation, name string, scheme *openapi3.SecurityScheme) {
	if spec.Components == nil {
		spec.Components = &openapi3.Components{}
	}
	if spec.Components.SecuritySchemes == nil {
		spec.Components.SecuritySchemes = openapi3.SecuritySchemes{}
	}
	if _, ok := spec.Components.SecuritySchemes[name]; !ok {
		spec.Components.SecuritySchemes[name] = &openapi3.SecuritySchemeRef{Value: scheme}
	}
	if op.Security == nil {
		op.Security = openapi3.NewSecurityRequirements()
	}
	for _, req := range *op.Security {
		if _, ok := req[name]; ok {
			return
		}
	}
	op.Security.With(openapi3.NewSecurityRequirement().Authenticate(name))
}

func requestBody(r Request, contentType string) *openapi3.RequestBody {
	media := r.BodyType
	if media == "" {
		media = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}
	if len(r.Form) > 0 {
		if media == "" || !strings.Contains(media, "form") {
			media = string(model.FormURLEncoded)
			for _, f := range r.Form {
				if f.File {
					media = string(model.MultipartForm)
				}
			}
		}
		props := openapi3.Schemas{}
		example := map[string]any{}
		for _, f := range r.Form {
			s := &openapi3.Schema{Type: &openapi3.Types{"string"}}
			if f.File {
				s.Format = "binary"
			} else {
				example[f.Key] = f.Value
			}
			props[f.Key] = &openapi3.SchemaRef{Value: s}
		}
		return &openapi3.RequestBody{Content: openapi3.Content{media: &openapi3.MediaType{
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"object"}, Properties: props}}, Example: example,
		}}}
	}
	if strings.TrimSpace(r.Body) == "" {
		return nil
	}
	value, isJSON := parseJSON(r.Body)
	if media == "" {
		media = "text/plain"
		if isJSON {
			media = "application/json"
		}
	}
	mt := &openapi3.MediaType{Example: value}
	if !isJSON {
		mt.Example = r.Body
	}
	mt.Schema = &openapi3.SchemaRef{Value: InferSchema(mt.Example)}
	return &openapi3.RequestBody{Content: openapi3.Content{media: mt}}
}

func addResponse(op *openapi3.Operation, r Response) {
	code := strconv.Itoa(r.Status)
	if op.Responses.Value(code) != nil {
		return // keep the first example per status
	}
	desc := http.StatusText(r.Status)
	if desc == "" {
		desc = "Response " + code
	}
	resp := &openapi3.Response{Description: &desc}
	media := strings.TrimSpace(strings.Split(r.ContentType, ";")[0])
	if strings.TrimSpace(r.Body) != "" {
		value, isJSON := parseJSON(r.Body)
		if media == "" {
			media = "text/plain"
			if isJSON {
				media = "application/json"
			}
		}
		var example any = r.Body
		if isJSON {
			example = value
		}
		resp.Content = openapi3.Content{media: &openapi3.MediaType{
			Example: example, Schema: &openapi3.SchemaRef{Value: InferSchema(example)},
		}}
	}
	op.Responses.Set(code, &openapi3.ResponseRef{Value: resp})
}

func parseJSON(s string) (any, bool) {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, false
	}
	return v, true
}

var (
	dateTime = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}`)
	email    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	uuidFmt  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// InferSchema derives a schema from an example value. Properties are not
// marked required: one example does not say which fields are optional, and a
// too-strict schema would make the contract check fail on valid responses.
func InferSchema(v any) *openapi3.Schema {
	switch t := v.(type) {
	case nil:
		return &openapi3.Schema{Nullable: true}
	case bool:
		return &openapi3.Schema{Type: &openapi3.Types{"boolean"}}
	case float64:
		if t == float64(int64(t)) {
			return &openapi3.Schema{Type: &openapi3.Types{"integer"}}
		}
		return &openapi3.Schema{Type: &openapi3.Types{"number"}}
	case string:
		s := &openapi3.Schema{Type: &openapi3.Types{"string"}}
		switch {
		case uuidFmt.MatchString(t):
			s.Format = "uuid"
		case dateTime.MatchString(t):
			s.Format = "date-time"
		case email.MatchString(t):
			s.Format = "email"
		}
		return s
	case []any:
		items := &openapi3.Schema{}
		if len(t) > 0 {
			items = InferSchema(t[0])
		}
		return &openapi3.Schema{Type: &openapi3.Types{"array"}, Items: &openapi3.SchemaRef{Value: items}}
	case map[string]any:
		props := openapi3.Schemas{}
		for k, x := range t {
			props[k] = &openapi3.SchemaRef{Value: InferSchema(x)}
		}
		return &openapi3.Schema{Type: &openapi3.Types{"object"}, Properties: props}
	}
	return &openapi3.Schema{}
}

// NewSpec returns an empty OpenAPI document.
func NewSpec(title string) *openapi3.T {
	if title == "" {
		title = "Imported API"
	}
	return &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: title, Version: "1.0.0"},
		Paths:   openapi3.NewPaths(),
	}
}
