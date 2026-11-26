package components

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// Clipboard is how fields and editors copy and paste (replaced in tests).
var Clipboard interface {
	WriteAll(string) error
	ReadAll() (string, error)
} = SystemClipboard{}

// SystemClipboard uses the OS clipboard: xclip/xsel/wl-clipboard (atotto),
// then, under WSL, the Windows clipboard through powershell.exe.
type SystemClipboard struct{}

func (SystemClipboard) WriteAll(s string) error {
	err := clipboard.WriteAll(s)
	if err != nil && InWSL() {
		err = powershell(`[Console]::InputEncoding=[Text.Encoding]::UTF8; Set-Clipboard -Value ([Console]::In.ReadToEnd())`, s, nil)
	}
	return err
}

func (SystemClipboard) ReadAll() (string, error) {
	text, err := clipboard.ReadAll()
	if err != nil && InWSL() {
		var out bytes.Buffer
		if err = powershell(`[Console]::OutputEncoding=[Text.Encoding]::UTF8; Get-Clipboard -Raw`, "", &out); err == nil {
			// Get-Clipboard ends with a line break the user did not copy.
			text = strings.TrimSuffix(strings.ReplaceAll(out.String(), "\r\n", "\n"), "\n")
		}
	}
	return text, err
}

var (
	wslOnce sync.Once
	inWSL   bool
)

// InWSL reports whether lazyapi runs under the Windows Subsystem for Linux.
func InWSL() bool {
	wslOnce.Do(func() {
		if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
			inWSL = true
			return
		}
		data, err := os.ReadFile("/proc/sys/kernel/osrelease")
		inWSL = err == nil && strings.Contains(strings.ToLower(string(data)), "microsoft")
	})
	return inWSL
}

func powershell(script, stdin string, stdout *bytes.Buffer) error {
	path, err := exec.LookPath("powershell.exe")
	if err != nil {
		return errors.New("no clipboard: install wl-clipboard or xclip, or make powershell.exe reachable")
	}
	c := exec.Command(path, "-NoProfile", "-NonInteractive", "-Command", script) //nolint:gosec // fixed script
	c.Stdin = strings.NewReader(stdin)
	if stdout != nil {
		c.Stdout = stdout
	}
	return c.Run()
}

// CopyCmd puts text on the clipboard in the background and tells the user.
// When the OS clipboard is unreachable (SSH, a bare WSL) it asks the
// terminal to copy it (OSC 52), which most modern terminals honor.
func CopyCmd(text, done string) tea.Cmd {
	return func() tea.Msg {
		if err := Clipboard.WriteAll(text); err != nil {
			return tea.BatchMsg{
				tea.SetClipboard(text),
				notifyCmd(done+" (through the terminal)", Success),
			}
		}
		return ShowNotificationMsg{Message: done, Type: Success}
	}
}

// PasteCmd reads the clipboard in the background and delivers it as a
// tea.PasteMsg, the message a terminal paste (or a file drop) produces, so
// the focused field handles both the same way.
func PasteCmd() tea.Cmd {
	return func() tea.Msg {
		text, err := Clipboard.ReadAll()
		if err != nil {
			return ShowNotificationMsg{Message: "Clipboard unavailable: " + err.Error() + " · the terminal's paste (ctrl+shift+v) works too", Type: Error}
		}
		return tea.PasteMsg{Content: text}
	}
}
