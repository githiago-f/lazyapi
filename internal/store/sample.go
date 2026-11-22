package store

import (
	"github.com/getkin/kin-openapi/openapi3"
)

// maxSampleDepth stops recursive schemas (trees, linked lists).
const maxSampleDepth = 6

// SchemaSample builds a plausible value for a schema: declared example,
// default or first enum value when present, otherwise a value derived from
// the type and format. The result validates against simple schemas; it is
// meant for mock responses and request templates, not for fuzzing.
func SchemaSample(s *openapi3.SchemaRef) any {
	return sample(s, 0)
}

func sample(ref *openapi3.SchemaRef, depth int) any {
	if ref == nil || ref.Value == nil || depth > maxSampleDepth {
		return nil
	}
	s := ref.Value
	switch {
	case s.Example != nil:
		return s.Example
	case s.Default != nil:
		return s.Default
	case len(s.Enum) > 0:
		return s.Enum[0]
	}

	if len(s.AllOf) > 0 {
		merged := map[string]any{}
		for _, part := range s.AllOf {
			if m, ok := sample(part, depth+1).(map[string]any); ok {
				for k, v := range m {
					merged[k] = v
				}
			}
		}
		if obj, ok := objectSample(s, depth).(map[string]any); ok {
			for k, v := range obj {
				merged[k] = v
			}
		}
		return merged
	}
	if len(s.OneOf) > 0 {
		return sample(s.OneOf[0], depth+1)
	}
	if len(s.AnyOf) > 0 {
		return sample(s.AnyOf[0], depth+1)
	}

	switch {
	case s.Type.Is("object") || (s.Type == nil && len(s.Properties) > 0):
		return objectSample(s, depth)
	case s.Type.Is("array"):
		item := sample(s.Items, depth+1)
		if item == nil {
			return []any{}
		}
		return []any{item}
	case s.Type.Is("string"):
		return stringSample(s)
	case s.Type.Is("integer"):
		return numberSample(s, true)
	case s.Type.Is("number"):
		return numberSample(s, false)
	case s.Type.Is("boolean"):
		return true
	}
	return nil
}

func objectSample(s *openapi3.Schema, depth int) any {
	obj := map[string]any{}
	for name, prop := range s.Properties {
		if v := sample(prop, depth+1); v != nil {
			obj[name] = v
		}
	}
	return obj
}

func stringSample(s *openapi3.Schema) string {
	switch s.Format {
	case "date-time":
		return "2024-01-01T00:00:00Z"
	case "date":
		return "2024-01-01"
	case "time":
		return "00:00:00"
	case "email":
		return "user@example.com"
	case "uuid":
		return "00000000-0000-4000-8000-000000000000"
	case "uri", "url":
		return "https://example.com"
	case "hostname":
		return "example.com"
	case "ipv4":
		return "192.0.2.1"
	case "ipv6":
		return "2001:db8::1"
	case "byte":
		return "c3RyaW5n"
	}
	v := "string"
	for uint64(len(v)) < s.MinLength {
		v += "-"
	}
	return v
}

func numberSample(s *openapi3.Schema, integer bool) any {
	n := 0.0
	switch {
	case s.ExclusiveMin.Value != nil: // OpenAPI 3.1: exclusiveMinimum is the bound
		n = *s.ExclusiveMin.Value + 1
	case s.Min != nil:
		n = *s.Min
		if s.ExclusiveMin.IsTrue() {
			n++
		}
	case s.Max != nil && *s.Max < 0:
		n = *s.Max
	}
	if integer {
		return int64(n)
	}
	return n
}
