package env

import (
	"reflect"
	"testing"
)

func TestMissing(t *testing.T) {
	ctx := Context{Env: map[string]string{"HOST": "h", "EMPTY": ""}, Vars: map[string]string{"id": "1"}}
	got := Missing(ctx, "/users/{{var.id}}/{{var.other}}", "Bearer {{env.TOKEN}}", "{{env.HOST}}{{env.EMPTY}}{{env.TOKEN}}", "{{lazyapi.x}}")
	if want := []string{"var.other", "env.TOKEN"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Missing = %v, want %v", got, want)
	}
}

func TestCompletions(t *testing.T) {
	names := []string{"env.TOKEN", "env.USER_ID", "var.userId"}
	if got := Completions("/users/{{env.U", names); !reflect.DeepEqual(got, []string{"/users/{{env.USER_ID}}"}) {
		t.Errorf("env.U → %v", got)
	}
	if got := Completions("/x/{{", names); len(got) != 5 {
		t.Errorf("{{ offers every name and both scopes, got %v", got)
	}
	if got := Completions("/x/{{env.TOKEN}}", names); got != nil {
		t.Errorf("closed template must not complete: %v", got)
	}
	if got := Completions("/x/{{va", names); !reflect.DeepEqual(got, []string{"/x/{{var.userId}}", "/x/{{var."}) {
		t.Errorf("va → %v", got)
	}
}
