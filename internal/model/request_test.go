package model

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSend_WithMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "test-header")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/test",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
	}

	resp, body, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if resp.Header.Get("X-Custom") != "test-header" {
		t.Errorf("X-Custom = %q, want %q", resp.Header.Get("X-Custom"), "test-header")
	}
	if strings.TrimSpace(body) != `{"status":"ok"}` {
		t.Errorf("Body = %q, want %q", body, `{"status":"ok"}`)
	}
}

func TestSend_ResolvesEnvVars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/{{env.PATH}}",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Env:        map[string]string{"PATH": "test-path"},
	}

	_, _, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_ResolvesPathParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/items/42" {
			t.Errorf("Path = %q, want %q", r.URL.Path, "/items/42")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/items/{id}",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Params:     map[string]string{"id": "42"},
	}

	_, _, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_ResolvesQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "test" {
			t.Errorf("query q = %q, want %q", got, "test")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/search",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Query:      map[string]string{"q": "test"},
	}

	_, _, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_ResolvesHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer mytoken" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer mytoken")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/test",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Env:        map[string]string{"TOKEN": "mytoken"},
		Headers:    map[string]string{"Authorization": "Bearer {{env.TOKEN}}"},
	}

	_, _, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_ResolvesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		if strings.TrimSpace(string(body[:n])) != `{"data":"hello"}` {
			t.Errorf("Body = %q, want %q", strings.TrimSpace(string(body[:n])), `{"data":"hello"}`)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/test",
		Method:     POST,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Vars:       map[string]string{"data": "hello"},
		Body:       Body{Raw: `{"data":"{{var.data}}"}`},
	}

	_, _, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_AuthBasic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("Authorization = %q, want Basic prefix", auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/test",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Auth: []AuthScheme{
			{Type: AuthBasic, Username: "user", Password: "pass"},
		},
	}

	_, _, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_AuthBearer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer mytoken" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer mytoken")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/test",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Auth: []AuthScheme{
			{Type: AuthBearer, Token: "mytoken"},
		},
	}

	_, _, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_AuthAPIKeyHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key"); got != "mykey" {
			t.Errorf("X-API-Key = %q, want %q", got, "mykey")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/test",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Auth: []AuthScheme{
			{Type: AuthAPIKey, KeyName: "X-API-Key", KeyIn: "header", KeyValue: "mykey"},
		},
	}

	_, _, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_AuthAPIKeyQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("api_key"); got != "my key" {
			t.Errorf("api_key = %q, want %q", got, "my key")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/test",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Query:      map[string]string{"existing": "value"},
		Auth: []AuthScheme{
			{Type: AuthAPIKey, KeyName: "api_key", KeyIn: "query", KeyValue: "my key"},
		},
	}

	_, _, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_AuthOAuth2(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer mytoken" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer mytoken")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/test",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Auth: []AuthScheme{
			{Type: AuthOAuth2, AccessToken: "mytoken"},
		},
	}

	_, _, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_Error(t *testing.T) {
	req := &Request{
		URI:    "/test",
		Method: GET,
		// No ServerURL set -> invalid URL
	}

	_, _, err := req.Send()
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestSend_DefaultClientUsedWhenNotSet(t *testing.T) {
	// HTTPClient is nil, should use http.DefaultClient
	req := &Request{
		URI:    "/test",
		Method: GET,
	}
	if req.HTTPClient != nil {
		t.Error("expected HTTPClient to be nil by default")
	}
}

func TestSend_SetsContentTypeHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/test",
		Method:     POST,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Body:       Body{Raw: `{"key":"value"}`, Type: ApplicationJSON},
	}

	_, _, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_MultipleAuthSchemes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/test",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Auth: []AuthScheme{
			{Type: AuthBearer, Token: "token1"},
			{Type: AuthBearer, Token: "token2"},
		},
	}

	_, _, err := req.Send()
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_AuthBasicResolvesEnv(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "alice" || pass != "s3cret" {
			t.Errorf("BasicAuth = (%q, %q, %v), want (alice, s3cret, true)", user, pass, ok)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
		Env:        map[string]string{"USER": "alice", "PASS": "s3cret"},
		Auth: []AuthScheme{
			{Type: AuthBasic, Username: "{{env.USER}}", Password: "{{env.PASS}}"},
		},
	}

	if _, _, err := req.Send(); err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSend_Timeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	req := &Request{
		URI:       "/slow",
		Method:    GET,
		ServerURL: srv.URL,
		Timeout:   50 * time.Millisecond,
	}

	start := time.Now()
	_, _, err := req.Send()
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("request took %v, timeout not applied", elapsed)
	}
}

func TestSend_SetsContentTypeFromBodyType(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Content-Type")
	}))
	defer srv.Close()

	req := &Request{URI: "/", Method: POST, ServerURL: srv.URL, Body: Body{Type: ApplicationJSON, Raw: `{}`}}
	if _, _, err := req.Send(); err != nil {
		t.Fatal(err)
	}
	if got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}

	req.Headers = map[string]string{"content-type": "application/merge-patch+json"}
	if _, _, err := req.Send(); err != nil {
		t.Fatal(err)
	}
	if got != "application/merge-patch+json" {
		t.Errorf("explicit header overridden: %q", got)
	}
}

// waitFirst is a RoundTripper that holds a request before sending it, like
// the MCP permission guard while the user is asked.
type waitFirst struct{ d time.Duration }

func (w waitFirst) RoundTrip(r *http.Request) (*http.Response, error) {
	time.Sleep(w.d)
	return http.DefaultTransport.RoundTrip(r)
}

// Time spent before the transport asks for a connection is not the
// server's: an "answers fast" test must not fail because the user took
// 20 s to allow the host.
func TestSendTimedExcludesTimeBeforeTheTransport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close()
	r := &Request{Method: GET, ServerURL: srv.URL, URI: "/", HTTPClient: &http.Client{Transport: waitFirst{300 * time.Millisecond}}}
	_, body, timing, err := r.SendTimed(context.Background())
	if err != nil || body != "ok" {
		t.Fatalf("body %q, %v", body, err)
	}
	if timing.Total <= 0 || timing.Total >= 300*time.Millisecond {
		t.Errorf("Total = %v, want the request only", timing.Total)
	}
}
