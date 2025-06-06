package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	internalError = tcell.ColorRed
	clientError   = tcell.ColorGreenYellow
	ok            = tcell.ColorGreen
	redirect      = tcell.ColorOlive
)

type ResponseHandler struct {
	ResponseView *tview.TextView
}

var View *ResponseHandler

func init() {
	View = newResponsePreview()
}

func (r *ResponseHandler) Handle(res string, code int32) {
	resBuf := bytes.NewBuffer([]byte{})
	json.Indent(resBuf, []byte(res), "", "  ")

	r.ResponseView.SetText(resBuf.String())

	title := []string{"Response", fmt.Sprintf("%d", code)}
	r.ResponseView.SetTitle(strings.Join(title, " - "))

	if code >= 200 {
		r.ResponseView.SetTitleColor(ok)
	}
	if code >= 300 {
		r.ResponseView.SetTitleColor(redirect)
	}
	if code >= 400 {
		r.ResponseView.SetTitleColor(clientError)
	}
	if code >= 500 {
		r.ResponseView.SetTitleColor(internalError)
	}
}

func newResponsePreview() *ResponseHandler {
	view := &ResponseHandler{ResponseView: tview.NewTextView()}
	view.ResponseView.SetBorder(true)

	return view
}
