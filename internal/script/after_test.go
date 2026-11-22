package script

import (
	"strings"
	"testing"
)

func TestAfter_RunsAfterTestsAndCanSend(t *testing.T) {
	data := baseData()
	var sent []string
	data.Send = func(target string, o SendOptions) (*Response, error) {
		sent = append(sent, target+" "+o.Params["id"])
		return okResponse(""), nil
	}
	src := `
test("created", () => expect(response.status).toBe(200));
test("fails but cleanup still runs", () => expect(1).toBe(2));
export function after(res) {
  console.log("cleanup", res.status);
  lazyapi.send("DELETE /users/{id}", { params: { id: String(res.json().id) } });
}`
	res := Run(src, data, Options{})
	if !res.AfterRan || res.AfterError != "" {
		t.Fatalf("after: %+v", res)
	}
	if len(sent) != 1 || sent[0] != "DELETE /users/{id} 7" {
		t.Errorf("sent = %v", sent)
	}
	if res.PassCount != 1 || res.FailCount != 1 {
		t.Errorf("tests = %+v", res.Results)
	}
	if n := len(res.Logs); n != 1 || res.Logs[0].Message != "cleanup 200" {
		t.Errorf("logs = %+v", res.Logs)
	}
}

func TestAfter_RunsWhenTopLevelThrowsAndReportsErrors(t *testing.T) {
	src := `throw new Error("boom");
export function after() { throw new Error("cleanup failed"); }`
	res := Run(src, baseData(), Options{Filename: "t.js"})
	if !strings.Contains(res.ScriptError, "boom") {
		t.Errorf("script error = %q", res.ScriptError)
	}
	if !res.AfterRan || !strings.Contains(res.AfterError, "after(): Error: cleanup failed") || !res.Failed() {
		t.Errorf("after error: %+v", res)
	}

	if res := Run(`test("x", () => {});`, baseData(), Options{}); res.AfterRan {
		t.Error("no after export: AfterRan must be false")
	}
}

func TestBefore_DoesNotRunAfter(t *testing.T) {
	data := baseData()
	sends := 0
	data.Send = func(string, SendOptions) (*Response, error) { sends++; return okResponse(""), nil }
	src := `export function before(req) {}
export function after() { lazyapi.send("cleanup"); }`
	if res := Before(src, data, Options{}); res.Error != "" {
		t.Fatal(res.Error)
	}
	if sends != 0 {
		t.Error("before pass must not call after()")
	}
}
