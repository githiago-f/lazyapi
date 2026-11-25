// Package oauth gets OAuth 2.0 / OpenID Connect access tokens: endpoint
// discovery (.well-known/openid-configuration, RFC 8414), the
// client_credentials, password, authorization_code (PKCE, browser and a
// loopback redirect) and device_code grants, refresh tokens, and a token
// store persisted next to the spec (.lazyapi/.local/tokens.json, 0600).
//
// AccessToken never interacts with the user: it returns a cached token, refreshes
// it, or runs a grant that needs no browser. Grants that need the user
// (authorization code, device code) return a *LoginRequiredError until
// Login ran once; the refresh token then keeps the session alive.
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Metadata is the part of an OpenID Provider / authorization server
// metadata document lazyapi uses.
type Metadata struct {
	Issuer                        string   `json:"issuer"`
	AuthorizationEndpoint         string   `json:"authorization_endpoint,omitempty"`
	TokenEndpoint                 string   `json:"token_endpoint,omitempty"`
	DeviceAuthorizationEndpoint   string   `json:"device_authorization_endpoint,omitempty"`
	UserinfoEndpoint              string   `json:"userinfo_endpoint,omitempty"`
	EndSessionEndpoint            string   `json:"end_session_endpoint,omitempty"`
	RevocationEndpoint            string   `json:"revocation_endpoint,omitempty"`
	ScopesSupported               []string `json:"scopes_supported,omitempty"`
	GrantTypesSupported           []string `json:"grant_types_supported,omitempty"`
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported,omitempty"`
}

// DiscoveryURLs are the documents tried for an issuer, in order. A URL that
// already points at a .well-known document is used as is.
func DiscoveryURLs(issuer string) []string {
	issuer = strings.TrimSpace(issuer)
	if issuer == "" {
		return nil
	}
	if strings.Contains(issuer, "/.well-known/") {
		return []string{issuer}
	}
	base := strings.TrimRight(issuer, "/")
	return []string{
		base + "/.well-known/openid-configuration",
		base + "/.well-known/oauth-authorization-server",
	}
}

type cachedMeta struct {
	meta    *Metadata
	fetched time.Time
}

var (
	metaMu    sync.Mutex
	metaCache = map[string]cachedMeta{}
)

// metadataTTL is how long a discovery document is reused in memory.
const metadataTTL = time.Hour

// Discover fetches the metadata of an issuer (e.g. a Keycloak realm,
// https://sso.example.com/realms/acme) or of a .well-known URL. Documents
// are cached in memory for an hour.
func Discover(ctx context.Context, client *http.Client, issuer string) (*Metadata, error) {
	urls := DiscoveryURLs(issuer)
	if len(urls) == 0 {
		return nil, fmt.Errorf("oauth: no issuer to discover")
	}
	metaMu.Lock()
	if c, ok := metaCache[urls[0]]; ok && time.Since(c.fetched) < metadataTTL {
		metaMu.Unlock()
		return c.meta, nil
	}
	metaMu.Unlock()
	if client == nil {
		client = http.DefaultClient
	}

	var errs []string
	for _, u := range urls {
		meta, err := fetchMetadata(ctx, client, u)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		metaMu.Lock()
		metaCache[urls[0]] = cachedMeta{meta: meta, fetched: time.Now()}
		metaMu.Unlock()
		return meta, nil
	}
	return nil, fmt.Errorf("oauth discovery for %s failed: %s", issuer, strings.Join(errs, "; "))
}

func fetchMetadata(ctx context.Context, client *http.Client, u string) (*Metadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	var meta Metadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("GET %s: not a metadata document: %w", u, err)
	}
	if meta.TokenEndpoint == "" && meta.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("GET %s: no token_endpoint or authorization_endpoint", u)
	}
	return &meta, nil
}

// ResetDiscovery forgets the cached metadata documents (tests).
func ResetDiscovery() {
	metaMu.Lock()
	defer metaMu.Unlock()
	metaCache = map[string]cachedMeta{}
}
