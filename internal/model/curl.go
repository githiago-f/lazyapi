package model

import (
	"context"
	"io"
	"sort"
	"strings"

	"github.com/githiago-f/lazyapi/internal/env"
)

// Curl renders the request exactly as it would be sent (templates resolved,
// auth applied) as a curl command. Multipart file parts use -F name=@path.
// The command contains credentials when the request carries them.
func (r *Request) Curl(ctx context.Context) (string, error) {
	// File parts are referenced (-F name=@path), not read.
	noFiles := *r
	noFiles.Body.Files = nil
	req, err := noFiles.HTTPRequest(ctx)
	if err != nil {
		return "", err
	}
	parts := []string{"curl"}
	if req.Method != "GET" {
		parts = append(parts, "-X", req.Method)
	}
	parts = append(parts, shellQuote(req.URL.String()))

	names := make([]string, 0, len(req.Header))
	for k := range req.Header {
		names = append(names, k)
	}
	sort.Strings(names)
	multipartBody := strings.HasPrefix(req.Header.Get("Content-Type"), string(MultipartForm))
	for _, k := range names {
		if multipartBody && strings.EqualFold(k, "Content-Type") {
			continue // curl -F sets it with its own boundary
		}
		for _, v := range req.Header[k] {
			parts = append(parts, "-H", shellQuote(k+": "+v))
		}
	}

	switch {
	case multipartBody:
		tctx := r.templateContext()
		if fields, ok := jsonFields(env.Resolve(r.Body.Raw, tctx)); ok {
			for _, f := range fields {
				parts = append(parts, "-F", shellQuote(f.name+"="+f.value))
			}
		}
		fileNames := make([]string, 0, len(r.Body.Files))
		for n := range r.Body.Files {
			fileNames = append(fileNames, n)
		}
		sort.Strings(fileNames)
		for _, n := range fileNames {
			parts = append(parts, "-F", shellQuote(n+"=@"+r.Body.Files[n]))
		}
	case req.Body != nil:
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return "", err
		}
		if len(data) > 0 {
			parts = append(parts, "--data-raw", shellQuote(string(data)))
		}
	}
	return strings.Join(parts, " "), nil
}

// shellQuote wraps s in single quotes for POSIX shells.
func shellQuote(s string) string {
	if s != "" && strings.IndexFunc(s, unsafeShellRune) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// unsafeShellRune reports whether r needs quoting in a POSIX shell word.
func unsafeShellRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	default:
		return !strings.ContainsRune("-_./:=@,+%", r)
	}
}
