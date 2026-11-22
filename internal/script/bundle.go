package script

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

const urlNamespace = "lazyapi-url"

// Bundle compiles source (ESM/TypeScript, with imports) into a single script
// goja can execute.
func Bundle(source string, opts Options) (string, error) {
	resolveDir := opts.ResolveDir
	if resolveDir == "" {
		resolveDir = "."
	}
	resolveDir, err := filepath.Abs(resolveDir)
	if err != nil {
		return "", err
	}
	root := opts.Root
	if root == "" {
		root = resolveDir
	}
	if root, err = filepath.Abs(root); err != nil {
		return "", err
	}

	name := opts.Filename
	if name == "" {
		name = "script.js"
	}

	result := api.Build(api.BuildOptions{
		Stdin: &api.StdinOptions{
			Contents:   source,
			ResolveDir: resolveDir,
			Sourcefile: filepath.Base(name),
			Loader:     loaderFor(name),
		},
		Bundle: true,
		Write:  false,
		// CommonJS assigns module.exports before the module body runs, so the
		// before() export is reachable even when the top level throws (it
		// usually reads `response`, which does not exist yet in that phase).
		Format:    api.FormatCommonJS,
		Banner:    map[string]string{"js": "(function (module, exports) {"},
		Footer:    map[string]string{"js": "})(__lazyapi_module, __lazyapi_module.exports);"},
		Platform:  api.PlatformNeutral,
		Target:    api.ES2017,
		Sourcemap: api.SourceMapInline,
		LogLevel:  api.LogLevelSilent,
		// Packages fetched from CDNs are ESM; prefer their "module" builds.
		MainFields: []string{"module", "main"},
		Plugins:    []api.Plugin{importPlugin(root, opts.Modules)},
	})
	if len(result.Errors) > 0 {
		return "", buildError(result.Errors)
	}
	if len(result.OutputFiles) == 0 {
		return "", errors.New("bundler produced no output")
	}
	return string(result.OutputFiles[0].Contents), nil
}

func loaderFor(name string) api.Loader {
	switch strings.ToLower(path.Ext(name)) {
	case ".ts", ".mts", ".cts":
		return api.LoaderTS
	case ".tsx":
		return api.LoaderTSX
	case ".jsx":
		return api.LoaderJSX
	case ".json":
		return api.LoaderJSON
	default:
		return api.LoaderJS
	}
}

func buildError(msgs []api.Message) error {
	lines := make([]string, 0, len(msgs))
	for _, m := range msgs {
		text := m.Text
		// Errors raised by our plugin already carry the full explanation.
		if m.PluginName != "" && len(m.Notes) == 0 && m.Location == nil {
			lines = append(lines, text)
			continue
		}
		if m.Location != nil {
			text = fmt.Sprintf("%s:%d:%d: %s", m.Location.File, m.Location.Line, m.Location.Column, text)
		}
		lines = append(lines, text)
	}
	return errors.New(strings.Join(lines, "\n"))
}

func isURL(p string) bool {
	return strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "http://")
}

func isRelative(p string) bool {
	return strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/")
}

// importPlugin routes https:// imports to the module loader, keeps local
// imports inside root, and rejects bare package names (there is no
// node_modules resolution in scripts).
func importPlugin(root string, modules *ModuleLoader) api.Plugin {
	return api.Plugin{
		Name: "lazyapi-imports",
		Setup: func(build api.PluginBuild) {
			build.OnResolve(api.OnResolveOptions{Filter: ".*"}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
				if args.Namespace == urlNamespace {
					return resolveFromURL(args.Importer, args.Path)
				}
				switch {
				case isURL(args.Path):
					if modules == nil {
						return api.OnResolveResult{}, fmt.Errorf("URL imports are disabled in this context: %s", args.Path)
					}
					return api.OnResolveResult{Path: args.Path, Namespace: urlNamespace}, nil
				case isRelative(args.Path):
					full := args.Path
					if !filepath.IsAbs(full) {
						full = filepath.Join(args.ResolveDir, filepath.FromSlash(full))
					}
					if !within(root, full) {
						return api.OnResolveResult{}, fmt.Errorf("import %q is outside the allowed directory %s", args.Path, root)
					}
					return api.OnResolveResult{}, nil // let esbuild resolve extensions/index files
				default:
					return api.OnResolveResult{}, fmt.Errorf(
						"bare import %q is not supported; import it from a URL instead, e.g. \"https://esm.sh/%s\"",
						args.Path, args.Path)
				}
			})

			build.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: urlNamespace}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
				data, err := modules.Fetch(args.Path)
				if err != nil {
					return api.OnLoadResult{}, err
				}
				contents := string(data)
				u, _ := url.Parse(args.Path)
				loader := api.LoaderJS
				if u != nil {
					loader = loaderFor(u.Path)
				}
				return api.OnLoadResult{Contents: &contents, Loader: loader}, nil
			})
		},
	}
}

// resolveFromURL resolves an import found inside a URL module.
func resolveFromURL(importer, spec string) (api.OnResolveResult, error) {
	if isURL(spec) {
		return api.OnResolveResult{Path: spec, Namespace: urlNamespace}, nil
	}
	if !isRelative(spec) {
		return api.OnResolveResult{}, fmt.Errorf("bare import %q inside %s is not supported", spec, importer)
	}
	base, err := url.Parse(importer)
	if err != nil {
		return api.OnResolveResult{}, fmt.Errorf("invalid importer URL %q: %w", importer, err)
	}
	ref, err := url.Parse(spec)
	if err != nil {
		return api.OnResolveResult{}, fmt.Errorf("invalid import %q: %w", spec, err)
	}
	return api.OnResolveResult{Path: base.ResolveReference(ref).String(), Namespace: urlNamespace}, nil
}

// within reports whether path is root or inside it.
func within(root, p string) bool {
	rel, err := filepath.Rel(root, filepath.Clean(p))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
