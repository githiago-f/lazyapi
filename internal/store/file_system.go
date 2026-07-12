// Package store defines mappers and filesystem commands
package store

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/githiago-f/lazyapi/internal/app/pane/requests"
	"github.com/githiago-f/lazyapi/internal/model"
	"gopkg.in/yaml.v3"
)

type RequestFilesMsg struct {
	Paths []string
}

type LoadedRequestListMsg struct {
	Items []requests.RequestItem
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

type ExampleSavedMsg struct {
	Success bool
	Error   string
}

type TagsUpdatedMsg struct {
	Success bool
	Error   string
}

func UpdateOperationTagsCmd(ref model.OpenAPIRef, tags []string) tea.Cmd {
	return func() tea.Msg {
		if err := UpdateOperationTags(ref, tags); err != nil {
			return TagsUpdatedMsg{Error: err.Error()}
		}
		return TagsUpdatedMsg{Success: true}
	}
}

func UpdateDraftTagsCmd(draftPath string, tags []string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(draftPath)
		if err != nil {
			return TagsUpdatedMsg{Error: fmt.Sprintf("failed to read draft: %v", err)}
		}
		var req model.Request
		if err := yaml.Unmarshal(data, &req); err != nil {
			return TagsUpdatedMsg{Error: fmt.Sprintf("failed to parse draft: %v", err)}
		}
		req.Tags = tags

		file, err := os.Create(draftPath)
		if err != nil {
			return TagsUpdatedMsg{Error: fmt.Sprintf("failed to write draft: %v", err)}
		}
		encoder := yaml.NewEncoder(file)
		if err := encoder.Encode(req); err != nil {
			return TagsUpdatedMsg{Error: fmt.Sprintf("failed to encode draft: %v", err)}
		}
		if err := file.Close(); err != nil {
			return TagsUpdatedMsg{Error: fmt.Sprintf("failed to close draft: %v", err)}
		}
		return TagsUpdatedMsg{Success: true}
	}
}

func PersistServerURLCmd(ref model.OpenAPIRef, url string) tea.Cmd {
	return func() tea.Msg {
		path := tempPathForRef(ref)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var req model.Request
		if err := yaml.Unmarshal(data, &req); err != nil {
			return nil
		}
		if req.ServerURL == url {
			return nil
		}
		req.ServerURL = url
		file, err := os.Create(path)
		if err != nil {
			return nil
		}
		encoder := yaml.NewEncoder(file)
		if err := encoder.Encode(req); err != nil {
			_ = file.Close()
			return nil
		}
		if err := file.Close(); err != nil {
			return nil
		}
		return nil
	}
}

func SaveResponseExampleCmd(ref model.OpenAPIRef, statusCode int, header http.Header, body string) tea.Cmd {
	return func() tea.Msg {
		spec, err := ParseSpec(ref.FilePath)
		if err != nil {
			return ExampleSavedMsg{Error: "Error parsing spec: " + err.Error()}
		}
		if err := SaveResponseExample(spec, ref, statusCode, header, body); err != nil {
			return ExampleSavedMsg{Error: "Error saving example: " + err.Error()}
		}
		if err := SaveSpec(ref.FilePath, spec); err != nil {
			return ExampleSavedMsg{Error: "Error writing spec: " + err.Error()}
		}
		return ExampleSavedMsg{Success: true}
	}
}

var newDraftCounter int

func tempDirForFile(filePath string) string {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		abs = filePath
	}
	safe := sanitizePath(abs)
	return filepath.Join(os.TempDir(), "lazyapi", safe)
}

