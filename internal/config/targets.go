package config

import (
	_ "embed"
	"fmt"
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

type targetSpec struct {
	Name    string         `yaml:"name"`
	Skills  targetPathPair `yaml:"skills"`
	Agents  targetPathPair `yaml:"agents,omitempty"`
	Aliases []string       `yaml:"aliases,omitempty"`
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
			if alias == name && spec.Name != "" && spec.Skills.Project != "" {
				return targets[spec.Name], true
			}
		}
	}
	return TargetConfig{}, false
}

// LookupGlobalTarget returns the known global target config for a name.
func LookupGlobalTarget(name string) (TargetConfig, bool) {
	targets := DefaultTargets()
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
			if alias == name && spec.Name != "" && spec.Skills.Global != "" {
				return targets[spec.Name], true
			}
		}
	}
	return TargetConfig{}, false
}

// ResolveTargetNameCandidate resolves a user-provided target name against a set
// of candidate names, preferring exact matches and then a single unambiguous
// alias/canonical match within the same target spec.
func ResolveTargetNameCandidate(name string, candidates []string) (string, bool, error) {
	return resolveTargetNameCandidate(name, candidates, sameTargetSpecName)
}

// CanonicalTargetName returns the canonical target name for a known target
// spec. Unknown names are returned unchanged with ok=false.
func CanonicalTargetName(name string) (canonical string, ok bool) {
	specs, err := loadTargetSpecs()
	if err != nil {
		return name, false
	}

	for _, spec := range specs {
		allNames := make([]string, 0, 1+len(spec.Aliases))
		allNames = append(allNames, spec.Name)
		allNames = append(allNames, spec.Aliases...)
		for _, candidate := range allNames {
			if candidate == name {
				return spec.Name, true
			}
		}
	}

	return name, false
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

func resolveTargetNameCandidate(name string, candidates []string, matchers ...func(string, string) bool) (string, bool, error) {
	for _, candidate := range candidates {
		if candidate == name {
			return candidate, true, nil
		}
	}

	for _, matcher := range matchers {
		matches := make([]string, 0, len(candidates))
		seen := make(map[string]struct{}, len(candidates))
		for _, candidate := range candidates {
			if candidate == name || !matcher(name, candidate) {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			matches = append(matches, candidate)
		}

		switch len(matches) {
		case 0:
			continue
		case 1:
			return matches[0], true, nil
		default:
			sort.Strings(matches)
			return "", false, fmt.Errorf("target %q is ambiguous; matches %s", name, strings.Join(matches, ", "))
		}
	}

	return "", false, nil
}

func sameTargetSpecName(a, b string) bool {
	specs, err := loadTargetSpecs()
	if err != nil {
		return false
	}

	for _, spec := range specs {
		allNames := make([]string, 0, 1+len(spec.Aliases))
		allNames = append(allNames, spec.Name)
		allNames = append(allNames, spec.Aliases...)
		hasA := false
		hasB := false
		for _, name := range allNames {
			if name == a {
				hasA = true
			}
			if name == b {
				hasB = true
			}
		}
		if hasA && hasB {
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
		allNames := make([]string, 0, 1+len(spec.Aliases))
		allNames = append(allNames, spec.Name)
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
		for _, p := range []string{spec.Skills.Project, spec.Skills.Global} {
			if p != "" && !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	return paths
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
