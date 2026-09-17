package config

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"skillshare/internal/mcp"
)

// MCPConfig keeps the portable MCP declaration intact when existing commands
// load and save config. MCP-specific validation is handled by the MCP service.
type MCPConfig struct {
	Targets []string   `yaml:"targets,omitempty" json:"targets,omitempty"`
	Servers *yaml.Node `yaml:"servers,omitempty" json:"-"`
}

func (c *MCPConfig) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("mcp must be a mapping")
	}
	for i := 0; i < len(node.Content); i += 2 {
		if key := node.Content[i].Value; key != "targets" && key != "servers" {
			return fmt.Errorf("unknown MCP configuration field")
		}
	}
	*c = MCPConfig{}
	for i := 0; i < len(node.Content); i += 2 {
		switch node.Content[i].Value {
		case "targets":
			if err := node.Content[i+1].Decode(&c.Targets); err != nil {
				return fmt.Errorf("mcp.targets must be a list")
			}
		case "servers":
			c.Servers = node.Content[i+1]
		}
	}
	return nil
}

func validateMCP(cfg *MCPConfig, external string) error {
	if cfg == nil {
		return nil
	}
	if external != "" && cfg.Servers != nil {
		return fmt.Errorf("choose sources.mcp or mcp.servers, not both")
	}
	seen := map[string]bool{}
	for _, target := range cfg.Targets {
		valid := false
		for _, supported := range mcp.Targets {
			if target == supported {
				valid = true
			}
		}
		if !valid || seen[target] {
			return fmt.Errorf("invalid or duplicate MCP target %q", target)
		}
		seen[target] = true
	}
	if cfg.Servers == nil {
		return nil
	}
	if cfg.Servers.Kind != yaml.MappingNode {
		return fmt.Errorf("mcp.servers must be a mapping")
	}
	data, err := yaml.Marshal(cfg.Servers)
	if err != nil {
		return err
	}
	var servers map[string]mcp.Server
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := d.Decode(&servers); err != nil {
		return fmt.Errorf("invalid MCP server fields")
	}
	for name, server := range servers {
		if err := server.Validate(name); err != nil {
			return err
		}
	}
	return nil
}
