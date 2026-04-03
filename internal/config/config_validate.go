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

	// Source path validation
	if cfg.Source == "" {
		errs = append(errs, "source path is empty")
	} else {
		expanded := ExpandPath(cfg.Source)
		info, statErr := os.Stat(expanded)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				errs = append(errs, fmt.Sprintf("source path does not exist: %s", cfg.Source))
			} else {
				errs = append(errs, fmt.Sprintf("cannot access source path: %v", statErr))
			}
		} else if !info.IsDir() {
			errs = append(errs, fmt.Sprintf("source path is not a directory: %s", cfg.Source))
		}
	}

	// Global sync mode
	if !IsValidSyncMode(cfg.Mode) {
		errs = append(errs, fmt.Sprintf("invalid global sync mode %q (valid: %s)", cfg.Mode, strings.Join(ValidSyncModes, ", ")))
	}
	if !IsValidTargetNaming(cfg.TargetNaming) {
		errs = append(errs, fmt.Sprintf("invalid global target naming %q (valid: %s)", cfg.TargetNaming, strings.Join(ValidTargetNamings, ", ")))
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

	// Source path is always .skillshare/skills/ — validate it exists
	sourcePath := filepath.Join(projectRoot, ".skillshare", "skills")
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

		if sc.Path == "" {
			// Known built-in targets are resolved from targets.yaml;
			// custom targets must have an explicit path.
			if _, known := LookupProjectTarget(entry.Name); !known {
				errs = append(errs, fmt.Sprintf("target %q: missing path (custom targets require skills.path)", entry.Name))
			}
		} else {
			absPath := sc.Path
			if !filepath.IsAbs(sc.Path) {
				absPath = filepath.Join(projectRoot, filepath.FromSlash(sc.Path))
			} else {
				absPath = ExpandPath(sc.Path)
			}
			errs = append(errs, validateTargetPath(entry.Name, absPath)...)
		}
	}

	if len(errs) > 0 {
		return warnings, errors.New(strings.Join(errs, "; "))
	}
	return warnings, nil
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
