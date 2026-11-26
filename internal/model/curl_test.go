package model

import (
	"strings"
	"testing"
)

func TestCurl(t *testing.T) {
	req := &Request{
		URI: "/users/{id}", Method: POST, ServerURL: "https://api.example.com",
		Params:  map[string]string{"id": "7"},
		Query:   map[string]string{"q": "it's"},
		Headers: map[string]string{"X-Trace": "abc"},
		Body:    Body{Type: ApplicationJSON, Raw: `{"name":"{{env.N}}"}`},
		Env:     map[string]string{"N": "ana", "TOKEN": "t0k"},
		Auth:    []AuthScheme{{Type: AuthBearer, Token: "{{env.TOKEN}}"}},
	}
	got, err := req.Curl(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := `curl -X POST 'https://api.example.com/users/7?q=it%27s' -H 'Authorization: Bearer t0k' -H 'Content-Type: application/json' -H 'X-Trace: abc' --data-raw '{"name":"ana"}'`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}

	mp := &Request{URI: "/up", Method: POST, ServerURL: "http://x",
		Body: Body{Type: MultipartForm, Raw: `{"caption":"hi there"}`, Files: map[string]string{"file": "/tmp/a b.png"}}}
	got, err = mp.Curl(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "Content-Type") || !strings.Contains(got, `-F 'caption=hi there'`) || !strings.Contains(got, `-F 'file=@/tmp/a b.png'`) {
		t.Errorf("multipart curl = %s", got)
	}

	bin := &Request{URI: "/up", Method: PUT, ServerURL: "http://x", Body: Body{Type: "image/png", File: "/tmp/a b.png"}}
	got, err = bin.Curl(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if want := `curl -X PUT http://x/up -H 'Content-Type: image/png' --data-binary '@/tmp/a b.png'`; got != want {
		t.Errorf("file curl = %s\nwant       %s", got, want)
	}

	get := &Request{URI: "/", Method: GET, ServerURL: "http://x"}
	if got, _ := get.Curl(t.Context()); got != "curl http://x/" {
		t.Errorf("GET = %s", got)
	}
}
