package config

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// errAgentsEnabled rejects enabled under agents: only skills can be switched off.
const errAgentsEnabled = "agents.enabled is not supported (only skills.enabled)"

// ValidateConfig validates a global config semantically (after YAML parsing).
// Returns warnings (non-fatal) and error (fatal, should return 400).
func ValidateConfig(cfg *Config) (warnings []string, err error) {
	warnings, invalid, err := ValidateConfigForSync(cfg)
	// A raw config (the dashboard's config save) has not expanded projects into targets yet.
	return warnings, errors.Join(joinTargetErrors(err, invalid), cfg.ProjectNamingError(cfg.Projects, cfg.Mode))
}

// ValidateConfigForSync validates a global config for sync, which runs every
// target it can: invalid maps each target whose own settings are wrong to its
// problems, and err holds the problems that stop the whole sync.
func ValidateConfigForSync(cfg *Config) (warnings []string, invalid map[string]error, err error) {
	var errs []string

	// Source path validation checks the effective source, including the default
	// <BaseDir>/skills fallback when source/sources.skills are omitted.
	sourcePath := cfg.EffectiveSkillsSource()
	info, statErr := os.Stat(sourcePath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			errs = append(errs, fmt.Sprintf("source path does not exist: %s", sourcePath))
		} else {
			errs = append(errs, fmt.Sprintf("cannot access source path: %v", statErr))
		}
	} else if !info.IsDir() {
		errs = append(errs, fmt.Sprintf("source path is not a directory: %s", sourcePath))
	}

	// Global sync mode
	if !IsValidSyncMode(cfg.Mode) {
		errs = append(errs, fmt.Sprintf("invalid global sync mode %q (valid: %s)", cfg.Mode, strings.Join(ValidSyncModes, ", ")))
	}
	if !IsValidTargetNaming(cfg.TargetNaming) {
		errs = append(errs, fmt.Sprintf("invalid global target naming %q (valid: %s)", cfg.TargetNaming, strings.Join(ValidTargetNamings, ", ")))
	}

	// git_root scope keyword. An unknown value silently falls back to the skills
	// scope in ScopeDir, so commit/push/pull would operate on the wrong repo
	// without warning — reject it here instead.
	if !ValidGitRoot(cfg.GitRoot) {
		errs = append(errs, fmt.Sprintf("invalid git_root %q (valid: %s)", cfg.GitRoot, strings.Join(ValidGitRoots, ", ")))
	}

	// A target that is another config directory of an Agent gets its paths from there.
	if err := cfg.expandAgentConfigDirs(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := cfg.ValidateExtras(); err != nil {
		errs = append(errs, err.Error())
	}

	invalid = map[string]error{}
	for name, target := range cfg.Targets {
		if problems := validateGlobalTarget(name, target, cfg.Mode, cfg.TargetNaming); len(problems) > 0 {
			invalid[name] = errors.New(strings.Join(problems, "; "))
		}
	}

	if len(errs) > 0 {
		err = errors.New(strings.Join(errs, "; "))
	}
	return warnings, invalid, err
}

// validateGlobalTarget returns the problems of one global target's settings.
// globalMode and globalNaming are the defaults the target inherits.
func validateGlobalTarget(name string, target TargetConfig, globalMode, globalNaming string) []string {
	var problems []string
	if err := ValidateTargetInstructions(target.Instructions, false); err != nil {
		problems = append(problems, err.Error())
	}
	if target.Agents != nil && target.Agents.Enabled != nil {
		problems = append(problems, errAgentsEnabled)
	}
	sc := target.SkillsConfig()
	if !IsValidSyncMode(sc.Mode) {
		return append(problems, fmt.Sprintf("invalid sync mode %q (valid: %s)", sc.Mode, strings.Join(ValidSyncModes, ", ")))
	}
	if !IsValidTargetNaming(sc.TargetNaming) {
		return append(problems, fmt.Sprintf("invalid target naming %q (valid: %s)", sc.TargetNaming, strings.Join(ValidTargetNamings, ", ")))
	}
	naming, mode := cmp.Or(sc.TargetNaming, globalNaming), cmp.Or(sc.Mode, globalMode)
	// Skills off syncs no skill, and the target's agents must still sync.
	if err := TargetNamingModeConfigError(naming, mode, target.ProjectRoot()); err != nil && sc.IsEnabled() {
		problems = append(problems, err.Error())
	}
	if sc.Path == "" {
		// Known built-in targets get their path from targets.yaml at runtime;
		// custom targets must specify a path explicitly.
		if _, known := LookupGlobalTarget(name); !known && target.Agent == "" {
			problems = append(problems, "missing path (custom targets require skills.path)")
		}
	} else if err := validateTargetPath(ExpandPath(sc.Path)); err != nil {
		problems = append(problems, err.Error())
	}
	return problems
}

