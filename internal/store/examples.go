package store

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
)

// ParamExample returns the best sample value declared for a parameter, in
// order: example, first named example, schema example, schema default,
// first schema enum value. It returns "" when nothing is declared.
func ParamExample(p *openapi3.Parameter) string {
	if p == nil {
		return ""
	}
	if p.Example != nil {
		return formatExample(p.Example)
	}
	if len(p.Examples) > 0 {
		names := make([]string, 0, len(p.Examples))
		for name := range p.Examples {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			if ex := p.Examples[name]; ex != nil && ex.Value != nil && ex.Value.Value != nil {
				return formatExample(ex.Value.Value)
			}
		}
	}
	if p.Schema != nil && p.Schema.Value != nil {
		s := p.Schema.Value
		switch {
		case s.Example != nil:
			return formatExample(s.Example)
		case s.Default != nil:
			return formatExample(s.Default)
		case len(s.Enum) > 0:
			return formatExample(s.Enum[0])
		}
	}
	return ""
}

func formatExample(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool, int, int64:
		return fmt.Sprint(val)
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprint(val)
		}
		return string(data)
	}
}

// DeclaredStatuses lists the response codes declared for an operation
// (e.g. "200", "4XX", "default"), sorted. Nil if the operation is unknown or
// declares no responses.
func DeclaredStatuses(spec *openapi3.T, ref model.OpenAPIRef) []string {
	op := FindOperation(spec, ref)
	if op == nil || op.Responses == nil {
		return nil
	}
	codes := make([]string, 0, op.Responses.Len())
	for code := range op.Responses.Map() {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}

// StatusDeclared reports whether code matches one of the declared response
// codes. Ranges ("2XX") and "default" are honoured. When nothing is declared
// any status below 400 is accepted.
func StatusDeclared(declared []string, code int) bool {
	if len(declared) == 0 {
		return code < 400
	}
	exact := strconv.Itoa(code)
	rng := exact[:1] + "XX"
	for _, d := range declared {
		switch strings.ToUpper(d) {
		case exact, rng, "DEFAULT":
			return true
		}
	}
	return false
}

// FindOperation returns the operation addressed by ref, or nil.
func FindOperation(spec *openapi3.T, ref model.OpenAPIRef) *openapi3.Operation {
	if spec == nil || spec.Paths == nil {
		return nil
	}
	pathItem := spec.Paths.Find(ref.Path)
	if pathItem == nil {
		return nil
	}
	return pathItem.GetOperation(strings.ToUpper(ref.Method))
}

// BodyExample returns the media type and example body declared for an
// operation's request body. JSON media types are preferred; the example comes
// from the media type's example, its first named example, or the schema's
// example. raw is "" when nothing is declared.
func BodyExample(op *openapi3.Operation) (mediaType, raw string) {
	if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
		return "", ""
	}
	content := op.RequestBody.Value.Content
	types := make([]string, 0, len(content))
	for mt := range content {
		types = append(types, mt)
	}
	slices.SortFunc(types, func(a, b string) int {
		aj, bj := strings.Contains(a, "json"), strings.Contains(b, "json")
		switch {
		case aj && !bj:
			return -1
		case bj && !aj:
			return 1
		}
		return strings.Compare(a, b)
	})
	for _, mt := range types {
		m := content[mt]
		if m == nil {
			continue
		}
		var ex any
		switch {
		case m.Example != nil:
			ex = m.Example
		case len(m.Examples) > 0:
			names := make([]string, 0, len(m.Examples))
			for name := range m.Examples {
				names = append(names, name)
			}
			slices.Sort(names)
			for _, name := range names {
				if e := m.Examples[name]; e != nil && e.Value != nil && e.Value.Value != nil {
					ex = e.Value.Value
					break
				}
			}
		case m.Schema != nil && m.Schema.Value != nil && m.Schema.Value.Example != nil:
			ex = m.Schema.Value.Example
		}
		if ex == nil {
			if mediaType == "" {
				mediaType = mt
			}
			continue
		}
		if s, ok := ex.(string); ok && !strings.Contains(mt, "json") {
			return mt, s
		}
		data, err := json.MarshalIndent(ex, "", "  ")
		if err != nil {
			continue
		}
		return mt, string(data)
	}
	return mediaType, ""
}

// SampleBody is BodyExample with a fallback: when the operation declares a
// body schema but no example, it returns a value built by SchemaSample. It
// is meant for editors (the user sees the body before sending), never for
// `run`, which must not invent data.
func SampleBody(op *openapi3.Operation) (mediaType, raw string) {
	mediaType, raw = BodyExample(op)
	if raw != "" || mediaType == "" {
		return mediaType, raw
	}
	m := op.RequestBody.Value.Content[mediaType]
	if m == nil || m.Schema == nil {
		return mediaType, ""
	}
	v := SchemaSample(m.Schema)
	if v == nil {
		return mediaType, ""
	}
	if s, ok := v.(string); ok && !strings.Contains(mediaType, "json") {
		return mediaType, s
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mediaType, ""
	}
	return mediaType, string(data)
}

// FillSampleBody sets a synthetic body (SampleBody) on req when it has none.
func FillSampleBody(spec *openapi3.T, req *model.Request, ref model.OpenAPIRef) {
	if req.Body.Raw != "" {
		return
	}
	if mt, raw := SampleBody(FindOperation(spec, ref)); raw != "" {
		req.Body.Type = model.MimeType(mt)
		req.Body.Raw = raw
	}
}

// ResponseExample returns the example declared for a response status (the
// media type example, else the first named example, else the schema's), as
// text: JSON values are indented. ok is false when there is none.
func ResponseExample(op *openapi3.Operation, status int) (mediaType, body string, ok bool) {
	if op == nil || op.Responses == nil {
		return "", "", false
	}
	ref := op.Responses.Value(strconv.Itoa(status))
	if ref == nil {
		ref = op.Responses.Value(fmt.Sprintf("%dXX", status/100))
	}
	if ref == nil {
		ref = op.Responses.Default()
	}
	if ref == nil || ref.Value == nil {
		return "", "", false
	}
	types := make([]string, 0, len(ref.Value.Content))
	for mt := range ref.Value.Content {
		types = append(types, mt)
	}
	slices.Sort(types)
	for _, mt := range types {
		m := ref.Value.Content[mt]
		if m == nil {
			continue
		}
		var ex any
		switch {
		case m.Example != nil:
			ex = m.Example
		case len(m.Examples) > 0:
			names := make([]string, 0, len(m.Examples))
			for n := range m.Examples {
				names = append(names, n)
			}
			slices.Sort(names)
			for _, n := range names {
				if e := m.Examples[n]; e != nil && e.Value != nil && e.Value.Value != nil {
					ex = e.Value.Value
					break
				}
			}
		case m.Schema != nil && m.Schema.Value != nil && m.Schema.Value.Example != nil:
			ex = m.Schema.Value.Example
		}
		if ex == nil {
			continue
		}
		if s, isStr := ex.(string); isStr {
			return mt, s, true
		}
		data, err := json.MarshalIndent(ex, "", "  ")
		if err != nil {
			continue
		}
		return mt, string(data), true
	}
	return "", "", false
}

// BodyTypes lists the request body media types an operation declares,
// sorted (nil when it has no request body).
func BodyTypes(spec *openapi3.T, ref model.OpenAPIRef) []string {
	op := FindOperation(spec, ref)
	if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
		return nil
	}
	types := make([]string, 0, len(op.RequestBody.Value.Content))
	for mt := range op.RequestBody.Value.Content {
		types = append(types, mt)
	}
	slices.Sort(types)
	return types
}
