// Package store implements functions for defining and
// consuming storage files
package store

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	requestlist "github.com/githiago-f/lazyapi/internal/app/pane/requestlist"
	"github.com/githiago-f/lazyapi/internal/model"
	"gopkg.in/yaml.v3"
)

type RequestFilesMsg struct {
	Paths []string
}

type LoadedRequestListMsg struct {
	Items []list.Item
}

type LoadedFile struct {
	Data model.Request
}

type FileSaved int

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

func OpenRequestFile(filePath string) tea.Cmd {
	return func() tea.Msg {
		file, err := os.Open(filePath)
		if err != nil {
			msg := fmt.Sprintf("Error when trying to open file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}

		decoder := yaml.NewDecoder(file)

		var request model.Request
		decoder.Decode(&request)
		request.FileName = filePath

		return LoadedFile{Data: request}
	}
}

func SaveFile(data model.Request) tea.Cmd {
	return func() tea.Msg {
		file, err := os.Open(data.FileName)
		if err != nil {
			msg := fmt.Sprintf("Error when trying to open file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}

		encoder := yaml.NewEncoder(file)
		encoder.Encode(data)

		return FileSaved(0)
	}
}
