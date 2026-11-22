package jq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/itchyny/gojq"
)

// jqTimeout bounds a filter so a runaway expression (e.g. `repeat(.)`)
// cannot freeze the UI.
const jqTimeout = 2 * time.Second

// Apply applies a jq filter to a JSON document. Each result is printed as
// indented JSON on its own block; strings are printed as JSON strings.
func Apply(filter, body string) (string, error) {
	code, err := compile(filter)
	if err != nil {
		return "", err
	}
	var input any
	dec := json.NewDecoder(strings.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&input); err != nil {
		return "", errors.New("jq: the body is not JSON")
	}
	input = normalizeNumbers(input)

	ctx, cancel := context.WithTimeout(context.Background(), jqTimeout)
	defer cancel()
	var out []string
	iter := code.RunWithContext(ctx, input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			var halt *gojq.HaltError
			if errors.As(err, &halt) && halt.Value() == nil {
				break
			}
			if ctx.Err() != nil {
				return "", errors.New("jq: filter took too long")
			}
			return "", fmt.Errorf("jq: %w", err)
		}
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "", fmt.Errorf("jq: %w", err)
		}
		out = append(out, string(data))
		if len(out) > 10000 {
			return "", errors.New("jq: too many results")
		}
	}
	return strings.Join(out, "\n"), nil
}

// normalizeNumbers turns json.Number into int or float64, the types gojq
// understands, keeping large integers exact.
func normalizeNumbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
		f, _ := t.Float64()
		return f
	case map[string]any:
		for k, x := range t {
			t[k] = normalizeNumbers(x)
		}
		return t
	case []any:
		for i, x := range t {
			t[i] = normalizeNumbers(x)
		}
		return t
	}
	return v
}

// Check reports whether filter is a valid jq program ("" is valid: no filter).
func Check(filter string) error {
	if filter == "" {
		return nil
	}
	_, err := compile(filter)
	return err
}

func compile(filter string) (*gojq.Code, error) {
	q, err := gojq.Parse(filter)
	if err != nil {
		return nil, fmt.Errorf("jq: %w", err)
	}
	code, err := gojq.Compile(q)
	if err != nil {
		return nil, fmt.Errorf("jq: %w", err)
	}
	return code, nil
}
