package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Grants, as written in specs and .lazyapi/config.yml.
const (
	GrantAuthorizationCode = "authorizationCode"
	GrantClientCredentials = "clientCredentials"
	GrantDeviceCode        = "deviceCode"
	GrantImplicit          = "implicit"
	GrantPassword          = "password"
)

// Grants lists the grants the editor offers.
var Grants = []string{GrantAuthorizationCode, GrantClientCredentials, GrantDeviceCode, GrantPassword, GrantImplicit}

// NormalizeGrant maps spellings (client_credentials, device_code, …) to the
// names above ("" stays "").
func NormalizeGrant(g string) string {
	switch strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(g)) {
	case "authorizationcode", "code", "pkce":
		return GrantAuthorizationCode
	case "clientcredentials", "client":
		return GrantClientCredentials
	case "devicecode", "device", "urn:ietf:params:oauth:granttype:devicecode":
		return GrantDeviceCode
	case "password", "ropc":
		return GrantPassword
	case "implicit":
		return GrantImplicit
	}
	return g
}

// Interactive reports whether a grant needs the user (a browser or a code).
func Interactive(grant string) bool {
	switch NormalizeGrant(grant) {
	case GrantAuthorizationCode, GrantDeviceCode, GrantImplicit:
		return true
	}
	return false
}

// Config is a resolved OAuth client: no templates left.
type Config struct {
	Name  string // security scheme name (for messages and logout)
	Grant string
	// Issuer is an issuer URL or a .well-known document; missing endpoints
	// are discovered from it.
	Issuer       string
	AuthURL      string
	TokenURL     string
	DeviceURL    string
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
	Scopes       string // space separated
	// RedirectURI is the loopback address for the authorization code grant
	// (DefaultRedirectURI when empty).
	RedirectURI string
	// Params are extra authorization/token request parameters (audience,
	// kc_idp_hint, prompt…).
	Params map[string]string
}

// LoginRequiredError means the grant needs the user: run Login (the TUI
// does it automatically, the CLI with `lazyapi login`).
type LoginRequiredError struct {
	Name   string
	Grant  string
	Reason string
}

func (e *LoginRequiredError) Error() string {
	name := e.Name
	if name == "" {
		name = "oauth2"
	}
	msg := fmt.Sprintf("%s: login required (%s grant)", name, e.Grant)
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	return msg + ` — run "lazyapi login" or use Login in the TUI (ctrl+k)`
}

// IsLoginRequired reports whether err (or what it wraps) asks for a login,
// and for which scheme.
func IsLoginRequired(err error) (*LoginRequiredError, bool) {
	var e *LoginRequiredError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Resolve fills missing endpoints from the issuer's discovery document.
func (c Config) Resolve(ctx context.Context, client *http.Client) (Config, error) {
	c.Grant = NormalizeGrant(c.Grant)
	if c.Issuer == "" {
		return c, nil
	}
	if c.TokenURL != "" && (c.AuthURL != "" || c.Grant != GrantAuthorizationCode) && (c.DeviceURL != "" || c.Grant != GrantDeviceCode) {
		return c, nil
	}
	meta, err := Discover(ctx, client, c.Issuer)
	if err != nil {
		return c, err
	}
	if c.TokenURL == "" {
		c.TokenURL = meta.TokenEndpoint
	}
	if c.AuthURL == "" {
		c.AuthURL = meta.AuthorizationEndpoint
	}
	if c.DeviceURL == "" {
		c.DeviceURL = meta.DeviceAuthorizationEndpoint
	}
	return c, nil
}

// grantLocks serializes token requests per cache key, so parallel requests
// share one token request (and one refresh).
var (
	grantLocksMu sync.Mutex
	grantLocks   = map[string]*sync.Mutex{}
)

func lockFor(key string) *sync.Mutex {
	grantLocksMu.Lock()
	defer grantLocksMu.Unlock()
	l, ok := grantLocks[key]
	if !ok {
		l = &sync.Mutex{}
		grantLocks[key] = l
	}
	return l
}

// AccessToken returns a usable access token without involving the user: a stored
// one, a refreshed one, or a new one from the client_credentials or
// password grant. Interactive grants without a session return a
// *LoginRequiredError.
func AccessToken(ctx context.Context, client *http.Client, c Config, store *Store) (string, error) {
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
		return "", nil // nothing configured: send without a token
	}
	key := cacheKey(c)
	l := lockFor(key)
	l.Lock()
	defer l.Unlock()

	tok := store.Get(key)
	if tok.Valid() {
		return tok.AccessToken, nil
	}
	resolved, err := c.Resolve(ctx, client)
	if err != nil {
		return "", err
	}
	if tok != nil && tok.RefreshToken != "" && resolved.TokenURL != "" {
		fresh, err := refresh(ctx, client, resolved, tok.RefreshToken)
		if err == nil {
			if err := store.Put(key, fresh); err != nil {
				return "", fmt.Errorf("%s: saving the token: %w", c.Name, err)
			}
			return fresh.AccessToken, nil
		}
		if Interactive(c.Grant) {
			return "", &LoginRequiredError{Name: c.Name, Grant: c.Grant, Reason: "the session expired (" + err.Error() + ")"}
		}
	}
	switch c.Grant {
	case GrantClientCredentials, GrantPassword:
		fresh, err := direct(ctx, client, resolved)
		if err != nil {
			return "", err
		}
		if err := store.Put(key, fresh); err != nil {
			return "", fmt.Errorf("%s: saving the token: %w", c.Name, err)
		}
		return fresh.AccessToken, nil
	default:
		return "", &LoginRequiredError{Name: c.Name, Grant: c.Grant}
	}
}

