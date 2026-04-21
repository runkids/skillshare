package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	hookpkg "skillshare/internal/hooks"
	"skillshare/internal/oplog"
	pluginpkg "skillshare/internal/plugins"
	"skillshare/internal/skillignore"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

type syncLogStats struct {
	Targets      int
	Failed       int
	DryRun       bool
	Force        bool
	ProjectScope bool
}

// syncJSONOutput is the JSON representation for sync --json output.
type syncJSONOutput struct {
	Targets       int                    `json:"targets"`
	Linked        int                    `json:"linked"`
	Local         int                    `json:"local"`
	Updated       int                    `json:"updated"`
	Pruned        int                    `json:"pruned"`
	IgnoredCount  int                    `json:"ignored_count"`
	IgnoredSkills []string               `json:"ignored_skills"`
	DryRun        bool                   `json:"dry_run"`
	Duration      string                 `json:"duration"`
	Details       []syncJSONTargetDetail `json:"details"`
	Extras        []syncExtrasJSONEntry  `json:"extras,omitempty"`
	Plugins       []pluginpkg.SyncResult `json:"plugins,omitempty"`
	Hooks         []hookpkg.SyncResult   `json:"hooks,omitempty"`
}

type syncJSONTargetDetail struct {
	Name    string `json:"name"`
	Mode    string `json:"mode"`
	Linked  int    `json:"linked"`
	Local   int    `json:"local"`
	Updated int    `json:"updated"`
	Pruned  int    `json:"pruned"`
	Error   string `json:"error,omitempty"`
}

// syncModeStats aggregates per-target sync results for UI summary.
type syncModeStats struct {
	linked, local, updated, pruned int
}

