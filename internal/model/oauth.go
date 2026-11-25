package model

import (
	"context"
	"net/http"
	"sort"

	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/oauth"
)

// AuthSecrets are the credentials for one security scheme, usually coming
// from .lazyapi/config.yml. Values may use {{env.X}} templates.
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
	AuthURL      string `yaml:"authUrl,omitempty"`      // oauth2: overrides the spec
	// Issuer is an OpenID Connect issuer (e.g. a Keycloak realm,
	// https://sso.example.com/realms/acme) or a .well-known URL: the
	// authorization, token and device endpoints are discovered from it.
	Issuer string `yaml:"issuer,omitempty"`
	// GrantType overrides the spec's flow: authorizationCode, deviceCode,
	// clientCredentials or password.
	GrantType string `yaml:"grantType,omitempty"`
	// RedirectURI is the loopback redirect of the authorization code grant
	// (default http://localhost:8765/callback).
	RedirectURI string `yaml:"redirectUri,omitempty"`
	// Params are extra authorization/token parameters (audience, kc_idp_hint…).
	Params map[string]string `yaml:"params,omitempty"`
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
		set(&s.AuthURL, sec.AuthURL)
		set(&s.Issuer, sec.Issuer)
		set(&s.GrantType, sec.GrantType)
		set(&s.RedirectURI, sec.RedirectURI)
		if len(sec.Params) > 0 {
			params := make(map[string]string, len(s.Params)+len(sec.Params))
			for k, v := range s.Params {
				params[k] = v
			}
			for k, v := range sec.Params {
				params[k] = v
			}
			s.Params = params
		}
		out[i] = s
	}
	return out
}

// ResetTokenCache forgets every cached OAuth2 token and discovery document
// (tests). Token files on disk are untouched.
func ResetTokenCache() {
	oauth.ResetStores()
	oauth.ResetDiscovery()
}

// OAuthConfig resolves a scheme's templates into an OAuth client.
func OAuthConfig(s AuthScheme, tctx env.Context) oauth.Config {
	params := make(map[string]string, len(s.Params))
	for k, v := range s.Params {
		params[k] = env.Resolve(v, tctx)
	}
	grant := s.GrantType
	if grant == "" && s.Type == AuthOpenIDConnect {
		grant = oauth.GrantAuthorizationCode
	}
	return oauth.Config{
		Name:         s.SchemeName,
		Grant:        grant,
		Issuer:       env.Resolve(s.Issuer, tctx),
		AuthURL:      env.Resolve(s.AuthURL, tctx),
		TokenURL:     env.Resolve(s.TokenURL, tctx),
		ClientID:     env.Resolve(s.ClientID, tctx),
		ClientSecret: env.Resolve(s.ClientSecret, tctx),
		Username:     env.Resolve(s.Username, tctx),
		Password:     env.Resolve(s.Password, tctx),
		Scopes:       env.Resolve(s.Scopes, tctx),
		RedirectURI:  env.Resolve(s.RedirectURI, tctx),
		Params:       params,
	}
}

// oauthToken returns an access token for an OAuth2/OIDC scheme from the
// spec's token store, refreshing or requesting one when that needs no user.
// Grants that need the user fail with an *oauth.LoginRequiredError.
func oauthToken(ctx context.Context, client *http.Client, s AuthScheme, tctx env.Context, specPath string) (string, error) {
	return oauth.AccessToken(ctx, client, OAuthConfig(s, tctx), oauth.StoreFor(specPath))
}

// OAuthSchemes are the schemes a user can log in to: the OAuth2/OIDC
// schemes with their secrets applied, plus environment auth entries that
// name an issuer or token URL but no scheme of the spec (as OpenID Connect
// or OAuth2 schemes), sorted by name.
func OAuthSchemes(schemes []AuthScheme, secrets map[string]AuthSecrets) []AuthScheme {
	var out []AuthScheme
	seen := map[string]bool{}
	for _, s := range ApplySecrets(schemes, secrets) {
		if s.Type.IsOAuth() && !seen[s.SchemeName] {
			seen[s.SchemeName] = true
			out = append(out, s)
		}
	}
	for name, sec := range secrets {
		if seen[name] || (sec.Issuer == "" && sec.TokenURL == "") {
			continue
		}
		t := AuthOAuth2
		if sec.Issuer != "" {
			t = AuthOpenIDConnect
		}
		out = append(out, ApplySecrets([]AuthScheme{{Type: t, SchemeName: name}}, secrets)...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SchemeName < out[j].SchemeName })
	return out
}
