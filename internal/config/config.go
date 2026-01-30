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
		"agents":       {Path: filepath.Join(home, ".config", "agents", "skills")}, // Global, portable across AI coding agents
		"amp":          {Path: filepath.Join(home, ".config", "agents", "skills")}, // Same as agents
		"antigravity":  {Path: filepath.Join(home, ".gemini", "antigravity", "global_skills")},
		"claude":       {Path: filepath.Join(home, ".claude", "skills")},
		"cline":        {Path: filepath.Join(home, ".cline", "skills")},
		"codebuddy":    {Path: filepath.Join(home, ".codebuddy", "skills")},
		"codex":        {Path: filepath.Join(home, ".codex", "skills")},
		"commandcode":  {Path: filepath.Join(home, ".commandcode", "skills")},
		"continue":     {Path: filepath.Join(home, ".continue", "skills")},
		"copilot":      {Path: filepath.Join(home, ".copilot", "skills")},
		"crush":        {Path: filepath.Join(home, ".config", "crush", "skills")},
		"cursor":       {Path: filepath.Join(home, ".cursor", "skills")},
		"droid":        {Path: filepath.Join(home, ".factory", "skills")},
		"gemini":       {Path: filepath.Join(home, ".gemini", "skills")},
		"goose":        {Path: filepath.Join(home, ".config", "goose", "skills")},
		"junie":        {Path: filepath.Join(home, ".junie", "skills")},
		"kilocode":     {Path: filepath.Join(home, ".kilocode", "skills")},
		"kimi":         {Path: filepath.Join(home, ".kimi", "skills")},
		"kiro":         {Path: filepath.Join(home, ".kiro", "skills")},
		"kode":         {Path: filepath.Join(home, ".kode", "skills")},
		"letta":        {Path: filepath.Join(home, ".letta", "skills")},
		"mcpjam":       {Path: filepath.Join(home, ".mcpjam", "skills")},
		"moltbot":      {Path: filepath.Join(home, ".moltbot", "skills")},  // legacy name for openclaw
		"openclaw":     {Path: filepath.Join(home, ".openclaw", "skills")},
		"mux":          {Path: filepath.Join(home, ".mux", "skills")},
		"neovate":      {Path: filepath.Join(home, ".neovate", "skills")},
		"opencode":     {Path: filepath.Join(home, ".config", "opencode", "skills")},
		"openhands":    {Path: filepath.Join(home, ".openhands", "skills")},
		"pi":           {Path: filepath.Join(home, ".pi", "agent", "skills")},
		"pochi":        {Path: filepath.Join(home, ".pochi", "skills")},
		"qoder":        {Path: filepath.Join(home, ".qoder", "skills")},
		"qwen":         {Path: filepath.Join(home, ".qwen", "skills")},
		"roo":          {Path: filepath.Join(home, ".roo", "skills")},
		"trae":         {Path: filepath.Join(home, ".trae", "skills")},
		"windsurf":     {Path: filepath.Join(home, ".codeium", "windsurf", "skills")},
		"zencoder":     {Path: filepath.Join(home, ".zencoder", "skills")},
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
