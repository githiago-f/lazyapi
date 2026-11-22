package importer

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type harFile struct {
	Log struct {
		Entries []harEntry `json:"entries"`
	} `json:"log"`
}

type harEntry struct {
	Request struct {
		Method   string    `json:"method"`
		URL      string    `json:"url"`
		Headers  []harPair `json:"headers"`
		PostData *struct {
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
			Params   []struct {
				Name     string `json:"name"`
				Value    string `json:"value"`
				FileName string `json:"fileName"`
			} `json:"params"`
		} `json:"postData"`
	} `json:"request"`
	Response struct {
		Status  int       `json:"status"`
		Headers []harPair `json:"headers"`
		Content struct {
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
			Encoding string `json:"encoding"`
		} `json:"content"`
	} `json:"response"`
}

type harPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// staticTypes are responses that are not API calls (page assets).
var staticTypes = []string{"image/", "font/", "text/css", "javascript", "text/html", "video/", "audio/", "application/wasm"}

// ParseHAR reads a HAR capture. Only API-looking traffic is kept (page
// assets and failed requests are dropped); requests to the same method and
// path template are merged, keeping one response example per status.
func ParseHAR(data []byte) ([]Request, error) {
	var h harFile
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("not a HAR file: %w", err)
	}
	if len(h.Log.Entries) == 0 {
		return nil, errors.New("the HAR file has no entries")
	}
	var out []Request
	index := map[string]int{}
	for _, e := range h.Log.Entries {
		u, err := url.Parse(e.Request.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || e.Response.Status == 0 {
			continue
		}
		if e.Request.Method == "GET" && isStatic(e.Response.Content.MimeType) {
			continue
		}
		if e.Request.Method == "OPTIONS" {
			continue // CORS preflights
		}
		template, _ := templatePath(u.EscapedPath())
		key := e.Request.Method + " " + u.Host + template

		body := e.Response.Content.Text
		if e.Response.Content.Encoding == "base64" {
			if decoded, err := base64.StdEncoding.DecodeString(body); err == nil {
				body = string(decoded)
			}
		}
		resp := Response{Status: e.Response.Status, ContentType: e.Response.Content.MimeType, Body: body}

		if i, ok := index[key]; ok {
			out[i].Responses = append(out[i].Responses, resp)
			continue
		}
		r := Request{Method: e.Request.Method, URL: e.Request.URL, Responses: []Response{resp}}
		for _, hd := range e.Request.Headers {
			if !strings.HasPrefix(hd.Name, ":") { // HTTP/2 pseudo-headers
				r.Headers = append(r.Headers, KV{Key: hd.Name, Value: hd.Value})
			}
		}
		if pd := e.Request.PostData; pd != nil {
			r.BodyType = strings.TrimSpace(strings.Split(pd.MimeType, ";")[0])
			switch {
			case len(pd.Params) > 0:
				for _, p := range pd.Params {
					r.Form = append(r.Form, KV{Key: p.Name, Value: p.Value, File: p.FileName != ""})
				}
			default:
				r.Body = pd.Text
			}
		}
		index[key] = len(out)
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, errors.New("no API requests found in the HAR file (only page assets or failed requests)")
	}
	return out, nil
}

func isStatic(mime string) bool {
	mime = strings.ToLower(mime)
	for _, t := range staticTypes {
		if strings.Contains(mime, t) {
			return true
		}
	}
	return false
}
