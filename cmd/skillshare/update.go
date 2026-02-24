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
		err := cmdUpdateProject(rest, cwd)
		logUpdateOp(config.ProjectConfigPath(cwd), rest, start, err)
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

	ui.Header("Global Update")
	ui.Info("Storage    %s", cfg.Source)
	fmt.Println()

	// --- Resolve targets ---
	var targets []resolvedMatch
	seen := map[string]bool{}
	var resolveWarnings []string

	if opts.all {
		// Recursive discovery for --all
		err := filepath.Walk(cfg.Source, func(path string, info os.FileInfo, err error) error {
			if err != nil || path == cfg.Source {
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
					rel, _ := filepath.Rel(cfg.Source, path)
					if !seen[rel] {
						seen[rel] = true
						targets = append(targets, resolvedMatch{relPath: rel, isRepo: true})
					}
					return filepath.SkipDir
				}
			}

			// Regular skill
			if !info.IsDir() && info.Name() == "SKILL.md" {
				skillDir := filepath.Dir(path)
				meta, metaErr := install.ReadMeta(skillDir)
				if metaErr == nil && meta != nil && meta.Source != "" {
					rel, _ := filepath.Rel(cfg.Source, skillDir)
					if rel != "." && !seen[rel] {
						seen[rel] = true
						targets = append(targets, resolvedMatch{relPath: rel, isRepo: false})
					}
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to scan skills: %w", err)
		}
	} else {
		// Resolve by specific names/groups
		for _, name := range opts.names {
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
					if !seen[m.relPath] {
						seen[m.relPath] = true
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
			if !seen[match.relPath] {
				seen[match.relPath] = true
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
				if !seen[m.relPath] {
					seen[m.relPath] = true
					targets = append(targets, m)
				}
			}
		}
	}

	for _, w := range resolveWarnings {
		ui.Warning("%s", w)
	}

	if len(targets) == 0 {
		if opts.all {
			ui.Header("Updating 0 skill(s)")
			fmt.Println()
			ui.SuccessMsg("Updated 0, skipped 0 of 0 skill(s)")
			return nil
		}
		if len(resolveWarnings) > 0 {
			return fmt.Errorf("no valid skills to update")
		}
		return fmt.Errorf("no skills found")
	}

	// --- Execute ---
	if len(targets) == 1 {
		// Single target: verbose path
		t := targets[0]
		var updateErr error
		if t.isRepo {
			updateErr = updateTrackedRepo(cfg, t.relPath, opts.dryRun, opts.force, opts.skipAudit, opts.diff, opts.threshold)
		} else {
			updateErr = updateRegularSkill(cfg, t.relPath, opts.dryRun, opts.force, opts.skipAudit, opts.diff, opts.threshold, opts.auditVerbose)
		}

		var opNames []string
		if opts.all {
			opNames = []string{"--all"}
		} else {
			opNames = opts.names
		}
		logUpdateOp(config.ConfigPath(), opNames, start, updateErr)
		return updateErr
	}

	// Multiple targets: batch path with progress bar
	total := len(targets)
	ui.Header(fmt.Sprintf("Updating %d skill(s)", total))
	fmt.Println()

	if opts.dryRun {
		ui.Warning("[dry-run] No changes will be made")
	}

	progressBar := ui.StartProgress("Updating skills", total)

	var result updateResult
	var auditEntries []batchAuditEntry
	var blockedEntries []batchBlockedEntry

	// Group skills by RepoURL to optimize updates
	repoGroups := make(map[string][]resolvedMatch)
	var standaloneSkills []resolvedMatch
	var trackedRepos []resolvedMatch

	for _, t := range targets {
		if t.isRepo {
			trackedRepos = append(trackedRepos, t)
			continue
		}

		itemPath := filepath.Join(cfg.Source, t.relPath)
		meta, err := install.ReadMeta(itemPath)
		if err == nil && meta != nil && meta.RepoURL != "" {
			repoGroups[meta.RepoURL] = append(repoGroups[meta.RepoURL], t)
		} else {
			standaloneSkills = append(standaloneSkills, t)
		}
	}

	// 1. Process tracked repositories (git pull)
	for _, t := range trackedRepos {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.relPath))
		itemPath := filepath.Join(cfg.Source, t.relPath)
		updated, auditResult, err := updateTrackedRepoQuick(t.relPath, itemPath, opts.dryRun, opts.force, opts.skipAudit, opts.threshold)
		if err != nil {
			if isSecurityError(err) {
				result.securityFailed++
				blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.relPath, errMsg: err.Error()})
			} else {
				result.skipped++
			}
		} else if updated {
			result.updated++
		} else {
			result.skipped++
		}
		if auditResult != nil {
			auditEntries = append(auditEntries, batchAuditEntryFromAuditResult(t.relPath, auditResult, opts.skipAudit))
		}
		progressBar.Increment()
	}

	// 2. Process grouped skills (one clone per repo)
	for repoURL, groupTargets := range repoGroups {
		if opts.dryRun {
			for _, t := range groupTargets {
				progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.relPath))
				progressBar.Increment()
				result.skipped++
			}
			continue
		}

		progressBar.UpdateTitle(fmt.Sprintf("Updating %d skills from %s", len(groupTargets), repoURL))

		// Map repo-internal subdir → local absolute path
		skillTargetMap := make(map[string]string)
		pathToTarget := make(map[string]resolvedMatch)
		for _, t := range groupTargets {
			itemPath := filepath.Join(cfg.Source, t.relPath)
			meta, _ := install.ReadMeta(itemPath)
			if meta != nil {
				skillTargetMap[meta.Subdir] = itemPath
				pathToTarget[meta.Subdir] = t
			}
		}

		batchOpts := install.InstallOptions{
			Force:          true,
			Update:         true,
			SkipAudit:      opts.skipAudit,
			AuditThreshold: opts.threshold,
		}
		if ui.IsTTY() {
			batchOpts.OnProgress = func(line string) {
				progressBar.UpdateTitle(line)
			}
		}

		batchResult, err := install.UpdateSkillsFromRepo(repoURL, skillTargetMap, batchOpts)
		if err != nil {
			for _, t := range groupTargets {
				progressBar.UpdateTitle(fmt.Sprintf("Failed %s: %v", t.relPath, err))
				result.skipped++
				progressBar.Increment()
			}
			continue
		}

		for subdir := range skillTargetMap {
			t := pathToTarget[subdir]
			progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.relPath))

			if ui.IsTTY() {
				time.Sleep(50 * time.Millisecond)
			}

			if err := batchResult.Errors[subdir]; err != nil {
				if isSecurityError(err) {
					result.securityFailed++
					blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.relPath, errMsg: err.Error()})
				} else {
					result.skipped++
				}
			} else if res := batchResult.Results[subdir]; res != nil {
				result.updated++
				auditEntries = append(auditEntries, batchAuditEntryFromInstallResult(t.relPath, res))
			} else {
				result.skipped++
			}
			progressBar.Increment()
		}
	}

	// 3. Process standalone skills
	for _, t := range standaloneSkills {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.relPath))
		itemPath := filepath.Join(cfg.Source, t.relPath)
		updated, installRes, err := updateSkillFromMeta(t.relPath, itemPath, opts.dryRun, opts.skipAudit, opts.threshold)
		if err != nil && isSecurityError(err) {
			result.securityFailed++
			blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.relPath, errMsg: err.Error()})
		} else if updated {
			result.updated++
		} else {
			result.skipped++
		}
		if installRes != nil {
			auditEntries = append(auditEntries, batchAuditEntryFromInstallResult(t.relPath, installRes))
		}
		progressBar.Increment()
	}

	progressBar.Stop()

	if !opts.dryRun {
		displayUpdateBlockedSection(blockedEntries)
		displayUpdateAuditResults(auditEntries, opts.auditVerbose)
		fmt.Println()
		ui.SuccessMsg("Updated %d, skipped %d of %d skill(s)", result.updated, result.skipped, total)
		if result.securityFailed > 0 {
			ui.Warning("Blocked: %d (security)", result.securityFailed)
		}
	}

	if result.updated > 0 && !opts.dryRun {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to distribute changes")
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

	var batchErr error
	if result.securityFailed > 0 {
		batchErr = fmt.Errorf("%d repo(s) blocked by security audit", result.securityFailed)
	}
	logUpdateOp(config.ConfigPath(), opNames, start, batchErr)

	return batchErr
}

func logUpdateOp(cfgPath string, args []string, start time.Time, cmdErr error) {
	e := oplog.NewEntry("update", statusFromErr(cmdErr), time.Since(start))
	if len(args) == 1 {
		e.Args = map[string]any{"name": args[0]}
	} else if len(args) > 1 {
		e.Args = map[string]any{"names": args}
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.Write(cfgPath, oplog.OpsFile, e) //nolint:errcheck
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
  --project, -p       Use project-level config in current directory
  --global, -g        Use global config (~/.config/skillshare)
  --help, -h          Show this help

Examples:
  skillshare update my-skill              # Update single skill from source
  skillshare update a b c                 # Update multiple skills at once
  skillshare update --group frontend      # Update all skills in frontend/
  skillshare update x -G backend          # Mix names and groups
  skillshare update _team-skills          # Update tracked repo (git pull)
  skillshare update team-skills           # _ prefix is optional for repos
  skillshare update --all                 # Update all tracked repos + skills
  skillshare update --all -T high         # Use HIGH threshold for this run
  skillshare update --all --dry-run       # Preview updates
  skillshare update _team --force         # Discard changes and update`)
}
