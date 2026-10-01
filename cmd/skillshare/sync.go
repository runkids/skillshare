package main

import (
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/hooks"
	"skillshare/internal/mcp"
	"skillshare/internal/oplog"
	"skillshare/internal/skillignore"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

type syncLogStats struct {
	Targets int
	// FailedTargets names each target whose skills or agents failed, once.
	FailedTargets []string
	DryRun        bool
	Force         bool
	ProjectScope  bool
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
	ContextCost   *contextCostJSON       `json:"context_cost,omitempty"`
	MCP           *mcp.Result            `json:"mcp,omitempty"`
	Hooks         *hooks.Result          `json:"hooks,omitempty"`
}

type syncJSONTargetDetail struct {
	Name    string `json:"name"`
	Mode    string `json:"mode"`
	Linked  int    `json:"linked"`
	Local   int    `json:"local"`
	Updated int    `json:"updated"`
	Pruned  int    `json:"pruned"`
	Error   string `json:"error,omitempty"`
	// SkillsOff is true for a target with skills switched off (nothing synced).
	SkillsOff bool `json:"skills_off,omitempty"`
}

// syncModeStats aggregates per-target sync results for UI summary.
type syncModeStats struct {
	linked, local, updated, pruned int
}

func cmdSync(args []string) error {
	if i := slices.Index(args, "plugins"); i >= 0 {
		return cmdPlugin(append([]string{"sync"}, slices.Delete(slices.Clone(args), i, i+1)...))
	}
	// "mcp" is a subcommand in any position, e.g. "sync -g --dry-run mcp".
	if i := slices.Index(args, "mcp"); i >= 0 {
		return cmdSyncMCP(slices.Delete(slices.Clone(args), i, i+1))
	}
	if i := slices.Index(args, "hooks"); i >= 0 {
		return cmdSyncHooks(slices.Delete(slices.Clone(args), i, i+1))
	}
	if wantsHelp(args) {
		printSyncHelp()
		return nil
	}

	// Subcommand: sync extras, also after flags, e.g. "sync -g extras".
	if i := slices.Index(args, "extras"); i >= 0 {
		return cmdSyncExtras(slices.Delete(slices.Clone(args), i, i+1))
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

	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	// Extract kind filter (e.g. "skillshare sync agents"). Sync flags take no
	// values, so the kind may also follow them: "sync --dry-run agents".
	if i := slices.IndexFunc(rest, func(arg string) bool {
		_, remaining := parseKindArg([]string{arg})
		return len(remaining) == 0
	}); i > 0 {
		rest = append([]string{rest[i]}, slices.Delete(slices.Clone(rest), i, i+1)...)
	}
	kind, rest := parseKindArg(rest)

	dryRun, force, jsonOutput, quiet := parseSyncFlags(rest)

	var mcpResult *mcp.Result
	var mcpService *mcp.Service
	var hooksResult *hooks.Result
	var hooksService *hooks.Service
	if hasAll {
		preflightErr := func(err error) error {
			if jsonOutput {
				return writeJSONError(err)
			}
			return err
		}
		// Loading can migrate legacy skill/extras configuration. Complete that
		// existing normalization before fingerprinting the MCP source.
		if mode == modeProject {
			_, err = config.LoadProject(cwd)
		} else {
			_, err = config.Load()
		}
		if err != nil {
			return preflightErr(err)
		}
		scope := "-g"
		if mode == modeProject {
			scope = "-p"
		}
		mcpService, _, err = mcpContext([]string{scope})
		if err != nil {
			return preflightErr(err)
		}
		plan, planErr := mcpService.Preview()
		if planErr != nil {
			return preflightErr(fmt.Errorf("MCP preflight: %w", planErr))
		}
		mcpResult = &mcp.Result{Plan: plan, Applied: []string{}, BackupIDs: []string{}}
		if plan.Blocked {
			err = fmt.Errorf("MCP conflicts found; no resources synchronized")
			if jsonOutput {
				// Changes are credential-free and tell scripts which entries conflict.
				return writeJSONResult(map[string]any{"error": err.Error(), "mcp": mcpResult}, err)
			}
			_ = printMCPPlan(plan, false)
			return err
		}
		hooksService, _, err = hooksContext([]string{scope})
		if err != nil {
			return preflightErr(err)
		}
		hooksPlan, planErr := hooksService.Preview()
		if planErr != nil {
			return preflightErr(fmt.Errorf("hooks preflight: %w", planErr))
		}
		hooksResult = &hooks.Result{Plan: hooksPlan, Applied: []string{}, BackupIDs: []string{}}
		if hooksPlan.Blocked {
			err = fmt.Errorf("hooks conflicts found; no resources synchronized")
			if jsonOutput {
				return writeJSONResult(map[string]any{"error": err.Error(), "hooks": hooksResult}, err)
			}
			printHooksPlan(hooksPlan)
			return err
		}
	}
	finishMCP := func(previous error) error {
		if previous != nil {
			// Keep MCP untouched after a resource failure; drop the preflight plan
			// so JSON output does not look processed.
			mcpResult = nil
			if !jsonOutput {
				ui.Warning("MCP settings were not applied because resource sync failed")
			}
			return previous
		}
		if len(mcpResult.Plan.Changes) == 0 && !mcpResult.Plan.Migrates {
			return nil
		}
		var applyErr error
		if !dryRun {
			// Skills sync ran after the preflight; plan again under the MCP lock so
			// unrelated config writes cannot fail the run with a stale revision.
			mcpResult, applyErr = mcpService.Apply("")
			logMCPOp(mcpService.ConfigPath, "sync mcp", start, applyErr)
		}
		if !jsonOutput && !quiet && mcpResult != nil {
			_ = printMCPPlan(mcpResult.Plan, false)
			for _, id := range mcpResult.BackupIDs {
				ui.Info("MCP backup: %s", id)
			}
			printMCPMigrated(mcpResult)
		}
		return applyErr
	}
	finishHooks := func(previous error) error {
		if previous != nil {
			hooksResult = nil
			if !jsonOutput {
				ui.Warning("Hooks were not applied because an earlier sync step failed")
			}
			return previous
		}
		if len(hooksResult.Plan.Changes) == 0 {
			return nil
		}
		var applyErr error
		if !dryRun {
			// Resources synced after the preflight: preview again and apply that revision.
			hooksResult, applyErr = applyHooksSync(hooksService, start)
		}
		if !jsonOutput && !quiet && hooksResult != nil {
			printHooksPlan(hooksResult.Plan)
			for _, id := range hooksResult.BackupIDs {
				ui.Info("Hooks backup: %s", id)
			}
		}
		return applyErr
	}
	// finishNative applies MCP, then hooks, only after every resource synced.
	finishNative := func(previous error) error {
		if !hasAll {
			return previous
		}
		return finishHooks(finishMCP(previous))
	}

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
			return syncAgentsOnlyProject(cwd, dryRun, force, jsonOutput, start)
		}

		stats, results, projIgnoreStats, projCtxCost, invalid, err := cmdSyncProject(cwd, dryRun, force, jsonOutput, quiet)
		stats.ProjectScope = true

		// Append agent sync when kind=all or --all
		if kind == kindAll || hasAll {
			agentFailed, agentErr := syncAgentsProject(cwd, invalid, dryRun, force, jsonOutput, start)
			if agentErr != nil && err == nil {
				err = agentErr
			}
			stats.FailedTargets = mergeFailedTargets(stats.FailedTargets, agentFailed)
		}
		logSyncOp(config.ProjectConfigPath(cwd), stats, start, err)

		if jsonOutput {
			err = finishNative(err)
			if hasAll {
				projCfg, loadErr := config.LoadProject(cwd)
				if loadErr == nil && len(projCfg.Extras) > 0 {
					agentPaths := collectAgentTargetPathsProject(cwd)
					extrasEntries, extrasErr := runExtrasSyncEntries(projCfg.Extras, func(extra config.ExtraConfig) string {
						return config.ResolveExtrasSourceDirProject(extra, projCfg.EffectiveExtrasSource(cwd), cwd)
					}, dryRun, force, cwd, agentPaths)
					if err == nil {
						err = extrasErr
					}
					return syncOutputJSON(results, dryRun, start, projIgnoreStats, err, projCtxCost, mcpResult, hooksResult, extrasEntries)
				}
			}
			return syncOutputJSON(results, dryRun, start, projIgnoreStats, err, projCtxCost, mcpResult, hooksResult)
		}
		err = finishNative(err)
		if hasAll {
			// Run project extras sync after project skills sync (text mode)
			if extrasErr := cmdSyncExtras(append([]string{"-p"}, rest...)); extrasErr != nil {
				ui.Warning("Extras sync: %v", extrasErr)
				if err == nil {
					err = extrasErr
				}
			}
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

	// Validate config before sync. A target with invalid settings fails alone:
	// it is reported as failed and skipped, and the other targets still sync.
	warnings, invalid, validErr := config.ValidateConfigForSync(cfg)
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

	syncCfg := withoutTargets(cfg, invalid)

	// Agent-only mode: skip skill discovery/sync entirely
	if kind == kindAgents {
		invalidNames := slices.Sorted(maps.Keys(invalid))
		if !jsonOutput {
			for _, name := range invalidNames {
				ui.Error("%s: %s", name, invalidConfigMessage(invalid[name]))
			}
		}
		agentStats, agentErr := syncAgentsGlobal(syncCfg, dryRun, force, jsonOutput, start)
		if agentErr == nil && len(invalidNames) > 0 {
			agentErr = fmt.Errorf("some targets failed to sync")
		}
		logSyncOp(config.ConfigPath(), syncLogStats{
			Targets:       len(cfg.Targets),
			FailedTargets: mergeFailedTargets(agentStats.failed, invalidNames),
			DryRun:        dryRun,
			Force:         force,
		}, start, agentErr)
		return agentErr
	}

	// Phase 1: Discovery (skills)
	var spinner *ui.Spinner
	if !jsonOutput {
		spinner = ui.StartSpinner("Discovering skills")
	}
	discoveredSkills, ignoreStats, discoverErr := sync.DiscoverSourceSkillsWithStatsAndContext(cfg.EffectiveSkillsSource())
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
		backupTargetsBeforeSync(syncCfg)
	}

	// Phase 2: Per-target sync (parallel)
	if !jsonOutput {
		ui.Header("Syncing skills")
		if dryRun {
			ui.Warning("Dry run mode - no changes will be made")
		}
		for _, root := range cfg.MissingProjects() {
			ui.Warning("project %s: folder not found, skipped", root)
		}
	}

	var entries []syncTargetEntry
	for name, target := range cfg.Targets {
		entries = append(entries, syncTargetEntry{name: name, target: target, mode: getTargetMode(target.SkillsConfig().Mode, cfg.Mode), configErr: invalid[name]})
	}

	var results []syncTargetResult
	var failedTargets int
	if jsonOutput {
		results, failedTargets = runParallelSyncQuiet(entries, cfg.EffectiveSkillsSource(), discoveredSkills, sync.EffectiveFileIgnorePatterns(cfg.Ignore), dryRun, force, "")
	} else {
		results, failedTargets = runParallelSync(entries, cfg.EffectiveSkillsSource(), discoveredSkills, sync.EffectiveFileIgnorePatterns(cfg.Ignore), dryRun, force, "")
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

		// One-liner path-overlap hint — points users at `doctor` for details.
		printSyncOverlapHint(cfg.Targets, false, jsonOutput)

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

	// Compute token cost once — used by both text summary and JSON output
	analyzeEntries, analyzeErr := buildAnalyzeEntries(discoveredSkills, syncCfg.Targets, cfg.Mode, cfg.EffectiveSkillsSource(), "")

	if !jsonOutput && !quiet && analyzeErr == nil && len(analyzeEntries) > 0 {
		printTokenSummary(analyzeEntries)
		if violations := checkBudget(analyzeEntries, cfg.ContextBudget); len(violations) > 0 {
			printBudgetWarning(violations, true)
		}
	}

	logStats := syncLogStats{
		Targets:       len(cfg.Targets),
		FailedTargets: failedSkillTargets(results),
		DryRun:        dryRun,
		Force:         force,
	}

	// Agents are included in --all in both human-readable and JSON output.
	if kind == kindAll || hasAll {
		agentStats, agentErr := syncAgentsGlobal(syncCfg, dryRun, force, jsonOutput, start)
		if agentErr != nil && syncErr == nil {
			syncErr = agentErr
		}
		logStats.FailedTargets = mergeFailedTargets(logStats.FailedTargets, agentStats.failed)
	}
	logSyncOp(config.ConfigPath(), logStats, start, syncErr)

	if jsonOutput {
		syncErr = finishNative(syncErr)
		var ctxCost *contextCostJSON
		if analyzeErr == nil && len(analyzeEntries) > 0 {
			ctxCost = buildContextCostJSON(analyzeEntries, cfg.ContextBudget)
		}
		if hasAll && len(cfg.Extras) > 0 {
			agentPaths := collectAgentTargetPathsGlobal(cfg)
			extrasEntries, extrasErr := runExtrasSyncEntries(cfg.Extras, func(extra config.ExtraConfig) string {
				return config.ResolveExtrasSourceDir(extra, cfg.EffectiveExtrasSource(), cfg.EffectiveSkillsSource())
			}, dryRun, force, "", agentPaths)
			if syncErr == nil {
				syncErr = extrasErr
			}
			return syncOutputJSON(results, dryRun, start, ignoreStats, syncErr, ctxCost, mcpResult, hooksResult, extrasEntries)
		}
		return syncOutputJSON(results, dryRun, start, ignoreStats, syncErr, ctxCost, mcpResult, hooksResult)
	}

	var extrasErr error
	if hasAll {
		if extrasErr = cmdSyncExtras(append([]string{"-g"}, rest...)); extrasErr != nil {
			ui.Warning("Extras sync: %v", extrasErr)
		}
	}

	// An extras failure fails the run but, as in JSON mode, does not hold back MCP or hooks.
	if err := finishNative(syncErr); err != nil {
		return err
	}
	return extrasErr
}

func parseSyncFlags(args []string) (dryRun, force, jsonOutput, quiet bool) {
	for _, arg := range args {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--force", "-f":
			force = true
		case "--json":
			jsonOutput = true
		case "--quiet", "-q":
			quiet = true
		}
	}
	return dryRun, force, jsonOutput, quiet
}

func logSyncOp(cfgPath string, stats syncLogStats, start time.Time, cmdErr error) {
	failed := len(stats.FailedTargets)
	status := statusFromErr(cmdErr)
	if failed > 0 && failed < stats.Targets {
		status = "partial"
	}
	e := oplog.NewEntry("sync", status, time.Since(start))
	e.Args = map[string]any{
		"targets_total":  stats.Targets,
		"targets_failed": failed,
		"dry_run":        stats.DryRun,
		"force":          stats.Force,
		"scope":          "global",
	}
	if failed > 0 {
		e.Args["failed_targets"] = stats.FailedTargets
	}
	if stats.ProjectScope {
		e.Args["scope"] = "project"
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

// withoutTargets returns cfg, or a copy of it without the targets in skip.
func withoutTargets(cfg *config.Config, skip map[string]error) *config.Config {
	if len(skip) == 0 {
		return cfg
	}
	c := *cfg
	c.Targets = maps.Clone(cfg.Targets)
	for name := range skip {
		delete(c.Targets, name)
	}
	return &c
}

// invalidConfigMessage is the failure shown for a target with invalid settings.
func invalidConfigMessage(err error) string {
	return "invalid config: " + err.Error()
}

// failedSkillTargets returns the targets whose skills sync failed.
func failedSkillTargets(results []syncTargetResult) []string {
	var names []string
	for _, r := range results {
		if r.errMsg != "" {
			names = append(names, r.name)
		}
	}
	return mergeFailedTargets(names)
}

// mergeFailedTargets returns the distinct target names of lists, sorted.
func mergeFailedTargets(lists ...[]string) []string {
	names := slices.Concat(lists...)
	slices.Sort(names)
	return slices.Compact(names)
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
func syncOutputJSON(results []syncTargetResult, dryRun bool, start time.Time, iStats *skillignore.IgnoreStats, syncErr error, ctxCost *contextCostJSON, mcpResult *mcp.Result, hooksResult *hooks.Result, extras ...[]syncExtrasJSONEntry) error {
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

			SkillsOff: r.skillsOff,
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
	if len(extras) > 0 && extras[0] != nil {
		output.Extras = extras[0]
	}
	output.ContextCost = ctxCost
	output.MCP = mcpResult
	output.Hooks = hooksResult
	return writeJSONResult(&output, syncErr)
}

func backupTargetsBeforeSync(cfg *config.Config) {
	// Pre-sync backups are automatic, so retention must be too — otherwise
	// every sync adds a snapshot that nothing ever removes.
	defer func() {
		if _, err := backup.Cleanup(backup.RetentionConfig(cfg)); err != nil {
			ui.Warning("Failed to clean up old backups: %v", err)
		}
	}()

	backedUp := false
	for name, target := range cfg.Targets {
		if !target.SkillsConfig().IsEnabled() {
			continue
		}
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
	if err != nil {
		ui.Warning("Failed to resolve agent backup targets: %v", err)
		return
	}
	if len(agentTargets) == 0 {
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

func printSyncHelp() {
	fmt.Println(`Usage: skillshare sync [agents|extras|mcp|hooks|plugins] [options]

Sync skills from source to all configured targets.

Options:
  --all             Sync skills, agents, extras, MCP and hooks
  --dry-run, -n     Preview changes without applying
  --force, -f       Force sync (overwrite local changes)
  --json            Output results as JSON
  --quiet, -q       Suppress token summary and budget warnings
  --project, -p     Use project-level config
  --global, -g      Use global config
  --help, -h        Show this help

Subcommands:
  agents            Sync only agents
  plugins [name]    Apply plugin sync selection (see: skillshare plugin --help)
  mcp               Sync only MCP settings (no --force; conflicts require review)
  hooks             Sync only hooks (same as skillshare hooks sync)
  extras            Sync only extras (see: skillshare sync extras --help)

Examples:
  skillshare sync                Sync skills to all targets
  skillshare sync --dry-run      Preview sync changes
  skillshare sync --all          Sync skills, agents, extras, MCP and hooks
  skillshare sync -p             Sync project-level skills
  skillshare sync agents         Sync agents only
  skillshare sync plugins        Apply selected plugin installations/removals
  skillshare sync plugins --dry-run --json   Preview plugin changes

Plugins are not included in --all. Plugin sync uses its own options;
--force and --quiet do not apply. Enable/disable saves selection only;
run sync plugins to install selected targets or uninstall deselected targets.`)
}