// joinTargetErrors folds the per-target problems into err, as one error that
// names each target, for callers that reject the whole config.
func joinTargetErrors(err error, invalid map[string]error) error {
	var errs []string
	if err != nil {
		errs = append(errs, err.Error())
	}
	for _, name := range slices.Sorted(maps.Keys(invalid)) {
		errs = append(errs, fmt.Sprintf("target %q: %v", name, invalid[name]))
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "; "))
}

// ValidateProjectConfig validates a project config semantically.
func ValidateProjectConfig(cfg *ProjectConfig, projectRoot string) (warnings []string, err error) {
	warnings, invalid, err := ValidateProjectConfigForSync(cfg, projectRoot)
	return warnings, joinTargetErrors(err, invalid)
}

// ValidateProjectConfigForSync is ValidateConfigForSync for a project config.
func ValidateProjectConfigForSync(cfg *ProjectConfig, projectRoot string) (warnings []string, invalid map[string]error, err error) {
	var errs []string

	sourcePath := cfg.EffectiveSkillsSource(projectRoot)
	agentsSourcePath := cfg.EffectiveAgentsSource(projectRoot)
	if info, statErr := os.Stat(sourcePath); statErr != nil {
		if os.IsNotExist(statErr) {
			// For project mode, missing source is a warning not an error —
			// it gets created by init/install flows.
			warnings = append(warnings, fmt.Sprintf("source directory does not exist yet: %s", sourcePath))
		} else {
			errs = append(errs, fmt.Sprintf("cannot access source path: %v", statErr))
		}
	} else if !info.IsDir() {
		errs = append(errs, fmt.Sprintf("source path is not a directory: %s", sourcePath))
	}

	if !IsValidTargetNaming(cfg.TargetNaming) {
		errs = append(errs, fmt.Sprintf("invalid project target naming %q (valid: %s)", cfg.TargetNaming, strings.Join(ValidTargetNamings, ", ")))
	}

	if err := cfg.ValidateExtras(projectRoot); err != nil {
		errs = append(errs, err.Error())
	}

	problems := map[string][]string{}
	for _, entry := range cfg.Targets {
		entry.defaultTargetNaming = cfg.TargetNaming // a raw config has not been through LoadProject
		problems[entry.Name] = append(problems[entry.Name], validateProjectTarget(entry, projectRoot, sourcePath, agentsSourcePath)...)
	}
	invalid = map[string]error{}
	for name, p := range problems {
		if len(p) > 0 {
			invalid[name] = errors.New(strings.Join(p, "; "))
		}
	}

	if len(errs) > 0 {
		err = errors.New(strings.Join(errs, "; "))
	}
	return warnings, invalid, err
}

