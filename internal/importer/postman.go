package importer

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/project"
)

// Extras are side outputs of an import besides operations.
type Extras struct {
	Title string
	// Variables are Postman collection variables, written to .env.postman.
	Variables map[string]string
}

type pmCollection struct {
	Info     struct{ Name string } `json:"info"`
	Item     []pmItem              `json:"item"`
	Variable []pmKV                `json:"variable"`
	Auth     *pmAuth               `json:"auth"`
	Event    []pmEvent             `json:"event"`
}

type pmItem struct {
	Name        string          `json:"name"`
	Description json.RawMessage `json:"description"`
	Item        []pmItem        `json:"item"`
	Request     json.RawMessage `json:"request"`
	Response    []pmResponse    `json:"response"`
	Event       []pmEvent       `json:"event"`
	Auth        *pmAuth         `json:"auth"`
}

type pmRequest struct {
	Method      string          `json:"method"`
	Header      []pmKV          `json:"header"`
	URL         json.RawMessage `json:"url"`
	Body        *pmBody         `json:"body"`
	Auth        *pmAuth         `json:"auth"`
	Description json.RawMessage `json:"description"`
}

type pmKV struct {
	Key      string `json:"key"`
	Value    any    `json:"value"`
	Disabled bool   `json:"disabled"`
	Type     string `json:"type"`
	Src      any    `json:"src"`
	In       string `json:"in"`
}

