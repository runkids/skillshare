package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

// ProjectSchemaURL is the JSON Schema URL for the project config file.
const ProjectSchemaURL = "https://raw.githubusercontent.com/runkids/skillshare/main/schemas/project-config.schema.json"

// projectSchemaComment is the YAML Language Server directive prepended to saved project config files.
var projectSchemaComment = []byte("# yaml-language-server: $schema=" + ProjectSchemaURL + "\n")

// ProjectTargetEntry supports both string and object forms in YAML.
// String: "claude"
// Object: { name: "my-custom-ide", path: ".my-ide/skills/" }
type ProjectTargetEntry struct {
	Name    string
	Path    string
	Mode    string // "merge", "copy", or "symlink"; default "merge"
	Include []string
	Exclude []string
}

func (t *ProjectTargetEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		t.Name = strings.TrimSpace(value.Value)
		return nil
	}

	var decoded struct {
		Name    string   `yaml:"name"`
		Path    string   `yaml:"path"`
		Mode    string   `yaml:"mode"`
		Include []string `yaml:"include"`
		Exclude []string `yaml:"exclude"`
	}
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	t.Name = strings.TrimSpace(decoded.Name)
	t.Path = strings.TrimSpace(decoded.Path)
	t.Mode = strings.TrimSpace(decoded.Mode)
	t.Include = decoded.Include
	t.Exclude = decoded.Exclude
	return nil
}

func (t ProjectTargetEntry) MarshalYAML() (interface{}, error) {
	hasPath := strings.TrimSpace(t.Path) != ""
	hasMode := strings.TrimSpace(t.Mode) != ""
	hasInclude := len(t.Include) > 0
	hasExclude := len(t.Exclude) > 0

	if !hasPath && !hasMode && !hasInclude && !hasExclude {
		return t.Name, nil
	}

	obj := map[string]any{"name": t.Name}
	if hasPath {
		obj["path"] = t.Path
	}
	if hasMode {
		obj["mode"] = t.Mode
	}
	if hasInclude {
		obj["include"] = t.Include
	}
	if hasExclude {
		obj["exclude"] = t.Exclude
	}
	return obj, nil
}

// SkillEntry represents a remote skill entry in config (shared by global and project).
type SkillEntry struct {
	Name    string `yaml:"name"`
	Source  string `yaml:"source"`
	Tracked bool   `yaml:"tracked,omitempty"`
	Group   string `yaml:"group,omitempty"`
}

// FullName returns the full relative path for the skill entry.
// If Group is set, returns "group/name"; otherwise returns Name.
// For backward compatibility, if Name already contains "/" and Group is empty,
// returns Name as-is (legacy format).
func (s SkillEntry) FullName() string {
	if s.Group != "" {
		return s.Group + "/" + s.Name
	}
	return s.Name
}

// EffectiveParts returns the effective (group, bareName) for this skill entry.
// If Group is set, returns (Group, Name).
// For backward compat, if Name contains "/" and Group is empty,
// splits at the last "/" to derive group and bare name.
func (s SkillEntry) EffectiveParts() (group, name string) {
	if s.Group != "" {
		return s.Group, s.Name
	}
	if idx := strings.LastIndex(s.Name, "/"); idx >= 0 {
		return s.Name[:idx], s.Name[idx+1:]
	}
	return "", s.Name
}

// ProjectConfig holds project-level config (.skillshare/config.yaml).
type ProjectConfig struct {
	Targets []ProjectTargetEntry `yaml:"targets"`
	Audit   AuditConfig          `yaml:"audit,omitempty"`
	Hub     HubConfig            `yaml:"hub,omitempty"`
}

// ProjectConfigPath returns the project config path for the given root.
func ProjectConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".skillshare", "config.yaml")
}

// LoadProject loads the project config from the given root.
func LoadProject(projectRoot string) (*ProjectConfig, error) {
	path := ProjectConfigPath(projectRoot)

	// Migrate skills[] to registry.yaml (one-time, silent)
	_ = migrateProjectSkillsToRegistry(path, projectRoot)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project config not found: run 'skillshare init -p' first")
		}
		return nil, fmt.Errorf("failed to read project config: %w", err)
	}

	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse project config: %w", err)
	}

	threshold, err := normalizeAuditBlockThreshold(cfg.Audit.BlockThreshold)
	if err != nil {
		return nil, fmt.Errorf("project config has invalid audit.block_threshold: %w", err)
	}
	cfg.Audit.BlockThreshold = threshold

	for _, target := range cfg.Targets {
		if strings.TrimSpace(target.Name) == "" {
			return nil, fmt.Errorf("project config has target with empty name")
		}
	}

	return &cfg, nil
}

// Save writes the project config to the given root.
func (c *ProjectConfig) Save(projectRoot string) error {
	path := ProjectConfigPath(projectRoot)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create project config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}

	data = append(projectSchemaComment, data...)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write project config: %w", err)
	}

	return nil
}

// migrateProjectSkillsToRegistry extracts skills[] from project config.yaml into registry.yaml.
// Uses raw YAML parsing because ProjectConfig struct no longer has a Skills field.
func migrateProjectSkillsToRegistry(configPath, projectRoot string) error {
	registryDir := filepath.Join(projectRoot, ".skillshare")
	registryPath := RegistryPath(registryDir)

	if _, err := os.Stat(registryPath); err == nil {
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var legacy struct {
		Skills []SkillEntry `yaml:"skills,omitempty"`
	}
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return nil
	}

	if len(legacy.Skills) == 0 {
		return nil
	}

	reg := &Registry{Skills: legacy.Skills}
	if err := reg.Save(registryDir); err != nil {
		return fmt.Errorf("failed to create registry.yaml during project migration: %w", err)
	}

	// Strip skills from project config.yaml
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	delete(raw, "skills")
	cleaned, err := yaml.Marshal(raw)
	if err != nil {
		return nil
	}
	cleaned = append(projectSchemaComment, cleaned...)
	return os.WriteFile(configPath, cleaned, 0644)
}

// ResolveProjectTargets converts project config targets into absolute target paths.
func ResolveProjectTargets(projectRoot string, cfg *ProjectConfig) (map[string]TargetConfig, error) {
	resolved := make(map[string]TargetConfig)
	for _, entry := range cfg.Targets {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}

		var targetPath string
		if strings.TrimSpace(entry.Path) != "" {
			targetPath = entry.Path
		} else if known, ok := LookupProjectTarget(name); ok {
			targetPath = known.Path
		} else {
			return nil, fmt.Errorf("unknown target '%s' (missing path)", name)
		}

		absPath := targetPath
		if utils.HasTildePrefix(absPath) {
			absPath = expandPath(absPath)
		}
		if !filepath.IsAbs(targetPath) {
			absPath = filepath.Join(projectRoot, filepath.FromSlash(targetPath))
		}

		resolved[name] = TargetConfig{
			Path:    absPath,
			Mode:    entry.Mode,
			Include: append([]string(nil), entry.Include...),
			Exclude: append([]string(nil), entry.Exclude...),
		}
	}

	return resolved, nil
}
