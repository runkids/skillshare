package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

// TargetConfig holds configuration for a single target
type TargetConfig struct {
	Path string `yaml:"path"`
	Mode string `yaml:"mode,omitempty"` // symlink (default), copy
}

// Config holds the application configuration
type Config struct {
	Source  string                  `yaml:"source"`
	Mode    string                  `yaml:"mode,omitempty"` // default mode: symlink
	Targets map[string]TargetConfig `yaml:"targets"`
	Ignore  []string                `yaml:"ignore,omitempty"`
}

// DefaultTargets returns the well-known CLI skills directories
func DefaultTargets() map[string]TargetConfig {
	home, _ := os.UserHomeDir()
	return map[string]TargetConfig{
		"agents":      {Path: filepath.Join(home, ".config", "agents", "skills")}, // Global, portable across AI coding agents
		"amp":         {Path: filepath.Join(home, ".amp", "skills")},
		"antigravity": {Path: filepath.Join(home, ".gemini", "antigravity", "skills")},
		"claude":      {Path: filepath.Join(home, ".claude", "skills")},
		"codex":       {Path: filepath.Join(home, ".codex", "skills")},
		"copilot":     {Path: filepath.Join(home, ".copilot", "skills")},
		"crush":       {Path: filepath.Join(home, ".config", "crush", "skills")},
		"cursor":      {Path: filepath.Join(home, ".cursor", "skills")},
		"gemini":      {Path: filepath.Join(home, ".gemini", "skills")},
		"goose":       {Path: filepath.Join(home, ".config", "goose", "skills")},
		"letta":       {Path: filepath.Join(home, ".letta", "skills")},
		"opencode":    {Path: filepath.Join(home, ".config", "opencode", "skills")},
	}
}

// ConfigPath returns the config file path, respecting SKILLSHARE_CONFIG env var
func ConfigPath() string {
	// Allow override for testing
	if envPath := os.Getenv("SKILLSHARE_CONFIG"); envPath != "" {
		return envPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "skillshare", "config.yaml")
}

// Load reads the config from the default location
func Load() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found: run 'skillshare init' first")
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Expand ~ in paths
	cfg.Source = expandPath(cfg.Source)
	for name, target := range cfg.Targets {
		target.Path = expandPath(target.Path)
		cfg.Targets[name] = target
	}

	return &cfg, nil
}

// Save writes the config to the default location
func (c *Config) Save() error {
	path := ConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if utils.HasTildePrefix(path) {
		home, err := os.UserHomeDir()
		if err != nil {
			// Cannot expand ~, return original path
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
