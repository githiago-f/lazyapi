// Package model implements every possible value that deals with data
package model

import (
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

func (m *Method) UnmarshalYAML(node *yaml.Node) error {
	var value string
	if err := node.Decode(&value); err != nil {
		return err
	}

	switch strings.ToUpper(value) {
	case "GET":
		*m = GET
	case "POST":
		*m = POST
	case "PATCH":
		*m = PATCH
	case "PUT":
		*m = PUT
	case "DELETE":
		*m = DELETE
	case "OPTIONS":
		*m = OPTIONS
	case "HEAD":
		*m = HEAD
	}

	return nil
}

func (m Method) Label() string {
	switch m {
	case GET:
		return "GET"
	case POST:
		return "POST"
	case PATCH:
		return "PATCH"
	case PUT:
		return "PUT"
	case DELETE:
		return "DELETE"
	case OPTIONS:
		return "OPTIONS"
	case HEAD:
		return "HEAD"
	default:
		return ""
	}
}
