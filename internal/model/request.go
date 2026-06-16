package model

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Request struct {
	FileName string

	About About `yaml:"about"`

	URI    string `yaml:"uri"`
	Method Method `yaml:"method"`

	Body    Body              `yaml:"body"`
	Headers map[string]string `yaml:"headers"`
	Params  map[string]string `yaml:"pathParams"`
	Query   map[string]string `yaml:"query"`
}

type FailureMsg struct {
	Message string
}

type SuccessMsg struct {
	Response *http.Response
}

func Failure(msg string) tea.Msg {
	return FailureMsg{
		Message: msg,
	}
}

func (r *Request) RunRequest() tea.Cmd {
	return func() tea.Msg {
		url, err := url.Parse(r.URI)
		if err != nil {
			return Failure("Failed parsing url: " + err.Error())
		}

		body := io.NopCloser(strings.NewReader(r.Body.Raw))
		defer body.Close()

		response, err := http.DefaultClient.Do(&http.Request{
			Method: r.Method.Label(),
			URL:    url,
			Body:   body,
		})
		if err != nil {
			return Failure("Failed executing request: " + err.Error())
		}

		return SuccessMsg{Response: response}
	}
}
