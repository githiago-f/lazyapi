// Package workspace is the git-native layout of a LazyAPI project: a
// .lazyapi directory next to the OpenAPI spec.
//
//	api/
//	  openapi.yml
//	  .lazyapi/
//	    config.yml           environments: server, vars, http, auth (committed)
//	    envs/<name>.env      variables of an environment (committed)
//	    secrets/<name>.env   credentials of an environment (git-ignored)
//	    tests/<op>.js        test scripts (x-lazyapi-tests)
//	    flows/*.js           flows
//	    data/*.csv|json      data files for data-driven runs
//	    lazyapi.lock         pinned URL imports
//	    .local/              drafts, history, caches (git-ignored)
//	    .gitignore           secrets/ and .local/
package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	// Dir is the workspace directory, next to the spec.
	Dir = ".lazyapi"
	// ConfigName is the environments file inside Dir.
	ConfigName = "config.yml"
	// TestsDir, FlowsDir and DataDir are relative to the spec's directory.
	TestsDir = Dir + "/tests"
	FlowsDir = Dir + "/flows"
	DataDir  = Dir + "/data"
	// LockName is the lockfile of URL imports, inside Dir.
	LockName = "lazyapi.lock"
)

// gitignore keeps credentials and machine-local state out of git.
const gitignore = "# Written by lazyapi: credentials and local state are never committed.\nsecrets/\n.local/\n"

// Root is the .lazyapi directory of the spec (in the working directory
// when specPath is "").
func Root(specPath string) string {
	dir := "."
	if specPath != "" {
		dir = filepath.Dir(specPath)
	}
	return filepath.Join(dir, Dir)
}

// Config is the environments file of the spec.
func Config(specPath string) string { return filepath.Join(Root(specPath), ConfigName) }

// EnvFile holds the (committed) variables of an environment.
func EnvFile(specPath, name string) string {
	return filepath.Join(Root(specPath), "envs", name+".env")
}

// SecretsFile holds the (git-ignored) credentials of an environment.
func SecretsFile(specPath, name string) string {
	return filepath.Join(Root(specPath), "secrets", name+".env")
}

// Lock is the lockfile of URL imports.
func Lock(specPath string) string { return filepath.Join(Root(specPath), LockName) }

// Local is the git-ignored directory for drafts, history and caches.
func Local(specPath string) string { return filepath.Join(Root(specPath), ".local") }

// EnvFiles returns the env files of an environment that exist: the
// committed variables first, then the secrets (which win).
func EnvFiles(specPath, name string) []string {
	var out []string
	for _, p := range []string{EnvFile(specPath, name), SecretsFile(specPath, name)} {
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// Ensure creates the .lazyapi directory and its .gitignore (never
// overwriting an existing one). It returns the directory.
func Ensure(specPath string) (string, error) {
	root := Root(specPath)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
		if err := os.WriteFile(p, []byte(gitignore), 0o644); err != nil {
			return "", err
		}
	}
	return root, nil
}

// WriteSecrets writes an environment's secrets file (0600), creating the
// workspace and its .gitignore first. It never overwrites an existing file.
func WriteSecrets(specPath, name string, data []byte) (string, error) {
	if _, err := Ensure(specPath); err != nil {
		return "", err
	}
	p := SecretsFile(specPath, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return "", err
	}
	return p, f.Close()
}
