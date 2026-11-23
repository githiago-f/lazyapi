package script

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// cdn serves a tiny esm.sh-like module graph over HTTPS and counts hits.
type cdn struct {
	*httptest.Server
	mu    sync.Mutex
	files map[string]string
	hits  map[string]int
}

func newCDN(t *testing.T) *cdn {
	t.Helper()
	c := &cdn{
		files: map[string]string{
			// entry re-exports from an origin-absolute path, like esm.sh does
			"/lib@1":               `export * from "/lib@1.0.0/index.mjs";`,
			"/lib@1.0.0/index.mjs": `export const hello = (n) => "hi " + n; export { two } from "./two.mjs";`,
			"/lib@1.0.0/two.mjs":   `export const two = 2;`,
			"/types.ts":            `export const typed = (n: number): number => n * 2;`,
		},
		hits: map[string]int{},
	}
	c.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		c.hits[r.URL.Path]++
		body, ok := c.files[r.URL.Path]
		c.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(c.Close)
	return c
}

func (c *cdn) set(path, body string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files[path] = body
}

func (c *cdn) hitCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.hits {
		n += v
	}
	return n
}

func (c *cdn) loader(t *testing.T, lockDir string) *ModuleLoader {
	t.Helper()
	m := NewModuleLoader(lockDir)
	m.CacheDir = filepath.Join(lockDir, ".cache")
	m.Client = c.Client()
	return m
}

const importScript = `
import { hello, two } from "%s/lib@1";
import { typed } from "%s/types.ts";
test("remote", () => {
  expect(hello("ana")).toBe("hi ana");
  expect(two).toBe(2);
  expect(typed(2)).toBe(4);
});
`

func scriptFor(c *cdn) string {
	return strings.ReplaceAll(importScript, "%s", c.URL)
}

func readLock(t *testing.T, dir string) lockFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".lazyapi", LockFileName))
	if err != nil {
		t.Fatalf("lockfile: %v", err)
	}
	var lf lockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		t.Fatal(err)
	}
	return lf
}

func TestModules_FetchCacheAndLock(t *testing.T) {
	c := newCDN(t)
	dir := t.TempDir()

	m := c.loader(t, dir)
	mustPass(t, Run(scriptFor(c), baseData(), Options{Modules: m}))
	if err := m.SaveLock(); err != nil {
		t.Fatal(err)
	}

	lf := readLock(t, dir)
	if len(lf.Modules) != 4 {
		t.Fatalf("lock has %d modules, want 4: %v", len(lf.Modules), lf.Modules)
	}
	for u, sum := range lf.Modules {
		if !strings.HasPrefix(sum, "sha256-") {
			t.Errorf("%s integrity = %q", u, sum)
		}
	}

	// Second run is served from the cache; offline works.
	hits := c.hitCount()
	m2 := c.loader(t, dir)
	m2.Offline = true
	mustPass(t, Run(scriptFor(c), baseData(), Options{Modules: m2}))
	if c.hitCount() != hits {
		t.Errorf("offline run hit the network")
	}
}

func TestModules_IntegrityMismatchAndReload(t *testing.T) {
	c := newCDN(t)
	dir := t.TempDir()
	m := c.loader(t, dir)
	mustPass(t, Run(scriptFor(c), baseData(), Options{Modules: m}))
	if err := m.SaveLock(); err != nil {
		t.Fatal(err)
	}

	// Upstream changes and the cache is lost.
	c.set("/lib@1.0.0/two.mjs", `export const two = 3;`)
	if err := os.RemoveAll(filepath.Join(dir, ".cache")); err != nil {
		t.Fatal(err)
	}

	r := Run(scriptFor(c), baseData(), Options{Modules: c.loader(t, dir)})
	if !strings.Contains(r.ScriptError, "integrity mismatch") {
		t.Fatalf("expected integrity error, got %q", r.ScriptError)
	}

	m3 := c.loader(t, dir)
	m3.Reload = true
	r = Run(scriptFor(c), baseData(), Options{Modules: m3})
	if r.ScriptError != "" || r.Results[0].Passed {
		t.Fatalf("after reload the new module (two=3) should load and the test fail: %+v", r)
	}
	if err := m3.SaveLock(); err != nil {
		t.Fatal(err)
	}
	u := c.URL + "/lib@1.0.0/two.mjs"
	if readLock(t, dir).Modules[u] != Integrity([]byte(`export const two = 3;`)) {
		t.Error("reload did not update the lock entry")
	}
}

func TestModules_FrozenOfflineAndAllowlist(t *testing.T) {
	c := newCDN(t)
	dir := t.TempDir()

	frozen := c.loader(t, dir)
	frozen.Frozen = true
	if r := Run(scriptFor(c), baseData(), Options{Modules: frozen}); !strings.Contains(r.ScriptError, "--frozen-lockfile") {
		t.Errorf("frozen: %q", r.ScriptError)
	}

	offline := c.loader(t, dir)
	offline.Offline = true
	if r := Run(scriptFor(c), baseData(), Options{Modules: offline}); !strings.Contains(r.ScriptError, "--offline") {
		t.Errorf("offline: %q", r.ScriptError)
	}

	allow := c.loader(t, dir)
	allow.AllowedHosts = []string{"esm.sh"}
	if r := Run(scriptFor(c), baseData(), Options{Modules: allow}); !strings.Contains(r.ScriptError, "not in the allowed list") {
		t.Errorf("allowlist: %q", r.ScriptError)
	}

	u, _ := url.Parse(c.URL)
	allow2 := c.loader(t, dir)
	allow2.AllowedHosts = []string{u.Hostname()}
	mustPass(t, Run(scriptFor(c), baseData(), Options{Modules: allow2}))
}

func TestModules_RejectsHTTPAndMissing(t *testing.T) {
	c := newCDN(t)
	m := c.loader(t, t.TempDir())

	r := Run(`import "http://example.com/x.js";`, baseData(), Options{Modules: m})
	if !strings.Contains(r.ScriptError, "only https://") {
		t.Errorf("http: %q", r.ScriptError)
	}
	r = Run(`import "`+c.URL+`/nope.js";`, baseData(), Options{Modules: m})
	if !strings.Contains(r.ScriptError, "404") {
		t.Errorf("404: %q", r.ScriptError)
	}
}

func TestModules_NoLockfile(t *testing.T) {
	c := newCDN(t)
	dir := t.TempDir()
	m := c.loader(t, "")
	m.CacheDir = filepath.Join(dir, "cache")
	m.Frozen = true // ignored without a lockfile
	mustPass(t, Run(scriptFor(c), baseData(), Options{Modules: m}))
	if err := m.SaveLock(); err != nil {
		t.Fatal(err)
	}
}
