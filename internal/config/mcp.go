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
	// A section that does not parse is kept as written, so a mistake in MCP
	// settings never stops skills commands from loading or saving the config.
	raw *yaml.Node
	err error
}

func (c *MCPConfig) UnmarshalYAML(node *yaml.Node) error {
	*c = MCPConfig{raw: node}
	c.err = c.decode(node)
	return nil
}

func (c *MCPConfig) MarshalYAML() (any, error) {
	if c.err != nil {
		return c.raw, nil
	}
	type plain MCPConfig
	return (*plain)(c), nil
}

func (c *MCPConfig) decode(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("mcp must be a mapping")
	}
	for i := 0; i < len(node.Content); i += 2 {
		if key := node.Content[i].Value; key != "targets" && key != "servers" {
			return fmt.Errorf("unknown MCP configuration field")
		}
	}
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

// ValidateMCP checks the MCP section. Only the config editor and MCP commands
// call it; other commands ignore MCP settings.
func ValidateMCP(cfg *MCPConfig, external string) error {
	if cfg == nil {
		return nil
	}
	if cfg.err != nil {
		return cfg.err
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
