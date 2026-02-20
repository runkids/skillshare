package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

// TargetConfig holds configuration for a single target
type TargetConfig struct {
	Path    string   `yaml:"path"`
	Mode    string   `yaml:"mode,omitempty"` // merge, symlink, or copy
	Include []string `yaml:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

// AuditConfig holds security audit policy settings.
type AuditConfig struct {
	BlockThreshold string `yaml:"block_threshold,omitempty"` // CRITICAL/HIGH/MEDIUM/LOW/INFO
}

// HubEntry represents a single saved hub source.
type HubEntry struct {
	Label   string `yaml:"label"`
	URL     string `yaml:"url"`
	BuiltIn bool   `yaml:"builtin,omitempty"`
}

// HubConfig holds hub persistence settings.
type HubConfig struct {
	Default string     `yaml:"default,omitempty"`
	Hubs    []HubEntry `yaml:"hubs,omitempty"`
}

// Config holds the application configuration
type Config struct {
	Source  string                  `yaml:"source"`
	Mode    string                  `yaml:"mode,omitempty"` // default mode: symlink
	Targets map[string]TargetConfig `yaml:"targets"`
	Skills  []SkillEntry            `yaml:"skills,omitempty"`
	Ignore  []string                `yaml:"ignore,omitempty"`
	Audit   AuditConfig             `yaml:"audit,omitempty"`
	Hub     HubConfig               `yaml:"hub,omitempty"`
}

const defaultAuditBlockThreshold = "CRITICAL"

// GlobalSchemaURL is the JSON Schema URL for the global config file.
const GlobalSchemaURL = "https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json"

// schemaComment is the YAML Language Server directive prepended to saved config files.
var schemaComment = []byte("# yaml-language-server: $schema=" + GlobalSchemaURL + "\n")

// BaseDir returns the skillshare data root directory.
// Priority:
//  1. $XDG_CONFIG_HOME/skillshare  (any platform, if set)
//  2. %AppData%/skillshare         (Windows only, via os.UserConfigDir())
//  3. ~/.config/skillshare         (Linux, macOS, Windows fallback)
func BaseDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "skillshare")
	}
	if runtime.GOOS == "windows" {
		if dir, err := os.UserConfigDir(); err == nil {
			return filepath.Join(dir, "skillshare")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "skillshare")
}

// DataDir returns the data directory (XDG_DATA_HOME).
// Priority:
//  1. $XDG_DATA_HOME/skillshare  (any platform, if set)
//  2. %AppData%/skillshare       (Windows only, via os.UserConfigDir())
//  3. ~/.local/share/skillshare  (Linux, macOS)
func DataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "skillshare")
	}
	if runtime.GOOS == "windows" {
		if dir, err := os.UserConfigDir(); err == nil {
			return filepath.Join(dir, "skillshare")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "skillshare")
}

// StateDir returns the state directory (XDG_STATE_HOME).
// Priority:
//  1. $XDG_STATE_HOME/skillshare  (any platform, if set)
//  2. %AppData%/skillshare        (Windows only, via os.UserConfigDir())
//  3. ~/.local/state/skillshare   (Linux, macOS)
func StateDir() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "skillshare")
	}
	if runtime.GOOS == "windows" {
		if dir, err := os.UserConfigDir(); err == nil {
			return filepath.Join(dir, "skillshare")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "skillshare")
}

// CacheDir returns the cache directory (XDG_CACHE_HOME).
// Priority:
//  1. $XDG_CACHE_HOME/skillshare  (any platform, if set)
//  2. %AppData%/skillshare        (Windows only, via os.UserConfigDir())
//  3. ~/.cache/skillshare         (Linux, macOS)
func CacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "skillshare")
	}
	if runtime.GOOS == "windows" {
		if dir, err := os.UserConfigDir(); err == nil {
			return filepath.Join(dir, "skillshare")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "skillshare")
}

// ConfigPath returns the config file path, respecting SKILLSHARE_CONFIG env var
func ConfigPath() string {
	// Allow override for testing
	if envPath := os.Getenv("SKILLSHARE_CONFIG"); envPath != "" {
		return envPath
	}
	return filepath.Join(BaseDir(), "config.yaml")
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

	for _, skill := range cfg.Skills {
		if strings.TrimSpace(skill.Name) == "" {
			return nil, fmt.Errorf("config has skill with empty name")
		}
		if strings.TrimSpace(skill.Source) == "" {
			return nil, fmt.Errorf("config has skill '%s' with empty source", skill.Name)
		}
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

	data = append(schemaComment, data...)

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
