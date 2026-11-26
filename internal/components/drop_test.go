package components

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDroppedFiles(t *testing.T) {
	dir := t.TempDir()
	spaced := filepath.Join(dir, "my photo.png")
	plain := filepath.Join(dir, "data.csv")
	quote := filepath.Join(dir, "it's.txt")
	for _, p := range []string{spaced, plain, quote} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cases := map[string]struct {
		paste string
		want  []string
	}{
		"bare path":                {plain, []string{plain}},
		"unquoted spaces":          {spaced, []string{spaced}},
		"single quotes (GNOME)":    {"'" + spaced + "' ", []string{spaced}},
		"double quotes":            {`"` + spaced + `"`, []string{spaced}},
		"escaped spaces (kitty)":   {filepath.Join(dir, `my\ photo.png`), []string{spaced}},
		"escaped quote":            {filepath.Join(dir, `it\'s.txt`), []string{quote}},
		"file URI":                 {"file://" + filepath.ToSlash(filepath.Join(dir, "my%20photo.png")), []string{spaced}},
		"several, space separated": {"'" + spaced + "' '" + plain + "' ", []string{spaced, plain}},
		"several, one per line":    {spaced + "\n" + plain + "\n", []string{spaced, plain}},
		"text is not a drop":       {"hello world", nil},
		"a missing file":           {plain + " " + filepath.Join(dir, "nope"), nil},
		"a directory":              {dir, nil},
		"empty":                    {"  ", nil},
	}
	for name, c := range cases {
		if got := DroppedFiles(c.paste); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: DroppedFiles(%q) = %q, want %q", name, c.paste, got, c.want)
		}
	}
	if got := CleanDrop("'" + spaced + "' "); got != spaced {
		t.Errorf("CleanDrop unquotes a drop: %q", got)
	}
	if got := CleanDrop("'quoted text'"); got != "'quoted text'" {
		t.Errorf("CleanDrop keeps text: %q", got)
	}
}

func TestWindowsToWSL(t *testing.T) {
	cases := map[string]string{
		`C:\Users\ana\My Pictures\a.png`:       "/mnt/c/Users/ana/My Pictures/a.png",
		`d:/data/x.csv`:                        "/mnt/d/data/x.csv",
		`\\wsl.localhost\Ubuntu\home\ana\x.js`: "/home/ana/x.js",
		`\\wsl$\Debian\tmp\y`:                  "/tmp/y",
	}
	for in, want := range cases {
		if got := windowsToWSL(in); got != want {
			t.Errorf("windowsToWSL(%q) = %q, want %q", in, got, want)
		}
	}
	// Backslashes in Windows paths are separators, not escapes.
	if got := splitDrop(`"C:\a b\c.png" C:\d\e.png`); !reflect.DeepEqual(got, []string{`C:\a b\c.png`, `C:\d\e.png`}) {
		t.Errorf("splitDrop(windows) = %q", got)
	}
}

func TestFieldClipboardAndDrop(t *testing.T) {
	clip := &memClipboard{text: "pasted"}
	Clipboard = clip
	defer func() { Clipboard = SystemClipboard{} }()

	f := InitField("", "abc")
	if !f.CanCopy() {
		t.Fatal("a field with a value can copy")
	}
	m, cmd := f.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	f = m.(Field)
	if cmd == nil {
		t.Fatal("ctrl+c copies")
	}
	cmd()
	if clip.text != "abc" || f.Value() != "abc" {
		t.Errorf("ctrl+c: clipboard %q, value %q", clip.text, f.Value())
	}

	clip.text = "xyz"
	_, cmd = f.Update(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	paste, ok := cmd().(tea.PasteMsg)
	if !ok || paste.Content != "xyz" {
		t.Fatalf("ctrl+v reads the clipboard as a paste: %#v", paste)
	}
	m, _ = f.Update(paste)
	f = m.(Field)
	if f.Value() != "abcxyz" {
		t.Errorf("paste inserts at the cursor: %q", f.Value())
	}

	// Line breaks become spaces; a dropped file is its bare path.
	f.SetValue("")
	m, _ = f.Update(tea.PasteMsg{Content: "a\nb\r\nc\n"})
	if f = m.(Field); f.Value() != "a b c" {
		t.Errorf("multi-line paste: %q", f.Value())
	}
	file := filepath.Join(t.TempDir(), "a b.txt")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f.SetValue("")
	m, _ = f.Update(tea.PasteMsg{Content: "'" + file + "' "})
	if f = m.(Field); f.Value() != file {
		t.Errorf("drop: %q, want %q", f.Value(), file)
	}
}
