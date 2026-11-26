package components

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Terminals have no drag-and-drop event: dropping files on the window
// pastes their paths, each terminal its own way — quoted ('/a b.png' in
// GNOME Terminal, "C:\a b.png" in Windows Terminal), with escaped spaces
// (/a\ b.png in iTerm2, kitty), as file:// URIs, one per line or space
// separated. DroppedFiles recognizes such a paste.

// DroppedFiles returns the absolute paths in a paste when it is nothing but
// paths of existing regular files (a drop), else nil.
func DroppedFiles(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 64<<10 {
		return nil
	}
	// One drop per line: each line on its own.
	if strings.ContainsAny(text, "\r\n") {
		var paths []string
		for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' }) {
			if strings.TrimSpace(line) == "" {
				continue
			}
			found := DroppedFiles(line)
			if found == nil {
				return nil
			}
			paths = append(paths, found...)
		}
		return paths
	}
	// One path with unquoted spaces ("/tmp/my file.png").
	if p, ok := droppedFile(unquote(text)); ok {
		return []string{p}
	}
	words := splitDrop(text)
	if len(words) == 0 {
		return nil
	}
	paths := make([]string, 0, len(words))
	for _, w := range words {
		p, ok := droppedFile(w)
		if !ok {
			return nil
		}
		paths = append(paths, p)
	}
	return paths
}

// CleanDrop is what a text field inserts for a paste: the paths, unquoted
// and separated by spaces, when the paste is a drop; the text otherwise.
func CleanDrop(text string) string {
	if paths := DroppedFiles(text); paths != nil {
		return strings.Join(paths, " ")
	}
	return text
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

var windowsPath = regexp.MustCompile(`^([A-Za-z]:[\\/]|\\\\)`)

// splitDrop splits like a shell: quotes group, a backslash escapes the next
// character — except in Windows paths (C:\…, \\server\…), where it is a
// separator.
func splitDrop(s string) []string {
	var (
		words   []string
		cur     strings.Builder
		inWord  bool
		quote   rune
		literal bool // a Windows path: backslashes are separators
	)
	end := func() {
		if inWord {
			words = append(words, cur.String())
		}
		cur.Reset()
		inWord, literal = false, false
	}
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if !inWord && quote == 0 && !isSpace(r) {
			inWord = true
			rest := string(runes[i:])
			literal = windowsPath.MatchString(strings.TrimLeft(rest, `'"`))
		}
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			if r == '\\' && quote == '"' && i+1 < len(runes) && runes[i+1] == '"' {
				i++
				r = '"'
			}
			cur.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
		case r == '\\' && !literal && i+1 < len(runes):
			i++
			cur.WriteRune(runes[i])
		case isSpace(r):
			end()
		default:
			cur.WriteRune(r)
		}
	}
	end()
	return words
}

func isSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

// droppedFile maps one dropped word to an existing regular file.
func droppedFile(w string) (string, bool) {
	if w == "" {
		return "", false
	}
	if strings.HasPrefix(w, "file://") {
		u, err := url.Parse(w)
		if err != nil {
			return "", false
		}
		w = u.Path
	}
	if windowsPath.MatchString(w) {
		if !InWSL() {
			return "", false
		}
		w = fromWindows(w)
	}
	if rest, ok := strings.CutPrefix(w, "~/"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		w = filepath.Join(home, rest)
	}
	info, err := os.Stat(w)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	abs, err := filepath.Abs(w)
	if err != nil {
		return "", false
	}
	return abs, true
}

// fromWindows turns a Windows path into its WSL path: wslpath knows custom
// mount roots; without it, C:\x is /mnt/c/x and \\wsl$\<distro>\x is /x.
func fromWindows(p string) string {
	if wslpath, err := exec.LookPath("wslpath"); err == nil {
		if out, err := exec.Command(wslpath, "-u", p).Output(); err == nil { //nolint:gosec // the user's own drop
			return strings.TrimSpace(string(out))
		}
	}
	return windowsToWSL(p)
}

func windowsToWSL(p string) string {
	slashed := strings.ReplaceAll(p, `\`, "/")
	if strings.HasPrefix(slashed, "//") {
		// //wsl$/<distro>/home/x or //wsl.localhost/<distro>/home/x
		parts := strings.SplitN(strings.TrimPrefix(slashed, "//"), "/", 3)
		if len(parts) == 3 && (strings.EqualFold(parts[0], "wsl$") || strings.EqualFold(parts[0], "wsl.localhost")) {
			return "/" + parts[2]
		}
		return slashed
	}
	return "/mnt/" + strings.ToLower(slashed[:1]) + slashed[2:]
}
