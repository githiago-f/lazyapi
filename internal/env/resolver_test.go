package env

import "testing"

func TestResolve_EnvVar(t *testing.T) {
	result := Resolve("{{env.HOST}}", Context{Env: map[string]string{"HOST": "localhost"}})
	if result != "localhost" {
		t.Errorf("Resolve = %q, want %q", result, "localhost")
	}
}

func TestResolve_Var(t *testing.T) {
	result := Resolve("{{var.id}}", Context{Vars: map[string]string{"id": "42"}})
	if result != "42" {
		t.Errorf("Resolve = %q, want %q", result, "42")
	}
}

func TestResolve_Multiple(t *testing.T) {
	ctx := Context{Env: map[string]string{"HOST": "localhost", "PORT": "8080"}}
	result := Resolve("{{env.HOST}}:{{env.PORT}}", ctx)
	if result != "localhost:8080" {
		t.Errorf("Resolve = %q, want %q", result, "localhost:8080")
	}
}

func TestResolve_Mixed(t *testing.T) {
	ctx := Context{
		Vars: map[string]string{"id": "42"},
		Env:  map[string]string{"HOST": "local"},
	}
	result := Resolve("{{var.id}}-{{env.HOST}}", ctx)
	if result != "42-local" {
		t.Errorf("Resolve = %q, want %q", result, "42-local")
	}
}

func TestResolve_LazyapiPrefix(t *testing.T) {
	result := Resolve("{{lazyapi.version}}", Context{})
	if result != "" {
		t.Errorf("Resolve with lazyapi prefix = %q, want empty", result)
	}
}

func TestResolve_UnknownPrefix(t *testing.T) {
	result := Resolve("{{unknown.foo}}", Context{})
	if result != "{{unknown.foo}}" {
		t.Errorf("Resolve with unknown prefix = %q, want unchanged", result)
	}
}

func TestResolve_Unresolved(t *testing.T) {
	result := Resolve("{{env.MISSING}}", Context{Env: map[string]string{"OTHER": "val"}})
	if result != "" {
		t.Errorf("Resolve for missing key = %q, want empty", result)
	}
}

func TestResolve_NilContext(t *testing.T) {
	if got := Resolve("{{env.X}}", Context{}); got != "" {
		t.Errorf("Resolve with nil Env = %q, want empty", got)
	}
	if got := Resolve("{{var.X}}", Context{}); got != "" {
		t.Errorf("Resolve with nil Vars = %q, want empty", got)
	}
}

func TestResolve_EmptyInput(t *testing.T) {
	result := Resolve("", Context{})
	if result != "" {
		t.Errorf("Resolve empty = %q, want empty", result)
	}
}

func TestResolve_NoTemplates(t *testing.T) {
	result := Resolve("hello world", Context{})
	if result != "hello world" {
		t.Errorf("Resolve plain = %q, want %q", result, "hello world")
	}
}

func TestResolve_PartialTemplateInString(t *testing.T) {
	ctx := Context{Env: map[string]string{"X": "mid"}}
	result := Resolve("prefix_{{env.X}}_suffix", ctx)
	if result != "prefix_mid_suffix" {
		t.Errorf("Resolve partial = %q, want %q", result, "prefix_mid_suffix")
	}
}

func TestResolve_SameValueMultipleTimes(t *testing.T) {
	ctx := Context{Env: map[string]string{"NAME": "test"}}
	result := Resolve("{{env.NAME}}/{{env.NAME}}", ctx)
	if result != "test/test" {
		t.Errorf("Resolve repeated = %q, want %q", result, "test/test")
	}
}

func TestResolve_EmptyKey(t *testing.T) {
	// {{env.}} is not matched by the regex (empty key after dot)
	result := Resolve("{{env.}}", Context{Env: map[string]string{"": "val"}})
	if result != "{{env.}}" {
		t.Errorf("Resolve empty key = %q, want unchanged", result)
	}
}

func TestResolve_MixedKnownAndUnknown(t *testing.T) {
	ctx := Context{Env: map[string]string{"A": "1"}}
	result := Resolve("{{env.A}}-{{env.B}}", ctx)
	if result != "1-" {
		t.Errorf("Resolve mixed = %q, want %q", result, "1-")
	}
}