func NewDraftPath(filePath string) string {
	newDraftCounter++
	dir := tempDirForFile(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ""
	}
	return filepath.Join(dir, fmt.Sprintf("draft.new.%d", newDraftCounter))
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
		var listItems []requests.RequestItem
		for _, filePath := range paths {
			spec, err := ParseSpec(filePath)
			if err != nil {
				msg := fmt.Sprintf("Error when trying to parse file %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			ops := ListOperations(spec, filePath)
			listItems = append(listItems, ops...)

			drafts := ListDrafts(filePath)
			listItems = append(listItems, drafts...)
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
			defer func() {
				if err = file.Close(); err != nil {
					panic(err)
				}
			}()

			var request model.Request
			decoder := yaml.NewDecoder(file)
			if err := decoder.Decode(&request); err == nil {
				request.OpenAPIRef = &model.OpenAPIRef{
					FilePath: ref.FilePath,
					Path:     ref.Path,
					Method:   ref.Method,
				}
				servers, defaultURL := LoadServers(ref.FilePath)
				request.Servers = servers
				if !contains(servers, request.ServerURL) {
					request.ServerURL = defaultURL
				}
				if request.ServerURL == "" {
					request.ServerURL = defaultURL
				}
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

		var request model.Request
		decoder := yaml.NewDecoder(file)
		if err := decoder.Decode(&request); err != nil {
			msg := fmt.Sprintf("Error when decoding draft file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}
		request.DraftPath = draftPath
		request.FileName = fileName
		prevURL := request.ServerURL
		request.Servers, request.ServerURL = LoadServers(fileName)
		if prevURL != "" && contains(request.Servers, prevURL) {
			request.ServerURL = prevURL
		}

		err = file.Close()
		if err != nil {
			msg := fmt.Sprintf("Error closing file: %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}

		return LoadedFile{Data: request}
	}
}

func tempPathForRef(ref model.OpenAPIRef) string {
	dir := tempDirForFile(ref.FilePath)
	safe := sanitizePath(ref.Path)
	return filepath.Join(dir, fmt.Sprintf("tmp.%s.%s", ref.Method, safe))
}

func ListDrafts(filePath string) []requests.RequestItem {
	dir := tempDirForFile(filePath)
	pattern := filepath.Join(dir, "draft.*")
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
			Tags:      req.Tags,
			FileName:  filePath,
			DraftPath: draftFile,
		})
	}
	return items
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
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
	return filepath.Join(tempDirForFile(filePath), "tmp")
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
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			msg := fmt.Sprintf("Error creating temp directory: %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}
		file, err := os.Create(path)
		if err != nil {
			msg := fmt.Sprintf("Error when trying to save temp file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}

		encoder := yaml.NewEncoder(file)
		err = encoder.Encode(data)
		if err != nil {
			msg := fmt.Sprintf("Error when encoding temp file, %v", err)
			return tea.Batch(tea.Println(msg), tea.Quit)
		}

		err = file.Close()
		if err != nil {
			msg := fmt.Sprintf("Error when closing temp file, %v", err)
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
			defer func() {
				if cerr := file.Close(); cerr != nil {
					panic(cerr)
				}
			}()

			var req model.Request
			decoder := yaml.NewDecoder(file)
			if err := decoder.Decode(&req); err != nil {
				msg := fmt.Sprintf("Error when decoding draft for duplicate, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			req.FileName = item.FileName
			prevURL := req.ServerURL
			req.Servers, req.ServerURL = LoadServers(item.FileName)
			if prevURL != "" && contains(req.Servers, prevURL) {
				req.ServerURL = prevURL
			}
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
			if err = AddOperationToSpec(spec, data.URI, data.Method.Label(), data); err != nil {
				msg := fmt.Sprintf("Error when adding operation to spec, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			if err = SaveSpec(data.FileName, spec); err != nil {
				msg := fmt.Sprintf("Error when writing file, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			savedPath = data.FileName
			if err = os.Remove(data.DraftPath); err != nil {
				msg := fmt.Sprintf("Error removing tmp file: %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
		} else if data.OpenAPIRef != nil {
			ref := *data.OpenAPIRef
			spec, err := ParseSpec(ref.FilePath)
			if err != nil {
				msg := fmt.Sprintf("Error when trying to save file, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			if err = ApplyRequestToOperation(spec, ref, data); err != nil {
				msg := fmt.Sprintf("Error when applying changes to spec, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			if err = SaveSpec(ref.FilePath, spec); err != nil {
				msg := fmt.Sprintf("Error when writing file, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			savedPath = ref.FilePath

			if err = os.Remove(tempPathForRef(ref)); err != nil {
				msg := fmt.Sprintf("Error removing tmp file: %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
		} else {
			file, err := os.Create(data.FileName)
			if err != nil {
				msg := fmt.Sprintf("Error when trying to save file, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			defer func() {
				if cerr := file.Close(); cerr != nil {
					panic(cerr)
				}
			}()

			encoder := yaml.NewEncoder(file)
			err = encoder.Encode(data)
			if err != nil {
				msg := fmt.Sprintf("Error when encoding file, %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}

			savedPath = data.FileName
			if err = os.Remove(TempPath(data.FileName)); err != nil {
				msg := fmt.Sprintf("Error removing tmp file: %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
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
			if err = RemoveOperationFromSpec(spec, ref); err != nil {
				msg := fmt.Sprintf("Error when removing operation: %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			if err = SaveSpec(ref.FilePath, spec); err != nil {
				msg := fmt.Sprintf("Error when writing file: %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			if err = os.Remove(tempPathForRef(ref)); err != nil {
				msg := fmt.Sprintf("Error removing tmp file: %v", err)
				return tea.Batch(tea.Println(msg), tea.Quit)
			}
			return FileSaved{Path: ref.FilePath}
		}

		return nil
	}
}
