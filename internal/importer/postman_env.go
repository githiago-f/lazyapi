package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/project"
	"github.com/githiago-f/lazyapi/internal/workspace"
)

// PostmanEnvironment is a Postman environment export
// (*.postman_environment.json).
type PostmanEnvironment struct {
	Name string
	// Values are the enabled variables; Secret lists the ones Postman marks
	// as secret (they are written to the env file like the others, never to
	// the config).
	Values   map[string]string
	Secret   []string
	Disabled []string
}

type pmEnvironment struct {
	Name   string `json:"name"`
	Scope  string `json:"_postman_variable_scope"`
	Values []struct {
		Key     string          `json:"key"`
		Value   json.RawMessage `json:"value"`
		Type    string          `json:"type"`
		Enabled *bool           `json:"enabled"`
	} `json:"values"`
}

// ParsePostmanEnvironment reads a Postman environment export.
func ParsePostmanEnvironment(data []byte) (PostmanEnvironment, error) {
	var e pmEnvironment
	if err := json.Unmarshal(data, &e); err != nil {
		return PostmanEnvironment{}, fmt.Errorf("not a Postman environment: %w", err)
	}
	if e.Values == nil {
		if e.Scope == "" {
			return PostmanEnvironment{}, errors.New(`not a Postman environment (no "values"); export it from Environments → … → Export`)
		}
	}
	out := PostmanEnvironment{Name: e.Name, Values: map[string]string{}}
	for _, v := range e.Values {
		if v.Key == "" {
			continue
		}
		if v.Enabled != nil && !*v.Enabled {
			out.Disabled = append(out.Disabled, v.Key)
			continue
		}
		var s string
		if json.Unmarshal(v.Value, &s) != nil {
			s = strings.TrimSpace(string(v.Value)) // numbers, booleans
			if s == "null" {
				s = ""
			}
		}
		out.Values[v.Key] = s
		if v.Type == "secret" {
			out.Secret = append(out.Secret, v.Key)
		}
	}
	return out, nil
}

var (
	envNameUnsafe = regexp.MustCompile(`[^a-z0-9_-]+`)
	serverKey     = regexp.MustCompile(`(?i)^(base_?url|baseuri|host|url|server|api_?url|endpoint)$`)
)

// EnvironmentName turns a Postman environment name into a config key.
func EnvironmentName(name string) string {
	n := strings.Trim(envNameUnsafe.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-"), "-")
	if n == "" {
		return "postman"
	}
	return n
}

// WriteEnvironment writes the values of a Postman environment to the
// workspace: plain variables to .lazyapi/envs/<name>.env (committed),
// secret ones to .lazyapi/secrets/<name>.env (0600, git-ignored), and adds
// the environment to .lazyapi/config.yml (created if missing). Existing
// files and environments are never overwritten.
func WriteEnvironment(specPath, name string, e PostmanEnvironment, res *Result) error {
	secret := func(k string) bool { return isSecret(e, k) || sensitiveName.MatchString(k) }
	header := fmt.Sprintf("Postman environment %q", e.Name)
	if err := writeEnvFiles(specPath, name, header, e.Values, secret, res); err != nil {
		return err
	}
	if len(e.Disabled) > 0 {
		res.note("disabled variables were skipped: %s", strings.Join(e.Disabled, ", "))
	}

	entry := project.Environment{Auth: res.Auth}
	for _, k := range sortedKeys(e.Values) {
		if v := e.Values[k]; serverKey.MatchString(k) && strings.Contains(v, "://") && !isSecret(e, k) {
			entry.Server = strings.TrimRight(v, "/")
			res.note("server for %s: %s (from %s)", name, entry.Server, k)
			break
		}
	}
	cfgPath := workspace.Config(specPath)
	created, err := project.AddEnvironment(cfgPath, name, entry)
	if err != nil {
		return err
	}
	if created {
		res.Files = append(res.Files, cfgPath)
	}
	for _, scheme := range sortedAuth(res.Auth) {
		res.note("%s credentials come from the env file (auth.%s in environment %s)", scheme, scheme, name)
	}
	res.note("use it with: lazyapi run %s --env-name %s (ctrl+v in the TUI)", specPath, name)
	return nil
}

// sensitiveName marks variables that go to the secrets file by name.
var sensitiveName = regexp.MustCompile(`(?i)(token|secret|pass|key|auth|cookie|credential|private|session)`)

// writeEnvFiles splits values between .lazyapi/envs/<name>.env (committed)
// and .lazyapi/secrets/<name>.env (git-ignored). Neither may exist yet.
func writeEnvFiles(specPath, name, header string, values map[string]string, secret func(string) bool, res *Result) error {
	plainPath, secretPath := workspace.EnvFile(specPath, name), workspace.SecretsFile(specPath, name)
	for _, p := range []string{plainPath, secretPath} {
		if _, err := os.Stat(p); err == nil {
			return fmt.Errorf("%s already exists; choose another --name or remove it", p)
		}
	}
	var plain, secrets strings.Builder
	var nPlain, nSecret int
	fmt.Fprintf(&plain, "# %s: variables (used as {{env.NAME}}). Committed: no secrets here.\n", header)
	fmt.Fprintf(&secrets, "# %s: secrets (used as {{env.NAME}}). Git-ignored.\n", header)
	for _, k := range sortedKeys(values) {
		if secret(k) {
			fmt.Fprintf(&secrets, "%s=%s\n", k, envValue(values[k]))
			nSecret++
		} else {
			fmt.Fprintf(&plain, "%s=%s\n", k, envValue(values[k]))
			nPlain++
		}
	}
	if _, err := workspace.Ensure(specPath); err != nil {
		return err
	}
	if nPlain > 0 {
		if err := os.MkdirAll(filepath.Dir(plainPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(plainPath, []byte(plain.String()), 0o644); err != nil {
			return err
		}
		res.Files = append(res.Files, plainPath)
		res.note("%d variables written to %s (commit it)", nPlain, plainPath)
	}
	if nSecret > 0 {
		p, err := workspace.WriteSecrets(specPath, name, []byte(secrets.String()))
		if err != nil {
			return err
		}
		res.Files = append(res.Files, p)
		res.note("%d secrets written to %s (git-ignored)", nSecret, p)
	}
	return nil
}

func isSecret(e PostmanEnvironment, key string) bool {
	for _, s := range e.Secret {
		if s == key {
			return true
		}
	}
	return false
}

// envValue keeps a value on one line in a .env file.
func envValue(v string) string {
	if strings.ContainsAny(v, "\n\r") || strings.TrimSpace(v) != v || strings.HasPrefix(v, `"`) || strings.HasPrefix(v, "'") {
		data, _ := json.Marshal(v)
		return string(data)
	}
	return v
}

func sortedAuth(m map[string]model.AuthSecrets) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
