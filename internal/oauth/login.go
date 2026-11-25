package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DefaultRedirectURI is where the authorization code grant listens. Register
// it (or http://localhost:8765/*) as a valid redirect URI of the client —
// in Keycloak: Clients → <client> → Valid redirect URIs.
const DefaultRedirectURI = "http://localhost:8765/callback"

// LoginTimeout bounds how long Login waits for the user.
var LoginTimeout = 5 * time.Minute

// DevicePollInterval is the device grant's polling interval when the server
// does not give one.
var DevicePollInterval = 5 * time.Second

// UI is how Login talks to the user.
type UI struct {
	// Open opens a URL in the browser (OpenBrowser when nil). Errors are
	// not fatal: Notice still shows the URL.
	Open func(url string) error
	// Notice shows a message (where to log in, the device code).
	Notice func(msg string)
	// NoBrowser only shows URLs.
	NoBrowser bool
}

func (u UI) notice(msg string) {
	if u.Notice != nil {
		u.Notice(msg)
	}
}

func (u UI) open(link string) {
	if u.NoBrowser {
		return
	}
	open := u.Open
	if open == nil {
		open = OpenBrowser
	}
	if err := open(link); err != nil {
		u.notice("Could not open a browser (" + err.Error() + "); open the URL yourself.")
	}
}

// Login gets a new token, asking the user when the grant needs it, and
// stores it. Non-interactive grants just request a token.
func Login(ctx context.Context, client *http.Client, c Config, store *Store, ui UI) (Token, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if store == nil {
		store = memory
	}
	c.Grant = NormalizeGrant(c.Grant)
	if c.Grant == "" {
		c.Grant = GrantAuthorizationCode
	}
	if c.ClientID == "" {
		return Token{}, fmt.Errorf("%s: no client id (set clientId in the scheme or the environment's auth)", c.Name)
	}
	resolved, err := c.Resolve(ctx, client)
	if err != nil {
		return Token{}, err
	}
	var tok Token
	switch resolved.Grant {
	case GrantClientCredentials, GrantPassword:
		tok, err = direct(ctx, client, resolved)
	case GrantDeviceCode:
		tok, err = deviceLogin(ctx, client, resolved, ui)
	case GrantAuthorizationCode:
		tok, err = codeLogin(ctx, client, resolved, ui)
	case GrantImplicit:
		return Token{}, fmt.Errorf("%s: the implicit grant is not supported; use authorizationCode (with PKCE) for the same client", c.Name)
	default:
		return Token{}, fmt.Errorf("%s: unknown grant %q", c.Name, c.Grant)
	}
	if err != nil {
		return Token{}, err
	}
	if err := store.Put(cacheKey(c), tok); err != nil {
		return tok, fmt.Errorf("%s: saving the token: %w", c.Name, err)
	}
	return tok, nil
}

// Logout forgets the stored tokens of a scheme ("" forgets all).
func Logout(name string, store *Store) (int, error) {
	if store == nil {
		store = memory
	}
	return store.Delete(name)
}

func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// AuthCodeURL is the authorization request of the code grant with PKCE.
func AuthCodeURL(c Config, redirect, state, verifier string) (string, error) {
	if c.AuthURL == "" {
		return "", fmt.Errorf("%s: no authorization URL (set authUrl or issuer)", c.Name)
	}
	u, err := url.Parse(c.AuthURL)
	if err != nil {
		return "", fmt.Errorf("%s: authorization URL: %w", c.Name, err)
	}
	sum := sha256.Sum256([]byte(verifier))
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", redirect)
	q.Set("state", state)
	q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:]))
	q.Set("code_challenge_method", "S256")
	if c.Scopes != "" {
		q.Set("scope", c.Scopes)
	}
	for k, v := range c.Params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// codeLogin runs the authorization code grant: a loopback server receives
