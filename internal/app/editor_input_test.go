package app

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/githiago-f/lazyapi/internal/components"
)

// openTab opens the editor on the selected request and enters the request
// tab at index (Docs 0, Params 1, Auth 2, Headers 3, Body 4).
func openTab(t *testing.T, tm interface{ Send(tea.Msg) }, index int) {
	t.Helper()
	for i := 0; i < 4; i++ {
		tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	for i := 0; i < index; i++ {
		tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	tm.Send(press("enter"))
}

// Two quick esc presses reach the program as one alt+esc: they still count
// as two, so hurrying out of a tab never gets stuck.
func TestTUI_QuickEscapesCount(t *testing.T) {
	tm, _ := newTUI(t)
	waitFor(t, tm, "/orders")
	tm.Send(press("enter"))
	waitFor(t, tm, "No response yet")
	openTab(t, tm, 3)
	waitFor(t, tm, "adds a header")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape, Mod: tea.ModAlt}) // esc esc: blur, leave the tab
	tm.Send(press("esc"))                                          // leave the editor
	waitFor(t, tm, "Press s to send")
}

// esc on the open method list closes the list, not the editor.
func TestTUI_EscClosesMethodListOnly(t *testing.T) {
	tm, _ := newTUI(t)
	waitFor(t, tm, "/orders")
	tm.Send(press("enter"))
	waitFor(t, tm, "No response yet")
	tm.Send(press("enter")) // the method has the focus: open its list
	waitFor(t, tm, "OPTIONS")
	tm.Send(press("esc"))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab}) // still in the editor: tab moves on
	if screen := finalScreen(t, tm); !strings.Contains(screen, "No response yet") {
		t.Errorf("esc on the list left the editor:\n%s", screen)
	}
}

// ctrl+v pastes into a header row (through the clipboard fallback).
func TestTUI_PasteIntoHeader(t *testing.T) {
	clip := &memClip{text: "abc 123\n"}
	components.Clipboard = clip
	t.Cleanup(func() { components.Clipboard = components.SystemClipboard{} })
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get("X-Token")
	}))
	t.Cleanup(srv.Close)
	spec := filepath.Join(t.TempDir(), "api.yml")
	if err := os.WriteFile(spec, []byte(fmt.Sprintf(`openapi: 3.0.0
info: {title: API, version: "1"}
servers: [{url: %s}]
paths:
  /items:
    get:
      responses: {"200": {description: ok}}
`, srv.URL)), 0o644); err != nil {
		t.Fatal(err)
	}
	tm := start(t, spec)
	waitFor(t, tm, "/items")
	tm.Send(press("enter"))
	waitFor(t, tm, "No response yet")
	openTab(t, tm, 3)
	waitFor(t, tm, "adds a header")
	tm.Type("X-Token")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	waitFor(t, tm, "abc 123")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	select {
	case v := <-got:
		if v != "abc 123" {
			t.Errorf("X-Token = %q", v)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("not sent")
	}
}

// A file dropped on a multipart body (the terminal pastes its quoted path)
// is sent as a file part.
func TestTUI_DropFileOnMultipartBody(t *testing.T) {
	got := make(chan map[string]string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := map[string]string{}
		if _, params, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err == nil {
			mr := multipart.NewReader(r.Body, params["boundary"])
			for {
				p, err := mr.NextPart()
				if err != nil {
					break
				}
				data, _ := io.ReadAll(p)
				parts[p.FormName()+"|"+p.FileName()] = string(data)
			}
		}
		got <- parts
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	file := filepath.Join(dir, "my photo.png")
	if err := os.WriteFile(file, []byte("PNGDATA"), 0o600); err != nil {
		t.Fatal(err)
	}
	spec := filepath.Join(dir, "api.yml")
	if err := os.WriteFile(spec, []byte(fmt.Sprintf(`openapi: 3.0.0
info: {title: API, version: "1"}
servers: [{url: %s}]
paths:
  /avatar:
    post:
      requestBody:
        content:
          multipart/form-data:
            example: {caption: hi}
      responses: {"200": {description: ok}}
`, srv.URL)), 0o644); err != nil {
		t.Fatal(err)
	}
	tm := start(t, spec)
	waitFor(t, tm, "/avatar")
	tm.Send(press("enter"))
	waitFor(t, tm, "No response yet")
	openTab(t, tm, 4)
	waitFor(t, tm, "multipart/form-data", "caption")
	tm.Send(tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl}) // a new row
	tm.Type("avatar")
	tm.Send(tea.PasteMsg{Content: "'" + file + "' "})
	waitFor(t, tm, "@ ")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	select {
	case parts := <-got:
		if parts["avatar|my photo.png"] != "PNGDATA" || parts["caption|"] != "hi" {
			t.Errorf("parts = %v", parts)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("not sent")
	}
}

// typeFast sends text one key per character, the way some terminals
// (Windows Terminal, tmux without bracketed paste) deliver a file drop.
func typeFast(tm interface{ Send(tea.Msg) }, text string) {
	for _, r := range text {
		tm.Send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// A drop that the terminal types (no bracketed paste) is recognized as a
// drop: with the focus on the method, the file becomes the body, and no
// letter of the path fires a shortcut (q quits, s sends, l switches tabs).
func TestTUI_TypedDropGoesToBody(t *testing.T) {
	burstWindow = 30 * time.Millisecond
	t.Cleanup(func() { burstWindow = 0 })
	tm, _ := newTUI(t)
	file := filepath.Join(t.TempDir(), "q sl.png")
	if err := os.WriteFile(file, []byte("PNG"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tm, "/orders")
	tm.Send(press("enter"))
	waitFor(t, tm, "No response yet")
	typeFast(tm, "'"+file+"' ")
	time.Sleep(time.Second) // the screen changes in place: check the last frame
	screen := finalScreen(t, tm)
	for _, want := range []string{"application/octet-stream", "q sl.png", "✓ 3 B"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("missing %q:\n%s", want, screen)
		}
	}
}

// Typing, however fast, still types when it is not a file.
func TestTUI_FastTypingIsReplayed(t *testing.T) {
	burstWindow = 30 * time.Millisecond
	t.Cleanup(func() { burstWindow = 0 })
	tm, _ := newTUI(t)
	waitFor(t, tm, "/orders")
	tm.Send(press("enter"))
	waitFor(t, tm, "No response yet")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab}) // the URI
	typeFast(tm, "/v2?q=1")
	time.Sleep(500 * time.Millisecond)
	if screen := finalScreen(t, tm); !strings.Contains(screen, "/orders/v2?q=1") {
		t.Fatalf("typed text lost:\n%s", screen)
	}
}
