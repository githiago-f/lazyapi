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
