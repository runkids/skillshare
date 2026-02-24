package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const registryFileName = "registry.yaml"

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

// Save writes registry.yaml to the given directory.
func (r *Registry) Save(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	data, err := yaml.Marshal(r)
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
