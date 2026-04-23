package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

// marshalYAML marshals v with 2-space indentation (Go yaml default is 4).
func marshalYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ValidSyncModes lists all valid sync mode values.
var ValidSyncModes = []string{"merge", "symlink", "copy"}

// ValidTargetNamings lists all valid target naming values.
var ValidTargetNamings = []string{"flat", "standard"}

// IsValidSyncMode reports whether mode is a valid sync mode (or empty, meaning inherit).
func IsValidSyncMode(mode string) bool {
	if mode == "" {
		return true // empty = inherit global default
	}
	return slices.Contains(ValidSyncModes, mode)
}

// IsValidTargetNaming reports whether naming is a valid target naming mode
// (or empty, meaning inherit the global default).
func IsValidTargetNaming(naming string) bool {
	if naming == "" {
		return true // empty = inherit global default
	}
	return slices.Contains(ValidTargetNamings, naming)
}

// EffectiveTargetNaming resolves the runtime target naming mode.
// Empty values inherit the global default; the final fallback is "flat".
func EffectiveTargetNaming(naming string) string {
	if naming == "" {
		return "flat"
	}
	return naming
}

// ResourceTargetConfig holds per-resource-kind target configuration (skills, agents, etc.).
type ResourceTargetConfig struct {
	Path         string   `yaml:"path,omitempty"`
	Mode         string   `yaml:"mode,omitempty"` // merge, symlink, or copy
	TargetNaming string   `yaml:"target_naming,omitempty"`
	Include      []string `yaml:"include,omitempty"`
	Exclude      []string `yaml:"exclude,omitempty"`
}

// IsEmpty reports whether all fields are zero-valued.
func (r ResourceTargetConfig) IsEmpty() bool {
	return r.Path == "" && r.Mode == "" && r.TargetNaming == "" && len(r.Include) == 0 && len(r.Exclude) == 0
}

// TargetConfig holds configuration for a single target.
// Legacy flat fields (Path/Mode/Include/Exclude) are kept for backward-compatible
// YAML reading; migrateTargetConfigs moves them into Skills on load.
type TargetConfig struct {
	Path    string   `yaml:"path,omitempty"`
	Mode    string   `yaml:"mode,omitempty"` // merge, symlink, or copy
	Include []string `yaml:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`

	Skills *ResourceTargetConfig `yaml:"skills,omitempty"`
	Agents *ResourceTargetConfig `yaml:"agents,omitempty"`

	defaultTargetNaming string `yaml:"-"`
}

// SkillsConfig returns the effective skills configuration.
// If Skills sub-key is set, it is returned directly.
// Otherwise the legacy flat fields are returned for backward compatibility.
func (tc *TargetConfig) SkillsConfig() ResourceTargetConfig {
	if tc.Skills != nil {
		sc := *tc.Skills
		if sc.TargetNaming == "" {
			sc.TargetNaming = tc.defaultTargetNaming
		}
		return sc
	}
	return ResourceTargetConfig{
		Path:         tc.Path,
		Mode:         tc.Mode,
		TargetNaming: tc.defaultTargetNaming,
		Include:      tc.Include,
		Exclude:      tc.Exclude,
	}
}

// AgentsConfig returns the agents configuration, or an empty value if not set.
func (tc *TargetConfig) AgentsConfig() ResourceTargetConfig {
	if tc.Agents != nil {
		return *tc.Agents
	}
	return ResourceTargetConfig{}
}

// EnsureSkills returns the Skills sub-key, creating it from legacy flat fields if nil.
// Use this before writing to Skills fields.
func (tc *TargetConfig) EnsureSkills() *ResourceTargetConfig {
	if tc.Skills == nil {
		sc := ResourceTargetConfig{
			Path:    tc.Path,
			Mode:    tc.Mode,
			Include: tc.Include,
			Exclude: tc.Exclude,
		}
		tc.Skills = &sc
		tc.Path = ""
		tc.Mode = ""
		tc.Include = nil
		tc.Exclude = nil
	}
	return tc.Skills
}

// EnsureAgents returns the Agents sub-key, creating it if nil.
// Use this before writing to Agents fields.
func (tc *TargetConfig) EnsureAgents() *ResourceTargetConfig {
	if tc.Agents == nil {
		tc.Agents = &ResourceTargetConfig{}
	}
	return tc.Agents
}

