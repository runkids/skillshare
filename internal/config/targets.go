package config

import (
	_ "embed"
	"path/filepath"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

type targetSpec struct {
	GlobalName  string   `yaml:"global_name"`
	ProjectName string   `yaml:"project_name"`
	GlobalPath  string   `yaml:"global_path"`
	ProjectPath string   `yaml:"project_path"`
	Aliases     []string `yaml:"aliases,omitempty"` // Deprecated: backward compat for old project_name values. Remove once safe.
}

type targetsFile struct {
	Targets []targetSpec `yaml:"targets"`
}

//go:embed targets.yaml
var defaultTargetsData []byte

var (
	loadedTargets   []targetSpec
	loadTargetsErr  error
	loadTargetsOnce sync.Once
)

func loadTargetSpecs() ([]targetSpec, error) {
	loadTargetsOnce.Do(func() {
		var file targetsFile
		if err := yaml.Unmarshal(defaultTargetsData, &file); err != nil {
			loadTargetsErr = err
			return
		}
		loadedTargets = file.Targets
	})

	return loadedTargets, loadTargetsErr
}

// DefaultTargets returns the well-known CLI skills directories for global mode.
func DefaultTargets() map[string]TargetConfig {
	specs, err := loadTargetSpecs()
	if err != nil {
		return map[string]TargetConfig{}
	}

	targets := make(map[string]TargetConfig)
	for _, spec := range specs {
		if spec.GlobalName == "" || spec.GlobalPath == "" {
			continue
		}
		path := normalizeTargetPath(spec.GlobalPath)
		targets[spec.GlobalName] = TargetConfig{Path: path}
	}

	return targets
}

// ProjectTargets returns the well-known CLI skills directories for project mode.
func ProjectTargets() map[string]TargetConfig {
	specs, err := loadTargetSpecs()
	if err != nil {
		return map[string]TargetConfig{}
	}

	targets := make(map[string]TargetConfig)
	for _, spec := range specs {
		if spec.ProjectName == "" || spec.ProjectPath == "" {
			continue
		}
		path := normalizeTargetPath(spec.ProjectPath)
		targets[spec.ProjectName] = TargetConfig{Path: path}
	}

	return targets
}

// LookupProjectTarget returns the known project target config for a name.
// It first checks canonical project names, then falls back to aliases.
func LookupProjectTarget(name string) (TargetConfig, bool) {
	targets := ProjectTargets()
	if target, ok := targets[name]; ok {
		return target, true
	}

	// Fallback: check aliases (backward compat — remove once safe)
	specs, err := loadTargetSpecs()
	if err != nil {
		return TargetConfig{}, false
	}
	for _, spec := range specs {
		for _, alias := range spec.Aliases {
			if alias == name && spec.ProjectName != "" && spec.ProjectPath != "" {
				return targets[spec.ProjectName], true
			}
		}
	}
	return TargetConfig{}, false
}

// LookupGlobalTarget returns the known global target config for a name.
func LookupGlobalTarget(name string) (TargetConfig, bool) {
	targets := DefaultTargets()
	target, ok := targets[name]
	return target, ok
}

// GroupedProjectTarget represents a project target, optionally grouped with
// other targets that share the same project path.
type GroupedProjectTarget struct {
	Name    string   // canonical name (alphabetically first among members)
	Path    string   // normalized project path
	Members []string // other project names sharing this path (nil if unique)
}

// GroupedProjectTargets returns project targets deduplicated by path.
// When multiple targets share the same project path, they are merged into a
// single entry whose Name is the alphabetically-first member. Members lists
// all other project names that share the path. Single-path targets have nil Members.
func GroupedProjectTargets() []GroupedProjectTarget {
	specs, err := loadTargetSpecs()
	if err != nil {
		return nil
	}

	// Group by normalized project path, preserving insertion order.
	type pathGroup struct {
		path  string
		names []string
	}
	pathMap := make(map[string]*pathGroup)
	var pathOrder []string

	for _, spec := range specs {
		if spec.ProjectName == "" || spec.ProjectPath == "" {
			continue
		}
		path := normalizeTargetPath(spec.ProjectPath)
		if pg, ok := pathMap[path]; ok {
			pg.names = append(pg.names, spec.ProjectName)
		} else {
			pathMap[path] = &pathGroup{path: path, names: []string{spec.ProjectName}}
			pathOrder = append(pathOrder, path)
		}
	}

	var result []GroupedProjectTarget
	for _, path := range pathOrder {
		pg := pathMap[path]
		sort.Strings(pg.names)

		if len(pg.names) == 1 {
			result = append(result, GroupedProjectTarget{
				Name: pg.names[0],
				Path: pg.path,
			})
			continue
		}

		// Prefer "universal" as canonical name for shared-path groups.
		canonical := pg.names[0]
		for _, name := range pg.names {
			if name == "universal" {
				canonical = name
				break
			}
		}
		members := make([]string, 0, len(pg.names)-1)
		for _, name := range pg.names {
			if name != canonical {
				members = append(members, name)
			}
		}
		result = append(result, GroupedProjectTarget{
			Name:    canonical,
			Path:    pg.path,
			Members: members,
		})
	}

	return result
}

// MatchesTargetName checks whether a skill-declared target name matches a
// config target name.  It handles alias matching (e.g. "claude" matches
// the alias "claude-code") by looking up the target spec registry.
func MatchesTargetName(skillTarget, configTarget string) bool {
	if skillTarget == configTarget {
		return true
	}

	specs, err := loadTargetSpecs()
	if err != nil {
		return false
	}

	for _, spec := range specs {
		allNames := make([]string, 0, 2+len(spec.Aliases))
		allNames = append(allNames, spec.GlobalName, spec.ProjectName)
		allNames = append(allNames, spec.Aliases...) // backward compat — remove once safe
		hasSkill := false
		hasConfig := false
		for _, n := range allNames {
			if n == skillTarget {
				hasSkill = true
			}
			if n == configTarget {
				hasConfig = true
			}
		}
		if hasSkill && hasConfig {
			return true
		}
	}

	// Fallback: check whether both names resolve to the same project or global path.
	// This handles cases like "codex" and "universal" sharing ".agents/skills".
	skillPaths := resolveTargetPaths(specs, skillTarget)
	configPaths := resolveTargetPaths(specs, configTarget)
	for _, sp := range skillPaths {
		for _, cp := range configPaths {
			if sp == cp {
				return true
			}
		}
	}

	return false
}

// KnownTargetNames returns all known target names (both global and project).
func KnownTargetNames() []string {
	specs, err := loadTargetSpecs()
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var names []string
	for _, spec := range specs {
		candidates := make([]string, 0, 2+len(spec.Aliases))
		candidates = append(candidates, spec.GlobalName, spec.ProjectName)
		candidates = append(candidates, spec.Aliases...) // backward compat — remove once safe
		for _, n := range candidates {
			if n != "" && !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
	}
	return names
}

// resolveTargetPaths collects the non-empty project and global paths for a
// target name across all specs (including aliases).
func resolveTargetPaths(specs []targetSpec, name string) []string {
	var paths []string
	seen := make(map[string]bool)
	for _, spec := range specs {
		allNames := make([]string, 0, 2+len(spec.Aliases))
		allNames = append(allNames, spec.GlobalName, spec.ProjectName)
		allNames = append(allNames, spec.Aliases...)
		match := false
		for _, n := range allNames {
			if n == name {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		for _, p := range []string{spec.ProjectPath, spec.GlobalPath} {
			if p != "" && !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	return paths
}

func normalizeTargetPath(path string) string {
	if path == "" {
		return path
	}
	if utils.HasTildePrefix(path) {
		path = expandPath(path)
	}
	return filepath.FromSlash(path)
}
