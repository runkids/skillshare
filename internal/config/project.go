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
// New format with resource sub-keys: { name: claude, skills: {mode: merge}, agents: {path: .claude/agents} }
type ProjectTargetEntry struct {
	Name    string
	Path    string   // legacy flat field; migrated to Skills on unmarshal
	Mode    string   // legacy flat field; migrated to Skills on unmarshal
	Include []string // legacy flat field; migrated to Skills on unmarshal
	Exclude []string // legacy flat field; migrated to Skills on unmarshal

	Skills *ResourceTargetConfig
	Agents *ResourceTargetConfig

	wasMigrated         bool   // true if flat fields were migrated during unmarshal; not serialized
	defaultTargetNaming string `yaml:"-"`
}

func (t *ProjectTargetEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		t.Name = strings.TrimSpace(value.Value)
		return nil
	}

	var decoded struct {
		Name    string                `yaml:"name"`
		Path    string                `yaml:"path"`
		Mode    string                `yaml:"mode"`
		Include []string              `yaml:"include"`
		Exclude []string              `yaml:"exclude"`
		Skills  *ResourceTargetConfig `yaml:"skills"`
		Agents  *ResourceTargetConfig `yaml:"agents"`
	}
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	t.Name = strings.TrimSpace(decoded.Name)
	t.Path = strings.TrimSpace(decoded.Path)
	t.Mode = strings.TrimSpace(decoded.Mode)
	t.Include = decoded.Include
	t.Exclude = decoded.Exclude
	t.Skills = decoded.Skills
	t.Agents = decoded.Agents

	// Migrate legacy flat fields into Skills sub-key.
	hasFlatFields := t.Path != "" || t.Mode != "" || len(t.Include) > 0 || len(t.Exclude) > 0
	if hasFlatFields {
		if t.Skills == nil {
			t.Skills = &ResourceTargetConfig{
				Path:    t.Path,
				Mode:    t.Mode,
				Include: t.Include,
				Exclude: t.Exclude,
			}
		} else {
			// Mixed format: merge flat fields into existing Skills (don't overwrite)
			if t.Skills.Path == "" && t.Path != "" {
				t.Skills.Path = t.Path
			}
			if t.Skills.Mode == "" && t.Mode != "" {
				t.Skills.Mode = t.Mode
			}
			if len(t.Skills.Include) == 0 && len(t.Include) > 0 {
				t.Skills.Include = t.Include
			}
			if len(t.Skills.Exclude) == 0 && len(t.Exclude) > 0 {
				t.Skills.Exclude = t.Exclude
			}
		}
		t.Path = ""
		t.Mode = ""
		t.Include = nil
		t.Exclude = nil
		t.wasMigrated = true
	}
	return nil
}

