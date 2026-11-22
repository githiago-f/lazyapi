package model

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPOptions_InsecureRedirectsAndMaxBody(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/big":
			_, _ = w.Write([]byte(strings.Repeat("x", 100)))
		default:
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()

	if c, err := (HTTPOptions{}).Client(); c != nil || err != nil {
		t.Errorf("zero options should use the default client: %v %v", c, err)
	}

	// Self-signed certificate: fails by default, works with Insecure.
	req := &Request{URI: "/", Method: GET, ServerURL: srv.URL}
	if _, _, err := req.Send(); err == nil {
		t.Error("self-signed TLS should fail without --insecure")
	}
	client, err := HTTPOptions{Insecure: true, NoRedirects: true}.Client()
	if err != nil {
		t.Fatal(err)
	}
	req.HTTPClient = client
	resp, body, timing, err := req.SendTimed(t.Context())
	if err != nil || body != "ok" || timing.Total <= 0 || timing.TLS <= 0 {
		t.Fatalf("insecure: %q %+v %v", body, timing, err)
	}
	_ = resp

	req.URI = "/redirect"
	if resp, _, _ := req.Send(); resp.StatusCode != http.StatusFound {
		t.Errorf("NoRedirects should return the 302, got %d", resp.StatusCode)
	}

	req.URI, req.MaxBody = "/big", 10
	if _, _, err := req.Send(); err == nil || !strings.Contains(err.Error(), "larger than 10 bytes") {
		t.Errorf("max body: %v", err)
	}

	if _, err := (HTTPOptions{ClientCert: "a.pem"}).Client(); err == nil {
		t.Error("cert without key should fail")
	}
	if _, err := (HTTPOptions{Proxy: "::bad"}).Client(); err == nil {
		t.Error("invalid proxy should fail")
	}
}
