package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

// TargetConfig holds configuration for a single target
type TargetConfig struct {
	Path string `yaml:"path"`
	Mode string `yaml:"mode,omitempty"` // symlink (default), copy
}

// AuditConfig holds security audit policy settings.
type AuditConfig struct {
	BlockThreshold string `yaml:"block_threshold,omitempty"` // CRITICAL/HIGH/MEDIUM/LOW/INFO
}

// Config holds the application configuration
type Config struct {
	Source  string                  `yaml:"source"`
	Mode    string                  `yaml:"mode,omitempty"` // default mode: symlink
	Targets map[string]TargetConfig `yaml:"targets"`
	Ignore  []string                `yaml:"ignore,omitempty"`
	Audit   AuditConfig             `yaml:"audit,omitempty"`
}

const defaultAuditBlockThreshold = "CRITICAL"

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

	threshold, err := normalizeAuditBlockThreshold(cfg.Audit.BlockThreshold)
	if err != nil {
		return nil, fmt.Errorf("invalid audit.block_threshold: %w", err)
	}
	cfg.Audit.BlockThreshold = threshold

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

func normalizeAuditBlockThreshold(v string) (string, error) {
	threshold := strings.ToUpper(strings.TrimSpace(v))
	if threshold == "" {
		return defaultAuditBlockThreshold, nil
	}
	switch threshold {
	case "CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO":
		return threshold, nil
	default:
		return "", fmt.Errorf("unsupported value %q", v)
	}
}