// Status is the stored session of a client (nil when there is none).
func Status(c Config, store *Store) *Token {
	if store == nil {
		store = memory
	}
	c.Grant = NormalizeGrant(c.Grant)
	if c.Grant == "" {
		c.Grant = GrantAuthorizationCode
	}
	return store.Get(cacheKey(c))
}

// direct runs the client_credentials or password grant.
func direct(ctx context.Context, client *http.Client, c Config) (Token, error) {
	if c.TokenURL == "" {
		return Token{}, fmt.Errorf("%s: no token URL (set tokenUrl or issuer)", c.Name)
	}
	form := url.Values{}
	if c.Grant == GrantPassword {
		form.Set("grant_type", "password")
		form.Set("username", c.Username)
		form.Set("password", c.Password)
	} else {
		form.Set("grant_type", "client_credentials")
	}
	return tokenRequest(ctx, client, c, form)
}

func refresh(ctx context.Context, client *http.Client, c Config, refreshToken string) (Token, error) {
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}}
	tok, err := tokenRequest(ctx, client, c, form)
	if err == nil && tok.RefreshToken == "" {
		tok.RefreshToken = refreshToken // not rotated
	}
	return tok, err
}

// TokenError is an error response of a token endpoint (RFC 6749 §5.2).
type TokenError struct {
	Status      int
	Code        string `json:"error"`
	Description string `json:"error_description"`
	Body        string
}

func (e *TokenError) Error() string {
	switch {
	case e.Code != "" && e.Description != "":
		return fmt.Sprintf("%s: %s", e.Code, e.Description)
	case e.Code != "":
		return e.Code
	}
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body)
}

// tokenRequest posts a form to the token endpoint with the client's
// credentials (HTTP Basic when there is a secret, client_id otherwise).
func tokenRequest(ctx context.Context, client *http.Client, c Config, form url.Values) (Token, error) {
	if c.ClientID != "" {
		form.Set("client_id", c.ClientID)
	}
	if c.Scopes != "" && form.Get("grant_type") != "authorization_code" {
		form.Set("scope", c.Scopes)
	}
	for k, v := range c.Params {
		if form.Get(k) == "" {
			form.Set(k, v)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("%s: token request: %w", c.Name, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if c.ClientSecret != "" {
		req.SetBasicAuth(url.QueryEscape(c.ClientID), url.QueryEscape(c.ClientSecret))
	}
	resp, err := client.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("%s: token request to %s: %w", c.Name, c.TokenURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		te := &TokenError{Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
		_ = json.Unmarshal(body, te)
		return Token{}, fmt.Errorf("%s: token request to %s: %w", c.Name, c.TokenURL, te)
	}
	var raw struct {
		AccessToken  string      `json:"access_token"`
		TokenType    string      `json:"token_type"`
		RefreshToken string      `json:"refresh_token"`
		IDToken      string      `json:"id_token"`
		Scope        string      `json:"scope"`
		ExpiresIn    json.Number `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.AccessToken == "" {
		return Token{}, fmt.Errorf("%s: token response from %s has no access_token", c.Name, c.TokenURL)
	}
	tok := Token{
		AccessToken: raw.AccessToken, TokenType: raw.TokenType, RefreshToken: raw.RefreshToken,
		IDToken: raw.IDToken, Scope: raw.Scope, Name: c.Name, Issuer: c.Issuer,
	}
	ttl := 5 * time.Minute
	if secs, err := raw.ExpiresIn.Float64(); err == nil && secs > 0 {
		ttl = time.Duration(secs * float64(time.Second))
	}
	tok.Expiry = time.Now().Add(ttl)
	return tok, nil
}
