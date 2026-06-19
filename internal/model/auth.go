package model

type AuthType int

const (
	AuthBasic AuthType = iota
	AuthBearer
	AuthAPIKey
	AuthOAuth2
)

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
}
