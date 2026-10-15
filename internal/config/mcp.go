package config

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

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
	// Serve configures `skillshare mcp serve`; only that command reads it.
	Serve *MCPServeConfig `yaml:"serve,omitempty" json:"-"`
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
		if key := node.Content[i].Value; key != "targets" && key != "servers" && key != "projects" && key != "directTools" && key != "serve" {
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
		case "serve":
			// A misspelt key would silently leave editing off.
			if err := onlyKeys(node.Content[i+1], "mcp.serve", "edit"); err != nil {
				return err
			}
			if edit := mappingValue(node.Content[i+1], "edit"); edit != nil {
				if err := onlyKeys(edit, "mcp.serve.edit", "skills", "preview_ttl"); err != nil {
					return err
				}
			}
			if err := node.Content[i+1].Decode(&c.Serve); err != nil {
				return fmt.Errorf("mcp.serve.edit.skills must be a list and preview_ttl a duration")
			}
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
	if _, _, err := cfg.ServeEdit(); err != nil {
		return err
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

// MCPServeConfig is the mcp.serve section.
type MCPServeConfig struct {
	Edit *MCPServeEditConfig `yaml:"edit,omitempty"`
}

// MCPServeEditConfig turns on skill editing over `mcp serve`. Skills holds
// patterns matched like a target's include filter; nothing is editable without one.
type MCPServeEditConfig struct {
	Skills     []string `yaml:"skills,omitempty"`
	PreviewTTL string   `yaml:"preview_ttl,omitempty"`
}

// ServeEdit returns the checked mcp.serve.edit patterns and preview lifetime,
// or no patterns when editing is off. An MCP section that does not parse is
// reported, since editing cannot be known to be off.
func (c *MCPConfig) ServeEdit() ([]string, time.Duration, error) {
	if c == nil {
		return nil, 0, nil
	}
	if c.err != nil {
		return nil, 0, fmt.Errorf("mcp: %w", c.err)
	}
	if c.Serve == nil || c.Serve.Edit == nil {
		return nil, 0, nil
	}
	e := c.Serve.Edit
	for _, p := range e.Skills {
		// A blank pattern would be dropped, and no patterns match every skill.
		if strings.TrimSpace(p) == "" {
			return nil, 0, fmt.Errorf("mcp.serve.edit.skills: a pattern is empty")
		}
		// The same check target include patterns get.
		if _, err := filepath.Match(strings.TrimSpace(p), ""); err != nil {
			return nil, 0, fmt.Errorf("mcp.serve.edit.skills: %q: %w", p, err)
		}
	}
	ttl := 30 * time.Minute
	if e.PreviewTTL != "" {
		d, err := time.ParseDuration(e.PreviewTTL)
		if err != nil || d <= 0 {
			return nil, 0, fmt.Errorf("mcp.serve.edit.preview_ttl must be a positive duration such as 30m, not %q", e.PreviewTTL)
		}
		ttl = d
	}
	if len(e.Skills) == 0 {
		return nil, 0, nil
	}
	return e.Skills, ttl, nil
}

func onlyKeys(node *yaml.Node, label string, keys ...string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("%s must be a mapping", label)
	}
	for i := 0; i < len(node.Content); i += 2 {
		if !slices.Contains(keys, node.Content[i].Value) {
			return fmt.Errorf("unknown field %s.%s", label, node.Content[i].Value)
		}
	}
	return nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
