package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The published agent skills (skills/, and the contributors' .agents/skills)
// must stay valid Agent Skills, and the tools they tell agents to call must
// exist.
func TestAgentSkills(t *testing.T) {
	root := filepath.Join("..", "..")
	public, _ := filepath.Glob(filepath.Join(root, "skills", "*", "SKILL.md"))
	internal, _ := filepath.Glob(filepath.Join(root, ".agents", "skills", "*", "SKILL.md"))
	if len(public) == 0 {
		t.Fatal("no skills found")
	}
	frontmatter := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n`)
	validName := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	link := regexp.MustCompile(`\]\(([^)#]+)\)`)
	toolCall := regexp.MustCompile("`([a-z]+_[a-z_]+) \\{")

	tools := map[string]bool{}
	for _, tl := range toolList {
		tools[tl.Name] = true
	}
	for _, p := range append(public, internal...) {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		m := frontmatter.FindStringSubmatch(s)
		var fm struct {
			Name        string         `yaml:"name"`
			Description string         `yaml:"description"`
			Metadata    map[string]any `yaml:"metadata"`
		}
		if m == nil || yaml.Unmarshal([]byte(m[1]), &fm) != nil {
			t.Errorf("%s: no valid YAML frontmatter", p)
			continue
		}
		if dir := filepath.Base(filepath.Dir(p)); fm.Name != dir || !validName.MatchString(fm.Name) {
			t.Errorf("%s: name %q must be the folder name, lowercase with hyphens", p, fm.Name)
		}
		if fm.Description == "" || len(fm.Description) > 1024 {
			t.Errorf("%s: description must have 1–1024 characters (%d)", p, len(fm.Description))
		}
		isInternal := strings.Contains(p, ".agents")
		if hidden, _ := fm.Metadata["internal"].(bool); hidden != isInternal {
			t.Errorf("%s: metadata.internal = %v (contributor skills are internal, published ones are not)", p, hidden)
		}
		for _, l := range link.FindAllStringSubmatch(s, -1) {
			if strings.HasPrefix(l[1], "http") {
				continue
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(p), l[1])); err != nil {
				t.Errorf("%s: broken link %s", p, l[1])
			}
		}
		refs, _ := filepath.Glob(filepath.Join(filepath.Dir(p), "references", "*.md"))
		for _, f := range append([]string{p}, refs...) {
			text, _ := os.ReadFile(f)
			for _, c := range toolCall.FindAllStringSubmatch(string(text), -1) {
				if !tools[c[1]] {
					t.Errorf("%s: calls unknown MCP tool %s", f, c[1])
				}
			}
		}
	}
}
