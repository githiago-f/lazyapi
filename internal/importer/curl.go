package importer

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ParseCurl reads a curl command line (as copied from a browser, docs or a
// terminal) into a Request.
func ParseCurl(command string) (Request, error) {
	args, err := shellSplit(command)
	if err != nil {
		return Request{}, err
	}
	if len(args) == 0 || (args[0] != "curl" && !strings.HasSuffix(args[0], "/curl")) {
		return Request{}, errors.New(`not a curl command (it must start with "curl")`)
	}
	args = args[1:]

	r := Request{}
	var data []string
	var getMode bool
	value := func(i *int, flag string) (string, error) {
		*i++
		if *i >= len(args) {
			return "", fmt.Errorf("%s needs a value", flag)
		}
		return args[*i], nil
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		// --flag=value form
		if strings.HasPrefix(a, "--") && strings.Contains(a, "=") {
			name, v, _ := strings.Cut(a, "=")
			args = append(args[:i], append([]string{name, v}, args[i+1:]...)...)
			a = name
		}
		var v string
		switch a {
		case "-X", "--request":
			if v, err = value(&i, a); err != nil {
				return r, err
			}
			r.Method = strings.ToUpper(v)
		case "-H", "--header":
			if v, err = value(&i, a); err != nil {
				return r, err
			}
			if name, val, ok := strings.Cut(v, ":"); ok {
				r.Headers = append(r.Headers, KV{Key: strings.TrimSpace(name), Value: strings.TrimSpace(val)})
			}
		case "-d", "--data", "--data-raw", "--data-binary", "--data-ascii":
			if v, err = value(&i, a); err != nil {
				return r, err
			}
			data = append(data, v)
		case "--data-urlencode":
			if v, err = value(&i, a); err != nil {
				return r, err
			}
			if name, val, ok := strings.Cut(v, "="); ok {
				data = append(data, name+"="+url.QueryEscape(val))
			} else {
				data = append(data, url.QueryEscape(v))
			}
			r.BodyType = "application/x-www-form-urlencoded"
		case "--json":
			if v, err = value(&i, a); err != nil {
				return r, err
			}
			data = append(data, v)
			r.BodyType = "application/json"
		case "-F", "--form", "--form-string":
			if v, err = value(&i, a); err != nil {
				return r, err
			}
			name, val, _ := strings.Cut(v, "=")
			file := strings.HasPrefix(val, "@") && a != "--form-string"
			if file {
				val = strings.TrimPrefix(val, "@")
			}
			r.Form = append(r.Form, KV{Key: name, Value: val, File: file})
			r.BodyType = "multipart/form-data"
		case "-u", "--user":
			if v, err = value(&i, a); err != nil {
				return r, err
			}
			r.Headers = append(r.Headers, KV{Key: "Authorization", Value: "Basic " + base64.StdEncoding.EncodeToString([]byte(v))})
		case "-b", "--cookie":
			if v, err = value(&i, a); err != nil {
				return r, err
			}
			r.Headers = append(r.Headers, KV{Key: "Cookie", Value: v})
		case "-A", "--user-agent":
			if v, err = value(&i, a); err != nil {
				return r, err
			}
			r.Headers = append(r.Headers, KV{Key: "User-Agent", Value: v})
		case "-e", "--referer":
			if _, err = value(&i, a); err != nil {
				return r, err
			}
		case "--url":
			if v, err = value(&i, a); err != nil {
				return r, err
			}
			r.URL = v
		case "-G", "--get":
			getMode = true
		case "-o", "--output", "-w", "--write-out", "-m", "--max-time", "--connect-timeout", "--retry",
			"-x", "--proxy", "--cacert", "--cert", "--key", "-c", "--cookie-jar":
			if _, err = value(&i, a); err != nil { // options with a value we do not import
				return r, err
			}
		default:
			if strings.HasPrefix(a, "-") {
				continue // flags without values: -s, -k, -L, -i, -v, --compressed, ...
			}
			if r.URL == "" {
				r.URL = a
			}
		}
	}

	if r.URL == "" {
		return r, errors.New("no URL in the curl command")
	}
	if !strings.Contains(r.URL, "://") {
		r.URL = "http://" + r.URL
	}
	joined := strings.Join(data, "&")
	switch {
	case getMode && joined != "":
		sep := "?"
		if strings.Contains(r.URL, "?") {
			sep = "&"
		}
		r.URL += sep + joined
	case len(r.Form) > 0:
	case joined != "":
		r.Body = joined
		if r.BodyType == "" && len(data) > 0 && !looksJSON(joined) && headerValue(r.Headers, "Content-Type") == "" {
			r.BodyType = "application/x-www-form-urlencoded"
		}
		if strings.HasPrefix(r.BodyType, "application/x-www-form-urlencoded") || strings.HasPrefix(headerValue(r.Headers, "Content-Type"), "application/x-www-form-urlencoded") {
			if vals, err := url.ParseQuery(joined); err == nil {
				for k, vs := range vals {
					for _, v := range vs {
						r.Form = append(r.Form, KV{Key: k, Value: v})
					}
				}
				r.Body = ""
			}
		}
	}
	if r.Method == "" {
		r.Method = "GET"
		if (joined != "" && !getMode) || len(r.Form) > 0 {
			r.Method = "POST"
		}
	}
	return r, nil // no Name: Apply derives the operationId from method and path
}

func looksJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

func headerValue(h []KV, name string) string {
	for _, kv := range h {
		if strings.EqualFold(kv.Key, name) {
			return kv.Value
		}
	}
	return ""
}

// shellSplit splits a command line like a POSIX shell: quotes, backslash
// escapes, $'...' strings and backslash-newline continuations.
func shellSplit(s string) ([]string, error) {
	var (
		args   []string
		cur    strings.Builder
		inWord bool
		runes  = []rune(s)
		flush  = func() { args = append(args, cur.String()); cur.Reset(); inWord = false }
	)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case c == '\\' && i+1 < len(runes) && (runes[i+1] == '\n' || runes[i+1] == '\r'):
			i++
			if runes[i] == '\r' && i+1 < len(runes) && runes[i+1] == '\n' {
				i++
			}
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if inWord {
				flush()
			}
		case c == '\'':
			inWord = true
			j := i + 1
			for j < len(runes) && runes[j] != '\'' {
				j++
			}
			if j >= len(runes) {
				return nil, errors.New("unterminated single quote")
			}
			cur.WriteString(string(runes[i+1 : j]))
			i = j
		case c == '$' && i+1 < len(runes) && runes[i+1] == '\'':
			inWord = true
			i += 2
			for ; i < len(runes) && runes[i] != '\''; i++ {
				if runes[i] == '\\' && i+1 < len(runes) {
					i++
					switch runes[i] {
					case 'n':
						cur.WriteRune('\n')
					case 't':
						cur.WriteRune('\t')
					case 'r':
						cur.WriteRune('\r')
					default:
						cur.WriteRune(runes[i])
					}
					continue
				}
				cur.WriteRune(runes[i])
			}
			if i >= len(runes) {
				return nil, errors.New("unterminated $'...' string")
			}
		case c == '"':
			inWord = true
			i++
			for ; i < len(runes) && runes[i] != '"'; i++ {
				if runes[i] == '\\' && i+1 < len(runes) && strings.ContainsRune("\"\\$`\n", runes[i+1]) {
					i++
				}
				cur.WriteRune(runes[i])
			}
			if i >= len(runes) {
				return nil, errors.New("unterminated double quote")
			}
		case c == '\\' && i+1 < len(runes):
			inWord = true
			i++
			cur.WriteRune(runes[i])
		default:
			inWord = true
			cur.WriteRune(c)
		}
	}
	if inWord {
		flush()
	}
	return args, nil
}
