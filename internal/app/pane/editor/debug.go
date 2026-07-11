package editor

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/model"
)

func DebugRenderPane(width, height int) string {
	req := model.Request{
		Method:    model.GET,
		URI:       "/api/pets",
		ServerURL: "http://localhost:8080",
		Servers:   []string{"http://localhost:8080", "https://api.example.com"},
		About: model.About{
			Summary:     "List all pets",
			Description: "Returns a list of all pets in the system",
		},
		Body: model.Body{
			Type: model.ApplicationJSON,
			Raw:  "{\n  \"name\": \"Fluffy\"\n}",
		},
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer token123",
		},
		Query: map[string]string{
			"limit":  "10",
			"offset": "0",
		},
		Params: map[string]string{
			"petId": "123",
		},
	}

	m, _ := New(&req).Update(tea.WindowSizeMsg{Width: width, Height: height})
	return m.(RequestPane).View()
}
