package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

// targetNamesFromConfig extracts the target names from a global config's
// Targets map so they can be passed to validation helpers.
func targetNamesFromConfig(targets map[string]config.TargetConfig) []string {
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	return names
}

// filterUpdateOpts holds parsed filter modification flags.
type filterUpdateOpts struct {
	AddInclude    []string
	AddExclude    []string
	RemoveInclude []string
	RemoveExclude []string
}

func (o filterUpdateOpts) hasUpdates() bool {
	return len(o.AddInclude) > 0 || len(o.AddExclude) > 0 ||
		len(o.RemoveInclude) > 0 || len(o.RemoveExclude) > 0
}

// parseFilterFlags extracts --add-include, --add-exclude, --remove-include,
// --remove-exclude flags from args.  Returns the parsed opts and any
// remaining (non-filter) arguments.
func parseFilterFlags(args []string) (filterUpdateOpts, []string, error) {
	var opts filterUpdateOpts
	var rest []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--add-include":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--add-include requires a value")
			}
			i++
			opts.AddInclude = append(opts.AddInclude, args[i])
		case "--add-exclude":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--add-exclude requires a value")
			}
			i++
			opts.AddExclude = append(opts.AddExclude, args[i])
		case "--remove-include":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--remove-include requires a value")
			}
			i++
			opts.RemoveInclude = append(opts.RemoveInclude, args[i])
		case "--remove-exclude":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--remove-exclude requires a value")
			}
			i++
			opts.RemoveExclude = append(opts.RemoveExclude, args[i])
		default:
			rest = append(rest, args[i])
		}
	}

	return opts, rest, nil
}

// applyFilterUpdates modifies include/exclude slices according to opts.
// It validates patterns with filepath.Match, deduplicates, and returns
// a human-readable list of changes applied.
func applyFilterUpdates(include, exclude *[]string, opts filterUpdateOpts) ([]string, error) {
	var changes []string

	// Validate all patterns first
	for _, p := range opts.AddInclude {
		if _, err := filepath.Match(p, ""); err != nil {
			return nil, fmt.Errorf("invalid include pattern %q: %w", p, err)
		}
	}
	for _, p := range opts.AddExclude {
		if _, err := filepath.Match(p, ""); err != nil {
			return nil, fmt.Errorf("invalid exclude pattern %q: %w", p, err)
		}
	}

	// Apply additions (deduplicated)
	for _, p := range opts.AddInclude {
		if !containsPattern(*include, p) {
			*include = append(*include, p)
			changes = append(changes, fmt.Sprintf("added include: %s", p))
		}
	}
	for _, p := range opts.AddExclude {
		if !containsPattern(*exclude, p) {
			*exclude = append(*exclude, p)
			changes = append(changes, fmt.Sprintf("added exclude: %s", p))
		}
	}

	// Apply removals
	for _, p := range opts.RemoveInclude {
		if removePattern(include, p) {
			changes = append(changes, fmt.Sprintf("removed include: %s", p))
		}
	}
	for _, p := range opts.RemoveExclude {
		if removePattern(exclude, p) {
			changes = append(changes, fmt.Sprintf("removed exclude: %s", p))
		}
	}

	return changes, nil
}

func containsPattern(patterns []string, p string) bool {
	for _, existing := range patterns {
		if existing == p {
			return true
		}
	}
	return false
}

func removePattern(patterns *[]string, p string) bool {
	for i, existing := range *patterns {
		if existing == p {
			*patterns = append((*patterns)[:i], (*patterns)[i+1:]...)
			return true
		}
	}
	return false
}

// formatFilterList formats a filter list for display, or "(none)" if empty.
func formatFilterList(patterns []string) string {
	if len(patterns) == 0 {
		return "(none)"
	}
	return strings.Join(patterns, ", ")
}

// findUnknownSkillTargets returns warnings for skills whose targets field
// references unknown target names.  Shared by check and doctor commands.
// extraTargetNames contains user-configured target names (from global or
// project config) that should be treated as known in addition to the
// built-in target list.
func findUnknownSkillTargets(discovered []ssync.DiscoveredSkill, extraTargetNames []string) []string {
	knownNames := config.KnownTargetNames()
	knownSet := make(map[string]bool, len(knownNames)+len(extraTargetNames))
	for _, n := range knownNames {
		knownSet[n] = true
	}
	for _, n := range extraTargetNames {
		knownSet[n] = true
	}

	var warnings []string
	for _, skill := range discovered {
		if skill.Targets == nil {
			continue
		}
		for _, t := range skill.Targets {
			if !knownSet[t] {
				warnings = append(warnings, fmt.Sprintf("%s: unknown target %q", skill.RelPath, t))
			}
		}
	}
	return warnings
}
