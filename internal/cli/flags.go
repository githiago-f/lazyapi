package cli

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
)

// newFlagSet returns a FlagSet that reports errors instead of exiting and
// keeps its own usage output quiet (callers print usage on error).
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// parseInterleaved parses flags that may appear before, between or after
// positional arguments (the stdlib parser stops at the first positional).
// A literal "--" ends flag parsing.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var tail []string
	if i := slices.Index(args, "--"); i >= 0 {
		args, tail = args[:i], args[i+1:]
	}
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return append(positional, tail...), nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

// kvFlag collects repeatable name=value flags (e.g. --param id=1).
type kvFlag map[string]string

func (k kvFlag) String() string {
	parts := make([]string, 0, len(k))
	for name, v := range k {
		parts = append(parts, name+"="+v)
	}
	return strings.Join(parts, ",")
}

func (k kvFlag) Set(v string) error {
	name, value, ok := strings.Cut(v, "=")
	if !ok || name == "" {
		return fmt.Errorf("expected name=value, got %q", v)
	}
	k[name] = value
	return nil
}

// splitList parses a comma-separated flag value, dropping empty entries.
func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// listFlag collects a repeatable string flag (e.g. --flow a.js --flow b.js).
type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }
