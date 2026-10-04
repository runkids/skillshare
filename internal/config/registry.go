package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"skillshare/internal/sourcefs"
	"skillshare/internal/utils"
)

const registryFileName = "registry.yaml"

// SourceRoot returns the git repo root for the given source path.
// Walks up from source to find .git/ directory. If none found,
// returns source as-is. Handles --subdir where cfg.EffectiveSkillsSource() includes
// a subdirectory within the git repo.
func SourceRoot(source string) string {
	dir := source
	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return source
}

// RegistrySchemaURL is the JSON Schema URL for registry.yaml.
const RegistrySchemaURL = "https://raw.githubusercontent.com/runkids/skillshare/main/schemas/registry.schema.json"

var registrySchemaComment = []byte("# yaml-language-server: $schema=" + RegistrySchemaURL + "\n")

// Registry holds the skill registry (installed/tracked skills).
// Stored separately from Config to keep config.yaml focused on user-managed settings.
type Registry struct {
	Skills []SkillEntry `yaml:"skills,omitempty"`
}

// RegistryPath returns the registry file path for the given config directory.
func RegistryPath(dir string) string {
	return filepath.Join(dir, registryFileName)
}

// LoadRegistry reads registry.yaml from the given directory.
// Returns an empty Registry if the file does not exist.
func LoadRegistry(dir string) (*Registry, error) {
	path := RegistryPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{}, nil
		}
		return nil, fmt.Errorf("failed to read registry: %w", err)
	}

	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("failed to parse registry: %w", err)
	}

	for _, skill := range reg.Skills {
		if strings.TrimSpace(skill.Name) == "" {
			return nil, fmt.Errorf("registry has skill with empty name")
		}
		if strings.TrimSpace(skill.Source) == "" {
			return nil, fmt.Errorf("registry has skill '%s' with empty source", skill.Name)
		}
	}

	return &reg, nil
}

// MigrateRegistryToSource moves registry.yaml from config dir to source dir (one-time).
func MigrateRegistryToSource(configDir, sourceRoot string) {
	oldPath := filepath.Join(configDir, registryFileName)
	newPath := filepath.Join(sourceRoot, registryFileName)

	oldExists := fileExists(oldPath)
	newExists := fileExists(newPath)

	if !oldExists {
		return
	}
	if newExists {
		fmt.Fprintf(os.Stderr, "Warning: registry.yaml exists in both %s and %s — using %s\n", configDir, sourceRoot, sourceRoot)
		fmt.Fprintf(os.Stderr, "  You can safely delete: %s\n", oldPath)
		return
	}
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return
	}
	// Write through a handle at the source root, so a registry.yaml link
	// there is refused instead of written through.
	root, err := sourcefs.Open(sourceRoot)
	if err != nil {
		return
	}
	defer root.Close()
	if err := root.WriteFile(registryFileName, data, 0644); err != nil {
		return
	}
	os.Remove(oldPath)
	fmt.Fprintf(os.Stderr, "Migrated registry.yaml → %s\n", sourceRoot)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Save writes registry.yaml to the given directory.
func (r *Registry) Save(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	data, err := utils.MarshalYAML(r)
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	data = append(registrySchemaComment, data...)

	path := RegistryPath(dir)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry: %w", err)
	}

	return nil
}

// LoadUnifiedRegistry merges registries from both skills and agents source
// directories into a single Registry. Used in global mode where skills and
// agents have separate registry files.
// Skills entries get Kind="" (backward compat), agent entries get Kind="agent".
func LoadUnifiedRegistry(skillsDir, agentsDir string) (*Registry, error) {
	skillsReg, err := LoadRegistry(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load skills registry: %w", err)
	}

	agentsReg, err := LoadRegistry(agentsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load agents registry: %w", err)
	}

	// Ensure agent entries have Kind set
	for i := range agentsReg.Skills {
		if agentsReg.Skills[i].Kind == "" {
			agentsReg.Skills[i].Kind = "agent"
		}
	}

	unified := &Registry{
		Skills: make([]SkillEntry, 0, len(skillsReg.Skills)+len(agentsReg.Skills)),
	}
	unified.Skills = append(unified.Skills, skillsReg.Skills...)
	unified.Skills = append(unified.Skills, agentsReg.Skills...)

	return unified, nil
}

// SaveSplitByKind splits the unified registry by kind and saves each to
// the appropriate directory. Skills (Kind="" or "skill") go to skillsDir,
// agents (Kind="agent") go to agentsDir.
func (r *Registry) SaveSplitByKind(skillsDir, agentsDir string) error {
	skillsReg := &Registry{}
	agentsReg := &Registry{}

	for _, entry := range r.Skills {
		if entry.EffectiveKind() == "agent" {
			agentsReg.Skills = append(agentsReg.Skills, entry)
		} else {
			skillsReg.Skills = append(skillsReg.Skills, entry)
		}
	}

	if err := skillsReg.Save(skillsDir); err != nil {
		return fmt.Errorf("failed to save skills registry: %w", err)
	}

	if len(agentsReg.Skills) > 0 {
		if err := agentsReg.Save(agentsDir); err != nil {
			return fmt.Errorf("failed to save agents registry: %w", err)
		}
	}

	return nil
}
