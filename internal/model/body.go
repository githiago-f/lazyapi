package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MimeType string

const (
	ApplicationJSON MimeType = "application/json"
	PlainText       MimeType = "text/plain"
	FormURLEncoded  MimeType = "application/x-www-form-urlencoded"
	MultipartForm   MimeType = "multipart/form-data"
)

type Body struct {
	Type MimeType
	Raw  string
	// Files are multipart file parts: field name → local path. They are only
	// set by the user (CLI --file, the TUI editor), never derived from Raw,
	// so scripts and remote callers cannot make lazyapi read arbitrary files.
	Files map[string]string `yaml:"files,omitempty"`
	// File is a local file sent as the whole body (a binary body, with Type
	// as its Content-Type). Like Files, only the user sets it.
	File string `yaml:"file,omitempty"`
}

// OctetStream is the default type of a binary (file) body.
const OctetStream MimeType = "application/octet-stream"

// IsForm reports whether the body is form-urlencoded or multipart.
func (b Body) IsForm() bool {
	t := strings.ToLower(string(b.Type))
	return strings.HasPrefix(t, string(FormURLEncoded)) || strings.HasPrefix(t, string(MultipartForm))
}

// encode returns the wire body and its Content-Type for raw (already
// template-resolved). Form bodies given as a JSON object are encoded as
// fields; anything else is sent as-is with the body's type.
func (b Body) encode(raw string) (io.Reader, string, error) {
	t := strings.ToLower(string(b.Type))
	switch {
	case b.File != "":
		data, err := os.ReadFile(b.File)
		if err != nil {
			return nil, "", fmt.Errorf("body file: %w", err)
		}
		if b.Type == "" {
			return bytes.NewReader(data), string(OctetStream), nil
		}
		return bytes.NewReader(data), string(b.Type), nil
	case strings.HasPrefix(t, string(MultipartForm)):
		return b.multipart(raw)
	case strings.HasPrefix(t, string(FormURLEncoded)):
		fields, ok := jsonFields(raw)
		if !ok {
			break // already encoded (a=1&b=2)
		}
		values := url.Values{}
		for _, f := range fields {
			values.Add(f.name, f.value)
		}
		return strings.NewReader(values.Encode()), string(FormURLEncoded), nil
	}
	if raw == "" {
		return nil, string(b.Type), nil
	}
	return strings.NewReader(raw), string(b.Type), nil
}

func (b Body) multipart(raw string) (io.Reader, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if strings.TrimSpace(raw) != "" {
		fields, ok := jsonFields(raw)
		if !ok {
			return nil, "", fmt.Errorf("multipart body must be a JSON object of fields, e.g. {\"name\": \"ana\"}")
		}
		for _, f := range fields {
			if err := w.WriteField(f.name, f.value); err != nil {
				return nil, "", err
			}
		}
	}
	names := make([]string, 0, len(b.Files))
	for n := range b.Files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		path := b.Files[name]
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("file part %q: %w", name, err)
		}
		part, err := w.CreateFormFile(name, filepath.Base(path))
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(data); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return &buf, w.FormDataContentType(), nil
}

type field struct{ name, value string }

// jsonFields flattens a JSON object into form fields, sorted by name.
// Arrays repeat the field; objects are JSON-encoded.
func jsonFields(raw string) ([]field, bool) {
	var obj map[string]any
	if json.Unmarshal([]byte(raw), &obj) != nil {
		return nil, false
	}
	names := make([]string, 0, len(obj))
	for k := range obj {
		names = append(names, k)
	}
	sort.Strings(names)
	var out []field
	for _, k := range names {
		switch v := obj[k].(type) {
		case []any:
			for _, item := range v {
				out = append(out, field{k, scalar(item)})
			}
		default:
			out = append(out, field{k, scalar(v)})
		}
	}
	return out, true
}

func scalar(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64, bool:
		return fmt.Sprint(t)
	default:
		data, _ := json.Marshal(t)
		return string(data)
	}
}
