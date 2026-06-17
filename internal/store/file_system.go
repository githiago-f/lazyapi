package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

type DuplicateData struct {
	Data model.Request
}

var newDraftCounter int

func NewDraftPath(filePath string) string {
	newDraftCounter++
	return fmt.Sprintf("%s.lazyapi.draft.new.%d", filePath, newDraftCounter)
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

			drafts := ListDrafts(filePath)
			for _, d := range drafts {
				listItems = append(listItems, d)
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

func OpenDraftFile(draftPath, fileName string) tea.Cmd {
	return func() tea.Msg {
		file, err := os.Open(draftPath)
		if err != nil {
			msg := fmt.Sprintf("Error when trying to open draft file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}
		defer file.Close()

		var request model.Request
		decoder := yaml.NewDecoder(file)
		if err := decoder.Decode(&request); err != nil {
			msg := fmt.Sprintf("Error when decoding draft file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}
		request.DraftPath = draftPath
		request.FileName = fileName
		request.Servers, request.ServerURL = LoadServers(fileName)
		return LoadedFile{Data: request}
	}
}

func tempPathForRef(ref model.OpenAPIRef) string {
	safe := sanitizePath(ref.Path)
	return ref.FilePath + ".lazyapi.tmp." + ref.Method + "." + safe
}

func ListDrafts(filePath string) []requests.RequestItem {
	pattern := filePath + ".lazyapi.draft.*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}

	var items []requests.RequestItem
	for _, draftFile := range matches {
		data, err := os.ReadFile(draftFile)
		if err != nil {
			continue
		}
		var req model.Request
		if err := yaml.Unmarshal(data, &req); err != nil {
			continue
		}
		items = append(items, requests.RequestItem{
			Method:    req.Method,
			URI:       req.URI,
			About:     req.About,
			FileName:  filePath,
			DraftPath: draftFile,
		})
	}
	return items
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
		if data.DraftPath != "" {
			path = data.DraftPath
		} else if data.OpenAPIRef != nil {
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

func LoadForDuplicate(item requests.RequestItem) tea.Cmd {
	return func() tea.Msg {
		if item.DraftPath != "" {
			file, err := os.Open(item.DraftPath)
			if err != nil {
				msg := fmt.Sprintf("Error when opening draft for duplicate, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			defer file.Close()
			var req model.Request
			decoder := yaml.NewDecoder(file)
			if err := decoder.Decode(&req); err != nil {
				msg := fmt.Sprintf("Error when decoding draft for duplicate, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			req.FileName = item.FileName
			req.Servers, req.ServerURL = LoadServers(item.FileName)
			return DuplicateData{Data: req}
		}

		if item.OpenAPIRef != nil {
			spec, err := ParseSpec(item.FileName)
			if err != nil {
				msg := fmt.Sprintf("Error when parsing spec for duplicate, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			req := OperationToRequest(spec, *item.OpenAPIRef)
			return DuplicateData{Data: req}
		}

		return tea.Batch(tea.Println("Cannot duplicate: no source found"), tea.Quit)
	}
}

func SaveFile(data model.Request) tea.Cmd {
	return func() tea.Msg {
		var savedPath string
		if data.DraftPath != "" {
			spec, err := ParseSpec(data.FileName)
			if err != nil {
				msg := fmt.Sprintf("Error when trying to save file, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			if err := AddOperationToSpec(spec, data.URI, data.Method.Label(), data); err != nil {
				msg := fmt.Sprintf("Error when adding operation to spec, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			if err := SaveSpec(data.FileName, spec); err != nil {
				msg := fmt.Sprintf("Error when writing file, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			savedPath = data.FileName
			os.Remove(data.DraftPath)
		} else if data.OpenAPIRef != nil {
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

func DeleteRequestFile(item requests.RequestItem) tea.Cmd {
	return func() tea.Msg {
		if item.DraftPath != "" {
			if err := os.Remove(item.DraftPath); err != nil {
				msg := fmt.Sprintf("Error when deleting draft: %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			return FileSaved{Path: item.DraftPath}
		}

		if item.OpenAPIRef != nil {
			ref := *item.OpenAPIRef
			spec, err := ParseSpec(ref.FilePath)
			if err != nil {
				msg := fmt.Sprintf("Error when parsing spec: %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			if err := RemoveOperationFromSpec(spec, ref); err != nil {
				msg := fmt.Sprintf("Error when removing operation: %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			if err := SaveSpec(ref.FilePath, spec); err != nil {
				msg := fmt.Sprintf("Error when writing file: %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			os.Remove(tempPathForRef(ref))
			return FileSaved{Path: ref.FilePath}
		}

		return nil
	}
}
