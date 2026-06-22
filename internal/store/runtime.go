package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/model"
	"gopkg.in/yaml.v3"
)

func CompanionPath(specPath string) string {
	ext := filepath.Ext(specPath)
	base := strings.TrimSuffix(specPath, ext)
	return base + ".lazyapi.yml"
}

func runtimeKey(data model.Request) string {
	if data.OpenAPIRef != nil {
		return data.OpenAPIRef.Method + " " + data.OpenAPIRef.Path
	}
	if data.DraftPath != "" {
		return "__draft__:" + data.DraftPath
	}
	return data.Method.Label() + " " + data.URI
}

type RuntimeEntry struct {
	URI     string              `yaml:"uri,omitempty"`
	Body    string              `yaml:"body,omitempty"`
	Params  map[string]string   `yaml:"params,omitempty"`
	Query   map[string]string   `yaml:"query,omitempty"`
	Headers map[string]string   `yaml:"headers,omitempty"`
	Auth    []model.AuthScheme  `yaml:"auth,omitempty"`

	ParamsEnabled  map[string]bool `yaml:"paramsEnabled,omitempty"`
	QueryEnabled   map[string]bool `yaml:"queryEnabled,omitempty"`
	HeadersEnabled map[string]bool `yaml:"headersEnabled,omitempty"`
	AuthEnabled    []bool          `yaml:"authEnabled,omitempty"`
}

type runtimeFile struct {
	ServerURL string                       `yaml:"server_url,omitempty"`
	Requests  map[string]RuntimeEntry      `yaml:"requests"`
}

func readRuntimeFile(path string) (*runtimeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &runtimeFile{Requests: make(map[string]RuntimeEntry)}, nil
		}
		return nil, err
	}
	var rf runtimeFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("failed to parse runtime file %s: %w", path, err)
	}
	if rf.Requests == nil {
		rf.Requests = make(map[string]RuntimeEntry)
	}
	return &rf, nil
}

func writeRuntimeFile(path string, rf *runtimeFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(rf)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func SaveRuntimeMap(specPath string, data model.Request) error {
	path := CompanionPath(specPath)
	rf, err := readRuntimeFile(path)
	if err != nil {
		return err
	}

	key := runtimeKey(data)

	entry := RuntimeEntry{
		URI:    data.URI,
		Body:   data.Body.Raw,
		Params: data.Params,
		Query:  data.Query,
		Headers: make(map[string]string),
		Auth:    data.Auth,

		ParamsEnabled:  data.ParamsEnabled,
		QueryEnabled:   data.QueryEnabled,
		HeadersEnabled: data.HeadersEnabled,
		AuthEnabled:    data.AuthEnabled,
	}
	if len(data.Headers) > 0 {
		entry.Headers = data.Headers
	}
	if len(data.Auth) > 0 && len(data.AuthEnabled) == 0 && len(data.Auth) > 0 {
		entry.AuthEnabled = make([]bool, len(data.Auth))
		for i := range data.Auth {
			entry.AuthEnabled[i] = true
		}
	}

	rf.Requests[key] = entry
	return writeRuntimeFile(path, rf)
}

func LoadRuntimeMap(specPath, method, path string) (*RuntimeEntry, error) {
	rf, err := readRuntimeFile(CompanionPath(specPath))
	if err != nil {
		return nil, err
	}
	key := method + " " + path
	entry, ok := rf.Requests[key]
	if !ok {
		return nil, nil
	}
	return &entry, nil
}

func RemoveRuntimeEntry(specPath, method, path string) error {
	rf, err := readRuntimeFile(CompanionPath(specPath))
	if err != nil {
		return err
	}
	key := method + " " + path
	delete(rf.Requests, key)
	return writeRuntimeFile(CompanionPath(specPath), rf)
}

func SaveServerURL(specPath, url string) error {
	rf, err := readRuntimeFile(CompanionPath(specPath))
	if err != nil {
		return err
	}
	rf.ServerURL = url
	return writeRuntimeFile(CompanionPath(specPath), rf)
}

func SaveServerURLCmd(specPath, url string) tea.Cmd {
	return func() tea.Msg {
		if err := SaveServerURL(specPath, url); err != nil {
			return tea.Batch(tea.Println("Error saving server URL: "+err.Error()), tea.Quit)
		}
		return nil
	}
}

func LoadServerURL(specPath string) string {
	rf, err := readRuntimeFile(CompanionPath(specPath))
	if err != nil {
		return ""
	}
	return rf.ServerURL
}
