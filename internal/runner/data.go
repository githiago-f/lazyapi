package runner

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MaxDataRows bounds a data file, so a wrong file cannot start a huge run.
const MaxDataRows = 10000

// LoadData reads a data file for a data-driven run: CSV with a header row,
// or JSON (an array of flat objects; nested values are kept as JSON text).
func LoadData(path string) ([]map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []map[string]string
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		rows, err = parseJSONData(raw)
	case ".csv":
		rows, err = parseCSVData(raw)
	default:
		return nil, fmt.Errorf("%s: data files must be .csv or .json", path)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s: no rows", path)
	}
	if len(rows) > MaxDataRows {
		return nil, fmt.Errorf("%s: %d rows (at most %d)", path, len(rows), MaxDataRows)
	}
	return rows, nil
}

func parseCSVData(raw []byte) ([]map[string]string, error) {
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))))
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading the header: %w", err)
	}
	for i, h := range header {
		header[i] = strings.TrimSpace(h)
		if header[i] == "" {
			return nil, fmt.Errorf("column %d has no name in the header", i+1)
		}
	}
	var rows []map[string]string
	for line := 2; ; line++ {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		row := make(map[string]string, len(header))
		for i, h := range header {
			if i < len(rec) {
				row[h] = rec[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseJSONData(raw []byte) ([]map[string]string, error) {
	var items []map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&items); err != nil {
		return nil, fmt.Errorf("expected an array of objects: %w", err)
	}
	rows := make([]map[string]string, 0, len(items))
	for _, it := range items {
		row := make(map[string]string, len(it))
		for k, v := range it {
			switch t := v.(type) {
			case nil:
				row[k] = ""
			case string:
				row[k] = t
			case json.Number:
				row[k] = t.String()
			case bool:
				row[k] = fmt.Sprint(t)
			default:
				data, _ := json.Marshal(t)
				row[k] = string(data)
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}
