package importer

import (
	"strings"
	"testing"
)

func TestTranslatePostman(t *testing.T) {
	cases := map[string]string{
		"pm.test('ok', () => pm.response.to.have.status(200));":                "test('ok', () => expect(response).toHaveStatus(200));",
		"pm.expect(json.items).to.be.an('array').that.is.not.empty;":           "(expect(Array.isArray(json.items)).toBe(true), expect(json.items).not.toHaveLength(0));",
		"pm.expect(json.items[0]).to.have.property('sku').that.equals('a-1');": "expect(json.items[0]).toHaveProperty('sku', 'a-1');",
		"pm.expect(pm.response.responseTime).to.be.below(5000);":               "expect(response.time).toBeLessThan(5000);",
		"pm.expect(x).to.deep.equal({a: 1});":                                  "expect(x).toEqual({a: 1});",
		"pm.expect(x).to.not.include('y');":                                    "expect(x).not.toContain('y');",
		"pm.expect(x).to.be.oneOf([1, 2]);":                                    "expect([1, 2]).toContain(x);",
		"pm.environment.set('id', pm.response.json().id);":                     "env.set('id', response.json().id);",
		"var n = pm.iterationData.get('n') + pm.info.iteration;":               "var n = env.get('n') + lazyapi.iteration;",
		"pm.response.to.have.header('Content-Type');":                          "expect(response).toHaveHeader('Content-Type');",
		"pm.expect(x).to.have.members([1]);":                                   "// TODO(postman): pm.expect(x).to.have.members([1]);",
		"pm.sendRequest('https://x', () => {});":                               "// TODO(postman): pm.sendRequest('https://x', () => {});",
	}
	for in, want := range cases {
		got, _ := translatePostman(in, false)
		if got != want {
			t.Errorf("%s\n got: %s\nwant: %s", in, got, want)
		}
	}
	pre, _ := translatePostman("pm.request.headers.add({key: 'X-Trace', value: 'from-pre'});\npm.request.headers.remove('X-Old');", true)
	if pre != "req.headers['X-Trace'] = 'from-pre';\ndelete req.headers['X-Old'];" {
		t.Errorf("pre-request: %s", pre)
	}
	// A commented line inside a multi-line call breaks the syntax: the
	// original is kept as a comment.
	broken, n := translatePostman("pm.sendRequest('u', function () {\n  console.log(1);\n});", false)
	if n != 1 || !strings.HasPrefix(broken, "// TODO(postman): could not translate") {
		t.Errorf("broken: %d %s", n, broken)
	}
}
