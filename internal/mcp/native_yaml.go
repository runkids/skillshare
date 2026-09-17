package mcp

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Aliases and merges can make changing one entry affect another. Fail closed
// instead of flattening those relationships when editing an owned extension.
func plainYAML(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode || node.Tag == "!!merge" {
		return fmt.Errorf("Goose MCP configuration must not use YAML aliases or merges")
	}
	for _, child := range node.Content {
		if err := plainYAML(child); err != nil {
			return err
		}
	}
	return nil
}

func (n *Native) editYAML(changes map[string]map[string]any) ([]byte, error) {
	// Parse a fresh tree so repeated previews never mutate the original.
	var doc yaml.Node
	if err := parseYAML(n.data, &doc); err != nil {
		return nil, err
	}
	if field(&doc, "extensions") == nil {
		if err := put(&doc, "extensions", map[string]any{}); err != nil {
			return nil, err
		}
	}
	extensions := field(&doc, "extensions")
	for _, name := range sortedKeys(changes) {
		entry := changes[name]
		if entry != nil {
			if err := put(extensions, name, entry); err != nil {
				return nil, err
			}
			continue
		}
		for i := 0; i+1 < len(extensions.Content); i += 2 {
			if extensions.Content[i].Value == name {
				extensions.Content = append(extensions.Content[:i], extensions.Content[i+2:]...)
				break
			}
		}
	}
	// Expand newly populated empty flow mappings, while retaining comments and
	// all unrelated config nodes (including built-in extensions).
	if len(mapping(&doc).Content) > 0 {
		mapping(&doc).Style = 0
	}
	if len(extensions.Content) > 0 {
		extensions.Style = 0
	}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	if _, err := ParseNative(n.Target, out.Bytes()); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
