// Package model implements every possible value that deals with data
package model

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type Method int

const (
	POST Method = iota
	GET
	PATCH
	PUT
	DELETE
	OPTIONS
	HEAD
)

const LastMethod = HEAD

// methodLabels is the single Method ↔ string table.
var methodLabels = [...]string{
	POST:    "POST",
	GET:     "GET",
	PATCH:   "PATCH",
	PUT:     "PUT",
	DELETE:  "DELETE",
	OPTIONS: "OPTIONS",
	HEAD:    "HEAD",
}

// Label is the upper-case method name ("" for an unknown value).
func (m Method) Label() string {
	if m < 0 || int(m) >= len(methodLabels) {
		return ""
	}
	return methodLabels[m]
}

// ParseMethod reads a method name, case-insensitively.
func ParseMethod(label string) (Method, bool) {
	label = strings.ToUpper(strings.TrimSpace(label))
	for m, l := range methodLabels {
		if l == label {
			return Method(m), true
		}
	}
	return 0, false
}

// MethodFromLabel is ParseMethod with GET for unknown names.
func MethodFromLabel(label string) Method {
	if m, ok := ParseMethod(label); ok {
		return m
	}
	return GET
}

func (m Method) MarshalYAML() (interface{}, error) {
	return strings.ToLower(m.Label()), nil
}

func (m *Method) UnmarshalYAML(node *yaml.Node) error {
	var value string
	if err := node.Decode(&value); err != nil {
		return err
	}
	parsed, ok := ParseMethod(value)
	if !ok {
		return fmt.Errorf("unknown HTTP method %q", value)
	}
	*m = parsed
	return nil
}
