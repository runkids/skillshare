package config

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
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

// targetInstructions is the instruction file (CLAUDE.md, AGENTS.md, ...) a
// target's runtime reads. Import marks runtimes that follow an @path line, so a
// managed import line can replace a symlink.
type targetInstructions struct {
	Global  string `yaml:"global,omitempty"`
	Project string `yaml:"project,omitempty"`
	Import  bool   `yaml:"import,omitempty"`
	// SameAs names the target whose global file this one also reads.
	SameAs string `yaml:"same_as,omitempty"`
	// MaxChars is the length past which the runtime cuts the global file.
	MaxChars int `yaml:"max_chars,omitempty"`
	// ProjectFallback is the file read in a project that has no Project file.
	ProjectFallback string `yaml:"project_fallback,omitempty"`
	// Rules is a directory of extra .md files the runtime loads as well.
	Rules targetPathPair `yaml:"rules,omitempty"`
}

// InstructionsTarget is a target's instruction file. Path (and Rules) are
// tilde-expanded in global mode and relative to the project root in project
// mode. Fallback is set in project mode only.
type InstructionsTarget struct {
	Path     string
	Import   bool
	SameAs   string
	MaxChars int
	Fallback string
	Rules    string
}

type targetSpec struct {
	Name string `yaml:"name"`
	// Detect is the tool's install directory (e.g. ~/.codex). Only needed for
	// targets whose skills path is shared with another target, where the skills
	// path alone cannot tell the tools apart.
	Detect string `yaml:"detect,omitempty"`
	// ConfigDir is the directory the Agent keeps its files in, where the Agent can be told to
	// use another one (CLAUDE_CONFIG_DIR). A target may then be such another directory.
	ConfigDir    string             `yaml:"config_dir,omitempty"`
	Skills       targetPathPair     `yaml:"skills"`
	Agents       targetPathPair     `yaml:"agents,omitempty"`
	AlsoScans    targetAlsoScans    `yaml:"also_scans,omitempty"`
	Instructions targetInstructions `yaml:"instructions,omitempty"`
	// Files are plain files the tool reads besides its instruction file,
	// relative to its file root (see TargetFileRoot).
	Files   []string `yaml:"files,omitempty"`
	Aliases []string `yaml:"aliases,omitempty"`
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

	// Resolve native config homes per call, without changing the embedded defaults
	// or explicit paths saved in a user's config.
	specs := append([]targetSpec(nil), loadedTargets...)
	for i, spec := range specs {
		if home := targetConfigHome(spec.Name, runtime.GOOS); home != "" {
			specs[i].Skills.Global = filepath.Join(home, "skills")
		}
	}
	return specs, loadTargetsErr
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

// LookupInstructions returns the instruction file of a target name or alias.
// It reports false for unknown targets and targets without that file.
func LookupInstructions(name string, project bool) (InstructionsTarget, bool) {
	specs, err := loadTargetSpecs()
	if err != nil {
		return InstructionsTarget{}, false
	}
	for _, spec := range specs {
		if !targetSpecMatchesName(spec, name) {
			continue
		}
		return specInstructions(spec, project)
	}
	return InstructionsTarget{}, false
}

func specInstructions(spec targetSpec, project bool) (InstructionsTarget, bool) {
	in := spec.Instructions
	it := InstructionsTarget{Path: in.Global, Import: in.Import, SameAs: in.SameAs, MaxChars: in.MaxChars, Rules: in.Rules.Global}
	if project {
		it = InstructionsTarget{Path: in.Project, Import: in.Import, Fallback: in.ProjectFallback, Rules: in.Rules.Project}
	}
	if it.Path == "" {
		return InstructionsTarget{}, false
	}
	it.Path, it.Rules, it.Fallback = normalizeTargetPath(it.Path), normalizeTargetPath(it.Rules), normalizeTargetPath(it.Fallback)
	return it, true
}

// TargetInstructions returns the instruction file of a configured target. A
// target that is another config directory of an Agent reads the Agent's file
// moved into that directory; a target expanded from projects has none.
func TargetInstructions(name string, tc TargetConfig, project bool) (InstructionsTarget, bool) {
	if tc.ProjectRoot() != "" {
		return InstructionsTarget{}, false
	}
	if tc.Instructions != nil && strings.TrimSpace(tc.Instructions.Path) != "" {
		return customInstructions(*tc.Instructions, project), true
	}
	if tc.Agent == "" || tc.ConfigDir == "" || project {
		return LookupInstructions(name, project)
	}
	spec, ok := globalSpec(tc.Agent)
	if !ok || spec.ConfigDir == "" {
		return InstructionsTarget{}, false
	}
	it, ok := specInstructions(spec, false)
	if !ok {
		return InstructionsTarget{}, false
	}
	base, dir := normalizeTargetPath(spec.ConfigDir), expandPath(tc.ConfigDir)
	move := func(path string) string {
		rel, err := filepath.Rel(base, path)
		if path == "" || err != nil || strings.HasPrefix(rel, "..") {
			return ""
		}
		return filepath.Join(dir, rel)
	}
	if it.Path = move(it.Path); it.Path == "" {
		return InstructionsTarget{}, false
	}
	// An account's file is its own, never another target's.
	it.Rules, it.SameAs = move(it.Rules), ""
	return it, true
}

// customInstructions turns a user-set instruction file into an
// InstructionsTarget: tilde-expanded in global mode, left relative to the
// project root in project mode (instructions.Resolve joins it).
func customInstructions(ic TargetInstructionsConfig, project bool) InstructionsTarget {
	path := strings.TrimSpace(ic.Path)
	if project {
		path = filepath.Clean(filepath.FromSlash(path))
	} else {
		path = filepath.Clean(normalizeTargetPath(path))
	}
	return InstructionsTarget{Path: path, Import: ic.Import}
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

// DetectDir returns the install directory that identifies a target's tool,
// tilde-expanded and OS-normalised. Returns "" for unknown targets or targets
// without detect metadata (their skills path already identifies them).
func DetectDir(name string) string {
	specs, err := loadTargetSpecs()
	if err != nil {
		return ""
	}
	for _, spec := range specs {
		if spec.Name != name {
			continue
		}
		return normalizeTargetPath(spec.Detect)
	}
	return ""
}

// RuntimeScanPaths returns every path a target's runtime reads: its also_scans
// paths plus its built-in default skills path. Callers stay correct when a
// default path moves into also_scans (or back), and when the user has pinned
// the target to a different path in their config.
func RuntimeScanPaths(name string, isProject bool) []string {
	var scans []string
	var primary string
	if isProject {
		scans = AlsoScansProject(name)
		primary = ProjectTargets()[name].Path
	} else {
		scans = AlsoScansGlobal(name)
		primary = DefaultTargets()[name].Path
	}

	paths := make([]string, 0, len(scans)+1)
	seen := make(map[string]bool, len(scans)+1)
	for _, p := range scans {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	if primary != "" && !seen[primary] {
		paths = append(paths, primary)
	}
	return paths
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
	// A skill for claude also belongs in a project's claude target.
	if _, tool, ok := SplitProjectTarget(configTarget); ok {
		configTarget = tool
		if skillTarget == tool {
			return true
		}
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
		// Collect dot-dirs from skill, agent and also-scanned project paths.
		projectPaths := []string{spec.Skills.Project, spec.Agents.Project}
		projectPaths = append(projectPaths, spec.AlsoScans.Project...)
		for _, p := range projectPaths {
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

func targetConfigHome(name, goos string) string {
	switch name {
	case "deepseek-harness":
		if home := os.Getenv("DSH_HOME"); strings.TrimSpace(home) != "" {
			// Harness resolves relative overrides against the current directory.
			if path, err := filepath.Abs(expandPath(home)); err == nil {
				return path
			}
		}
	case "gitlab-duo":
		if home := os.Getenv("GLAB_CONFIG_DIR"); home != "" {
			return home
		}
		if home := os.Getenv("XDG_CONFIG_HOME"); home != "" {
			return filepath.Join(home, "gitlab", "duo")
		}
		if goos == "windows" {
			if home := os.Getenv("APPDATA"); home != "" {
				return filepath.Join(home, "GitLab", "duo")
			}
		}
	}
	return ""
}
