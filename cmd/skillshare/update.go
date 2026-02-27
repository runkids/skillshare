package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

// updateOptions holds parsed arguments for update command
type updateOptions struct {
	names        []string // positional args (0+)
	groups       []string // --group/-G values (repeatable)
	all          bool
	dryRun       bool
	force        bool
	skipAudit    bool
	diff         bool
	threshold    string
	auditVerbose bool
	prune        bool
}

// parseUpdateArgs parses command line arguments for the update command.
// Returns (opts, showHelp, error).
func parseUpdateArgs(args []string) (*updateOptions, bool, error) {
	opts := &updateOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--all" || arg == "-a":
			opts.all = true
		case arg == "--dry-run" || arg == "-n":
			opts.dryRun = true
		case arg == "--force" || arg == "-f":
			opts.force = true
		case arg == "--skip-audit":
			opts.skipAudit = true
		case arg == "--audit-threshold" || arg == "--threshold" || arg == "-T":
			i++
			if i >= len(args) {
				return nil, false, fmt.Errorf("%s requires a value", arg)
			}
			threshold, err := normalizeInstallAuditThreshold(args[i])
			if err != nil {
				return nil, false, err
			}
			opts.threshold = threshold
		case arg == "--diff":
			opts.diff = true
		case arg == "--audit-verbose":
			opts.auditVerbose = true
		case arg == "--prune":
			opts.prune = true
		case arg == "--group" || arg == "-G":
			i++
			if i >= len(args) {
				return nil, false, fmt.Errorf("--group requires a value")
			}
			opts.groups = append(opts.groups, args[i])
		case arg == "--help" || arg == "-h":
			return nil, true, nil
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown option: %s", arg)
		default:
			opts.names = append(opts.names, arg)
		}
	}

	if opts.all && (len(opts.names) > 0 || len(opts.groups) > 0) {
		return nil, false, fmt.Errorf("--all cannot be used with skill names or --group")
	}

	if len(opts.names) == 0 && len(opts.groups) == 0 && !opts.all {
		return nil, true, fmt.Errorf("specify a skill or repo name, or use --all")
	}

	return opts, false, nil
}

