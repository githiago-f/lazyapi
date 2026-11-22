package model

import (
	"fmt"
	"strings"
)

// RequestItem is one entry of the request list: an operation of a spec, or
// an unsaved draft (DraftPath set, OpenAPIRef nil). It is plain data; the
// TUI renders it (app/pane/requests).
type RequestItem struct {
	Method      Method
	URI         string
	About       About
	FileName    string
	Tags        []string
	RequestTime float64
	DraftPath   string

	OpenAPIRef *OpenAPIRef
}

// FilterValue is the text the list filter matches against.
func (ri RequestItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s %s", ri.Method.Label(), ri.URI, ri.About.Summary, strings.Join(ri.Tags, " "))
}
