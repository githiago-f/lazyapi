package store

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
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
		yamlFiles, err := Glob("./**/*.yml")
		if err != nil {
			return err
		}
		yamlFiles2, err := Glob("./**/*.yaml")
		if err != nil {
			return err
		}

		var openAPIFiles []string
		for _, f := range append(yamlFiles, yamlFiles2...) {
			if IsOpenAPIFile(f) {
				openAPIFiles = append(openAPIFiles, f)
			}
		}

		return RequestFilesMsg{
			Paths: openAPIFiles,
		}
	}
}

func LoadRequestsList(paths []string) tea.Cmd {
	return func() tea.Msg {
		listItems := []list.Item{}
		for _, filePath := range paths {
			spec, err := ParseSpec(filePath)
			if err != nil {
				msg := fmt.Sprintf("Error when trying to parse file %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			ops := ListOperations(spec, filePath)
			for _, op := range ops {
				listItems = append(listItems, op)
			}
		}
		return LoadedRequestListMsg{
			Items: listItems,
		}
	}
}

func OpenRequestFile(ref model.OpenAPIRef) tea.Cmd {
	return func() tea.Msg {
		tempPath := tempPathForRef(ref)

		if _, err := os.Stat(tempPath); err == nil {
			file, err := os.Open(tempPath)
			if err != nil {
				msg := fmt.Sprintf("Error when trying to open temp file, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			defer file.Close()

			var request model.Request
			decoder := yaml.NewDecoder(file)
			if err := decoder.Decode(&request); err == nil && request.OpenAPIRef != nil {
				return LoadedFile{Data: request}
			}
		}

		spec, err := ParseSpec(ref.FilePath)
		if err != nil {
			msg := fmt.Sprintf("Error when trying to open file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}

		request := OperationToRequest(spec, ref)
		return LoadedFile{Data: request}
	}
}

func tempPathForRef(ref model.OpenAPIRef) string {
	safe := sanitizePath(ref.Path)
	return ref.FilePath + ".lazyapi.tmp." + ref.Method + "." + safe
}

func sanitizePath(path string) string {
	r := strings.NewReplacer(
		"/", "_",
		"{", "_",
		"}", "_",
		" ", "_",
	)
	return r.Replace(path)
}

func TempPath(filePath string) string {
	return filePath + ".lazyapi.tmp"
}

func SaveTempFile(data model.Request) tea.Cmd {
	return func() tea.Msg {
		if data.FileName == "" {
			return nil
		}
		var path string
		if data.OpenAPIRef != nil {
			path = tempPathForRef(*data.OpenAPIRef)
		} else {
			path = TempPath(data.FileName)
		}
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

func SaveFile(data model.Request) tea.Cmd {
	return func() tea.Msg {
		var savedPath string
		if data.OpenAPIRef != nil {
			ref := *data.OpenAPIRef
			spec, err := ParseSpec(ref.FilePath)
			if err != nil {
				msg := fmt.Sprintf("Error when trying to save file, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			if err := ApplyRequestToOperation(spec, ref, data); err != nil {
				msg := fmt.Sprintf("Error when applying changes to spec, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			if err := SaveSpec(ref.FilePath, spec); err != nil {
				msg := fmt.Sprintf("Error when writing file, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			savedPath = ref.FilePath
			os.Remove(tempPathForRef(ref))
		} else {
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

			savedPath = data.FileName
			os.Remove(TempPath(data.FileName))
		}

		return FileSaved{Path: savedPath}
	}
}