func cmdSync(args []string) error {
	if wantsHelp(args) {
		printSyncHelp()
		return nil
	}

	// Subcommand: sync extras
	if len(args) > 0 && args[0] == "extras" {
		return cmdSyncExtras(args[1:])
	}

	// Extract --all flag before mode parsing
	hasAll := false
	var filteredArgs []string
	for _, a := range args {
		if a == "--all" {
			hasAll = true
		} else {
			filteredArgs = append(filteredArgs, a)
		}
	}
	args = filteredArgs

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

	// Extract kind filter (e.g. "skillshare sync agents").
	kind, rest := parseKindArg(rest)

	dryRun, force, jsonOutput := parseSyncFlags(rest)

	prevDiagOutput := sync.DiagOutput
	if jsonOutput {
		sync.DiagOutput = io.Discard
		defer func() {
			sync.DiagOutput = prevDiagOutput
		}()
	}

	if mode == modeProject {
		// Agent-only project sync
		if kind == kindAgents {
			return syncAgentsProject(cwd, dryRun, force, jsonOutput, start)
		}

		if hasAll && !jsonOutput {
			// Run project extras sync after project skills sync (text mode)
			defer func() {
				if extrasErr := cmdSyncExtras(append([]string{"-p"}, rest...)); extrasErr != nil {
					ui.Warning("Extras sync: %v", extrasErr)
				}
				if pluginErr := cmdPluginsSync([]string{"-p", "--target", "all"}); pluginErr != nil {
					ui.Warning("Plugins sync: %v", pluginErr)
				}
				if hooksErr := cmdHooksSync([]string{"-p", "--target", "all"}); hooksErr != nil {
					ui.Warning("Hooks sync: %v", hooksErr)
				}
			}()
		}

		stats, results, projIgnoreStats, err := cmdSyncProject(cwd, dryRun, force, jsonOutput)
		stats.ProjectScope = true
		logSyncOp(config.ProjectConfigPath(cwd), stats, start, err)

		// Append agent sync when kind=all or --all
		if kind == kindAll || hasAll {
			if agentErr := syncAgentsProject(cwd, dryRun, force, jsonOutput, start); agentErr != nil && err == nil {
				err = agentErr
			}
		}

		if jsonOutput {
			var extrasEntries []syncExtrasJSONEntry
			var pluginResults []pluginpkg.SyncResult
			var hookResults []hookpkg.SyncResult
			if hasAll {
				projCfg, loadErr := config.LoadProject(cwd)
				if loadErr == nil && len(projCfg.Extras) > 0 {
					agentPaths := collectAgentTargetPathsProject(cwd)
					extrasEntries = runExtrasSyncEntries(projCfg.Extras, func(extra config.ExtraConfig) string {
						return config.ExtrasSourceDirProject(cwd, extra.Name)
					}, dryRun, force, cwd, agentPaths)
				}
				pluginRoot, projectRoot, pluginRootErr := pluginRoots(modeProject)
				if pluginRootErr == nil {
					pluginResults, pluginRootErr = pluginpkg.SyncAll(pluginRoot, projectRoot, "all", true)
				}
				if pluginRootErr != nil && err == nil {
					err = pluginRootErr
				}
				hookRoot, hookProjectRoot, hookRootErr := hookRoots(modeProject)
				if hookRootErr == nil {
					hookResults, hookRootErr = hookpkg.SyncAll(hookRoot, hookProjectRoot, "all")
				}
				if hookRootErr != nil && err == nil {
					err = hookRootErr
				}
			}
			return syncOutputJSON(results, dryRun, start, projIgnoreStats, err, extrasEntries, pluginResults, hookResults)
		}
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		if jsonOutput {
			return writeJSONError(err)
		}
		return err
	}

	// Validate config before sync
	warnings, validErr := config.ValidateConfig(cfg)
	if validErr != nil {
		if jsonOutput {
			return writeJSONError(validErr)
		}
		return validErr
	}
	if !jsonOutput {
		for _, w := range warnings {
			ui.Warning("%s", w)
		}
	}

	// Agent-only mode: skip skill discovery/sync entirely
	if kind == kindAgents {
		_, agentErr := syncAgentsGlobal(cfg, dryRun, force, jsonOutput, start)
		logSyncOp(config.ConfigPath(), syncLogStats{DryRun: dryRun, Force: force}, start, agentErr)
		return agentErr
	}

	// Phase 1: Discovery (skills)
	var spinner *ui.Spinner
	if !jsonOutput {
		spinner = ui.StartSpinner("Discovering skills")
	}
	discoveredSkills, ignoreStats, discoverErr := sync.DiscoverSourceSkillsWithStats(cfg.Source)
	if discoverErr != nil {
		if spinner != nil {
			spinner.Fail("Discovery failed")
		}
		if jsonOutput {
			return writeJSONError(discoverErr)
		}
		return discoverErr
	}
	if spinner != nil {
		spinner.Success(fmt.Sprintf("Discovered %d skills", len(discoveredSkills)))
		reportCollisions(discoveredSkills, cfg.Targets)
	}

	// Backup targets before sync (only if not dry-run and there are skills)
	if !dryRun && len(discoveredSkills) > 0 && !jsonOutput {
		backupTargetsBeforeSync(cfg)
	}

	// Phase 2: Per-target sync (parallel)
	if !jsonOutput {
		ui.Header("Syncing skills")
		if dryRun {
			ui.Warning("Dry run mode - no changes will be made")
		}
	}

	var entries []syncTargetEntry
	for name, target := range cfg.Targets {
		entries = append(entries, syncTargetEntry{name: name, target: target, mode: getTargetMode(target.SkillsConfig().Mode, cfg.Mode)})
	}

	var results []syncTargetResult
	var failedTargets int
	if jsonOutput {
		results, failedTargets = runParallelSyncQuiet(entries, cfg.Source, discoveredSkills, dryRun, force, "")
	} else {
		results, failedTargets = runParallelSync(entries, cfg.Source, discoveredSkills, dryRun, force, "")
	}

	var syncErr error
	if failedTargets > 0 {
		syncErr = fmt.Errorf("some targets failed to sync")
	}

	if !jsonOutput {
		// Phase 3: Summary
		var totals syncModeStats
		for _, r := range results {
			totals.linked += r.stats.linked
			totals.local += r.stats.local
			totals.updated += r.stats.updated
			totals.pruned += r.stats.pruned
		}
		ui.SyncSummary(ui.SyncStats{
			Targets:  len(cfg.Targets),
			Linked:   totals.linked,
			Local:    totals.local,
			Updated:  totals.updated,
			Pruned:   totals.pruned,
			Duration: time.Since(start),
		})

		// Show ignored skills from .skillignore
		printIgnoredSkills(ignoreStats)

		// Opportunistic cleanup of expired trash items
		if !dryRun {
			if n, _ := trash.Cleanup(trash.TrashDir(), 0); n > 0 {
				ui.Info("Cleaned up %d expired trash item(s)", n)
			}
		}
	}

	// Registry entries are managed by install/uninstall, not sync.
	// Sync only manages symlinks — it must not prune registry entries
	// for installed skills whose files may be missing from disk.

	logSyncOp(config.ConfigPath(), syncLogStats{
		Targets: len(cfg.Targets),
		Failed:  failedTargets,
		DryRun:  dryRun,
		Force:   force,
	}, start, syncErr)

	if jsonOutput {
		var extrasEntries []syncExtrasJSONEntry
		var pluginResults []pluginpkg.SyncResult
		var hookResults []hookpkg.SyncResult
		if hasAll && len(cfg.Extras) > 0 {
			agentPaths := collectAgentTargetPathsGlobal(cfg)
			extrasEntries = runExtrasSyncEntries(cfg.Extras, func(extra config.ExtraConfig) string {
				return config.ResolveExtrasSourceDir(extra, cfg.ExtrasSource, cfg.Source)
			}, dryRun, force, "", agentPaths)
		}
		if hasAll {
			pluginRoot, _, pErr := pluginRoots(modeGlobal)
			if pErr == nil {
				pluginResults, pErr = pluginpkg.SyncAll(pluginRoot, "", "all", true)
			}
			if pErr != nil && syncErr == nil {
				syncErr = pErr
			}
			hookRoot, _, hErr := hookRoots(modeGlobal)
			if hErr == nil {
				hookResults, hErr = hookpkg.SyncAll(hookRoot, "", "all")
			}
			if hErr != nil && syncErr == nil {
				syncErr = hErr
			}
		}
		return syncOutputJSON(results, dryRun, start, ignoreStats, syncErr, extrasEntries, pluginResults, hookResults)
	}

	// Agent sync when kind=all or --all (after skill sync)
	if kind == kindAll || hasAll {
		if _, agentErr := syncAgentsGlobal(cfg, dryRun, force, jsonOutput, start); agentErr != nil && syncErr == nil {
			syncErr = agentErr
		}
	}

	if hasAll {
		if extrasErr := cmdSyncExtras(append([]string{"-g"}, rest...)); extrasErr != nil {
			ui.Warning("Extras sync: %v", extrasErr)
		}
		if pluginErr := cmdPluginsSync([]string{"-g", "--target", "all"}); pluginErr != nil {
			ui.Warning("Plugins sync: %v", pluginErr)
		}
		if hooksErr := cmdHooksSync([]string{"-g", "--target", "all"}); hooksErr != nil {
			ui.Warning("Hooks sync: %v", hooksErr)
		}
	}

	return syncErr
}

