package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

// ProjectTargetEntry supports both string and object forms in YAML.
// String: "claude-code"
// Object: { name: "my-custom-ide", path: ".my-ide/skills/" }
type ProjectTargetEntry struct {
	Name string
	Path string
	Mode string // "merge" or "symlink", default "merge"
}

func (t *ProjectTargetEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		t.Name = strings.TrimSpace(value.Value)
		return nil
	}

	var decoded struct {
		Name string `yaml:"name"`
		Path string `yaml:"path"`
		Mode string `yaml:"mode"`
	}
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	t.Name = strings.TrimSpace(decoded.Name)
	t.Path = strings.TrimSpace(decoded.Path)
	t.Mode = strings.TrimSpace(decoded.Mode)
	return nil
}

func (t ProjectTargetEntry) MarshalYAML() (interface{}, error) {
	hasPath := strings.TrimSpace(t.Path) != ""
	hasMode := strings.TrimSpace(t.Mode) != ""

	if !hasPath && !hasMode {
		return t.Name, nil
	}

	obj := map[string]string{"name": t.Name}
	if hasPath {
		obj["path"] = t.Path
	}
	if hasMode {
		obj["mode"] = t.Mode
	}
	return obj, nil
}

// ProjectSkill represents a remote skill entry in project config.
type ProjectSkill struct {
	Name    string `yaml:"name"`
	Source  string `yaml:"source"`
	Tracked bool   `yaml:"tracked,omitempty"`
}

// ProjectConfig holds project-level config (.skillshare/config.yaml).
type ProjectConfig struct {
	Targets []ProjectTargetEntry `yaml:"targets"`
	Skills  []ProjectSkill       `yaml:"skills,omitempty"`
	Audit   AuditConfig          `yaml:"audit,omitempty"`
}

// ProjectConfigPath returns the project config path for the given root.
func ProjectConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".skillshare", "config.yaml")
}

// LoadProject loads the project config from the given root.
func LoadProject(projectRoot string) (*ProjectConfig, error) {
	path := ProjectConfigPath(projectRoot)
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
	for _, skill := range cfg.Skills {
		if strings.TrimSpace(skill.Name) == "" {
			return nil, fmt.Errorf("project config has skill with empty name")
		}
		if strings.TrimSpace(skill.Source) == "" {
			return nil, fmt.Errorf("project config has skill '%s' with empty source", skill.Name)
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

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write project config: %w", err)
	}

	return nil
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

		resolved[name] = TargetConfig{Path: absPath, Mode: entry.Mode}
	}

	return resolved, nil
}
