package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateConfig validates a global config semantically (after YAML parsing).
// Returns warnings (non-fatal) and error (fatal, should return 400).
func ValidateConfig(cfg *Config) (warnings []string, err error) {
	var errs []string

	// Source path validation — requires the user to explicitly configure
	// either the legacy `source:` field or the new `sources.skills:` map,
	// then verifies the resolved path exists and is a directory.
	if cfg.Source == "" && cfg.Sources.Skills == "" {
		errs = append(errs, "source path is empty")
	} else {
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

	// Per-target validation
	for name, target := range cfg.Targets {
		sc := target.SkillsConfig()
		if !IsValidSyncMode(sc.Mode) {
			errs = append(errs, fmt.Sprintf("target %q: invalid sync mode %q (valid: %s)", name, sc.Mode, strings.Join(ValidSyncModes, ", ")))
			continue
		}
		if !IsValidTargetNaming(sc.TargetNaming) {
			errs = append(errs, fmt.Sprintf("target %q: invalid target naming %q (valid: %s)", name, sc.TargetNaming, strings.Join(ValidTargetNamings, ", ")))
			continue
		}
		path := sc.Path
		if path == "" {
			// Known built-in targets get their path from targets.yaml at runtime;
			// custom targets must specify a path explicitly.
			if _, known := LookupGlobalTarget(name); !known {
				errs = append(errs, fmt.Sprintf("target %q: missing path (custom targets require skills.path)", name))
				continue
			}
		} else {
			errs = append(errs, validateTargetPath(name, ExpandPath(path))...)
		}
	}

	if len(errs) > 0 {
		return warnings, errors.New(strings.Join(errs, "; "))
	}
	return warnings, nil
}

// ValidateProjectConfig validates a project config semantically.
func ValidateProjectConfig(cfg *ProjectConfig, projectRoot string) (warnings []string, err error) {
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

	// Target validation
	if !IsValidTargetNaming(cfg.TargetNaming) {
		errs = append(errs, fmt.Sprintf("invalid project target naming %q (valid: %s)", cfg.TargetNaming, strings.Join(ValidTargetNamings, ", ")))
	}
	for _, entry := range cfg.Targets {
		sc := entry.SkillsConfig()
		if !IsValidSyncMode(sc.Mode) {
			errs = append(errs, fmt.Sprintf("target %q: invalid sync mode %q (valid: %s)", entry.Name, sc.Mode, strings.Join(ValidSyncModes, ", ")))
			continue
		}
		if !IsValidTargetNaming(sc.TargetNaming) {
			errs = append(errs, fmt.Sprintf("target %q: invalid target naming %q (valid: %s)", entry.Name, sc.TargetNaming, strings.Join(ValidTargetNamings, ", ")))
			continue
		}

		var skillsBuiltin string
		if t, ok := LookupProjectTarget(entry.Name); ok {
			skillsBuiltin = t.Path
		}
		if sc.Path == "" && skillsBuiltin == "" {
			// Known built-in targets are resolved from targets.yaml;
			// custom targets must have an explicit path.
			errs = append(errs, fmt.Sprintf("target %q: missing path (custom targets require skills.path)", entry.Name))
		} else {
			skillsTargetPath := resolveProjectTargetPath(projectRoot, sc.Path, skillsBuiltin)
			if sc.Path != "" {
				errs = append(errs, validateTargetPath(entry.Name, skillsTargetPath)...)
			}
			if skillsTargetPath != "" && pathsOverlap(sourcePath, skillsTargetPath) {
				errs = append(errs, fmt.Sprintf("target %q: skills target path %s overlaps skills source %s — sync --force could destroy the source", entry.Name, skillsTargetPath, sourcePath))
			}
		}

		ac := entry.AgentsConfig()
		var agentsBuiltin string
		if t, ok := ProjectAgentTargets()[entry.Name]; ok {
			agentsBuiltin = t.Path
		}
		agentsTargetPath := resolveProjectTargetPath(projectRoot, ac.Path, agentsBuiltin)
		if agentsTargetPath != "" && pathsOverlap(agentsSourcePath, agentsTargetPath) {
			errs = append(errs, fmt.Sprintf("target %q: agents target path %s overlaps agents source %s — sync --force could destroy the source", entry.Name, agentsTargetPath, agentsSourcePath))
		}
	}

	if len(errs) > 0 {
		return warnings, errors.New(strings.Join(errs, "; "))
	}
	return warnings, nil
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
func validateTargetPath(name, expandedPath string) []string {
	if expandedPath == "" {
		return nil // path resolved by target registry; skip filesystem check
	}

	info, statErr := os.Stat(expandedPath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return nil // sync will auto-create and notify
		}
		return []string{fmt.Sprintf("target %q: cannot access path: %v", name, statErr)}
	}
	if !info.IsDir() {
		return []string{fmt.Sprintf("target %q: path is not a directory: %s", name, expandedPath)}
	}

	return nil
}