func (kv pmKV) value() string {
	switch v := kv.Value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

type pmBody struct {
	Mode       string `json:"mode"`
	Raw        string `json:"raw"`
	URLEncoded []pmKV `json:"urlencoded"`
	FormData   []pmKV `json:"formdata"`
	GraphQL    *struct {
		Query     string `json:"query"`
		Variables string `json:"variables"`
	} `json:"graphql"`
	Options struct {
		Raw struct {
			Language string `json:"language"`
		} `json:"raw"`
	} `json:"options"`
}

type pmAuth struct {
	Type   string `json:"type"`
	Bearer []pmKV `json:"bearer"`
	Basic  []pmKV `json:"basic"`
	APIKey []pmKV `json:"apikey"`
}

type pmEvent struct {
	Listen string `json:"listen"`
	Script struct {
		Exec json.RawMessage `json:"exec"`
	} `json:"script"`
}

type pmResponse struct {
	Code   int    `json:"code"`
	Header []pmKV `json:"header"`
	Body   string `json:"body"`
}

type pmURL struct {
	Raw      string `json:"raw"`
	Protocol string `json:"protocol"`
	Host     any    `json:"host"`
	Path     any    `json:"path"`
	Query    []pmKV `json:"query"`
}

var pmVar = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.\-]+)\s*\}\}`)

// ParsePostman reads a Postman collection (v2.0 or v2.1) into requests.
// Folders become tags, auth becomes headers (Apply turns them into security
// schemes without copying the secret), pre-request and test scripts become
// a LazyAPI test script that uses the `pm` compatibility layer, and
// {{variables}} become {{env.variables}} (values go to .env.postman).
func ParsePostman(data []byte) ([]Request, Extras, error) {
	return ParsePostmanWith(data, nil)
}

// ParsePostmanWith is ParsePostman with an environment's values available
// to resolve {{variables}} in request hosts (e.g. {{baseUrl}} defined only
// in the environment). Environment values win over collection variables;
// they are not added to Extras.Variables.
func ParsePostmanWith(data []byte, environment map[string]string) ([]Request, Extras, error) {
	var c pmCollection
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, Extras{}, fmt.Errorf("not a Postman collection: %w", err)
	}
	if len(c.Item) == 0 {
		return nil, Extras{}, errors.New("the collection has no requests")
	}
	extras := Extras{Title: c.Info.Name, Variables: map[string]string{}}
	for _, v := range c.Variable {
		if !v.Disabled && v.Key != "" {
			extras.Variables[v.Key] = v.value()
		}
	}

	resolve := map[string]string{}
	for k, v := range extras.Variables {
		resolve[k] = v
	}
	for k, v := range environment {
		resolve[k] = v
	}

	var out []Request
	var walk func(items []pmItem, tags []string, auth *pmAuth, pre, tests []string)
	walk = func(items []pmItem, tags []string, auth *pmAuth, pre, tests []string) {
		for _, it := range items {
			itemAuth := auth
			if it.Auth != nil {
				itemAuth = it.Auth
			}
			itemPre := append(append([]string{}, pre...), scripts(it.Event, "prerequest")...)
			itemTests := append(append([]string{}, tests...), scripts(it.Event, "test")...)
			if len(it.Request) == 0 || string(it.Request) == "null" {
				walk(it.Item, append(append([]string{}, tags...), it.Name), itemAuth, itemPre, itemTests)
				continue
			}
			r, err := postmanRequest(it, itemAuth, resolve)
			if err != nil {
				continue
			}
			if len(tags) > 0 {
				r.Tags = []string{tags[len(tags)-1]}
			}
			r.Tests = postmanScript(itemPre, itemTests)
			out = append(out, r)
		}
	}
	walk(c.Item, nil, c.Auth, scripts(c.Event, "prerequest"), scripts(c.Event, "test"))
	return out, extras, nil
}

func postmanRequest(it pmItem, auth *pmAuth, vars map[string]string) (Request, error) {
	var pr pmRequest
	if err := json.Unmarshal(it.Request, &pr); err != nil {
		// v2.0 allows a bare URL string as the request.
		var raw string
		if json.Unmarshal(it.Request, &raw) != nil {
			return Request{}, err
		}
		pr = pmRequest{Method: "GET", URL: json.RawMessage(strconvQuote(raw))}
	}
	r := Request{Name: it.Name, Method: strings.ToUpper(pr.Method), Description: text(it.Description)}
	if r.Description == "" {
		r.Description = text(pr.Description)
	}
	r.URL = postmanURL(pr.URL, vars)

	for _, h := range pr.Header {
		if !h.Disabled && h.Key != "" {
			r.Headers = append(r.Headers, KV{Key: h.Key, Value: envVars(h.value())})
		}
	}
	if pr.Auth != nil {
		auth = pr.Auth
	}
	r.Headers = append(r.Headers, authHeaders(auth)...)

	if b := pr.Body; b != nil {
		switch b.Mode {
		case "raw":
			r.Body = envVars(b.Raw)
			if b.Options.Raw.Language == "json" || looksJSON(b.Raw) {
				r.BodyType = "application/json"
			}
		case "urlencoded":
			r.BodyType = "application/x-www-form-urlencoded"
			for _, f := range b.URLEncoded {
				if !f.Disabled {
					r.Form = append(r.Form, KV{Key: f.Key, Value: envVars(f.value())})
				}
			}
		case "formdata":
			r.BodyType = "multipart/form-data"
			for _, f := range b.FormData {
				if f.Disabled {
					continue
				}
				if f.Type == "file" {
					r.Form = append(r.Form, KV{Key: f.Key, Value: fmt.Sprint(f.Src), File: true})
				} else {
					r.Form = append(r.Form, KV{Key: f.Key, Value: envVars(f.value())})
				}
			}
		case "graphql":
			if b.GraphQL != nil {
				body := map[string]any{"query": b.GraphQL.Query}
				var v any
				if json.Unmarshal([]byte(b.GraphQL.Variables), &v) == nil {
					body["variables"] = v
				}
				data, _ := json.Marshal(body)
				r.Body, r.BodyType = string(data), "application/json"
			}
		}
	}

	for _, resp := range it.Response {
		ct := ""
		for _, h := range resp.Header {
			if strings.EqualFold(h.Key, "Content-Type") {
				ct = h.value()
			}
		}
		if resp.Code > 0 {
			r.Responses = append(r.Responses, Response{Status: resp.Code, ContentType: ct, Body: resp.Body})
		}
	}
	return r, nil
}

// postmanURL rebuilds the URL, resolving collection variables in the host
// (e.g. {{baseUrl}}) and keeping path variables as {{name}} / :name.
func postmanURL(raw json.RawMessage, vars map[string]string) string {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		var u pmURL
		if json.Unmarshal(raw, &u) != nil {
			return ""
		}
		s = u.Raw
		if s == "" {
			host := strings.Join(toStrings(u.Host), ".")
			s = host + "/" + strings.Join(toStrings(u.Path), "/")
			if u.Protocol != "" {
				s = u.Protocol + "://" + s
			}
		}
	}
	// Resolve variables up to the first path segment (scheme://host[/base]).
	resolved := pmVar.ReplaceAllStringFunc(s, func(m string) string {
		name := pmVar.FindStringSubmatch(m)[1]
		if v, ok := vars[name]; ok && strings.Contains(v, "://") {
			return strings.TrimRight(v, "/")
		}
		return m
	})
	if !strings.Contains(resolved, "://") && !strings.HasPrefix(resolved, "{{") {
		resolved = "https://" + resolved
	}
	// Query values that are variables become {{env.x}}.
	if base, query, ok := strings.Cut(resolved, "?"); ok {
		resolved = base + "?" + envVars(query)
	}
	return resolved
}

func authHeaders(a *pmAuth) []KV {
	if a == nil {
		return nil
	}
	get := func(kvs []pmKV, key string) string {
		for _, kv := range kvs {
			if kv.Key == key {
				return envVars(kv.value())
			}
		}
		return ""
	}
	switch a.Type {
	case "bearer":
		return []KV{{Key: "Authorization", Value: "Bearer " + get(a.Bearer, "token")}}
	case "basic":
		creds := get(a.Basic, "username") + ":" + get(a.Basic, "password")
		return []KV{{Key: "Authorization", Value: "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))}}
	case "apikey":
		if get(a.APIKey, "in") == "query" {
			return nil // query API keys are kept as query parameters
		}
		name := get(a.APIKey, "key")
		if name == "" {
			name = "X-API-Key"
		}
		return []KV{{Key: name, Value: get(a.APIKey, "value")}}
	}
	return nil
}

// envVars rewrites Postman {{name}} placeholders as {{env.name}}; dynamic
// variables such as {{$guid}} are left untouched.
func envVars(s string) string {
	return pmVar.ReplaceAllStringFunc(s, func(m string) string {
		name := pmVar.FindStringSubmatch(m)[1]
		return "{{env." + name + "}}"
	})
}

func scripts(events []pmEvent, listen string) []string {
	var out []string
	for _, e := range events {
		if e.Listen != listen {
			continue
		}
		var lines []string
		if json.Unmarshal(e.Script.Exec, &lines) != nil {
			var one string
			if json.Unmarshal(e.Script.Exec, &one) == nil {
				lines = []string{one}
			}
		}
		if src := strings.TrimSpace(strings.Join(lines, "\n")); src != "" {
			out = append(out, src)
		}
	}
	return out
}

// postmanScript translates Postman scripts to the LazyAPI API: pre-request
// code runs in before(req), test code at the top level.
func postmanScript(pre, tests []string) string {
	if len(pre) == 0 && len(tests) == 0 {
		return ""
	}
	var b strings.Builder
	todos := 0
	var body strings.Builder
	if len(pre) > 0 {
		code, n := translatePostman(strings.Join(pre, "\n\n"), true)
		todos += n
		body.WriteString("export function before(req) {\n")
		for _, line := range strings.Split(code, "\n") {
			body.WriteString("  " + line + "\n")
		}
		body.WriteString("}\n\n")
	}
	if len(tests) > 0 {
		code, n := translatePostman(strings.Join(tests, "\n\n"), false)
		todos += n
		body.WriteString(code + "\n")
	}
	b.WriteString("// Imported from Postman and translated to the LazyAPI script API (see `lazyapi types`).\n")
	if todos > 0 {
		fmt.Fprintf(&b, "// %d line(s) marked TODO(postman) could not be translated: rewrite them.\n", todos)
	}
	b.WriteString("\n")
	b.WriteString(body.String())
	return b.String()
}

func text(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		Content string `json:"content"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.Content
	}
	return ""
}