// migrateTargetConfigs moves legacy flat fields into skills: sub-key.
// Returns true if any target was migrated.
func migrateTargetConfigs(targets map[string]TargetConfig) bool {
	migrated := false
	for name, tc := range targets {
		hasFlatFields := tc.Path != "" || tc.Mode != "" || len(tc.Include) > 0 || len(tc.Exclude) > 0
		if !hasFlatFields {
			continue
		}
		if tc.Skills == nil {
			tc.Skills = &ResourceTargetConfig{
				Path:    tc.Path,
				Mode:    tc.Mode,
				Include: tc.Include,
				Exclude: tc.Exclude,
			}
		} else {
			// Mixed format: merge flat fields into existing Skills (don't overwrite)
			if tc.Skills.Path == "" && tc.Path != "" {
				tc.Skills.Path = tc.Path
			}
			if tc.Skills.Mode == "" && tc.Mode != "" {
				tc.Skills.Mode = tc.Mode
			}
			if len(tc.Skills.Include) == 0 && len(tc.Include) > 0 {
				tc.Skills.Include = tc.Include
			}
			if len(tc.Skills.Exclude) == 0 && len(tc.Exclude) > 0 {
				tc.Skills.Exclude = tc.Exclude
			}
		}
		tc.Path = ""
		tc.Mode = ""
		tc.Include = nil
		tc.Exclude = nil
		targets[name] = tc
		migrated = true
	}
	return migrated
}

// AuditConfig holds security audit policy settings.
type AuditConfig struct {
	BlockThreshold   string   `yaml:"block_threshold,omitempty"`   // CRITICAL/HIGH/MEDIUM/LOW/INFO
	Profile          string   `yaml:"profile,omitempty"`           // default/strict/permissive
	DedupeMode       string   `yaml:"dedupe_mode,omitempty"`       // legacy/global
	EnabledAnalyzers []string `yaml:"enabled_analyzers,omitempty"` // allowlist; empty = all
}

