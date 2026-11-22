package model

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type captured struct {
	header http.Header
	body   []byte
}

// capture returns a server and a function that yields the last request it
// received (passed through a channel, so the race detector sees the handoff).
func capture(t *testing.T) (*httptest.Server, func() *http.Request) {
	t.Helper()
	ch := make(chan captured, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ch <- captured{header: r.Header.Clone(), body: body}
	}))
	t.Cleanup(srv.Close)
	return srv, func() *http.Request {
		c := <-ch
		r, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(string(c.body)))
		r.Header = c.header
		return r
	}
}

func TestBody_FormURLEncoded(t *testing.T) {
	srv, last := capture(t)
	req := &Request{URI: "/", Method: POST, ServerURL: srv.URL, Env: map[string]string{"N": "ana"},
		Body: Body{Type: FormURLEncoded, Raw: `{"name":"{{env.N}}","tags":["a","b"],"age":30,"ok":true}`}}
	if _, _, err := req.Send(); err != nil {
		t.Fatal(err)
	}
	r := last()
	if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		t.Errorf("content type = %q", r.Header.Get("Content-Type"))
	}
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	if r.PostForm.Get("name") != "ana" || strings.Join(r.PostForm["tags"], ",") != "a,b" || r.PostForm.Get("age") != "30" || r.PostForm.Get("ok") != "true" {
		t.Errorf("form = %v", r.PostForm)
	}

	// Already-encoded bodies are sent as-is.
	req.Body.Raw = "a=1&b=2"
	if _, _, err := req.Send(); err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(last().Body)
	if string(data) != "a=1&b=2" {
		t.Errorf("raw form = %q", data)
	}
}

func TestBody_Multipart(t *testing.T) {
	srv, last := capture(t)
	file := filepath.Join(t.TempDir(), "avatar.png")
	if err := os.WriteFile(file, []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := &Request{URI: "/", Method: POST, ServerURL: srv.URL,
		Headers: map[string]string{"Content-Type": "multipart/form-data"}, // no boundary: must be replaced
		Body:    Body{Type: MultipartForm, Raw: `{"name":"ana"}`, Files: map[string]string{"avatar": file}}}
	if _, _, err := req.Send(); err != nil {
		t.Fatal(err)
	}
	r := last()
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		t.Fatalf("content type = %q", r.Header.Get("Content-Type"))
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	got := map[string]string{}
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		data, _ := io.ReadAll(p)
		got[p.FormName()+"|"+p.FileName()] = string(data)
	}
	if got["name|"] != "ana" || got["avatar|avatar.png"] != "PNGDATA" {
		t.Errorf("parts = %v", got)
	}

	// A non-object multipart body is an error, and Raw never reads files.
	req.Body = Body{Type: MultipartForm, Raw: `"@` + file + `"`}
	if _, _, err := req.Send(); err == nil || !strings.Contains(err.Error(), "JSON object") {
		t.Errorf("non-object multipart: %v", err)
	}
	req.Body = Body{Type: MultipartForm, Raw: `{"avatar":"@` + file + `"}`}
	if _, _, err := req.Send(); err != nil {
		t.Fatal(err)
	}
	if b, _ := io.ReadAll(last().Body); strings.Contains(string(b), "PNGDATA") {
		t.Error("a value starting with @ must not read a file")
	}
}
