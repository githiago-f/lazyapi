// Package project reads .lazyapi/config.yml next to the spec: named
// environments (server, initial variables, HTTP options and credentials per
// security scheme). An environment's variables live in .lazyapi/envs/<name>.env
// (committed) and its secrets in .lazyapi/secrets/<name>.env (git-ignored).
//
//	default: local
//	environments:
//	  local:
//	    server: http://127.0.0.1:4010
//	    vars: {tenant: acme}
//	    auth:
//	      bearerAuth: {token: "{{env.API_TOKEN}}"}   # API_TOKEN in secrets/local.env
//	  staging:
//	    server: https://staging.example.com
//	    auth:
//	      oauth: {clientId: "{{env.CLIENT_ID}}", clientSecret: "{{env.CLIENT_SECRET}}"}
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/workspace"
	"gopkg.in/yaml.v3"
)

// FileName is the project configuration file, inside .lazyapi/.
const FileName = workspace.Dir + "/" + workspace.ConfigName

// Environment is one named target.
type Environment struct {
	Name string `yaml:"-"`
	// Server is a URL or an index into the spec's servers.
	Server string `yaml:"server,omitempty"`
	// Env is an extra env file, relative to the config file. The
	// environment's envs/<name>.env and secrets/<name>.env are always read.
	Env  string                       `yaml:"env,omitempty"`
	Vars map[string]string            `yaml:"vars,omitempty"`
	Auth map[string]model.AuthSecrets `yaml:"auth,omitempty"`
	// HTTP holds TLS, proxy and redirect settings for this environment.
	HTTP model.HTTPOptions `yaml:"http,omitempty"`
}

// Config is a parsed .lazyapi/config.yml.
type Config struct {
	Path         string                  `yaml:"-"`
	Default      string                  `yaml:"default,omitempty"`
	Environments map[string]*Environment `yaml:"environments"`
}

// Find looks for .lazyapi/config.yml next to the spec, then in the working
// directory. It returns nil (no error) when there is none.
func Find(specPath string) (*Config, error) {
	var dirs []string
	if specPath != "" {
		dirs = append(dirs, filepath.Dir(specPath))
	}
	dirs = append(dirs, ".")
	seen := map[string]bool{}
	for _, d := range dirs {
		abs, err := filepath.Abs(d)
		if err != nil || seen[abs] {
			continue
		}
		seen[abs] = true
		p := filepath.Join(abs, workspace.Dir, workspace.ConfigName)
		if _, err := os.Stat(p); err == nil {
			return Load(p)
		}
	}
	return nil, nil
}

// Load parses a config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.Path = path
	for name, e := range c.Environments {
		if e == nil {
			e = &Environment{}
			c.Environments[name] = e
		}
		e.Name = name
		rel := func(p *string) {
			if *p != "" && !filepath.IsAbs(*p) {
				*p = filepath.Join(filepath.Dir(path), *p)
			}
		}
		rel(&e.Env)
		rel(&e.HTTP.CACert)
		rel(&e.HTTP.ClientCert)
		rel(&e.HTTP.ClientKey)
	}
	if c.Default != "" && c.Environments[c.Default] == nil {
		return nil, fmt.Errorf("%s: default environment %q is not defined", path, c.Default)
	}
	return &c, nil
}

// Names lists the environments: the default first, then alphabetically.
func (c *Config) Names() []string {
	if c == nil {
		return nil
	}
	names := make([]string, 0, len(c.Environments))
	for n := range c.Environments {
		if n != c.Default {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	if c.Default != "" {
		names = append([]string{c.Default}, names...)
	}
	return names
}

// Select returns the named environment, or the default when name is "".
// It returns nil (no error) when name is "" and there is no default.
func (c *Config) Select(name string) (*Environment, error) {
	if name == "" {
		name = os.Getenv("LAZYAPI_ENV")
	}
	if c == nil {
		if name != "" {
			return nil, fmt.Errorf("environment %q requested but there is no %s", name, FileName)
		}
		return nil, nil
	}
	if name == "" {
		name = c.Default
	}
	if name == "" {
		return nil, nil
	}
	e := c.Environments[name]
	if e == nil {
		return nil, fmt.Errorf("unknown environment %q in %s (defined: %s)", name, c.Path, strings.Join(c.Names(), ", "))
	}
	return e, nil
}

// Resolved is what a run uses after combining flags with the environment.
type Resolved struct {
	Name   string // environment name ("" when none)
	Server string
	// EnvFiles are merged in order: envs/<name>.env, secrets/<name>.env,
	// the environment's env:, then an explicit --env file.
	EnvFiles []string
	Vars     map[string]string
	Auth     map[string]model.AuthSecrets
	HTTP     model.HTTPOptions
}

// Resolve picks the environment for specPath (name "" means the default or
// $LAZYAPI_ENV). An explicit server wins over the environment's; an
// explicit envFile is read last, so its values win.
func Resolve(specPath, name, server, envFile string) (Resolved, error) {
	r := Resolved{Server: server}
	cfg, err := Find(specPath)
	if err != nil {
		return r, err
	}
	e, err := cfg.Select(name)
	if err != nil {
		return r, err
	}
	if e != nil {
		r.Name, r.Vars, r.Auth, r.HTTP = e.Name, e.Vars, e.Auth, e.HTTP
		if r.Server == "" {
			r.Server = e.Server
		}
		r.EnvFiles = e.Files(cfg)
	}
	if envFile != "" {
		r.EnvFiles = append(r.EnvFiles, envFile)
	}
	return r, nil
}

// Files are the env files of the environment that exist, in merge order:
// .lazyapi/envs/<name>.env, .lazyapi/secrets/<name>.env, then env:.
func (e *Environment) Files(c *Config) []string {
	if e == nil {
		return nil
	}
	var out []string
	if c != nil && c.Path != "" {
		// c.Path is <dir>/.lazyapi/config.yml: the spec's directory is <dir>.
		spec := filepath.Join(filepath.Dir(filepath.Dir(c.Path)), "spec")
		out = workspace.EnvFiles(spec, e.Name)
	}
	if e.Env != "" {
		out = append(out, e.Env)
	}
	return out
}
