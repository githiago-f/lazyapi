// Package store implements functions for defining and
// consuming storage files
package store

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	requestlist "github.com/githiago-f/lazyapi/internal/components/request_list"
	"gopkg.in/yaml.v3"
)

type RequestFilesMsg struct {
	Paths []string
}

type LoadedRequestListMsg struct {
	Items []list.Item
}

func FindRequestFiles() tea.Cmd {
	return func() tea.Msg {
		files, err := Glob("./**/*.req.yml")
		if err != nil {
			return err
		}

		return RequestFilesMsg{
			Paths: files,
		}
	}
}

func LoadRequestsList(paths []string) tea.Cmd {
	return func() tea.Msg {
		requests := []list.Item{}
		for _, filePath := range paths {
			file, err := os.Open(filePath)
			if err != nil {
				msg := fmt.Sprintf("Error when trying to parse files %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}

			decoder := yaml.NewDecoder(file)
			var request requestlist.RequestItem
			decoder.Decode(&request)
			request.FileName = filePath

			requests = append(requests, request)
		}

		return LoadedRequestListMsg{
			Items: requests,
		}
	}
}
