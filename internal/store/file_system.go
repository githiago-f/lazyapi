// Package store implements functions for defining and
// consuming storage files
package store

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/app/pane/requests"
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

type FileSaved struct {
	Path string
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
		listItems := []list.Item{}
		for _, filePath := range paths {
			file, err := os.Open(filePath)
			if err != nil {
				msg := fmt.Sprintf("Error when trying to parse files %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}

			decoder := yaml.NewDecoder(file)
			var request requests.RequestItem
			decoder.Decode(&request)
			request.FileName = filePath

			listItems = append(listItems, request)
		}

		return LoadedRequestListMsg{
			Items: listItems,
		}
	}
}

func OpenRequestFile(filePath string) tea.Cmd {
	return func() tea.Msg {
		sourcePath := filePath
		tempPath := TempPath(filePath)

		// Check if a temp file exists (from a previous unsaved session)
		if _, err := os.Stat(tempPath); err == nil {
			sourcePath = tempPath
		}

		file, err := os.Open(sourcePath)
		if err != nil {
			msg := fmt.Sprintf("Error when trying to open file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}
		defer file.Close()

		decoder := yaml.NewDecoder(file)

		var request model.Request
		decoder.Decode(&request)
		request.FileName = filePath

		return LoadedFile{Data: request}
	}
}

func TempPath(filePath string) string {
	return filePath + ".lazyapi.tmp"
}

func SaveTempFile(data model.Request) tea.Cmd {
	return func() tea.Msg {
		if data.FileName == "" {
			return nil
		}
		path := TempPath(data.FileName)
		file, err := os.Create(path)
		if err != nil {
			msg := fmt.Sprintf("Error when trying to save temp file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}
		defer file.Close()

		encoder := yaml.NewEncoder(file)
		err = encoder.Encode(data)
		if err != nil {
			msg := fmt.Sprintf("Error when encoding temp file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}

		return nil
	}
}

func RemoveTempFile(filePath string) tea.Cmd {
	return func() tea.Msg {
		os.Remove(TempPath(filePath))
		return nil
	}
}

func SaveFile(data model.Request) tea.Cmd {
	return func() tea.Msg {
		file, err := os.Create(data.FileName)
		if err != nil {
			msg := fmt.Sprintf("Error when trying to save file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}
		defer file.Close()

		encoder := yaml.NewEncoder(file)
		err = encoder.Encode(data)
		if err != nil {
			msg := fmt.Sprintf("Error when encoding file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}

		return FileSaved{Path: data.FileName}
	}
}
