package main

import (
	"fmt"
	"path/filepath"
	"strconv"
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

type parsedTargetFilterFlags struct {
	Skills filterUpdateOpts
	Agents filterUpdateOpts
}

func (o parsedTargetFilterFlags) hasUpdates() bool {
	return o.Skills.hasUpdates() || o.Agents.hasUpdates()
}

type parsedTargetSettingFlags struct {
	SkillMode string
	AgentMode string
	Naming    string
	// Skills is set by --skills=true|false; nil leaves skills sync unchanged.
	Skills *bool
	DryRun bool
}

// parseFilterFlags extracts --add-include, --add-exclude, --remove-include,
// --remove-exclude flags from args for both skills and agents.
// Returns the parsed opts and any remaining (non-filter) arguments.
func parseFilterFlags(args []string) (parsedTargetFilterFlags, []string, error) {
	var opts parsedTargetFilterFlags
	var rest []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--add-include":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--add-include requires a value")
			}
			i++
			opts.Skills.AddInclude = append(opts.Skills.AddInclude, args[i])
		case "--add-exclude":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--add-exclude requires a value")
			}
			i++
			opts.Skills.AddExclude = append(opts.Skills.AddExclude, args[i])
		case "--remove-include":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--remove-include requires a value")
			}
			i++
			opts.Skills.RemoveInclude = append(opts.Skills.RemoveInclude, args[i])
		case "--remove-exclude":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--remove-exclude requires a value")
			}
			i++
			opts.Skills.RemoveExclude = append(opts.Skills.RemoveExclude, args[i])
		case "--add-agent-include":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--add-agent-include requires a value")
			}
			i++
			opts.Agents.AddInclude = append(opts.Agents.AddInclude, args[i])
		case "--add-agent-exclude":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--add-agent-exclude requires a value")
			}
			i++
			opts.Agents.AddExclude = append(opts.Agents.AddExclude, args[i])
		case "--remove-agent-include":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--remove-agent-include requires a value")
			}
			i++
			opts.Agents.RemoveInclude = append(opts.Agents.RemoveInclude, args[i])
		case "--remove-agent-exclude":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--remove-agent-exclude requires a value")
			}
			i++
			opts.Agents.RemoveExclude = append(opts.Agents.RemoveExclude, args[i])
		default:
			rest = append(rest, args[i])
		}
	}

	return opts, rest, nil
}

// settingValue returns the value after args[i]. A following flag counts as
// missing: parseModeArgs hands over the token after a value flag, so
// "--target-naming -p" arrives here with -p as its apparent value.
func settingValue(args []string, i int, flag, hint string) (string, error) {
	if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
		return "", fmt.Errorf("%s requires a value (%s)", flag, hint)
	}
	return args[i+1], nil
}

func parseTargetSettingFlags(args []string) (parsedTargetSettingFlags, error) {
	var settings parsedTargetSettingFlags

	for i := 0; i < len(args); i++ {
		var err error
		switch args[i] {
		case "--mode", "-m":
			settings.SkillMode, err = settingValue(args, i, "--mode", "merge, symlink, or copy")
			i++
		case "--agent-mode":
			settings.AgentMode, err = settingValue(args, i, "--agent-mode", "merge, symlink, or copy")
			i++
		case "--target-naming":
			settings.Naming, err = settingValue(args, i, "--target-naming", "flat, standard, or prefixed")
			i++
		case "--skills":
			var value string
			if value, err = settingValue(args, i, "--skills", "true or false"); err == nil {
				err = setSkillsFlag(&settings, value)
			}
			i++
		case "--dry-run", "-n":
			settings.DryRun = true
		default:
			if value, ok := strings.CutPrefix(args[i], "--skills="); ok {
				err = setSkillsFlag(&settings, value)
			}
		}
		if err != nil {
			return settings, err
		}
	}

	return settings, nil
}

// changesTarget reports whether any of --mode, --agent-mode or --target-naming is given.
func (s parsedTargetSettingFlags) changesTarget() bool {
	return s.SkillMode != "" || s.AgentMode != "" || s.Naming != ""
}

// checkTargetFlagCombination refuses flag groups the dispatch cannot apply
// together, so none is dropped silently: filters, --skills and the
// mode/naming settings each run as their own command.
func checkTargetFlagCombination(settings parsedTargetSettingFlags, filters parsedTargetFilterFlags) error {
	if settings.Skills != nil && filters.hasUpdates() {
		return fmt.Errorf("--skills/--no-skills cannot be combined with include/exclude flags; run them as separate commands")
	}
	if settings.DryRun && settings.Skills == nil {
		return fmt.Errorf("--dry-run only previews --skills=false; it is not supported with other target flags")
	}
	if settings.changesTarget() && (settings.Skills != nil || filters.hasUpdates()) {
		return fmt.Errorf("--mode/--agent-mode/--target-naming cannot be combined with include/exclude flags or --skills; run them as separate commands")
	}
	return nil
}

func setSkillsFlag(settings *parsedTargetSettingFlags, value string) error {
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("--skills must be true or false, got %q", value)
	}
	settings.Skills = &enabled
	return nil
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

func scopeFilterChanges(scope string, changes []string) []string {
	if scope != "agents" {
		return changes
	}

	scoped := make([]string, len(changes))
	for i, change := range changes {
		switch {
		case strings.HasPrefix(change, "added include: "):
			scoped[i] = strings.Replace(change, "added include: ", "added agent include: ", 1)
		case strings.HasPrefix(change, "added exclude: "):
			scoped[i] = strings.Replace(change, "added exclude: ", "added agent exclude: ", 1)
		case strings.HasPrefix(change, "removed include: "):
			scoped[i] = strings.Replace(change, "removed include: ", "removed agent include: ", 1)
		case strings.HasPrefix(change, "removed exclude: "):
			scoped[i] = strings.Replace(change, "removed exclude: ", "removed agent exclude: ", 1)
		default:
			scoped[i] = change
		}
	}
	return scoped
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
