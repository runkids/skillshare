package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"skillshare/internal/config"
)

// validateConfigForSync keeps sync strict about config semantics while allowing
// resource-aware partial execution. Skills-only syncs should still fail fast on
// unusable source/target paths; managed-resource syncs can continue and report
// partial failures later in the workflow.
func validateConfigForSync(cfg *config.Config, resources resourceSelection) ([]string, error) {
	var errs []string

	requireSourcePath := resources.skills && !resources.includesManaged()
	requireTargetPathAccess := resources.skills && !resources.includesManaged()

	if cfg.Source == "" {
		errs = append(errs, "source path is empty")
	} else if requireSourcePath {
		expanded := config.ExpandPath(cfg.Source)
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

	if !config.IsValidSyncMode(cfg.Mode) {
		errs = append(errs, fmt.Sprintf("invalid global sync mode %q (valid: %s)", cfg.Mode, strings.Join(config.ValidSyncModes, ", ")))
	}
	if !config.IsValidTargetNaming(cfg.TargetNaming) {
		errs = append(errs, fmt.Sprintf("invalid global target naming %q (valid: %s)", cfg.TargetNaming, strings.Join(config.ValidTargetNamings, ", ")))
	}

	for name, target := range cfg.Targets {
		sc := target.SkillsConfig()
		if !config.IsValidSyncMode(sc.Mode) {
			errs = append(errs, fmt.Sprintf("target %q: invalid sync mode %q (valid: %s)", name, sc.Mode, strings.Join(config.ValidSyncModes, ", ")))
			continue
		}
		if !config.IsValidTargetNaming(sc.TargetNaming) {
			errs = append(errs, fmt.Sprintf("target %q: invalid target naming %q (valid: %s)", name, sc.TargetNaming, strings.Join(config.ValidTargetNamings, ", ")))
			continue
		}

		if sc.Path == "" {
			if _, known := config.LookupGlobalTarget(name); !known {
				errs = append(errs, fmt.Sprintf("target %q: missing path (custom targets require skills.path)", name))
			}
			continue
		}

		if !requireTargetPathAccess {
			continue
		}

		expanded := config.ExpandPath(sc.Path)
		info, statErr := os.Stat(expanded)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			errs = append(errs, fmt.Sprintf("target %q: cannot access path: %v", name, statErr))
			continue
		}
		if !info.IsDir() {
			errs = append(errs, fmt.Sprintf("target %q: path is not a directory: %s", name, expanded))
		}
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}
	return nil, nil
}
