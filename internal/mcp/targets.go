package mcp

import (
	_ "embed"
	"path/filepath"
	gosync "sync"

	"gopkg.in/yaml.v3"
	"skillshare/internal/config"
)

//go:embed targets.yaml
var defaultMCPTargetsData []byte

// MCPTargetSpec describes a single AI tool's MCP config file location.
type MCPTargetSpec struct {
	Name          string `yaml:"name"`
	GlobalConfig  string `yaml:"global_config"`
	ProjectConfig string `yaml:"project_config"`
	Key           string `yaml:"key"`
	URLKey        string `yaml:"url_key,omitempty"`
}

// EffectiveURLKey returns the URL key to use for this target.
// If URLKey is not set, it defaults to "url".
func (s MCPTargetSpec) EffectiveURLKey() string {
	if s.URLKey != "" {
		return s.URLKey
	}
	return "url"
}

// GlobalConfigPath returns the expanded absolute path to the global MCP config
// file. Returns an empty string if this target has no global config.
func (s MCPTargetSpec) GlobalConfigPath() string {
	if s.GlobalConfig == "" {
		return ""
	}
	return filepath.FromSlash(config.ExpandPath(s.GlobalConfig))
}

// ProjectConfigPath returns the absolute path to the project-mode MCP config
// file by joining projectRoot with the relative ProjectConfig path.
// Returns an empty string if this target has no project config.
func (s MCPTargetSpec) ProjectConfigPath(projectRoot string) string {
	if s.ProjectConfig == "" {
		return ""
	}
	return filepath.Join(projectRoot, filepath.FromSlash(s.ProjectConfig))
}

type mcpTargetsFile struct {
	Targets []MCPTargetSpec `yaml:"targets"`
}

var (
	loadedMCPTargets  []MCPTargetSpec
	loadMCPTargetsErr error
	loadMCPTargetsOnce gosync.Once
)

func loadMCPTargetSpecs() ([]MCPTargetSpec, error) {
	loadMCPTargetsOnce.Do(func() {
		var file mcpTargetsFile
		if err := yaml.Unmarshal(defaultMCPTargetsData, &file); err != nil {
			loadMCPTargetsErr = err
			return
		}
		loadedMCPTargets = file.Targets
	})
	return loadedMCPTargets, loadMCPTargetsErr
}

// MCPTargets returns all MCP target specs from the embedded registry.
func MCPTargets() ([]MCPTargetSpec, error) {
	return loadMCPTargetSpecs()
}

// LookupMCPTarget returns the MCPTargetSpec for the given name.
// Returns (spec, true) if found, or (zero, false) if not.
func LookupMCPTarget(name string) (MCPTargetSpec, bool) {
	specs, err := loadMCPTargetSpecs()
	if err != nil {
		return MCPTargetSpec{}, false
	}
	for _, s := range specs {
		if s.Name == name {
			return s, true
		}
	}
	return MCPTargetSpec{}, false
}

// MCPTargetNames returns the names of all registered MCP targets.
func MCPTargetNames() []string {
	specs, err := loadMCPTargetSpecs()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(specs))
	for _, s := range specs {
		names = append(names, s.Name)
	}
	return names
}

// MCPTargetsForMode returns targets applicable to the given mode.
// When project is false (global mode), only targets with a non-empty GlobalConfig are returned.
// When project is true (project mode), only targets with a non-empty ProjectConfig are returned.
func MCPTargetsForMode(project bool) []MCPTargetSpec {
	specs, err := loadMCPTargetSpecs()
	if err != nil {
		return nil
	}
	var result []MCPTargetSpec
	for _, s := range specs {
		if project {
			if s.ProjectConfig != "" {
				result = append(result, s)
			}
		} else {
			if s.GlobalConfig != "" {
				result = append(result, s)
			}
		}
	}
	return result
}