// the redirect, the browser does the rest.
func codeLogin(ctx context.Context, client *http.Client, c Config, ui UI) (Token, error) {
	redirect := c.RedirectURI
	if redirect == "" {
		redirect = DefaultRedirectURI
	}
	ru, err := url.Parse(redirect)
	if err != nil || ru.Scheme != "http" || !loopback(ru.Hostname()) {
		return Token{}, fmt.Errorf("%s: redirect URI %q must be an http://localhost or http://127.0.0.1 address", c.Name, redirect)
	}
	port := ru.Port()
	if port == "" {
		port = "80"
	}
	host := ru.Hostname()
	if host == "localhost" {
		host = "127.0.0.1"
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return Token{}, fmt.Errorf("%s: listening for the redirect on %s: %w (set redirectUri to a free port)", c.Name, ru.Host, err)
	}
	if ru.Port() == "0" { // random port: the redirect must carry the real one
		ru.Host = net.JoinHostPort(ru.Hostname(), strconv.Itoa(ln.Addr().(*net.TCPAddr).Port))
		redirect = ru.String()
	}
	path := ru.Path
	if path == "" {
		path = "/"
	}

	state, verifier := randomString(24), randomString(48)
	authURL, err := AuthCodeURL(c, redirect, state, verifier)
	if err != nil {
		_ = ln.Close()
		return Token{}, err
	}

	type result struct {
		code string
		err  error
	}
	done := make(chan result, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var res result
		switch {
		case q.Get("state") != state:
			res.err = fmt.Errorf("%s: the redirect has a wrong state (another login in progress?)", c.Name)
		case q.Get("error") != "":
			res.err = fmt.Errorf("%s: %s: %s", c.Name, q.Get("error"), q.Get("error_description"))
		case q.Get("code") == "":
			res.err = fmt.Errorf("%s: the redirect has no code", c.Name)
		default:
			res.code = q.Get("code")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if res.err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, loginPage, "Login failed", html.EscapeString(res.err.Error()))
		} else {
			_, _ = fmt.Fprintf(w, loginPage, "Logged in", "You can close this tab and go back to lazyapi.")
		}
		select {
		case done <- res:
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()

	ui.notice(fmt.Sprintf("%s: log in in the browser — %s", c.Name, authURL))
	ui.open(authURL)

	wait, cancel := context.WithTimeout(ctx, LoginTimeout)
	defer cancel()
	var res result
	select {
	case res = <-done:
	case <-wait.Done():
		return Token{}, fmt.Errorf("%s: login not completed: %w", c.Name, wait.Err())
	}
	if res.err != nil {
		return Token{}, res.err
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {res.code},
		"redirect_uri":  {redirect},
		"code_verifier": {verifier},
	}
	return tokenRequest(ctx, client, c, form)
}

const loginPage = `<!doctype html><meta charset="utf-8"><title>lazyapi</title>
<body style="font-family:system-ui;background:#1e1e2e;color:#cdd6f4;display:grid;place-items:center;height:90vh">
<div><h1 style="color:#fab387">%s</h1><p>%s</p></div>`

func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// deviceLogin runs the device authorization grant (RFC 8628): the user
// enters a code on another device (or the opened browser) while we poll.
func deviceLogin(ctx context.Context, client *http.Client, c Config, ui UI) (Token, error) {
	if c.DeviceURL == "" {
		return Token{}, fmt.Errorf("%s: no device authorization endpoint (set issuer, or enable the device grant on the server)", c.Name)
	}
	form := url.Values{"client_id": {c.ClientID}}
	if c.Scopes != "" {
		form.Set("scope", c.Scopes)
	}
	for k, v := range c.Params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.DeviceURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if c.ClientSecret != "" {
		req.SetBasicAuth(url.QueryEscape(c.ClientID), url.QueryEscape(c.ClientSecret))
	}
	resp, err := client.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("%s: device authorization: %w", c.Name, err)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		te := &TokenError{Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
		_ = json.Unmarshal(body, te)
		return Token{}, fmt.Errorf("%s: device authorization: %w", c.Name, te)
	}
	var da struct {
		DeviceCode              string      `json:"device_code"`
		UserCode                string      `json:"user_code"`
		VerificationURI         string      `json:"verification_uri"`
		VerificationURL         string      `json:"verification_url"` // Google
		VerificationURIComplete string      `json:"verification_uri_complete"`
		ExpiresIn               json.Number `json:"expires_in"`
		Interval                json.Number `json:"interval"`
	}
	if err := json.Unmarshal(body, &da); err != nil || da.DeviceCode == "" {
		return Token{}, fmt.Errorf("%s: device authorization response has no device_code", c.Name)
	}
	verify := da.VerificationURI
	if verify == "" {
		verify = da.VerificationURL
	}
	ui.notice(fmt.Sprintf("%s: open %s and enter the code %s", c.Name, verify, da.UserCode))
	if da.VerificationURIComplete != "" {
		ui.open(da.VerificationURIComplete)
	} else if verify != "" {
		ui.open(verify)
	}

	interval := DevicePollInterval
	if n, err := da.Interval.Int64(); err == nil && n > 0 {
		interval = time.Duration(n) * time.Second
	}
	timeout := LoginTimeout
	if n, err := da.ExpiresIn.Int64(); err == nil && n > 0 && time.Duration(n)*time.Second < timeout {
		timeout = time.Duration(n) * time.Second
	}
	wait, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		select {
		case <-wait.Done():
			return Token{}, fmt.Errorf("%s: login not completed: %w", c.Name, wait.Err())
		case <-time.After(interval):
		}
		tok, err := tokenRequest(wait, client, c, url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"device_code": {da.DeviceCode},
		})
		if err == nil {
			return tok, nil
		}
		var te *TokenError
		if !errors.As(err, &te) {
			return Token{}, err
		}
		switch te.Code {
		case "authorization_pending":
		case "slow_down":
			interval += DevicePollInterval
		default:
			return Token{}, err
		}
	}
}

// OpenBrowser opens a URL with $BROWSER or the platform's opener (WSL
// opens the Windows browser).
func OpenBrowser(link string) error {
	if b := os.Getenv("BROWSER"); b != "" {
		return start(exec.Command(b, link))
	}
	var candidates [][]string
	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"open", link}}
	case "windows":
		candidates = [][]string{{"rundll32", "url.dll,FileProtocolHandler", link}}
	default:
		if isWSL() {
			candidates = append(candidates, []string{"wslview", link}, []string{"rundll32.exe", "url.dll,FileProtocolHandler", link})
		}
		candidates = append(candidates, []string{"xdg-open", link})
	}
	var last error
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err != nil {
			last = err
			continue
		}
		if err := start(exec.Command(c[0], c[1:]...)); err != nil {
			last = err
			continue
		}
		return nil
	}
	if last == nil {
		last = errors.New("no browser opener found")
	}
	return last
}

// start runs an opener without waiting for it (but reaps it).
func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func isWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	b, err := os.ReadFile("/proc/sys/kernel/osrelease")
	return err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft")
}
