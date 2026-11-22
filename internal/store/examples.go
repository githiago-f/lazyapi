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
