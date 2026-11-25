package config

import (
	_ "embed"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// The TUI's texts live in messages.yml, embedded at build time. T looks one
// up by its dotted key ("editor.auth.empty") and fills {name} placeholders.

//go:embed messages.yml
var embeddedMessages []byte

// MessagesEnv names a YAML file whose keys override the embedded texts.
const MessagesEnv = "LAZYAPI_MESSAGES"

var (
	messages = mustMessages()
	// MessagesErr is why the LAZYAPI_MESSAGES file was not used (nil: fine
	// or not set). The TUI reports it.
	MessagesErr = loadOverride(os.Getenv(MessagesEnv))
)

func mustMessages() map[string]string {
	m, err := parseMessages(embeddedMessages)
	if err != nil {
		panic("internal/config/messages.yml: " + err.Error()) // caught by the tests
	}
	return m
}

// parseMessages flattens a nested YAML mapping into dotted keys.
func parseMessages(data []byte) (map[string]string, error) {
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	out := map[string]string{}
	var walk func(prefix string, v any) error
	walk = func(prefix string, v any) error {
		switch v := v.(type) {
		case map[string]any:
			for k, child := range v {
				key := k
				if prefix != "" {
					key = prefix + "." + k
				}
				if err := walk(key, child); err != nil {
					return err
				}
			}
		case string:
			out[prefix] = v
		case nil:
			out[prefix] = ""
		default:
			out[prefix] = fmt.Sprint(v)
		}
		return nil
	}
	return out, walk("", root)
}

// loadOverride merges a messages file over the embedded texts. Unknown
// keys are an error (usually a typo), so nothing is changed then.
func loadOverride(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: %w", MessagesEnv, err)
	}
	over, err := parseMessages(data)
	if err != nil {
		return fmt.Errorf("%s %s: %w", MessagesEnv, path, err)
	}
	var unknown []string
	for k := range over {
		if _, ok := messages[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("%s %s: unknown keys %s", MessagesEnv, path, strings.Join(unknown, ", "))
	}
	for k, v := range over {
		messages[k] = v
	}
	return nil
}

// T returns the text of key with {name} placeholders replaced by the given
// name, value pairs. A missing key returns the key itself, so a gap shows
// on screen instead of an empty string.
func T(key string, pairs ...string) string {
	text, ok := messages[key]
	if !ok {
		return key
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		text = strings.ReplaceAll(text, "{"+pairs[i]+"}", pairs[i+1])
	}
	return text
}

// Plural picks key.one for n == 1 and key.other otherwise, with {n} filled
// in (plus the given pairs).
func Plural(key string, n int, pairs ...string) string {
	form := ".other"
	if n == 1 {
		form = ".one"
	}
	return T(key+form, append([]string{"n", strconv.Itoa(n)}, pairs...)...)
}

// Keys lists every message key (tests, docs).
func Keys() []string {
	keys := make([]string, 0, len(messages))
	for k := range messages {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