func (t ProjectTargetEntry) MarshalYAML() (interface{}, error) {
	hasSkills := t.Skills != nil && !t.Skills.IsEmpty()
	hasAgents := t.Agents != nil && !t.Agents.IsEmpty()
	hasPath := strings.TrimSpace(t.Path) != ""
	hasMode := strings.TrimSpace(t.Mode) != ""
	hasInclude := len(t.Include) > 0
	hasExclude := len(t.Exclude) > 0

	// New format: write skills/agents sub-keys
	if hasSkills || hasAgents {
		obj := map[string]any{"name": t.Name}
		if hasSkills {
			obj["skills"] = t.Skills
		}
		if hasAgents {
			obj["agents"] = t.Agents
		}
		return obj, nil
	}

	// Legacy flat fields (backward compat for pre-migration data)
	if hasPath || hasMode || hasInclude || hasExclude {
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

	// String format: no extra config
	return t.Name, nil
}

// SkillsConfig returns the effective skills configuration.
// If Skills sub-key is set, it is returned directly.
// Otherwise the legacy flat fields are returned for backward compatibility.
func (t *ProjectTargetEntry) SkillsConfig() ResourceTargetConfig {
	if t.Skills != nil {
		sc := *t.Skills
		if sc.TargetNaming == "" {
			sc.TargetNaming = t.defaultTargetNaming
		}
		return sc
	}
	return ResourceTargetConfig{
		Path:         t.Path,
		Mode:         t.Mode,
		TargetNaming: t.defaultTargetNaming,
		Include:      t.Include,
		Exclude:      t.Exclude,
	}
}

// AgentsConfig returns the agents configuration, or an empty value if not set.
func (t *ProjectTargetEntry) AgentsConfig() ResourceTargetConfig {
	if t.Agents != nil {
		return *t.Agents
	}
	return ResourceTargetConfig{}
}

// EnsureSkills returns the Skills sub-key, creating it from legacy flat fields if nil.
func (t *ProjectTargetEntry) EnsureSkills() *ResourceTargetConfig {
	if t.Skills == nil {
		sc := ResourceTargetConfig{
			Path:    t.Path,
			Mode:    t.Mode,
			Include: t.Include,
			Exclude: t.Exclude,
		}
		t.Skills = &sc
		t.Path = ""
		t.Mode = ""
		t.Include = nil
		t.Exclude = nil
	}
	return t.Skills
}

// SkillEntry represents a remote skill entry in config (shared by global and project).
type SkillEntry struct {
	Name    string `yaml:"name"`
	Kind    string `yaml:"kind,omitempty"`
	Source  string `yaml:"source"`
	Tracked bool   `yaml:"tracked,omitempty"`
	Group   string `yaml:"group,omitempty"`
}

// EffectiveKind returns the resource kind for this entry.
// Returns "skill" if Kind is empty (backward compatibility).
func (s SkillEntry) EffectiveKind() string {
	if s.Kind == "" {
		return "skill"
	}
	return s.Kind
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
	Targets      []ProjectTargetEntry `yaml:"targets"`
	TargetNaming string               `yaml:"target_naming,omitempty"`
	Extras       []ExtraConfig        `yaml:"extras,omitempty"`
	Audit        AuditConfig          `yaml:"audit,omitempty"`
	Hub          HubConfig            `yaml:"hub,omitempty"`
	GitLabHosts  []string             `yaml:"gitlab_hosts,omitempty"`
}

// EffectiveGitLabHosts returns GitLabHosts merged with SKILLSHARE_GITLAB_HOSTS env var.
func (c *ProjectConfig) EffectiveGitLabHosts() []string {
	return mergeGitLabHostsFromEnv(c.GitLabHosts)
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

	// Validate and normalize gitlab_hosts (config file only; env var merged at read time)
	hosts, err := normalizeGitLabHosts(cfg.GitLabHosts)
	if err != nil {
		return nil, fmt.Errorf("project config: %w", err)
	}
	cfg.GitLabHosts = hosts

	for _, target := range cfg.Targets {
		if strings.TrimSpace(target.Name) == "" {
			return nil, fmt.Errorf("project config has target with empty name")
		}
	}
	for i := range cfg.Targets {
		cfg.Targets[i].defaultTargetNaming = cfg.TargetNaming
	}

	// Persist migrated format if any target had legacy flat fields.
	// UnmarshalYAML sets wasMigrated on each entry it migrated.
	migrated := false
	for _, t := range cfg.Targets {
		if t.wasMigrated {
			migrated = true
			break
		}
	}
	if migrated {
		if mdata, merr := marshalYAML(&cfg); merr == nil {
			tmpPath := path + ".tmp"
			if writeErr := os.WriteFile(tmpPath, append(projectSchemaComment, mdata...), 0644); writeErr == nil {
				os.Rename(tmpPath, path)
			}
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

	data, err := marshalYAML(c)
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
	cleaned, err := marshalYAML(raw)
	if err != nil {
		return nil
	}
	cleaned = append(projectSchemaComment, cleaned...)
	return os.WriteFile(configPath, cleaned, 0644)
}

// needsTargetMigration detects if a project config still uses legacy flat
// target format by re-parsing targets and checking for flat fields.

// ResolveProjectTargets converts project config targets into absolute target paths.
func ResolveProjectTargets(projectRoot string, cfg *ProjectConfig) (map[string]TargetConfig, error) {
	resolved := make(map[string]TargetConfig)
	for _, entry := range cfg.Targets {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}

		sc := entry.SkillsConfig()

		var targetPath string
		if strings.TrimSpace(sc.Path) != "" {
			targetPath = sc.Path
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
			defaultTargetNaming: cfg.TargetNaming,
			Skills: &ResourceTargetConfig{
				Path:         absPath,
				Mode:         sc.Mode,
				TargetNaming: sc.TargetNaming,
				Include:      append([]string(nil), sc.Include...),
				Exclude:      append([]string(nil), sc.Exclude...),
			},
		}
	}

	return resolved, nil
}