func cmdUpdate(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	if mode == modeProject {
		// Parse opts for logging (cmdUpdateProject parses again internally)
		projOpts, _, _ := parseUpdateArgs(rest)
		err := cmdUpdateProject(rest, cwd)
		logUpdateOp(config.ProjectConfigPath(cwd), rest, projOpts, "project", start, err, nil)
		return err
	}

	opts, showHelp, parseErr := parseUpdateArgs(rest)
	if showHelp {
		printUpdateHelp()
		return parseErr
	}
	if parseErr != nil {
		return parseErr
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if opts.threshold == "" {
		opts.threshold = cfg.Audit.BlockThreshold
	}

	ui.Header(ui.WithModeLabel("Updating"))
	ui.StepStart("Source", cfg.Source)

	// --- Resolve targets ---
	var targets []updateTarget
	seen := map[string]bool{}
	var resolveWarnings []string

	if opts.all {
		// Recursive discovery for --all
		scanSpinner := ui.StartSpinner("Scanning skills...")
		walkRoot := utils.ResolveSymlink(cfg.Source)
		err := filepath.Walk(walkRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || path == walkRoot {
				return nil
			}
			if info.IsDir() && utils.IsHidden(info.Name()) {
				return filepath.SkipDir
			}
			if info.IsDir() && info.Name() == ".git" {
				return filepath.SkipDir
			}

			// Tracked repo
			if info.IsDir() && strings.HasPrefix(info.Name(), "_") {
				if install.IsGitRepo(path) {
					rel, _ := filepath.Rel(walkRoot, path)
					if !seen[rel] {
						seen[rel] = true
						targets = append(targets, updateTarget{name: rel, path: path, isRepo: true})
					}
					return filepath.SkipDir
				}
			}

			// Regular skill
			if !info.IsDir() && info.Name() == "SKILL.md" {
				skillDir := filepath.Dir(path)
				meta, metaErr := install.ReadMeta(skillDir)
				if metaErr == nil && meta != nil && meta.Source != "" {
					rel, _ := filepath.Rel(walkRoot, skillDir)
					if rel != "." && !seen[rel] {
						seen[rel] = true
						targets = append(targets, updateTarget{name: rel, path: skillDir, isRepo: false, meta: meta})
					}
				}
			}
			return nil
		})
		scanSpinner.Stop()
		if err != nil {
			return fmt.Errorf("failed to scan skills: %w", err)
		}
	} else {
		// Resolve by specific names/groups
		for _, name := range opts.names {
			// Glob pattern matching (e.g. "core-*", "_team-?")
			if isGlobPattern(name) {
				globMatches, globErr := resolveByGlob(cfg.Source, name)
				if globErr != nil {
					resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: %v", name, globErr))
					continue
				}
				if len(globMatches) == 0 {
					resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: no skills match pattern", name))
					continue
				}
				ui.Info("Pattern '%s' matched %d item(s)", name, len(globMatches))
				for _, m := range globMatches {
					if !seen[m.name] {
						seen[m.name] = true
						targets = append(targets, m)
					}
				}
				continue
			}

			if isGroupDir(name, cfg.Source) {
				groupMatches, groupErr := resolveGroupUpdatable(name, cfg.Source)
				if groupErr != nil {
					resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: %v", name, groupErr))
					continue
				}
				if len(groupMatches) == 0 {
					resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: no updatable skills in group", name))
					continue
				}
				ui.Info("'%s' is a group — expanding to %d updatable skill(s)", name, len(groupMatches))
				for _, m := range groupMatches {
					if !seen[m.name] {
						seen[m.name] = true
						targets = append(targets, m)
					}
				}
				continue
			}

			match, err := resolveByBasename(cfg.Source, name)
			if err != nil {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: %v", name, err))
				continue
			}
			if !seen[match.name] {
				seen[match.name] = true
				targets = append(targets, match)
			}
		}

		for _, group := range opts.groups {
			groupMatches, err := resolveGroupUpdatable(group, cfg.Source)
			if err != nil {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("--group %s: %v", group, err))
				continue
			}
			if len(groupMatches) == 0 {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("--group %s: no updatable skills in group", group))
				continue
			}
			for _, m := range groupMatches {
				if !seen[m.name] {
					seen[m.name] = true
					targets = append(targets, m)
				}
			}
		}
	}

	// Count repos vs skills for summary
	var repoCount, skillCount int
	for _, t := range targets {
		if t.isRepo {
			repoCount++
		} else {
			skillCount++
		}
	}
	ui.StepEnd("Items", fmt.Sprintf("%d tracked repo(s), %d skill(s)", repoCount, skillCount))

	for _, w := range resolveWarnings {
		ui.Warning("%s", w)
	}

	if len(targets) == 0 {
		if opts.all {
			ui.UpdateSummary(ui.UpdateStats{})
			return nil
		}
		if len(resolveWarnings) > 0 {
			return fmt.Errorf("no valid skills to update")
		}
		return fmt.Errorf("no skills found")
	}

	// --- Execute ---
	uc := &updateContext{sourcePath: cfg.Source, opts: opts}

	if len(targets) == 1 {
		// Single target: verbose path
		t := targets[0]
		var updateErr error
		if t.isRepo {
			updateErr = updateTrackedRepo(uc, t.name)
		} else {
			updateErr = updateRegularSkill(uc, t.name)
		}

		if updateErr == nil && !opts.dryRun {
			discoveryCache.Invalidate(cfg.Source)
		}

		var opNames []string
		if opts.all {
			opNames = []string{"--all"}
		} else {
			opNames = opts.names
		}
		logUpdateOp(config.ConfigPath(), opNames, opts, "global", start, updateErr, nil)
		return updateErr
	}

	// Multiple targets: batch path
	if opts.dryRun {
		ui.Warning("[dry-run] No changes will be made")
	}

	batchResult, batchErr := executeBatchUpdate(uc, targets)

	if batchResult.updated > 0 || batchResult.pruned > 0 {
		discoveryCache.Invalidate(cfg.Source)
	}

	// Build oplog names
	var opNames []string
	if opts.all {
		opNames = []string{"--all"}
	} else {
		opNames = append(opNames, opts.names...)
		for _, g := range opts.groups {
			opNames = append(opNames, "--group="+g)
		}
	}
	logUpdateOp(config.ConfigPath(), opNames, opts, "global", start, batchErr, &batchResult)

	return batchErr
}

