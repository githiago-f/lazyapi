package model

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func yamlStringNode(t *testing.T, value string) *yaml.Node {
	t.Helper()
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}
}

func TestMethodLabel(t *testing.T) {
	tests := []struct {
		method Method
		label  string
	}{
		{POST, "POST"},
		{GET, "GET"},
		{PATCH, "PATCH"},
		{PUT, "PUT"},
		{DELETE, "DELETE"},
		{OPTIONS, "OPTIONS"},
		{HEAD, "HEAD"},
	}
	for _, tt := range tests {
		got := tt.method.Label()
		if got != tt.label {
			t.Errorf("Method(%d).Label() = %q, want %q", tt.method, got, tt.label)
		}
	}
}

func TestMethodLabel_Unknown(t *testing.T) {
	m := Method(99)
	if got := m.Label(); got != "" {
		t.Errorf("Method(99).Label() = %q, want empty", got)
	}
}

func TestMethodFromLabel(t *testing.T) {
	tests := []struct {
		label  string
		method Method
	}{
		{"POST", POST},
		{"GET", GET},
		{"PATCH", PATCH},
		{"PUT", PUT},
		{"DELETE", DELETE},
		{"OPTIONS", OPTIONS},
		{"HEAD", HEAD},
	}
	for _, tt := range tests {
		got := MethodFromLabel(tt.label)
		if got != tt.method {
			t.Errorf("MethodFromLabel(%q) = %d, want %d", tt.label, got, tt.method)
		}
	}
}

func TestMethodFromLabel_Default(t *testing.T) {
	if got := MethodFromLabel("UNKNOWN"); got != GET {
		t.Errorf("MethodFromLabel(\"UNKNOWN\") = %d, want GET (%d)", got, GET)
	}
}

func TestMethodFromLabel_CaseInsensitive(t *testing.T) {
	if got := MethodFromLabel("get"); got != GET {
		t.Errorf("MethodFromLabel(\"get\") = %d, want GET", got)
	}
	if got := MethodFromLabel("Get"); got != GET {
		t.Errorf("MethodFromLabel(\"Get\") = %d, want GET", got)
	}
	if got := MethodFromLabel("GET"); got != GET {
		t.Errorf("MethodFromLabel(\"GET\") = %d, want GET", got)
	}
}

func TestMethodLabelRoundtrip(t *testing.T) {
	methods := []Method{POST, GET, PATCH, PUT, DELETE, OPTIONS, HEAD}
	for _, m := range methods {
		label := m.Label()
		back := MethodFromLabel(label)
		if back != m {
			t.Errorf("roundtrip failed for Method(%d): Label() = %q, FromLabel = %d", m, label, back)
		}
	}
}

func TestLastMethod(t *testing.T) {
	if LastMethod != HEAD {
		t.Errorf("LastMethod = %d, want HEAD (%d)", LastMethod, HEAD)
	}
}

func TestMethodMarshalYAML(t *testing.T) {
	tests := []struct {
		method Method
		want   string
	}{
		{POST, "post"},
		{GET, "get"},
		{PATCH, "patch"},
		{PUT, "put"},
		{DELETE, "delete"},
		{OPTIONS, "options"},
		{HEAD, "head"},
	}
	for _, tt := range tests {
		got, err := tt.method.MarshalYAML()
		if err != nil {
			t.Errorf("Method(%d).MarshalYAML() error: %v", tt.method, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Method(%d).MarshalYAML() = %v, want %q", tt.method, got, tt.want)
		}
	}
}

func TestMethodUnmarshalYAML(t *testing.T) {
	tests := []struct {
		yaml   string
		method Method
	}{
		{"post", POST},
		{"POST", POST},
		{"Post", POST},
		{"get", GET},
		{"GET", GET},
		{"patch", PATCH},
		{"put", PUT},
		{"delete", DELETE},
		{"options", OPTIONS},
		{"head", HEAD},
	}
	for _, tt := range tests {
		var m Method
		if err := m.UnmarshalYAML(yamlStringNode(t, tt.yaml)); err != nil {
			t.Errorf("UnmarshalYAML(%q) error: %v", tt.yaml, err)
			continue
		}
		if m != tt.method {
			t.Errorf("UnmarshalYAML(%q) = %d, want %d", tt.yaml, m, tt.method)
		}
	}
}

func TestMethodYAMLRoundtrip(t *testing.T) {
	methods := []Method{POST, GET, PATCH, PUT, DELETE, OPTIONS, HEAD}
	for _, m := range methods {
		serialized, err := m.MarshalYAML()
		if err != nil {
			t.Fatalf("MarshalYAML: %v", err)
		}
		str, ok := serialized.(string)
		if !ok {
			t.Fatalf("MarshalYAML returned non-string: %T", serialized)
		}
		var deserialized Method
		if err := deserialized.UnmarshalYAML(yamlStringNode(t, str)); err != nil {
			t.Fatalf("UnmarshalYAML: %v", err)
		}
		if deserialized != m {
			t.Errorf("YAML roundtrip failed for Method(%d): got %d", m, deserialized)
		}
	}
}
