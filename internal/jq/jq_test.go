package jq

import "testing"

func TestApply(t *testing.T) {
	out, err := Apply(".items[] | .id", `{"items":[{"id":1},{"id":12345678901234567}]}`)
	if err != nil || out != "1\n12345678901234567" {
		t.Errorf("got %q, %v", out, err)
	}
	if out, _ := Apply("{a: .x}", `{"x":"y"}`); out != "{\n  \"a\": \"y\"\n}" {
		t.Errorf("object output %q", out)
	}
	if _, err := Apply(".", "not json"); err == nil {
		t.Error("non-JSON body must fail")
	}
	if _, err := Apply(".[", "{}"); err == nil {
		t.Error("parse error expected")
	}
	if _, err := Apply("repeat(.)", `1`); err == nil {
		t.Error("runaway filter must stop")
	}
	if _, err := Apply("def f: f; f", `1`); err == nil {
		t.Error("infinite recursion must time out")
	}
}
