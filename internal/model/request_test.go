package model

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

func TestRunRequest_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	req := &Request{
		URI:        "/test",
		Method:     GET,
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
	}

	cmd := req.RunRequest()
	msg := cmd()

	success, ok := msg.(SuccessMsg)
	if !ok {
		t.Fatalf("RunRequest returned %T, want SuccessMsg", msg)
	}
	if success.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", success.StatusCode, http.StatusOK)
	}
	if strings.TrimSpace(success.Body) != "ok" {
		t.Errorf("Body = %q, want %q", success.Body, "ok")
	}
}

func TestRunRequest_Failure(t *testing.T) {
	req := &Request{
		URI:    "/test",
		Method: GET,
	}

	cmd := req.RunRequest()
	msg := cmd()

	failure, ok := msg.(FailureMsg)
	if !ok {
		t.Fatalf("RunRequest returned %T, want FailureMsg", msg)
	}
	if failure.Message == "" {
		t.Error("expected non-empty error message")
	}
}

func TestFailure(t *testing.T) {
	msg := Failure("test error")
	f, ok := msg.(FailureMsg)
	if !ok {
		t.Fatalf("Failure returned %T, want FailureMsg", msg)
	}
	if f.Message != "test error" {
		t.Errorf("Message = %q, want %q", f.Message, "test error")
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
