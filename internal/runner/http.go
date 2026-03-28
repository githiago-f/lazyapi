// Package runner defines network level executors, testing dsl commands and watchers
package runner

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/model"
)

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

func RunRequest(request model.Request) tea.Cmd {
	url, err := url.Parse(request.URI)
	if err != nil {
		return func() tea.Msg {
			return Failure("Failed parsing url: " + err.Error())
		}
	}

	return func() tea.Msg {
		body := io.NopCloser(strings.NewReader(request.Body.Raw))
		defer body.Close()

		response, err := http.DefaultClient.Do(&http.Request{
			Method: request.Method.Label(),
			URL:    url,
			Body:   body,
		})
		if err != nil {
			return Failure("Failed executing request: " + err.Error())
		}

		return SuccessMsg{Response: response}
	}
}
