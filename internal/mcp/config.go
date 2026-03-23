package mcp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

const mcpConfigFileName = "mcp.yaml"

const mcpConfigHeader = "# Skillshare MCP configuration\n# https://skillshare.runkids.cc/docs/commands/mcp\n\n"

// MCPConfig is the user's source of truth for MCP server definitions (mcp.yaml).
type MCPConfig struct {
	Servers map[string]MCPServer `yaml:"servers"`
}

// MCPServer defines a single MCP server entry.
type MCPServer struct {
	Command  string            `yaml:"command,omitempty"`
	Args     []string          `yaml:"args,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	URL      string            `yaml:"url,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty"`
	Targets  []string          `yaml:"targets,omitempty"`
	Disabled bool              `yaml:"disabled,omitempty"`
}

// IsRemote reports whether this server uses HTTP/SSE transport (has a URL).
func (s MCPServer) IsRemote() bool {
	return s.URL != ""
}

// MCPConfigPath returns the path to mcp.yaml inside the global config directory.
func MCPConfigPath(configDir string) string {
	return filepath.Join(configDir, mcpConfigFileName)
}

// ProjectMCPConfigPath returns the path to mcp.yaml inside a project's .skillshare directory.
func ProjectMCPConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".skillshare", mcpConfigFileName)
}

// LoadMCPConfig reads mcp.yaml from path. Returns an empty config (not an error) if the
// file does not exist.
func LoadMCPConfig(path string) (*MCPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &MCPConfig{Servers: map[string]MCPServer{}}, nil
		}
		return nil, fmt.Errorf("read mcp config: %w", err)
	}

	var cfg MCPConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse mcp config: %w", err)
	}

	// Ensure the map is never nil so callers can safely range over it.
	if cfg.Servers == nil {
		cfg.Servers = map[string]MCPServer{}
	}

	return &cfg, nil
}

// Save writes the config to path as YAML, preceded by a header comment.
// It creates all parent directories as needed.
func (c *MCPConfig) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create mcp config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal mcp config: %w", err)
	}

	content := mcpConfigHeader + string(data)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write mcp config: %w", err)
	}

	return nil
}

// ServersForTarget returns enabled servers that apply to the given target name.
// A server with no Targets list applies to all targets.
func (c *MCPConfig) ServersForTarget(name string) map[string]MCPServer {
	result := make(map[string]MCPServer)
	for k, s := range c.Servers {
		if s.Disabled {
			continue
		}
		if len(s.Targets) == 0 {
			result[k] = s
			continue
		}
		if slices.Contains(s.Targets, name) {
			result[k] = s
		}
	}
	return result
}

// Validate checks that each server has exactly one of command or url set.
func (c *MCPConfig) Validate() error {
	for name, s := range c.Servers {
		hasCommand := s.Command != ""
		hasURL := s.URL != ""
		if !hasCommand && !hasURL {
			return fmt.Errorf("server %q: must have either command or url", name)
		}
		if hasCommand && hasURL {
			return fmt.Errorf("server %q: cannot have both command and url", name)
		}
	}
	return nil
}
