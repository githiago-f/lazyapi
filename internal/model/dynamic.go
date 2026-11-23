package model

import "github.com/githiago-f/lazyapi/internal/env"

// DynamicRefs lists the {{$name}} references of the request (URL, params,
// query, headers, body).
func (r Request) DynamicRefs() []string {
	in := []string{r.ServerURL, r.URI, r.Body.Raw}
	for _, m := range []map[string]string{r.Params, r.Query, r.Headers} {
		for _, v := range m {
			in = append(in, v)
		}
	}
	return env.DynamicRefs(in...)
}

// WithDynamic returns a copy of the request with every {{$name}} replaced
// by fn(name), so the values that are sent, reported and seen by scripts
// are the same. Each occurrence gets its own value.
func (r Request) WithDynamic(fn env.DynamicFunc) (Request, error) {
	var err error
	resolve := func(s string) string {
		if err != nil {
			return s
		}
		var out string
		out, err = env.ResolveDynamic(s, fn)
		return out
	}
	resolveMap := func(m map[string]string) map[string]string {
		if m == nil {
			return nil
		}
		out := make(map[string]string, len(m))
		for k, v := range m {
			out[k] = resolve(v)
		}
		return out
	}
	r.ServerURL = resolve(r.ServerURL)
	r.URI = resolve(r.URI)
	r.Params = resolveMap(r.Params)
	r.Query = resolveMap(r.Query)
	r.Headers = resolveMap(r.Headers)
	r.Body.Raw = resolve(r.Body.Raw)
	return r, err
}
