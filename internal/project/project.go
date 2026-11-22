// Package project reads lazyapi.config.yml: named environments (server,
// env file, initial variables and credentials per security scheme).
//
//	default: local
//	environments:
//	  local:
//	    server: http://127.0.0.1:4010
//	    env: .env.local
//	    vars: {tenant: acme}
//	    auth:
//	      bearerAuth: {token: "{{env.API_TOKEN}}"}
//	  staging:
//	    server: https://staging.example.com
//	    env: .env.staging
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
	"gopkg.in/yaml.v3"
)

// FileName is the project configuration file.
const FileName = "lazyapi.config.yml"

// Environment is one named target.
type Environment struct {
	Name string `yaml:"-"`
	// Server is a URL or an index into the spec's servers.
	Server string `yaml:"server,omitempty"`
	// Env is an env file, relative to the config file.
	Env  string                       `yaml:"env,omitempty"`
	Vars map[string]string            `yaml:"vars,omitempty"`
	Auth map[string]model.AuthSecrets `yaml:"auth,omitempty"`
}

// Config is a parsed lazyapi.config.yml.
type Config struct {
	Path         string                  `yaml:"-"`
	Default      string                  `yaml:"default,omitempty"`
	Environments map[string]*Environment `yaml:"environments"`
}

// Find looks for lazyapi.config.yml next to the spec, then in the working
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
		p := filepath.Join(abs, FileName)
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
		if e.Env != "" && !filepath.IsAbs(e.Env) {
			e.Env = filepath.Join(filepath.Dir(path), e.Env)
		}
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
	Name    string // environment name ("" when none)
	Server  string
	EnvFile string
	Vars    map[string]string
	Auth    map[string]model.AuthSecrets
}

// Resolve picks the environment for specPath (name "" means the default or
// $LAZYAPI_ENV). Explicit server and envFile win over the environment's.
func Resolve(specPath, name, server, envFile string) (Resolved, error) {
	r := Resolved{Server: server, EnvFile: envFile}
	cfg, err := Find(specPath)
	if err != nil {
		return r, err
	}
	e, err := cfg.Select(name)
	if err != nil || e == nil {
		return r, err
	}
	r.Name, r.Vars, r.Auth = e.Name, e.Vars, e.Auth
	if r.Server == "" {
		r.Server = e.Server
	}
	if r.EnvFile == "" {
		r.EnvFile = e.Env
	}
	return r, nil
}