func logUpdateOp(cfgPath string, names []string, opts *updateOptions, mode string, start time.Time, cmdErr error, result *updateResult) {
	status := statusFromErr(cmdErr)
	if result != nil && result.updated > 0 && (result.securityFailed > 0 || cmdErr != nil) {
		status = "partial"
	}
	e := oplog.NewEntry("update", status, time.Since(start))
	a := map[string]any{"mode": mode}
	if opts != nil {
		if opts.all {
			a["all"] = true
		}
		if len(names) == 1 {
			a["name"] = names[0]
		} else if len(names) > 1 {
			a["names"] = names
		}
		if opts.force {
			a["force"] = true
		}
		if opts.dryRun {
			a["dry_run"] = true
		}
		if opts.skipAudit {
			a["skip_audit"] = true
		}
		if opts.threshold != "" {
			a["threshold"] = opts.threshold
		}
		if opts.diff {
			a["diff"] = true
		}
		if opts.prune {
			a["prune"] = true
		}
	} else if len(names) == 1 {
		a["name"] = names[0]
	} else if len(names) > 1 {
		a["names"] = names
	}
	if result != nil {
		if result.updated > 0 {
			a["updated"] = result.updated
		}
		if result.securityFailed > 0 {
			a["security_failed"] = result.securityFailed
		}
		if result.skipped > 0 {
			a["skipped"] = result.skipped
		}
		if result.pruned > 0 {
			a["pruned"] = result.pruned
		}
	}
	e.Args = a
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func printUpdateHelp() {
	fmt.Println(`Usage: skillshare update <name>... [options]
       skillshare update --group <group> [options]
       skillshare update --all [options]

Update one or more skills or tracked repositories.

For tracked repos (_repo-name): runs git pull
For regular skills: reinstalls from stored source metadata

If a positional name matches a group directory (not a repo or skill), it is
automatically expanded to all updatable skills in that group.

Safety: Tracked repos with uncommitted changes are skipped by default.
Use --force to discard local changes and update.

Arguments:
  name...             Skill name(s) or tracked repo name(s)
                      Supports glob patterns (e.g. "core-*", "_team-?")

Options:
  --all, -a           Update all tracked repos + skills with metadata
  --group, -G <name>  Update all updatable skills in a group (repeatable)
  --force, -f         Discard local changes and force update
  --dry-run, -n       Preview without making changes
  --skip-audit        Skip post-update security audit
  --audit-threshold, --threshold, -T <t>
                      Override update audit block threshold (critical|high|medium|low|info;
                      shorthand: c|h|m|l|i, plus crit, med)
  --diff              Show file-level change summary after update
  --audit-verbose     Show detailed per-skill audit findings in batch mode
  --prune             Remove stale skills (deleted upstream) instead of warning
  --project, -p       Use project-level config in current directory
  --global, -g        Use global config (~/.config/skillshare)
  --help, -h          Show this help

Examples:
  skillshare update my-skill              # Update single skill from source
  skillshare update a b c                 # Update multiple skills at once
  skillshare update "core-*"             # Update all matching a glob pattern
  skillshare update --group frontend      # Update all skills in frontend/
  skillshare update x -G backend          # Mix names and groups
  skillshare update _team-skills          # Update tracked repo (git pull)
  skillshare update team-skills           # _ prefix is optional for repos
  skillshare update --all                 # Update all tracked repos + skills
  skillshare update --all -T high         # Use HIGH threshold for this run
  skillshare update --all --dry-run       # Preview updates
  skillshare update _team --force         # Discard changes and update
  skillshare update --all --prune        # Update all + remove stale skills`)
}
