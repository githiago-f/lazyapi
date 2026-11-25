package model

type AuthType int

const (
	AuthBasic AuthType = iota
	AuthBearer
	AuthAPIKey
	AuthOAuth2
	// AuthOpenIDConnect is OAuth2 with endpoints discovered from the issuer
	// (OpenAPI type openIdConnect; Issuer is its openIdConnectUrl).
	AuthOpenIDConnect
)

// AuthTypes lists the types in selector order.
var AuthTypes = []AuthType{AuthBasic, AuthBearer, AuthAPIKey, AuthOAuth2, AuthOpenIDConnect}

// IsOAuth reports whether tokens come from an authorization server.
func (t AuthType) IsOAuth() bool { return t == AuthOAuth2 || t == AuthOpenIDConnect }

func (t AuthType) Label() string {
	switch t {
	case AuthBasic:
		return "Basic"
	case AuthBearer:
		return "Bearer"
	case AuthAPIKey:
		return "API Key"
	case AuthOAuth2:
		return "OAuth2"
	case AuthOpenIDConnect:
		return "OpenID Connect"
	default:
		return "Unknown"
	}
}

type AuthScheme struct {
	Type       AuthType `yaml:"type"`
	SchemeName string   `yaml:"schemeName,omitempty"`

	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`

	Token string `yaml:"token,omitempty"`

	KeyName  string `yaml:"keyName,omitempty"`
	KeyIn    string `yaml:"keyIn,omitempty"`
	KeyValue string `yaml:"keyValue,omitempty"`

	GrantType    string `yaml:"grantType,omitempty"`
	ClientID     string `yaml:"clientId,omitempty"`
	ClientSecret string `yaml:"clientSecret,omitempty"`
	AuthURL      string `yaml:"authUrl,omitempty"`
	TokenURL     string `yaml:"tokenUrl,omitempty"`
	Scopes       string `yaml:"scopes,omitempty"`
	AccessToken  string `yaml:"accessToken,omitempty"`

	// Issuer is the OpenID Connect issuer or discovery URL
	// (.well-known/openid-configuration); missing endpoints come from it.
	Issuer      string            `yaml:"issuer,omitempty"`
	RedirectURI string            `yaml:"redirectUri,omitempty"`
	Params      map[string]string `yaml:"params,omitempty"`

	// Disabled schemes are kept but not applied to the request.
	Disabled bool `yaml:"disabled,omitempty"`
}
