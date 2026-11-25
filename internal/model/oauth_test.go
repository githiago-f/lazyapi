package model

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestOAuth2ClientCredentialsIsFetchedAndCached(t *testing.T) {
	ResetTokenCache()
	var tokenCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenCalls.Add(1)
			_ = r.ParseForm()
			user, pass, _ := r.BasicAuth()
			if r.Form.Get("grant_type") != "client_credentials" || user != "cid" || pass != "s3cret" || r.Form.Get("scope") != "read" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"tok-1","expires_in":3600}`))
		case "/api":
			if r.Header.Get("Authorization") != "Bearer tok-1" {
				w.WriteHeader(http.StatusUnauthorized)
			}
		}
	}))
	defer srv.Close()

	req := &Request{
		URI: "/api", Method: GET, ServerURL: srv.URL,
		Env: map[string]string{"SECRET": "s3cret"},
		Auth: []AuthScheme{{Type: AuthOAuth2, SchemeName: "oauth", GrantType: "clientCredentials",
			TokenURL: srv.URL + "/token", Scopes: "read"}},
	}
	req.Auth = ApplySecrets(req.Auth, map[string]AuthSecrets{"oauth": {ClientID: "cid", ClientSecret: "{{env.SECRET}}"}})

	for i := 0; i < 3; i++ {
		resp, _, err := req.Send()
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("send %d: %v %v", i, resp, err)
		}
	}
	if n := tokenCalls.Load(); n != 1 {
		t.Errorf("token endpoint called %d times, want 1 (cached)", n)
	}

	// A failing token endpoint is a send error with the server's answer.
	ResetTokenCache()
	req.Auth = ApplySecrets(req.Auth, map[string]AuthSecrets{"oauth": {ClientSecret: "wrong"}})
	if _, _, err := req.Send(); err == nil || !strings.Contains(err.Error(), "invalid_client") {
		t.Errorf("expected token error, got %v", err)
	}
}

func TestOAuth2PasswordGrantAndFixedToken(t *testing.T) {
	ResetTokenCache()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "password" || r.Form.Get("username") != "ana" || r.Form.Get("password") != "pw" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"user-token"}`))
			return
		}
		_, _ = w.Write([]byte(r.Header.Get("Authorization")))
	}))
	defer srv.Close()

	req := &Request{URI: "/api", Method: GET, ServerURL: srv.URL, Auth: []AuthScheme{{
		Type: AuthOAuth2, SchemeName: "o", GrantType: "password", TokenURL: srv.URL + "/token",
	}}}
	req.Auth = ApplySecrets(req.Auth, map[string]AuthSecrets{"o": {ClientID: "app", Username: "ana", Password: "pw"}})
	if _, body, err := req.Send(); err != nil || body != "Bearer user-token" {
		t.Errorf("password grant: %q %v", body, err)
	}

	req.Auth = ApplySecrets(req.Auth, map[string]AuthSecrets{"o": {AccessToken: "fixed"}})
	if _, body, _ := req.Send(); body != "Bearer fixed" {
		t.Errorf("fixed token: %q", body)
	}
}

func TestEmptyCredentialsAreNotSent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get("Authorization") + "|" + r.Header.Get("X-Key")))
	}))
	defer srv.Close()
	req := &Request{URI: "/", Method: GET, ServerURL: srv.URL,
		Headers: map[string]string{"Authorization": "Bearer from-header"},
		Auth:    []AuthScheme{{Type: AuthBearer}, {Type: AuthAPIKey, KeyName: "X-Key", KeyIn: "header"}, {Type: AuthBasic}}}
	if _, body, _ := req.Send(); body != "Bearer from-header|" {
		t.Errorf("empty schemes must not override or send empty credentials: %q", body)
	}
}

func TestDisabledSchemeIsNotApplied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get("Authorization") + "|" + r.Header.Get("X-Key")))
	}))
	defer srv.Close()
	req := &Request{URI: "/", Method: GET, ServerURL: srv.URL, Auth: []AuthScheme{
		{Type: AuthBearer, Token: "t", Disabled: true},
		{Type: AuthAPIKey, KeyName: "X-Key", KeyIn: "header", KeyValue: "k"},
	}}
	if _, body, _ := req.Send(); body != "|k" {
		t.Errorf("disabled bearer must not be sent: %q", body)
	}
}
