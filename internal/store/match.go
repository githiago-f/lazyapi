package store

import (
	"net/url"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
)

// MatchPath finds the operation whose path template matches a concrete
// path ("/users/42" → "/users/{id}" with id=42). Literal segments win over
// parameters, so "/users/me" prefers "/users/me" to "/users/{id}".
func MatchPath(spec *openapi3.T, method, path string) (model.OpenAPIRef, map[string]string, bool) {
	if spec == nil || spec.Paths == nil {
		return model.OpenAPIRef{}, nil, false
	}
	method = strings.ToUpper(method)
	segs := splitPath(path)
	var (
		best       string
		bestParams map[string]string
		bestScore  = -1
	)
	for tmpl, item := range spec.Paths.Map() {
		if item == nil || item.GetOperation(method) == nil {
			continue
		}
		tsegs := splitPath(tmpl)
		if len(tsegs) != len(segs) {
			continue
		}
		params := map[string]string{}
		score := 0
		ok := true
		for i, ts := range tsegs {
			if strings.HasPrefix(ts, "{") && strings.HasSuffix(ts, "}") {
				v, err := url.PathUnescape(segs[i])
				if err != nil || v == "" {
					ok = false
					break
				}
				params[ts[1:len(ts)-1]] = v
				continue
			}
			if ts != segs[i] {
				ok = false
				break
			}
			score++
		}
		if ok && (score > bestScore || score == bestScore && tmpl < best) {
			best, bestParams, bestScore = tmpl, params, score
		}
	}
	if bestScore < 0 {
		return model.OpenAPIRef{}, nil, false
	}
	return model.OpenAPIRef{Path: best, Method: method}, bestParams, true
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
