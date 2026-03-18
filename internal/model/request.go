// Package model implements every possible value that deals with data
package model

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
