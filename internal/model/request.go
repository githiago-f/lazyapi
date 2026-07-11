package model

import (
	"encoding/base64"
	"io"
	"net/http"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/env"
)

type OpenAPIRef struct {
	FilePath string
	Path     string
	Method   string
}

type Request struct {
	FileName string

	About About `yaml:"about"`

	URI    string `yaml:"uri"`
	Method Method `yaml:"method"`

	Body    Body              `yaml:"body"`
	Headers map[string]string `yaml:"headers"`
	Params  map[string]string `yaml:"pathParams"`
	Query   map[string]string `yaml:"query"`

	Auth []AuthScheme `yaml:"auth,omitempty"`

	ServerURL string   `yaml:"-"`
	Servers   []string `yaml:"-"`

	OpenAPIRef *OpenAPIRef `yaml:"-"`
	DraftPath  string      `yaml:"-"`

	Env  map[string]string `yaml:"-"`
	Vars map[string]string `yaml:"-"`
}

type FailureMsg struct {
	Message string
}

type SuccessMsg struct {
	StatusCode int
	Status     string
	Header     http.Header
	Body       string
}

func Failure(msg string) tea.Msg {
	return FailureMsg{
		Message: msg,
	}
}

func (r *Request) Send() (*http.Response, string, error) {
	ctx := env.Context{
		Env:  r.Env,
		Vars: r.Vars,
	}

	fullURL := env.Resolve(r.ServerURL, ctx) + env.Resolve(r.URI, ctx)

	resolvedParams := make(map[string]string, len(r.Params))
	for name, value := range r.Params {
		resolvedParams[name] = env.Resolve(value, ctx)
	}
	paramKeys := make([]string, 0, len(resolvedParams))
	for name := range resolvedParams {
		paramKeys = append(paramKeys, name)
	}
	sort.Strings(paramKeys)
	for _, name := range paramKeys {
		fullURL = strings.ReplaceAll(fullURL, "{"+name+"}", resolvedParams[name])
	}

	resolvedBody := env.Resolve(r.Body.Raw, ctx)
	var bodyReader io.Reader
	if resolvedBody != "" {
		bodyReader = strings.NewReader(resolvedBody)
	}

	req, err := http.NewRequest(r.Method.Label(), fullURL, bodyReader)
	if err != nil {
		return nil, "", err
	}

	q := req.URL.Query()
	for name, value := range r.Query {
		q.Set(name, env.Resolve(value, ctx))
	}
	req.URL.RawQuery = q.Encode()

	for name, value := range r.Headers {
		req.Header.Set(name, env.Resolve(value, ctx))
	}

	for _, scheme := range r.Auth {
		switch scheme.Type {
		case AuthBasic:
			auth := scheme.Username + ":" + scheme.Password
			encoded := base64.StdEncoding.EncodeToString([]byte(auth))
			req.Header.Set("Authorization", "Basic "+encoded)
		case AuthBearer:
			req.Header.Set("Authorization", "Bearer "+env.Resolve(scheme.Token, ctx))
		case AuthAPIKey:
			keyValue := env.Resolve(scheme.KeyValue, ctx)
			switch scheme.KeyIn {
			case "header":
				req.Header.Set(scheme.KeyName, keyValue)
			case "query":
				q.Set(scheme.KeyName, keyValue)
			}
		case AuthOAuth2:
			if scheme.AccessToken != "" {
				req.Header.Set("Authorization", "Bearer "+env.Resolve(scheme.AccessToken, ctx))
			}
		}
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = response.Body.Close() }()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, "", err
	}

	return response, string(bodyBytes), nil
}

func (r *Request) RunRequest() tea.Cmd {
	return func() tea.Msg {
		response, body, err := r.Send()
		if err != nil {
			return Failure("Failed executing request: " + err.Error())
		}

		return SuccessMsg{
			StatusCode: response.StatusCode,
			Status:     response.Status,
			Header:     response.Header.Clone(),
			Body:       body,
		}
	}
}