// validateProjectTarget returns the problems of one project target's settings.
func validateProjectTarget(entry ProjectTargetEntry, projectRoot, sourcePath, agentsSourcePath string) []string {
	var problems []string
	if err := ValidateTargetInstructions(entry.Instructions, true); err != nil {
		problems = append(problems, err.Error())
	}
	if entry.Agents != nil && entry.Agents.Enabled != nil {
		problems = append(problems, errAgentsEnabled)
	}
	sc := entry.SkillsConfig()
	if !IsValidSyncMode(sc.Mode) {
		return append(problems, fmt.Sprintf("invalid sync mode %q (valid: %s)", sc.Mode, strings.Join(ValidSyncModes, ", ")))
	}
	if !IsValidTargetNaming(sc.TargetNaming) {
		return append(problems, fmt.Sprintf("invalid target naming %q (valid: %s)", sc.TargetNaming, strings.Join(ValidTargetNamings, ", ")))
	}
	if err := TargetNamingModeConfigError(sc.TargetNaming, sc.Mode, ""); err != nil && sc.IsEnabled() {
		problems = append(problems, err.Error())
	}

	var skillsBuiltin string
	if t, ok := LookupProjectTarget(entry.Name); ok {
		skillsBuiltin = t.Path
	}
	if sc.Path == "" && skillsBuiltin == "" {
		// Known built-in targets are resolved from targets.yaml;
		// custom targets must have an explicit path.
		problems = append(problems, "missing path (custom targets require skills.path)")
	} else {
		skillsTargetPath := resolveProjectTargetPath(projectRoot, sc.Path, skillsBuiltin)
		if sc.Path != "" {
			if err := validateTargetPath(skillsTargetPath); err != nil {
				problems = append(problems, err.Error())
			}
		}
		if skillsTargetPath != "" && pathsOverlap(sourcePath, skillsTargetPath) {
			problems = append(problems, fmt.Sprintf("skills target path %s overlaps skills source %s — sync --force could destroy the source", skillsTargetPath, sourcePath))
		}
	}

	ac := entry.AgentsConfig()
	var agentsBuiltin string
	if t, ok := LookupProjectAgentTarget(entry.Name); ok {
		agentsBuiltin = t.Path
	}
	agentsTargetPath := resolveProjectTargetPath(projectRoot, ac.Path, agentsBuiltin)
	if agentsTargetPath != "" && pathsOverlap(agentsSourcePath, agentsTargetPath) {
		problems = append(problems, fmt.Sprintf("agents target path %s overlaps agents source %s — sync --force could destroy the source", agentsTargetPath, agentsSourcePath))
	}
	return problems
}

// ValidateTargetInstructions checks a user-set instruction file: a file path,
// absolute or ~/ in global mode, relative to the project root in project mode.
// A nil config is valid.
func ValidateTargetInstructions(ic *TargetInstructionsConfig, project bool) error {
	if ic == nil {
		return nil
	}
	path := strings.TrimSpace(ic.Path)
	switch {
	case path == "":
		return errors.New("instructions.path is empty")
	case strings.HasSuffix(path, "/") || strings.HasSuffix(path, `\`):
		return fmt.Errorf("instructions.path %q must name a file, not a directory", path)
	case project && (filepath.IsAbs(path) || strings.HasPrefix(path, "~")):
		return fmt.Errorf("instructions.path %q must be relative to the project root", path)
	case !project && !filepath.IsAbs(path) && !strings.HasPrefix(path, "~/"):
		return fmt.Errorf("instructions.path %q must be absolute or start with ~/", path)
	}
	return nil
}

// resolveProjectTargetPath returns an absolute path for a project target.
// Uses configPath if non-empty, else builtinDefault. Returns "" if both empty.
func resolveProjectTargetPath(projectRoot, configPath, builtinDefault string) string {
	path := strings.TrimSpace(configPath)
	if path == "" {
		path = strings.TrimSpace(builtinDefault)
	}
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return ExpandPath(path)
	}
	return filepath.Join(projectRoot, filepath.FromSlash(path))
}

// pathsOverlap returns true when a and b refer to the same directory or one
// contains the other. Both inputs must be absolute. Used to reject configs
// where a source directory aliases a sync target, which sync --force could
// wipe.
func pathsOverlap(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if a == b {
		return true
	}
	upPrefix := ".." + string(filepath.Separator)
	if rel, err := filepath.Rel(a, b); err == nil && rel != ".." && !strings.HasPrefix(rel, upPrefix) {
		return true
	}
	if rel, err := filepath.Rel(b, a); err == nil && rel != ".." && !strings.HasPrefix(rel, upPrefix) {
		return true
	}
	return false
}

// validateTargetPath checks a single target's path is accessible and is a directory.
// Missing paths are accepted — sync will auto-create them with a visible notification.
func validateTargetPath(expandedPath string) error {
	if expandedPath == "" {
		return nil // path resolved by target registry; skip filesystem check
	}

	info, statErr := os.Stat(expandedPath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return nil // sync will auto-create and notify
		}
		return fmt.Errorf("cannot access path: %v", statErr)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", expandedPath)
	}

	return nil
}
