package config

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"

	"skillshare/internal/mcp"
)

// MCPConfig keeps the portable MCP declaration intact when existing commands
// load and save config. MCP-specific validation is handled by the MCP service.
type MCPConfig struct {
	Targets []string   `yaml:"targets,omitempty" json:"targets,omitempty"`
	Servers *yaml.Node `yaml:"servers,omitempty" json:"-"`
	// DirectTools is the default for Pi servers; see mcp.Source.
	DirectTools *yaml.Node `yaml:"directTools,omitempty" json:"-"`
	// Projects is global config only; the MCP service refuses it in a project config.
	Projects *yaml.Node `yaml:"projects,omitempty" json:"-"`
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
		if key := node.Content[i].Value; key != "targets" && key != "servers" && key != "projects" && key != "directTools" {
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
		case "projects":
			c.Projects = node.Content[i+1]
		case "directTools":
			c.DirectTools = node.Content[i+1]
		}
	}
	return nil
}

// ValidateMCP checks the MCP section. Only the config editor and MCP commands
// call it; other commands ignore MCP settings. accounts are the targets that are
// another config directory of an Agent, which MCP targets may name.
func ValidateMCP(cfg *MCPConfig, external string, accounts ...string) error {
	if cfg == nil {
		return nil
	}
	if cfg.err != nil {
		return cfg.err
	}
	known := func(targets []string) error {
		for _, target := range targets {
			if !slices.Contains(mcp.Targets, target) && !slices.Contains(accounts, target) {
				return fmt.Errorf("unsupported MCP target %q", target)
			}
		}
		return nil
	}
	projects, err := mcp.ParseProjects(cfg.Projects)
	if err != nil {
		return err
	}
	for _, project := range projects {
		if err := known(project.Targets); err != nil {
			return err
		}
		for _, server := range project.Servers {
			if err := known(server.Targets); err != nil {
				return err
			}
		}
	}
	if cfg.DirectTools != nil {
		var value any
		if err := cfg.DirectTools.Decode(&value); err != nil || !mcp.ValidDirectTools(value) {
			return fmt.Errorf("mcp.directTools must be true, false, \"search\" or a list of tool names")
		}
	}
	if external != "" && cfg.Servers != nil {
		return fmt.Errorf("choose sources.mcp or mcp.servers, not both")
	}
	seen := map[string]bool{}
	for _, target := range cfg.Targets {
		if known([]string{target}) != nil || seen[target] {
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
	servers, err := mcp.ParseServers(cfg.Servers)
	if err != nil {
		return fmt.Errorf("invalid MCP server fields")
	}
	for name, server := range servers {
		if err := server.Validate(name); err != nil {
			return err
		}
		if err := known(server.Targets); err != nil {
			return err
		}
	}
	return nil
}
