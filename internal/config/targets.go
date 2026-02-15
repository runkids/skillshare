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
	GlobalName  string `yaml:"global_name"`
	ProjectName string `yaml:"project_name"`
	GlobalPath  string `yaml:"global_path"`
	ProjectPath string `yaml:"project_path"`
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
func LookupProjectTarget(name string) (TargetConfig, bool) {
	targets := ProjectTargets()
	target, ok := targets[name]
	return target, ok
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

		canonical := pg.names[0]
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
// config target name.  It handles cross-mode matching (e.g. "claude" matches
// "claude-code") by looking up the target spec registry.
func MatchesTargetName(skillTarget, configTarget string) bool {
	if skillTarget == configTarget {
		return true
	}

	specs, err := loadTargetSpecs()
	if err != nil {
		return false
	}

	for _, spec := range specs {
		names := [2]string{spec.GlobalName, spec.ProjectName}
		hasSkill := false
		hasConfig := false
		for _, n := range names {
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
		for _, n := range []string{spec.GlobalName, spec.ProjectName} {
			if n != "" && !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
	}
	return names
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