func toStrings(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return out
	}
	return nil
}

func strconvQuote(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

// WriteExtras writes collection variables to .env.postman next to the spec
// and, when there is no lazyapi.config.yml, an environment that uses it.
// Existing files are never overwritten.
func WriteExtras(specPath string, x Extras, res *Result) error {
	if len(x.Variables) == 0 {
		return nil
	}
	dir := filepath.Dir(specPath)
	envPath := filepath.Join(dir, ".env.postman")
	if _, err := os.Stat(envPath); err == nil {
		res.note("%s exists and was not changed; collection variables: %s", envPath, strings.Join(sortedKeys(x.Variables), ", "))
		return nil
	}
	var b strings.Builder
	b.WriteString("# Postman collection variables (used as {{env.NAME}}). May contain secrets: do not commit.\n")
	for _, k := range sortedKeys(x.Variables) {
		fmt.Fprintf(&b, "%s=%s\n", k, x.Variables[k])
	}
	if err := os.WriteFile(envPath, []byte(b.String()), 0o600); err != nil {
		return err
	}
	res.Files = append(res.Files, envPath)
	res.note("collection variables written to %s; add it to .gitignore", envPath)

	cfgPath := filepath.Join(dir, project.FileName)
	if _, err := os.Stat(cfgPath); err == nil {
		res.note("use them with: --env %s (lazyapi.config.yml exists and was not changed)", envPath)
		for _, scheme := range sortedAuth(res.Auth) {
			res.note("add auth.%s to your environment to authenticate: %s", scheme, authHint(res.Auth[scheme]))
		}
		return nil
	}
	if _, err := project.AddEnvironment(cfgPath, "postman", project.Environment{Env: ".env.postman", Auth: res.Auth}); err != nil {
		return err
	}
	res.Files = append(res.Files, cfgPath)
	res.note("use them with: lazyapi run %s --env-name postman", specPath)
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// authHint shows the config lines for credentials that are references.
func authHint(a model.AuthSecrets) string {
	var parts []string
	for _, kv := range [][2]string{{"token", a.Token}, {"username", a.Username}, {"password", a.Password}, {"value", a.Value}} {
		if kv[1] != "" {
			parts = append(parts, fmt.Sprintf("%s: %q", kv[0], kv[1]))
		}
	}
	return strings.Join(parts, ", ")
}
