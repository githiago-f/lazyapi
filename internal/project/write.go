package project

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AddEnvironment adds a named environment to the config file at path,
// creating the file when it does not exist. Comments and the other
// environments are kept. It fails if the name is already defined.
func AddEnvironment(path, name string, e Environment) (created bool, err error) {
	var doc yaml.Node
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		created = true
		data = []byte("# Named environments: lazyapi run <spec> --env-name <name> (ctrl+v in the TUI).\nenvironments: {}\n")
	case err != nil:
		return false, err
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return false, fmt.Errorf("%s: expected a mapping at the top level", path)
	}
	root := doc.Content[0]
	envs := mapValue(root, "environments")
	if envs == nil {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "environments"},
			&yaml.Node{Kind: yaml.MappingNode})
		envs = root.Content[len(root.Content)-1]
	}
	if envs.Kind != yaml.MappingNode {
		return false, fmt.Errorf("%s: environments must be a mapping", path)
	}
	if mapValue(envs, name) != nil {
		return false, fmt.Errorf("%s already defines environment %q; choose another --name", path, name)
	}
	envs.Style = 0 // block style, also when the file had `environments: {}`
	var value yaml.Node
	if err := value.Encode(e); err != nil {
		return false, err
	}
	envs.Content = append(envs.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: name}, &value)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return false, err
	}
	if err := enc.Close(); err != nil {
		return false, err
	}
	return created, os.WriteFile(path, buf.Bytes(), 0o644)
}

func mapValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}
