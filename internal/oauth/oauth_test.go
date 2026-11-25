package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeIdP is a Keycloak-like provider under /realms/acme.
type fakeIdP struct {
	*httptest.Server
	t *testing.T

	mu         sync.Mutex
	challenges map[string]string // code → code_challenge
	devicePoll atomic.Int32
	tokenCalls atomic.Int32
	refreshes  atomic.Int32
	expiresIn  int
	lastForm   url.Values
}

func newIdP(t *testing.T) *fakeIdP {
	t.Helper()
	ResetDiscovery()
	ResetStores()
	idp := &fakeIdP{t: t, challenges: map[string]string{}, expiresIn: 300}
	mux := http.NewServeMux()
	realm := "/realms/acme"
	mux.HandleFunc(realm+"/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		base := idp.URL + realm + "/protocol/openid-connect"
		_ = json.NewEncoder(w).Encode(Metadata{
			Issuer:                      idp.URL + realm,
			AuthorizationEndpoint:       base + "/auth",
			TokenEndpoint:               base + "/token",
			DeviceAuthorizationEndpoint: base + "/auth/device",
		})
	})
	mux.HandleFunc(realm+"/protocol/openid-connect/auth", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("code_challenge_method") != "S256" || q.Get("client_id") != "cli" || q.Get("scope") != "openid" {
			http.Error(w, "bad authorize request "+r.URL.RawQuery, http.StatusBadRequest)
			return
		}
		code := fmt.Sprintf("code-%d", time.Now().UnixNano())
		idp.mu.Lock()
		idp.challenges[code] = q.Get("code_challenge")
		idp.mu.Unlock()
		http.Redirect(w, r, q.Get("redirect_uri")+"?code="+code+"&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})
	mux.HandleFunc(realm+"/protocol/openid-connect/auth/device", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"device_code":"dev-1","user_code":"ABCD-EFGH","verification_uri":"https://sso/device","interval":0,"expires_in":60}`))
	})
	mux.HandleFunc(realm+"/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		idp.tokenCalls.Add(1)
		_ = r.ParseForm()
		idp.mu.Lock()
		idp.lastForm = r.Form
		idp.mu.Unlock()
		fail := func(code string) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, `{"error":%q,"error_description":"test says no"}`, code)
		}
		tok := func(access string) {
			_, _ = fmt.Fprintf(w, `{"access_token":%q,"refresh_token":"refresh-%s","expires_in":%d}`, access, access, idp.expiresIn)
		}
		switch r.Form.Get("grant_type") {
		case "client_credentials":
			if u, p, _ := r.BasicAuth(); u != "svc" || p != "s3cret" {
				fail("invalid_client")
				return
			}
			tok("cc-token")
		case "authorization_code":
			idp.mu.Lock()
			challenge := idp.challenges[r.Form.Get("code")]
			idp.mu.Unlock()
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if challenge == "" || base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
				fail("invalid_grant")
				return
			}
			tok("code-token")
		case "urn:ietf:params:oauth:grant-type:device_code":
			if idp.devicePoll.Add(1) < 3 {
				fail("authorization_pending")
				return
			}
			tok("device-token")
		case "refresh_token":
			idp.refreshes.Add(1)
			if !strings.HasPrefix(r.Form.Get("refresh_token"), "refresh-") {
				fail("invalid_grant")
				return
			}
			tok("refreshed")
		default:
			fail("unsupported_grant_type")
		}
	})
	idp.Server = httptest.NewServer(mux)
	t.Cleanup(idp.Close)
	return idp
}

func (idp *fakeIdP) issuer() string { return idp.URL + "/realms/acme" }

func TestDiscoveryURLs(t *testing.T) {
	got := DiscoveryURLs("https://sso.example.com/realms/acme/")
	if got[0] != "https://sso.example.com/realms/acme/.well-known/openid-configuration" ||
		got[1] != "https://sso.example.com/realms/acme/.well-known/oauth-authorization-server" {
		t.Errorf("issuer: %v", got)
	}
	wk := "https://x/.well-known/openid-configuration"
	if got := DiscoveryURLs(wk); len(got) != 1 || got[0] != wk {
		t.Errorf("well-known URL is used as is: %v", got)
	}
}

func TestDiscoverKeycloakRealm(t *testing.T) {
	idp := newIdP(t)
	meta, err := Discover(context.Background(), nil, idp.issuer())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(meta.TokenEndpoint, "/realms/acme/protocol/openid-connect/token") || meta.DeviceAuthorizationEndpoint == "" {
		t.Errorf("metadata: %+v", meta)
	}
	if _, err := Discover(context.Background(), nil, idp.URL+"/realms/nope"); err == nil {
		t.Error("an unknown realm must fail")
	}
}

func TestClientCredentialsFromIssuerIsCachedAndPersisted(t *testing.T) {
	idp := newIdP(t)
	spec := filepath.Join(t.TempDir(), "api.yml")
	cfg := Config{Name: "kc", Grant: "client_credentials", Issuer: idp.issuer(), ClientID: "svc", ClientSecret: "s3cret"}
	for range 3 {
		tok, err := AccessToken(context.Background(), nil, cfg, StoreFor(spec))
		if err != nil || tok != "cc-token" {
			t.Fatalf("token %q %v", tok, err)
		}
	}
	if n := idp.tokenCalls.Load(); n != 1 {
		t.Errorf("token endpoint called %d times", n)
	}
	path := StoreFor(spec).Path()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("token file mode %v", info.Mode().Perm())
	}
	if gi, err := os.ReadFile(filepath.Join(filepath.Dir(spec), ".lazyapi", ".gitignore")); err != nil || !strings.Contains(string(gi), ".local/") {
		t.Errorf(".gitignore must keep tokens out of git: %q %v", gi, err)
	}

	// A new process reads the file: no token request.
	ResetStores()
	if tok, err := AccessToken(context.Background(), nil, cfg, StoreFor(spec)); err != nil || tok != "cc-token" || idp.tokenCalls.Load() != 1 {
		t.Errorf("persisted token: %q %v calls=%d", tok, err, idp.tokenCalls.Load())
	}

	bad := cfg
	bad.ClientSecret = "wrong"
	bad.Scopes = "other" // another cache key
	if _, err := AccessToken(context.Background(), nil, bad, StoreFor(spec)); err == nil || !strings.Contains(err.Error(), "invalid_client: test says no") {
		t.Errorf("token error: %v", err)
	}
}

func TestAuthorizationCodeNeedsLoginThenRefreshes(t *testing.T) {
	idp := newIdP(t)
	spec := filepath.Join(t.TempDir(), "api.yml")
	cfg := Config{Name: "kc", Grant: GrantAuthorizationCode, Issuer: idp.issuer(), ClientID: "cli", Scopes: "openid",
		RedirectURI: "http://127.0.0.1:0/callback"}

	_, err := AccessToken(context.Background(), nil, cfg, StoreFor(spec))
	if lr, ok := IsLoginRequired(err); !ok || lr.Name != "kc" {
		t.Fatalf("want login required, got %v", err)
	}

	var notices []string
	ui := UI{
		Notice: func(m string) { notices = append(notices, m) },
		// The "browser": follows the IdP's redirect to our loopback server.
		Open: func(link string) error {
			go func() {
				resp, err := http.Get(link)
				if err == nil {
					_ = resp.Body.Close()
				}
			}()
			return nil
		},
	}
	tok, err := Login(context.Background(), nil, cfg, StoreFor(spec), ui)
	if err != nil || tok.AccessToken != "code-token" {
		t.Fatalf("login: %+v %v", tok, err)
	}
	if len(notices) == 0 || !strings.Contains(notices[0], "/protocol/openid-connect/auth?") {
		t.Errorf("the URL is shown: %v", notices)
	}
	if got, err := AccessToken(context.Background(), nil, cfg, StoreFor(spec)); err != nil || got != "code-token" {
		t.Errorf("after login: %q %v", got, err)
	}
	if st := Status(cfg, StoreFor(spec)); st == nil || st.Name != "kc" || st.RefreshToken == "" {
		t.Errorf("status: %+v", st)
	}

	// Expired: the refresh token gets a new one without the user.
	key := cacheKey(cfg)
	old := StoreFor(spec).Get(key)
	old.Expiry = time.Now().Add(-time.Minute)
	_ = StoreFor(spec).Put(key, *old)
	if got, err := AccessToken(context.Background(), nil, cfg, StoreFor(spec)); err != nil || got != "refreshed" || idp.refreshes.Load() != 1 {
		t.Errorf("refresh: %q %v (%d)", got, err, idp.refreshes.Load())
	}

	if n, err := Logout("kc", StoreFor(spec)); err != nil || n != 1 {
		t.Errorf("logout: %d %v", n, err)
	}
	if _, err := AccessToken(context.Background(), nil, cfg, StoreFor(spec)); err == nil {
		t.Error("logged out: login required again")
	}
}

func TestDeviceCodeLogin(t *testing.T) {
	idp := newIdP(t)
	defer func(d time.Duration) { DevicePollInterval = d }(DevicePollInterval)
	DevicePollInterval = 10 * time.Millisecond
	cfg := Config{Name: "kc", Grant: "device_code", Issuer: idp.issuer(), ClientID: "cli"}
	var notice string
	tok, err := Login(context.Background(), nil, cfg, nil, UI{NoBrowser: true, Notice: func(m string) { notice = m }})
	if err != nil || tok.AccessToken != "device-token" {
		t.Fatalf("device login: %+v %v", tok, err)
	}
	if !strings.Contains(notice, "ABCD-EFGH") || !strings.Contains(notice, "https://sso/device") {
		t.Errorf("the user code is shown: %q", notice)
	}
	if idp.devicePoll.Load() != 3 {
		t.Errorf("polled %d times", idp.devicePoll.Load())
	}
}

func TestLoginErrors(t *testing.T) {
	idp := newIdP(t)
	_, err := Login(context.Background(), nil, Config{Name: "kc", Issuer: idp.issuer()}, nil, UI{})
	if err == nil || !strings.Contains(err.Error(), "no client id") {
		t.Errorf("no client: %v", err)
	}
	_, err = Login(context.Background(), nil, Config{Name: "kc", Grant: "implicit", Issuer: idp.issuer(), ClientID: "cli"}, nil, UI{})
	if err == nil || !strings.Contains(err.Error(), "implicit") {
		t.Errorf("implicit: %v", err)
	}
	_, err = Login(context.Background(), nil, Config{Name: "kc", Issuer: idp.issuer(), ClientID: "cli", RedirectURI: "https://example.com/cb"}, nil, UI{})
	if err == nil || !strings.Contains(err.Error(), "localhost") {
		t.Errorf("non-loopback redirect: %v", err)
	}
	var lr *LoginRequiredError
	if !errors.As(fmt.Errorf("wrapped: %w", &LoginRequiredError{Name: "x"}), &lr) {
		t.Error("errors.As")
	}
	if g := NormalizeGrant("client_credentials"); g != GrantClientCredentials {
		t.Errorf("normalize: %q", g)
	}
}

func TestNoClientIDSendsWithoutToken(t *testing.T) {
	ResetStores()
	if tok, err := AccessToken(context.Background(), nil, Config{Grant: GrantAuthorizationCode}, nil); tok != "" || err != nil {
		t.Errorf("%q %v", tok, err)
	}
}