func parseSyncFlags(args []string) (dryRun, force, jsonOutput bool) {
	for _, arg := range args {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--force", "-f":
			force = true
		case "--json":
			jsonOutput = true
		}
	}
	return dryRun, force, jsonOutput
}

func logSyncOp(cfgPath string, stats syncLogStats, start time.Time, cmdErr error) {
	status := statusFromErr(cmdErr)
	if stats.Failed > 0 && stats.Failed < stats.Targets {
		status = "partial"
	}
	e := oplog.NewEntry("sync", status, time.Since(start))
	e.Args = map[string]any{
		"targets_total":  stats.Targets,
		"targets_failed": stats.Failed,
		"dry_run":        stats.DryRun,
		"force":          stats.Force,
		"scope":          "global",
	}
	if stats.ProjectScope {
		e.Args["scope"] = "project"
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

// printIgnoredSkills prints the list of .skillignore-excluded skills with source hints.
func printIgnoredSkills(stats *skillignore.IgnoreStats) {
	if stats == nil || stats.IgnoredCount() == 0 {
		return
	}
	fmt.Println()
	fmt.Printf(ui.Dim+"%d skill(s) ignored by .skillignore:"+ui.Reset+"\n", stats.IgnoredCount())
	for _, name := range stats.IgnoredSkills {
		fmt.Printf(ui.Dim+"  • %s"+ui.Reset+"\n", name)
	}
	// Show source hint
	hasRoot := stats.RootFile != ""
	hasRootLocal := stats.RootLocalFile != ""
	hasRepo := len(stats.RepoFiles) > 0
	hasRepoLocal := len(stats.RepoLocalFiles) > 0
	repoPlural := "file"
	if len(stats.RepoFiles) > 1 {
		repoPlural = "files"
	}

	// Build source hint parts
	var parts []string
	if hasRoot {
		rootHint := "root .skillignore"
		if hasRootLocal {
			rootHint += " + .local"
		}
		parts = append(parts, rootHint)
	} else if hasRootLocal {
		parts = append(parts, "root .skillignore.local")
	}
	if hasRepo {
		repoHint := fmt.Sprintf("%d repo-level %s", len(stats.RepoFiles), repoPlural)
		if hasRepoLocal {
			repoHint += " + .local"
		}
		parts = append(parts, repoHint)
	} else if hasRepoLocal {
		parts = append(parts, fmt.Sprintf("%d repo-level .skillignore.local", len(stats.RepoLocalFiles)))
	}
	if len(parts) > 0 {
		fmt.Printf(ui.Dim+"  (from %s)"+ui.Reset+"\n", strings.Join(parts, " + "))
	}
}

// syncOutputJSON converts sync results to JSON and writes to stdout.
// extras is optional and included when --all is used.
func syncOutputJSON(results []syncTargetResult, dryRun bool, start time.Time, iStats *skillignore.IgnoreStats, syncErr error, extras []syncExtrasJSONEntry, plugins []pluginpkg.SyncResult, hooks []hookpkg.SyncResult) error {
	var totals syncModeStats
	var details []syncJSONTargetDetail
	for _, r := range results {
		totals.linked += r.stats.linked
		totals.local += r.stats.local
		totals.updated += r.stats.updated
		totals.pruned += r.stats.pruned
		details = append(details, syncJSONTargetDetail{
			Name:    r.name,
			Mode:    r.mode,
			Linked:  r.stats.linked,
			Local:   r.stats.local,
			Updated: r.stats.updated,
			Pruned:  r.stats.pruned,
			Error:   r.errMsg,
		})
	}
	output := syncJSONOutput{
		Targets:  len(results),
		Linked:   totals.linked,
		Local:    totals.local,
		Updated:  totals.updated,
		Pruned:   totals.pruned,
		DryRun:   dryRun,
		Duration: formatDuration(start),
		Details:  details,
	}
	ignoredSkills := []string{}
	if iStats != nil && len(iStats.IgnoredSkills) > 0 {
		ignoredSkills = iStats.IgnoredSkills
	}
	output.IgnoredCount = len(ignoredSkills)
	output.IgnoredSkills = ignoredSkills
	output.Extras = extras
	output.Plugins = plugins
	output.Hooks = hooks
	return writeJSONResult(&output, syncErr)
}

func backupTargetsBeforeSync(cfg *config.Config) {
	backedUp := false
	for name, target := range cfg.Targets {
		backupPath, err := backup.Create(name, target.SkillsConfig().Path)
		if err != nil {
			ui.Warning("Failed to backup %s: %v", name, err)
		} else if backupPath != "" {
			if !backedUp {
				ui.Header("Backing up")
				backedUp = true
			}
			ui.Success("%s -> %s", name, backupPath)
		}
	}

	// Also backup agent targets if any exist.
	backupDir, agentTargets, err := resolveGlobalAgentBackupContextFromCfg(cfg)
	if err != nil || len(agentTargets) == 0 {
		return
	}
	for _, at := range agentTargets {
		entryName := at.name + "-agents"
		bp, bErr := backup.CreateInDir(backupDir, entryName, at.agentPath)
		if bErr != nil {
			ui.Warning("Failed to backup %s: %v", entryName, bErr)
		} else if bp != "" {
			if !backedUp {
				ui.Header("Backing up")
				backedUp = true
			}
			ui.Success("%s -> %s", entryName, bp)
		}
	}
}

func syncTarget(name string, target config.TargetConfig, cfg *config.Config, dryRun, force bool) error {
	sc := target.SkillsConfig()
	// Determine mode: target-specific > global > default
	mode := sc.Mode
	if mode == "" {
		mode = cfg.Mode
	}
	if mode == "" {
		mode = "merge"
	}

	switch mode {
	case "merge":
		return syncMergeMode(name, target, cfg.Source, dryRun, force)
	case "copy":
		return syncCopyMode(name, target, cfg.Source, dryRun, force)
	default:
		return syncSymlinkMode(name, target, cfg.Source, dryRun, force)
	}
}

func syncTargetWithSkills(name string, target config.TargetConfig, cfg *config.Config, skills []sync.DiscoveredSkill, dryRun, force bool) error {
	_, err := syncTargetWithSkillsStats(name, target, cfg, skills, dryRun, force)
	return err
}

func syncTargetWithSkillsStats(name string, target config.TargetConfig, cfg *config.Config, skills []sync.DiscoveredSkill, dryRun, force bool) (syncModeStats, error) {
	sc := target.SkillsConfig()
	mode := sc.Mode
	if mode == "" {
		mode = cfg.Mode
	}
	if mode == "" {
		mode = "merge"
	}

	switch mode {
	case "merge":
		return syncMergeModeWithSkills(name, target, cfg.Source, skills, dryRun, force)
	case "copy":
		return syncCopyModeWithSkills(name, target, cfg.Source, skills, dryRun, force)
	default:
		err := syncSymlinkMode(name, target, cfg.Source, dryRun, force)
		return syncModeStats{}, err
	}
}

func syncMergeMode(name string, target config.TargetConfig, source string, dryRun, force bool) error {
	sc := target.SkillsConfig()
	result, err := sync.SyncTargetMerge(name, target, source, dryRun, force, "")
	if err != nil {
		return err
	}

	pruneResult, pruneErr := sync.PruneOrphanLinks(sc.Path, source, sc.Include, sc.Exclude, name, sc.TargetNaming, dryRun, force)
	if pruneErr != nil {
		ui.Warning("%s: prune failed: %v", name, pruneErr)
	}

	reportMergeResult(name, target, result, pruneResult, dryRun)
	return nil
}

func syncMergeModeWithSkills(name string, target config.TargetConfig, source string, skills []sync.DiscoveredSkill, dryRun, force bool) (syncModeStats, error) {
	sc := target.SkillsConfig()
	result, err := sync.SyncTargetMergeWithSkills(name, target, skills, source, dryRun, force, "")
	if err != nil {
		return syncModeStats{}, err
	}

	pruneResult, pruneErr := sync.PruneOrphanLinksWithSkills(sync.PruneOptions{
		TargetPath: sc.Path, SourcePath: source, Skills: skills,
		Include: sc.Include, Exclude: sc.Exclude, TargetNaming: sc.TargetNaming, TargetName: name,
		DryRun: dryRun, Force: force,
	})
	if pruneErr != nil {
		ui.Warning("%s: prune failed: %v", name, pruneErr)
	}

	reportMergeResult(name, target, result, pruneResult, dryRun)
	return mergeStats(result, pruneResult), nil
}

func reportMergeResult(name string, target config.TargetConfig, result *sync.MergeResult, pruneResult *sync.PruneResult, dryRun bool) {
	sc := target.SkillsConfig()
	linkedCount := len(result.Linked)
	updatedCount := len(result.Updated)
	skippedCount := len(result.Skipped)
	removedCount := 0
	if pruneResult != nil {
		removedCount = len(pruneResult.Removed)
		skippedCount += len(pruneResult.LocalDirs)
	}

	if linkedCount > 0 || updatedCount > 0 || removedCount > 0 {
		ui.Success("%s: merged (%d linked, %d local, %d updated, %d pruned)",
			name, linkedCount, skippedCount, updatedCount, removedCount)
	} else if skippedCount > 0 {
		ui.Success("%s: merged (%d local skills preserved)", name, skippedCount)
	} else {
		ui.Success("%s: merged (no skills)", name)
	}

	if len(sc.Include) > 0 {
		ui.Info("  include: %s", strings.Join(sc.Include, ", "))
	}
	if len(sc.Exclude) > 0 {
		ui.Info("  exclude: %s", strings.Join(sc.Exclude, ", "))
	}

	if pruneResult != nil {
		for _, warn := range pruneResult.Warnings {
			ui.Warning("  %s", warn)
		}
	}

	if result.DirCreated != "" {
		verb := "Created"
		if dryRun {
			verb = "Will create"
		}
		ui.Info("  %s target directory: %s", verb, result.DirCreated)
	}
}

func syncCopyMode(name string, target config.TargetConfig, source string, dryRun, force bool) error {
	sc := target.SkillsConfig()
	result, err := sync.SyncTargetCopy(name, target, source, dryRun, force)
	if err != nil {
		return err
	}

	pruneResult, pruneErr := sync.PruneOrphanCopies(sc.Path, source, sc.Include, sc.Exclude, name, sc.TargetNaming, dryRun)
	if pruneErr != nil {
		ui.Warning("%s: prune failed: %v", name, pruneErr)
	}

	reportCopyResult(name, target, result, pruneResult, dryRun)
	return nil
}

func syncCopyModeWithSkills(name string, target config.TargetConfig, source string, skills []sync.DiscoveredSkill, dryRun, force bool) (syncModeStats, error) {
	// Copy mode is slow (checksum + file copy per skill) — show a spinner with progress
	spinner := ui.StartSpinner(fmt.Sprintf("%s: copying skills", name))
	onProgress := func(cur, total int, skill string) {
		spinner.Update(fmt.Sprintf("%s: %d/%d %s", name, cur, total, skill))
	}

	result, err := sync.SyncTargetCopyWithSkills(name, target, skills, source, dryRun, force, onProgress)
	if err != nil {
		spinner.Fail(fmt.Sprintf("%s: copy failed", name))
		return syncModeStats{}, err
	}
	spinner.Stop()

	sc := target.SkillsConfig()
	pruneResult, pruneErr := sync.PruneOrphanCopiesWithSkills(sc.Path, skills, sc.Include, sc.Exclude, name, sc.TargetNaming, dryRun)
	if pruneErr != nil {
		ui.Warning("%s: prune failed: %v", name, pruneErr)
	}

	reportCopyResult(name, target, result, pruneResult, dryRun)
	return copyStats(result, pruneResult), nil
}

func mergeStats(result *sync.MergeResult, prune *sync.PruneResult) syncModeStats {
	s := syncModeStats{
		linked:  len(result.Linked),
		local:   len(result.Skipped),
		updated: len(result.Updated),
	}
	if prune != nil {
		s.pruned = len(prune.Removed)
		s.local += len(prune.LocalDirs)
	}
	return s
}

func copyStats(result *sync.CopyResult, prune *sync.PruneResult) syncModeStats {
	s := syncModeStats{
		linked:  len(result.Copied),
		local:   len(result.Skipped),
		updated: len(result.Updated),
	}
	if prune != nil {
		s.pruned = len(prune.Removed)
	}
	return s
}

func reportCopyResult(name string, target config.TargetConfig, result *sync.CopyResult, pruneResult *sync.PruneResult, dryRun bool) {
	sc := target.SkillsConfig()
	copiedCount := len(result.Copied)
	updatedCount := len(result.Updated)
	skippedCount := len(result.Skipped)
	removedCount := 0
	if pruneResult != nil {
		removedCount = len(pruneResult.Removed)
	}

	if copiedCount > 0 || updatedCount > 0 || removedCount > 0 {
		ui.Success("%s: copied (%d new, %d skipped, %d updated, %d pruned)",
			name, copiedCount, skippedCount, updatedCount, removedCount)
	} else if skippedCount > 0 {
		ui.Success("%s: copied (%d skipped, up to date)", name, skippedCount)
	} else {
		ui.Success("%s: copied (no skills)", name)
	}

	if len(sc.Include) > 0 {
		ui.Info("  include: %s", strings.Join(sc.Include, ", "))
	}
	if len(sc.Exclude) > 0 {
		ui.Info("  exclude: %s", strings.Join(sc.Exclude, ", "))
	}

	if pruneResult != nil {
		for _, warn := range pruneResult.Warnings {
			ui.Warning("  %s", warn)
		}
	}

	if result.DirCreated != "" {
		verb := "Created"
		if dryRun {
			verb = "Will create"
		}
		ui.Info("  %s target directory: %s", verb, result.DirCreated)
	}
}

func reportCollisions(skills []sync.DiscoveredSkill, targets map[string]config.TargetConfig) {
	global, perTarget := sync.CheckNameCollisionsForTargets(skills, targets)
	if len(global) == 0 {
		return
	}

	if len(perTarget) > 0 {
		// Deduplicate collisions across targets: group by skill name, collect affected targets
		type collisionInfo struct {
			Paths   []string
			Targets []string
		}
		deduped := make(map[string]*collisionInfo)
		var orderedNames []string
		var targetNames []string
		seenTargets := make(map[string]bool)

		for _, tc := range perTarget {
			if !seenTargets[tc.TargetName] {
				seenTargets[tc.TargetName] = true
				targetNames = append(targetNames, tc.TargetName)
			}
			for _, c := range tc.Collisions {
				if info, ok := deduped[c.Name]; ok {
					info.Targets = append(info.Targets, tc.TargetName)
				} else {
					deduped[c.Name] = &collisionInfo{
						Paths:   c.Paths,
						Targets: []string{tc.TargetName},
					}
					orderedNames = append(orderedNames, c.Name)
				}
			}
		}

		ui.Header("Name conflicts detected")

		// Summary line
		if len(targetNames) == len(seenTargets) && len(seenTargets) > 1 {
			ui.Warning("%d duplicate skill names affect %d targets (%s)",
				len(deduped), len(targetNames), strings.Join(targetNames, ", "))
		} else {
			ui.Warning("%d duplicate skill names detected", len(deduped))
		}

		// One entry per collision name
		for _, name := range orderedNames {
			info := deduped[name]
			// Show only parent directories for brevity (e.g., "skillshare/, skillshare2/")
			dirs := make([]string, 0, len(info.Paths))
			for _, p := range info.Paths {
				parts := strings.SplitN(p, "/", 2)
				if len(parts) > 0 {
					dirs = append(dirs, parts[0]+"/")
				}
			}
			ui.Info("  %-30s  %s", name, strings.Join(dirs, " vs "))
		}

		fmt.Println()
		ui.Info("Rename one in SKILL.md or adjust include/exclude filters")
		fmt.Println()
	} else {
		// Global collision exists but filters isolate them — show first few names
		const maxShow = 5
		names := make([]string, 0, maxShow)
		for i, c := range global {
			if i >= maxShow {
				break
			}
			names = append(names, c.Name)
		}
		fmt.Println()
		if len(global) <= maxShow {
			ui.Info("%d duplicate skill names (isolated by target filters): %s", len(global), strings.Join(names, ", "))
		} else {
			ui.Info("%d duplicate skill names (isolated by target filters): %s, ... and %d more", len(global), strings.Join(names, ", "), len(global)-maxShow)
		}
	}
}

func syncSymlinkMode(name string, target config.TargetConfig, source string, dryRun, force bool) error {
	sc := target.SkillsConfig()
	status := sync.CheckStatus(sc.Path, source)

	// Handle conflicts
	if status == sync.StatusConflict && !force {
		link, err := utils.ResolveLinkTarget(sc.Path)
		if err != nil {
			link = "(unable to resolve target)"
		}
		return fmt.Errorf("conflict - symlink points to %s (use --force to override)", link)
	}

	if status == sync.StatusConflict && force {
		if !dryRun {
			os.Remove(sc.Path)
		}
	}

	if err := sync.SyncTarget(name, target, source, dryRun, ""); err != nil {
		return err
	}

	switch status {
	case sync.StatusLinked:
		ui.Success("%s: already linked", name)
	case sync.StatusNotExist:
		ui.Success("%s: symlink created", name)
		ui.Warning("  Symlink mode: deleting files in %s will delete from source!", sc.Path)
		ui.Info("  Use 'skillshare target remove %s' to safely unlink", name)
	case sync.StatusHasFiles:
		ui.Success("%s: files migrated and linked", name)
		ui.Warning("  Symlink mode: deleting files in %s will delete from source!", sc.Path)
		ui.Info("  Use 'skillshare target remove %s' to safely unlink", name)
	case sync.StatusBroken:
		ui.Success("%s: broken link fixed", name)
	case sync.StatusConflict:
		ui.Success("%s: conflict resolved (forced)", name)
	}

	return nil
}

func printSyncHelp() {
	fmt.Println(`Usage: skillshare sync [agents] [options]

Sync skills from source to all configured targets.

Options:
  --all             Sync skills, agents, extras, plugins, and hooks
  --dry-run, -n     Preview changes without applying
  --force, -f       Force sync (overwrite local changes)
  --json            Output results as JSON
  --project, -p     Use project-level config
  --global, -g      Use global config
  --help, -h        Show this help

Subcommands:
  extras            Sync only extras (see: skillshare sync extras --help)

Examples:
  skillshare sync                Sync skills to all targets
  skillshare sync --dry-run      Preview sync changes
  skillshare sync --all          Sync skills, agents, extras, plugins, and hooks
  skillshare sync -p             Sync project-level skills
  skillshare sync agents         Sync agents only`)
}
