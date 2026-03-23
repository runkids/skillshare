package mcp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

const mcpStateFileName = "mcp_state.yaml"

// MCPState tracks which servers were last written to each target config file.
type MCPState struct {
	Targets map[string]MCPTargetState `yaml:"targets"`
}

// MCPTargetState records the last-synced state for a single target.
type MCPTargetState struct {
	Servers    []string `yaml:"servers"`
	ConfigPath string   `yaml:"config_path"`
	LastSync   string   `yaml:"last_sync"`
}

// MCPStatePath returns the path to mcp_state.yaml inside configDir.
func MCPStatePath(configDir string) string {
	return filepath.Join(configDir, mcpStateFileName)
}

// LoadMCPState reads mcp_state.yaml from path. Returns an empty state (not an
// error) if the file does not exist.
func LoadMCPState(path string) (*MCPState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &MCPState{Targets: map[string]MCPTargetState{}}, nil
		}
		return nil, fmt.Errorf("read mcp state: %w", err)
	}

	var s MCPState
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse mcp state: %w", err)
	}

	if s.Targets == nil {
		s.Targets = map[string]MCPTargetState{}
	}

	return &s, nil
}

// Save writes the state to path as YAML, creating parent directories as needed.
func (s *MCPState) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create mcp state dir: %w", err)
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal mcp state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write mcp state: %w", err)
	}

	return nil
}

// PreviousServers returns the server names that were last written to the given
// target, or nil if the target has no recorded state.
func (s *MCPState) PreviousServers(targetName string) []string {
	t, ok := s.Targets[targetName]
	if !ok {
		return nil
	}
	return t.Servers
}

// UpdateTarget records that servers were written to targetName's config file at
// configPath. Servers are stored sorted; LastSync is set to the current UTC time
// in RFC3339 format.
func (s *MCPState) UpdateTarget(targetName string, servers []string, configPath string) {
	sorted := make([]string, len(servers))
	copy(sorted, servers)
	sort.Strings(sorted)

	s.Targets[targetName] = MCPTargetState{
		Servers:    sorted,
		ConfigPath: configPath,
		LastSync:   time.Now().UTC().Format(time.RFC3339),
	}
}
