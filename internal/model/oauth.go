package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/githiago-f/lazyapi/internal/env"
)

// AuthSecrets are the credentials for one security scheme, usually coming
// from lazyapi.config.yml. Values may use {{env.X}} templates.
type AuthSecrets struct {
	Token        string `yaml:"token,omitempty"`        // http bearer
	Username     string `yaml:"username,omitempty"`     // http basic, oauth2 password grant
	Password     string `yaml:"password,omitempty"`     // http basic, oauth2 password grant
	Value        string `yaml:"value,omitempty"`        // apiKey
	ClientID     string `yaml:"clientId,omitempty"`     // oauth2
	ClientSecret string `yaml:"clientSecret,omitempty"` // oauth2
	Scopes       string `yaml:"scopes,omitempty"`       // oauth2, space separated (overrides the spec)
	AccessToken  string `yaml:"accessToken,omitempty"`  // oauth2: a fixed token, no token request
	TokenURL     string `yaml:"tokenUrl,omitempty"`     // oauth2: overrides the spec
}

// ApplySecrets fills credentials into schemes by security scheme name.
// Schemes without secrets are returned unchanged.
func ApplySecrets(schemes []AuthScheme, secrets map[string]AuthSecrets) []AuthScheme {
	if len(secrets) == 0 {
		return schemes
	}
	out := make([]AuthScheme, len(schemes))
	for i, s := range schemes {
		sec, ok := secrets[s.SchemeName]
		if !ok {
			out[i] = s
			continue
		}
		set := func(dst *string, v string) {
			if v != "" {
				*dst = v
			}
		}
		set(&s.Token, sec.Token)
		set(&s.Username, sec.Username)
		set(&s.Password, sec.Password)
		set(&s.KeyValue, sec.Value)
		set(&s.ClientID, sec.ClientID)
		set(&s.ClientSecret, sec.ClientSecret)
		set(&s.Scopes, sec.Scopes)
		set(&s.AccessToken, sec.AccessToken)
		set(&s.TokenURL, sec.TokenURL)
		out[i] = s
	}
	return out
}

type cachedToken struct {
	token   string
	expires time.Time
}

var (
	tokenMu    sync.Mutex
	tokenCache = map[string]cachedToken{}
)

// ResetTokenCache forgets every cached OAuth2 token (tests).
func ResetTokenCache() {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	tokenCache = map[string]cachedToken{}
}

// oauthToken returns an access token for client_credentials and password
// grants, requesting one from the token URL when none is cached. Tokens are
// reused until shortly before they expire.
func oauthToken(ctx context.Context, client *http.Client, s AuthScheme, tctx env.Context) (string, error) {
	grant := ""
	switch s.GrantType {
	case "clientCredentials", "client_credentials":
		grant = "client_credentials"
	case "password":
		grant = "password"
	default:
		return "", nil // other flows need a browser; use accessToken
	}
	tokenURL := env.Resolve(s.TokenURL, tctx)
	clientID := env.Resolve(s.ClientID, tctx)
	if tokenURL == "" || clientID == "" {
		return "", nil
	}
	secret := env.Resolve(s.ClientSecret, tctx)
	user, pass := env.Resolve(s.Username, tctx), env.Resolve(s.Password, tctx)
	scopes := env.Resolve(s.Scopes, tctx)

	key := strings.Join([]string{grant, tokenURL, clientID, user, scopes}, "\x00")
	tokenMu.Lock()
	if c, ok := tokenCache[key]; ok && time.Now().Before(c.expires) {
		tokenMu.Unlock()
		return c.token, nil
	}
	tokenMu.Unlock()

	form := url.Values{"grant_type": {grant}, "client_id": {clientID}}
	if scopes != "" {
		form.Set("scope", scopes)
	}
	if grant == "password" {
		form.Set("username", user)
		form.Set("password", pass)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("oauth2 token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if secret != "" {
		req.SetBasicAuth(url.QueryEscape(clientID), url.QueryEscape(secret))
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth2 token request to %s: %w", tokenURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth2 token request to %s: %s: %s", tokenURL, resp.Status, strings.TrimSpace(string(body)))
	}
	var tok struct {
		AccessToken string  `json:"access_token"`
		ExpiresIn   float64 `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("oauth2 token response from %s has no access_token", tokenURL)
	}
	ttl := time.Duration(tok.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if ttl > time.Minute {
		ttl -= 30 * time.Second // refresh a little early
	}
	tokenMu.Lock()
	tokenCache[key] = cachedToken{token: tok.AccessToken, expires: time.Now().Add(ttl)}
	tokenMu.Unlock()
	return tok.AccessToken, nil
}