// LogConfig holds log retention settings.
type LogConfig struct {
	MaxEntries *int `yaml:"max_entries,omitempty"` // nil = use default (1000), 0 = unlimited, >0 = limit
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

// ExtraTargetConfig holds configuration for one target of an extra resource.
type ExtraTargetConfig struct {
	Path    string `yaml:"path"`
	Mode    string `yaml:"mode,omitempty"`    // merge (default), symlink, or copy
	Flatten bool   `yaml:"flatten,omitempty"` // flatten subdirectories into target root
}

// ExtraConfig holds configuration for a non-skill resource type (rules, commands, etc.).
type ExtraConfig struct {
	Name    string              `yaml:"name"`
	Source  string              `yaml:"source,omitempty"`
	Targets []ExtraTargetConfig `yaml:"targets"`
}

// Config holds the application configuration
type Config struct {
	Source        string                  `yaml:"source"`
	AgentsSource  string                  `yaml:"agents_source,omitempty"`
	ExtrasSource  string                  `yaml:"extras_source,omitempty"`
	PluginsSource string                  `yaml:"plugins_source,omitempty"`
	HooksSource   string                  `yaml:"hooks_source,omitempty"`
	Mode          string                  `yaml:"mode,omitempty"` // default mode: merge
	TargetNaming  string                  `yaml:"target_naming,omitempty"`
	Targets       map[string]TargetConfig `yaml:"targets"`
	Extras        []ExtraConfig           `yaml:"extras,omitempty"`
	Ignore        []string                `yaml:"ignore,omitempty"`
	Audit         AuditConfig             `yaml:"audit,omitempty"`
	Hub           HubConfig               `yaml:"hub,omitempty"`
	Log           LogConfig               `yaml:"log,omitempty"`
	TUI           *bool                   `yaml:"tui,omitempty"` // nil = default true
	GitLabHosts   []string                `yaml:"gitlab_hosts,omitempty"`

	// RegistryDir is the resolved directory for registry.yaml (cached SourceRoot result).
	// Set during Load(), not serialized to YAML.
	RegistryDir string `yaml:"-"`
}

// EffectiveAgentsSource returns the agents source directory.
// Defaults to <BaseDir>/agents if not explicitly configured.
func (c *Config) EffectiveAgentsSource() string {
	if c.AgentsSource != "" {
		return ExpandPath(c.AgentsSource)
	}
	return filepath.Join(BaseDir(), "agents")
}

// EffectivePluginsSource returns the plugins source directory.
// Defaults to <BaseDir>/plugins if not explicitly configured.
func (c *Config) EffectivePluginsSource() string {
	if c.PluginsSource != "" {
		return ExpandPath(c.PluginsSource)
	}
	return filepath.Join(BaseDir(), "plugins")
}

// EffectiveHooksSource returns the hooks source directory.
// Defaults to <BaseDir>/hooks if not explicitly configured.
func (c *Config) EffectiveHooksSource() string {
	if c.HooksSource != "" {
		return ExpandPath(c.HooksSource)
	}
	return filepath.Join(BaseDir(), "hooks")
}

// HasAgentTarget reports whether any configured target has an agents path,
// either from the user's config agents: sub-key or from the built-in defaults.
func (c *Config) HasAgentTarget() bool {
	builtinAgents := DefaultAgentTargets()
	for name, tc := range c.Targets {
		// Check user config agents: sub-key
		if ac := tc.AgentsConfig(); ac.Path != "" {
			return true
		}
		// Check built-in defaults
		if _, ok := builtinAgents[name]; ok {
			return true
		}
	}
	return false
}

// EffectiveGitLabHosts returns GitLabHosts merged with SKILLSHARE_GITLAB_HOSTS env var.
// Use this instead of accessing GitLabHosts directly for runtime behavior;
// GitLabHosts contains only config-file values and is safe to persist via Save().
func (c *Config) EffectiveGitLabHosts() []string {
	return mergeGitLabHostsFromEnv(c.GitLabHosts)
}

// IsTUIEnabled reports whether interactive TUI is enabled.
// nil (absent from config) is treated as true for backward compatibility.
func (c *Config) IsTUIEnabled() bool {
	if c.TUI == nil {
		return true
	}
	return *c.TUI
}

const defaultAuditBlockThreshold = "CRITICAL"

// DefaultLogMaxEntries is the default maximum number of log entries to retain per file.
const DefaultLogMaxEntries = 1000

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

	// Migrate skills[] to registry.yaml (one-time, silent)
	_ = migrateSkillsToRegistry(path)

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

	// Validate and normalize gitlab_hosts (config file only; env var merged at read time)
	hosts, err := normalizeGitLabHosts(cfg.GitLabHosts)
	if err != nil {
		return nil, err
	}
	cfg.GitLabHosts = hosts

	// Migrate legacy flat target fields to skills: sub-key (one-time, persisted immediately)
	if migrateTargetConfigs(cfg.Targets) {
		if data, err := marshalYAML(&cfg); err == nil {
			tmpPath := path + ".tmp"
			if writeErr := os.WriteFile(tmpPath, append(schemaComment, data...), 0644); writeErr == nil {
				os.Rename(tmpPath, path)
			}
		}
	}

	// Expand ~ in paths
	cfg.Source = expandPath(cfg.Source)
	cfg.ExtrasSource = expandPath(cfg.ExtrasSource)
	cfg.PluginsSource = expandPath(cfg.PluginsSource)
	cfg.HooksSource = expandPath(cfg.HooksSource)
	defaults := DefaultTargets()
	for name, target := range cfg.Targets {
		target.defaultTargetNaming = cfg.TargetNaming
		if target.Skills != nil {
			target.Skills.Path = expandPath(target.Skills.Path)
		}
		if target.Agents != nil {
			target.Agents.Path = expandPath(target.Agents.Path)
		}
		// Fallback to built-in default path for known targets without explicit path
		if target.SkillsConfig().Path == "" {
			if def, ok := defaults[name]; ok {
				target.EnsureSkills().Path = expandPath(def.Path)
			}
		}
		cfg.Targets[name] = target
	}

	// Expand ~ in extras paths
	for i, extra := range cfg.Extras {
		cfg.Extras[i].Source = expandPath(extra.Source)
		for j, et := range extra.Targets {
			cfg.Extras[i].Targets[j].Path = expandPath(et.Path)
		}
	}

	// Cache SourceRoot so callers don't re-walk .git/ on every LoadRegistry call.
	if cfg.Source != "" {
		cfg.RegistryDir = SourceRoot(cfg.Source)
		MigrateRegistryToSource(filepath.Dir(path), cfg.RegistryDir)
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

	data, err := marshalYAML(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	data = append(schemaComment, data...)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// ExpandPath expands ~ to home directory.
func ExpandPath(path string) string {
	return expandPath(path)
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

// migrateSkillsToRegistry extracts skills[] from config.yaml into registry.yaml.
// Uses raw YAML parsing because Config struct no longer has a Skills field.
func migrateSkillsToRegistry(configPath string) error {
	configDir := filepath.Dir(configPath)
	registryPath := RegistryPath(configDir)

	// Only migrate if registry.yaml doesn't already exist
	if _, err := os.Stat(registryPath); err == nil {
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	// Parse with a temporary struct to extract skills
	var legacy struct {
		Skills []SkillEntry `yaml:"skills,omitempty"`
	}
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return nil
	}

	if len(legacy.Skills) == 0 {
		return nil
	}

	// Write registry.yaml
	reg := &Registry{Skills: legacy.Skills}
	if err := reg.Save(configDir); err != nil {
		return fmt.Errorf("failed to create registry.yaml during migration: %w", err)
	}

	// Strip skills from config.yaml using raw map
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	delete(raw, "skills")
	cleaned, err := marshalYAML(raw)
	if err != nil {
		return nil
	}
	cleaned = append(schemaComment, cleaned...)
	return os.WriteFile(configPath, cleaned, 0644)
}

// isValidGitLabHostname reports whether h is a bare hostname
// (no scheme, path, port, or empty after trim).
func isValidGitLabHostname(h string) bool {
	return h != "" &&
		!strings.Contains(h, "://") &&
		!strings.Contains(h, "/") &&
		!strings.Contains(h, ":")
}

// normalizeGitLabHosts validates and normalizes gitlab_hosts entries.
// Rejects entries containing "://", "/", ":", or empty after trim.
// Returns lowercased, trimmed hostnames.
func normalizeGitLabHosts(hosts []string) ([]string, error) {
	if len(hosts) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if !isValidGitLabHostname(h) {
			if h == "" {
				return nil, fmt.Errorf("gitlab_hosts: empty entry")
			}
			if strings.Contains(h, "://") {
				return nil, fmt.Errorf("gitlab_hosts: entry %q must be a hostname, not a URL (remove scheme)", h)
			}
			if strings.Contains(h, "/") {
				return nil, fmt.Errorf("gitlab_hosts: entry %q must be a hostname without path", h)
			}
			return nil, fmt.Errorf("gitlab_hosts: entry %q must be a hostname without port", h)
		}
		out = append(out, strings.ToLower(h))
	}
	return out, nil
}

// mergeGitLabHostsFromEnv merges SKILLSHARE_GITLAB_HOSTS env var entries
// with the already-normalized config file hosts. Returns deduplicated list.
// Invalid entries in the env var are silently skipped (unlike normalizeGitLabHosts
// which returns errors for config file entries).
func mergeGitLabHostsFromEnv(configHosts []string) []string {
	envVal := os.Getenv("SKILLSHARE_GITLAB_HOSTS")
	if envVal == "" {
		return configHosts
	}
	seen := make(map[string]bool, len(configHosts))
	for _, h := range configHosts {
		seen[h] = true
	}
	merged := append([]string(nil), configHosts...)
	for _, raw := range strings.Split(envVal, ",") {
		h := strings.ToLower(strings.TrimSpace(raw))
		if !isValidGitLabHostname(h) {
			continue // silently skip invalid entries from env
		}
		if !seen[h] {
			seen[h] = true
			merged = append(merged, h)
		}
	}
	return merged
}

func normalizeAuditBlockThreshold(v string) (string, error) {
	threshold := strings.ToUpper(strings.TrimSpace(v))
	if threshold == "" {
		return "", nil // empty → let callers decide default (e.g. ResolvePolicy)
	}
	switch threshold {
	case "CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO":
		return threshold, nil
	default:
		return "", fmt.Errorf("unsupported value %q", v)
	}
}
