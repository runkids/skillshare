package mcp

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Source is a single resolved declaration plus the files used to obtain it.
type Source struct {
	ConfigPath  string            `json:"configPath"`
	Path        string            `json:"path"`
	Targets     []string          `json:"targets"`
	Servers     map[string]Server `json:"servers"`
	configDoc   yaml.Node
	doc         yaml.Node
	configBytes []byte
	bytes       []byte
}

func mapping(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		return node.Content[0]
	}
	return node
}

func field(node *yaml.Node, key string) *yaml.Node {
	node = mapping(node)
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func put(node *yaml.Node, key string, value any) error {
	node = mapping(node)
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected YAML mapping")
	}
	var n yaml.Node
	if err := n.Encode(value); err != nil {
		return err
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = &n
			return nil
		}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &n)
	return nil
}

func expandHome(path string) (string, error) {
	if path == "~" || len(path) > 1 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func parseYAML(data []byte, node *yaml.Node) error {
	d := yaml.NewDecoder(bytes.NewReader(data))
	if err := d.Decode(node); err != nil {
		return fmt.Errorf("invalid YAML; check syntax and duplicate keys")
	}
	if d.Decode(new(yaml.Node)) != io.EOF {
		return fmt.Errorf("MCP configuration must contain one YAML document")
	}
	if mapping(node).Kind != yaml.MappingNode {
		return fmt.Errorf("expected a YAML mapping")
	}
	var check map[string]any
	if err := node.Decode(&check); err != nil {
		return fmt.Errorf("invalid YAML mapping; check duplicate keys")
	}
	return nil
}

// LoadSource resolves exactly one inline or external MCP source. Missing external
// files are errors, never empty manifests eligible for pruning.
func LoadSource(configPath string) (*Source, error) {
	path, err := filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}
	s := &Source{ConfigPath: path, Path: path, Servers: map[string]Server{}}
	s.configBytes, err = os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read MCP config: %w", err)
	}
	if err = parseYAML(s.configBytes, &s.configDoc); err != nil {
		return nil, err
	}
	mcp := field(&s.configDoc, "mcp")
	if mcp != nil {
		if mcp.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("mcp must be a mapping")
		}
		for i := 0; i < len(mcp.Content); i += 2 {
			if k := mcp.Content[i].Value; k != "targets" && k != "servers" {
				return nil, fmt.Errorf("unknown mcp field %q", k)
			}
		}
		if n := field(mcp, "targets"); n != nil {
			if err := n.Decode(&s.Targets); err != nil {
				return nil, fmt.Errorf("mcp.targets must be a list of clients")
			}
		}
	}
	if err := validateTargets(s.Targets); err != nil {
		return nil, err
	}
	var servers *yaml.Node
	if mcp != nil {
		servers = field(mcp, "servers")
	}
	if sources := field(&s.configDoc, "sources"); sources != nil {
		if sources.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("sources must be a mapping")
		}
		if external := field(sources, "mcp"); external != nil {
			if external.Tag != "!!str" || external.Value == "" {
				return nil, fmt.Errorf("sources.mcp must name a YAML file")
			}
			if servers != nil {
				return nil, fmt.Errorf("choose sources.mcp or mcp.servers, not both")
			}
			s.Path, err = expandHome(external.Value)
			if err != nil {
				return nil, err
			}
			if !filepath.IsAbs(s.Path) {
				s.Path = filepath.Join(filepath.Dir(path), s.Path)
			}
			if s.Path == path {
				return nil, fmt.Errorf("sources.mcp cannot refer to config.yaml itself")
			}
			s.bytes, err = os.ReadFile(s.Path)
			if err != nil {
				return nil, fmt.Errorf("read external MCP source: %w", err)
			}
			if err = parseYAML(s.bytes, &s.doc); err != nil {
				return nil, err
			}
			root := mapping(&s.doc)
			for i := 0; i < len(root.Content); i += 2 {
				if root.Content[i].Value != "servers" {
					return nil, fmt.Errorf("external MCP source only supports servers")
				}
			}
			servers = field(&s.doc, "servers")
			if servers == nil {
				return nil, fmt.Errorf("external MCP source requires servers (use servers: {} for an empty list)")
			}
		}
	}
	if servers != nil {
		if servers.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("MCP servers must be a mapping")
		}
		data, err := yaml.Marshal(servers)
		if err != nil {
			return nil, err
		}
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&s.Servers); err != nil {
			return nil, fmt.Errorf("invalid MCP server fields: use command/args/env or url/headers/bearerToken and optional targets/transport")
		}
	}
	for name, server := range s.Servers {
		if err := server.Validate(name); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

// CheckUnchanged rejects edits made after a source was read or previewed.
func (s *Source) CheckUnchanged() error {
	data, err := os.ReadFile(s.ConfigPath)
	if err != nil || !bytes.Equal(data, s.configBytes) {
		return fmt.Errorf("configuration changed; preview again")
	}
	if s.Path != s.ConfigPath {
		data, err = os.ReadFile(s.Path)
		if err != nil || !bytes.Equal(data, s.bytes) {
			return fmt.Errorf("external MCP source changed; preview again")
		}
	}
	return nil
}
