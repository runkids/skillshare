package config

import (
	_ "embed"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

// targetPathPair holds the global and project paths for a single resource kind.
type targetPathPair struct {
	Global  string `yaml:"global"`
	Project string `yaml:"project"`
}

// targetAlsoScans lists additional filesystem paths a target's runtime is
// known to scan beyond its primary skills path. Sourced from each tool's
// official documentation. Used by `skillshare doctor` to warn about
// cross-target discovery overlap (e.g. Codex's runtime scans ~/.agents/skills
// even though its primary skillshare path is ~/.codex/skills).
type targetAlsoScans struct {
	Global  []string `yaml:"global,omitempty"`
	Project []string `yaml:"project,omitempty"`
}

type targetSpec struct {
	Name      string          `yaml:"name"`
	Skills    targetPathPair  `yaml:"skills"`
	Agents    targetPathPair  `yaml:"agents,omitempty"`
	AlsoScans targetAlsoScans `yaml:"also_scans,omitempty"`
	Aliases   []string        `yaml:"aliases,omitempty"`
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
		if spec.Name == "" || spec.Skills.Global == "" {
			continue
		}
		path := normalizeTargetPath(spec.Skills.Global)
		targets[spec.Name] = TargetConfig{Path: path}
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
		if spec.Name == "" || spec.Skills.Project == "" {
			continue
		}
		path := normalizeTargetPath(spec.Skills.Project)
		targets[spec.Name] = TargetConfig{Path: path}
	}

	return targets
}

// DefaultAgentTargets returns the well-known agent directories for global mode.
// Only targets that define agent paths are included.
func DefaultAgentTargets() map[string]TargetConfig {
	specs, err := loadTargetSpecs()
	if err != nil {
		return map[string]TargetConfig{}
	}

	targets := make(map[string]TargetConfig)
	for _, spec := range specs {
		if spec.Name == "" || spec.Agents.Global == "" {
			continue
		}
		path := normalizeTargetPath(spec.Agents.Global)
		targets[spec.Name] = TargetConfig{Path: path}
	}

	return targets
}

// ProjectAgentTargets returns the well-known agent directories for project mode.
// Only targets that define agent paths are included.
func ProjectAgentTargets() map[string]TargetConfig {
	specs, err := loadTargetSpecs()
	if err != nil {
		return map[string]TargetConfig{}
	}

	targets := make(map[string]TargetConfig)
	for _, spec := range specs {
		if spec.Name == "" || spec.Agents.Project == "" {
			continue
		}
		path := normalizeTargetPath(spec.Agents.Project)
		targets[spec.Name] = TargetConfig{Path: path}
	}

	return targets
}

// LookupGlobalAgentTarget returns the known global agent target config for a
// target name or alias.
func LookupGlobalAgentTarget(name string) (TargetConfig, bool) {
	return lookupAgentTarget(name, false)
}

// LookupProjectAgentTarget returns the known project agent target config for a
// target name or alias.
func LookupProjectAgentTarget(name string) (TargetConfig, bool) {
	return lookupAgentTarget(name, true)
}

func lookupAgentTarget(name string, project bool) (TargetConfig, bool) {
	specs, err := loadTargetSpecs()
	if err != nil {
		return TargetConfig{}, false
	}
	for _, spec := range specs {
		if !targetSpecMatchesName(spec, name) {
			continue
		}
		path := spec.Agents.Global
		if project {
			path = spec.Agents.Project
		}
		if spec.Name == "" || path == "" {
			return TargetConfig{}, false
		}
		return TargetConfig{Path: normalizeTargetPath(path)}, true
	}
	return TargetConfig{}, false
}

// AlsoScansGlobal returns the additional filesystem paths a target's runtime
// scans in global mode beyond its primary skills path. Paths are tilde-expanded
// and OS-normalised. Returns nil for unknown targets or targets without
// also_scans metadata.
func AlsoScansGlobal(name string) []string {
	specs, err := loadTargetSpecs()
	if err != nil {
		return nil
	}
	for _, spec := range specs {
		if spec.Name != name {
			continue
		}
		if len(spec.AlsoScans.Global) == 0 {
			return nil
		}
		paths := make([]string, len(spec.AlsoScans.Global))
		for i, p := range spec.AlsoScans.Global {
			paths[i] = normalizeTargetPath(p)
		}
		return paths
	}
	return nil
}

// AlsoScansProject returns the project-mode equivalent of AlsoScansGlobal.
func AlsoScansProject(name string) []string {
	specs, err := loadTargetSpecs()
	if err != nil {
		return nil
	}
	for _, spec := range specs {
		if spec.Name != name {
			continue
		}
		if len(spec.AlsoScans.Project) == 0 {
			return nil
		}
		paths := make([]string, len(spec.AlsoScans.Project))
		for i, p := range spec.AlsoScans.Project {
			paths[i] = normalizeTargetPath(p)
		}
		return paths
	}
	return nil
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
		if targetSpecMatchesName(spec, name) && spec.Name != "" && spec.Skills.Project != "" {
			return TargetConfig{Path: normalizeTargetPath(spec.Skills.Project)}, true
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
		if spec.Name == "" || spec.Skills.Project == "" {
			continue
		}
		path := normalizeTargetPath(spec.Skills.Project)
		if pg, ok := pathMap[path]; ok {
			pg.names = append(pg.names, spec.Name)
		} else {
			pathMap[path] = &pathGroup{path: path, names: []string{spec.Name}}
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
		allNames := make([]string, 0, 1+len(spec.Aliases))
		allNames = append(allNames, spec.Name)
		allNames = append(allNames, spec.Aliases...)
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
		candidates := make([]string, 0, 1+len(spec.Aliases))
		candidates = append(candidates, spec.Name)
		candidates = append(candidates, spec.Aliases...)
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
		if !targetSpecMatchesName(spec, name) {
			continue
		}
		for _, p := range []string{spec.Skills.Project, spec.Skills.Global} {
			if p != "" && !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	return paths
}

func targetSpecMatchesName(spec targetSpec, name string) bool {
	if spec.Name == name {
		return true
	}
	for _, alias := range spec.Aliases {
		if alias == name {
			return true
		}
	}
	return false
}

// ProjectTargetDotDirs returns the set of hidden directory names (e.g. ".claude",
// ".cursor") that are first path segments in any project target path. These are
// well-known AI-tool config directories that should be skipped during skill
// discovery to avoid counting target-synced copies as source skills.
// ".skillshare" is always included.
func ProjectTargetDotDirs() map[string]bool {
	specs, err := loadTargetSpecs()
	if err != nil {
		return map[string]bool{".skillshare": true}
	}

	dirs := map[string]bool{".skillshare": true}
	for _, spec := range specs {
		// Collect dot-dirs from both skill and agent project paths.
		for _, p := range []string{spec.Skills.Project, spec.Agents.Project} {
			if p == "" {
				continue
			}
			first := strings.SplitN(p, "/", 2)[0]
			if strings.HasPrefix(first, ".") {
				dirs[first] = true
			}
		}
	}
	return dirs
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
