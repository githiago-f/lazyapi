package model

import (
	"io"
	"net/http"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

	ServerURL string   `yaml:"-"`
	Servers   []string `yaml:"-"`

	OpenAPIRef *OpenAPIRef `yaml:"-"`
	DraftPath  string      `yaml:"-"`
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
	fullURL := r.ServerURL + r.URI

	for name, value := range r.Params {
		fullURL = strings.ReplaceAll(fullURL, "{"+name+"}", value)
	}

	var bodyReader io.Reader
	if r.Body.Raw != "" {
		bodyReader = strings.NewReader(r.Body.Raw)
	}

	req, err := http.NewRequest(r.Method.Label(), fullURL, bodyReader)
	if err != nil {
		return nil, "", err
	}

	q := req.URL.Query()
	for name, value := range r.Query {
		q.Set(name, value)
	}
	req.URL.RawQuery = q.Encode()

	for name, value := range r.Headers {
		req.Header.Set(name, value)
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()

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
