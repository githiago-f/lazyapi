package oauth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/githiago-f/lazyapi/internal/workspace"
)

// TokenFile is the token store, inside the workspace's .local/ directory.
const TokenFile = "tokens.json"

// Token is an issued token set.
type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	IDToken      string    `json:"id_token,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	Expiry       time.Time `json:"expiry,omitzero"`
	// Name is the scheme the token was issued for (shown by logout/status).
	Name   string `json:"name,omitempty"`
	Issuer string `json:"issuer,omitempty"`
}

// expirySkew refreshes tokens a little before they expire.
const expirySkew = 30 * time.Second

// Valid reports whether the access token can still be used.
func (t *Token) Valid() bool {
	if t == nil || t.AccessToken == "" {
		return false
	}
	return t.Expiry.IsZero() || time.Now().Add(expirySkew).Before(t.Expiry)
}

// Store keeps tokens in memory and, when it has a path, in a 0600 JSON file.
type Store struct {
	path string // "" keeps tokens in memory only
	spec string

	mu     sync.Mutex
	tokens map[string]*Token
	loaded bool
}

var (
	storesMu sync.Mutex
	stores   = map[string]*Store{}
	memory   = &Store{tokens: map[string]*Token{}, loaded: true}
)

// StoreFor is the token store of a spec's workspace (.lazyapi/.local/
// tokens.json next to it). Without a spec path tokens are kept in memory.
func StoreFor(specPath string) *Store {
	if specPath == "" {
		return memory
	}
	path := filepath.Join(workspace.Local(specPath), TokenFile)
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	storesMu.Lock()
	defer storesMu.Unlock()
	s, ok := stores[path]
	if !ok {
		s = &Store{path: path, spec: specPath, tokens: map[string]*Token{}}
		stores[path] = s
	}
	return s
}

// ResetStores forgets every in-memory token (tests); files are untouched.
func ResetStores() {
	storesMu.Lock()
	stores = map[string]*Store{}
	storesMu.Unlock()
	memory.mu.Lock()
	memory.tokens = map[string]*Token{}
	memory.mu.Unlock()
}

// Path is the file backing the store ("" for memory only).
func (s *Store) Path() string { return s.path }

func (s *Store) load() {
	if s.loaded || s.path == "" {
		s.loaded = true
		return
	}
	s.loaded = true
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var tokens map[string]*Token
	if json.Unmarshal(data, &tokens) == nil && tokens != nil {
		s.tokens = tokens
	}
}

func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
	// .lazyapi/.local is git-ignored; make sure the .gitignore exists first.
	if _, err := workspace.Ensure(s.spec); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.tokens, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Get returns the token stored under key (nil when none).
func (s *Store) Get(key string) *Token {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	if t, ok := s.tokens[key]; ok {
		c := *t
		return &c
	}
	return nil
}

// Put stores a token (and writes the file).
func (s *Store) Put(key string, t Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	s.tokens[key] = &t
	return s.save()
}

// Delete removes the tokens of a scheme name ("" removes every token).
// It returns how many were removed.
func (s *Store) Delete(name string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	n := 0
	for k, t := range s.tokens {
		if name == "" || t.Name == name {
			delete(s.tokens, k)
			n++
		}
	}
	if n == 0 {
		return 0, nil
	}
	if len(s.tokens) == 0 && s.path != "" {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return n, err
		}
		return n, nil
	}
	return n, s.save()
}

// List returns the stored tokens sorted by scheme name.
func (s *Store) List() []Token {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	out := make([]Token, 0, len(s.tokens))
	for _, t := range s.tokens {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// cacheKey identifies a token: the same client, user and scopes at the same
// token endpoint (or issuer) share it. The interactive grants share one
// session per client, whatever the scopes: logging in once (with the code
// or the device grant) serves every operation of the scheme.
func cacheKey(c Config) string {
	where := c.Issuer
	if where == "" {
		where = c.TokenURL
	}
	grant, scopes := c.Grant, c.Scopes
	if Interactive(grant) {
		grant, scopes = "user", ""
	}
	h := sha256.Sum256([]byte(grant + "\x00" + where + "\x00" + c.ClientID + "\x00" + c.Username + "\x00" + scopes))
	return hex.EncodeToString(h[:12])
}
