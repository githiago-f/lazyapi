package editor

import (
	"encoding/json"
	"strings"

	"charm.land/lipgloss/v2"
	"gopkg.in/yaml.v3"
)

func formatContent(body string, contentType string, width int) string {
	if ct := detectType(contentType, body); ct == "json" {
		var parsed any
		if err := json.Unmarshal([]byte(body), &parsed); err == nil {
			if formatted, err := json.MarshalIndent(parsed, "", "  "); err == nil {
				body = string(formatted)
			}
		}
	} else if ct == "yaml" {
		var parsed any
		if err := yaml.Unmarshal([]byte(body), &parsed); err == nil {
			if formatted, err := yaml.Marshal(parsed); err == nil {
				body = string(formatted)
			}
		}
	}

	return wrapText(body, width)
}

func detectType(contentType string, body string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "json"):
		return "json"
	case strings.Contains(ct, "yaml"), strings.Contains(ct, "x-yaml"):
		return "yaml"
	case strings.Contains(ct, "html"):
		return "html"
	case strings.Contains(ct, "xml"):
		return "xml"
	}

	trimmed := strings.TrimSpace(body)
	if len(trimmed) > 0 && trimmed[0] == '{' || trimmed[0] == '[' {
		if json.Valid([]byte(trimmed)) {
			return "json"
		}
	}

	return "text"
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(wrapLine(line, width))
	}
	return result.String()
}

func wrapLine(line string, width int) string {
	if lipgloss.Width(line) <= width {
		return line
	}

	if strings.TrimSpace(line) == "" {
		return line
	}

	var result strings.Builder
	current := strings.TrimSpace(line)
	for lipgloss.Width(current) > width {
		idx := findBreak(current, width)
		if idx <= 0 {
			idx = width
		}
		if idx > len(current) {
			idx = len(current)
		}
		result.WriteString(current[:idx])
		result.WriteString("\n")
		current = strings.TrimSpace(current[idx:])
	}
	if len(current) > 0 {
		result.WriteString(current)
	}
	return result.String()
}

func findBreak(s string, width int) int {
	if len(s) <= width {
		return len(s)
	}

	candidate := width
	for candidate > 0 && s[candidate] != ' ' {
		candidate--
	}
	if candidate > 0 {
		return candidate
	}

	return width
}
