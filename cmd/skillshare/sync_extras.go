package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
)

// extrasAgentsName is the extras entry name that may overlap with the agents sync system.
const extrasAgentsName = "agents"

type syncExtrasJSONOutput struct {
	Extras   []syncExtrasJSONEntry `json:"extras"`
	Duration string                `json:"duration"`
}

type syncExtrasJSONEntry struct {
	Name    string                 `json:"name"`
	Targets []syncExtrasJSONTarget `json:"targets"`
}

type syncExtrasJSONTarget struct {
	Path      string   `json:"path"`
	Mode      string   `json:"mode"`
	Synced    int      `json:"synced"`
	Skipped   int      `json:"skipped"`
	Pruned    int      `json:"pruned"`
	Error     string   `json:"error,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	SkippedBy string   `json:"skipped_by,omitempty"`
}

func cmdSyncExtras(args []string) error {
	return syncExtras(args, false)
}

// syncExtras runs `sync extras`. As part of `sync --all` (partOfAll) it says
// nothing when no extras are configured.
func syncExtras(args []string, partOfAll bool) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	dryRun, force, jsonOutput, _ := parseSyncFlags(rest)

	cwd, _ := os.Getwd()
	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	if mode == modeProject {
		return cmdSyncExtrasProject(cwd, dryRun, force, jsonOutput, partOfAll, start)
	}
	return cmdSyncExtrasGlobal(dryRun, force, jsonOutput, partOfAll, start)
}

func cmdSyncExtrasGlobal(dryRun, force, jsonOutput, partOfAll bool, start time.Time) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Target problems fail only those targets' skills and agents, not extras.
	if _, _, err := config.ValidateConfigForSync(cfg); err != nil {
		return err
	}
	if len(cfg.Extras) == 0 {
		// Clean up empty extras directory
		removeEmptyDir(config.ExtrasParentDir(cfg.EffectiveSkillsSource()))

		if jsonOutput {
			return writeJSON(&syncExtrasJSONOutput{Extras: []syncExtrasJSONEntry{}, Duration: formatDuration(start)})
		}
		if partOfAll {
			return nil
		}
		ui.Info("No extras configured.")
		fmt.Println()
		ui.Info("Add extras to your config.yaml:")
		fmt.Println()
		fmt.Println("  extras:")
		fmt.Println("    - name: rules")
		fmt.Println("      targets:")
		fmt.Println("        - path: ~/.claude/rules")
		fmt.Println("        - path: ~/.cursor/rules")
		fmt.Println("          mode: copy")
		return nil
	}

	configDir := filepath.Dir(cfg.EffectiveSkillsSource())

	// Auto-migrate legacy extras directories (flat → extras/<name>/)
	if warnings := config.MigrateExtrasDir(configDir, cfg.Extras); len(warnings) > 0 {
		for _, w := range warnings {
			ui.Warning(w)
		}
	}

	// Detect overlap between extras "agents" and the agents sync system
	var agentTargetPaths map[string]bool
	for _, extra := range cfg.Extras {
		if extra.Name == extrasAgentsName {
			agentTargetPaths = collectAgentTargetPathsGlobal(cfg)
			break
		}
	}

	totals := extrasSyncTotals{width: extrasRowWidth(cfg.Extras)}
	var jsonEntries []syncExtrasJSONEntry

	if !jsonOutput {
		ui.Section("Extras")
	}

	opts := sync.ExtraRunOptions{
		DryRun:      dryRun,
		Force:       force,
		ResolvePath: config.ExpandPath,
		ResolveExtension: func(ext string) (*sync.ExtensionSpec, error) {
			return resolveExtension(ext, globalExtensionsDir())
		},
		AgentTargetPaths: agentTargetPaths,
	}

	for _, extra := range cfg.Extras {
		extraSource := config.ResolveExtrasSourceDir(extra, cfg.EffectiveExtrasSource(), cfg.EffectiveSkillsSource())

		run := sync.RunExtraTargets(extra, extraSource, opts)
		if run.SourceMissing {
			printMissingExtraSource(extra.Name, extraSource, jsonOutput)
			if jsonOutput {
				jsonEntries = append(jsonEntries, syncExtrasJSONEntry{Name: extra.Name, Targets: []syncExtrasJSONTarget{}})
			}
			continue
		}

		jsonEntry := syncExtrasJSONEntry{Name: extra.Name}
		for _, tr := range run.Targets {
			reportExtraTarget(extra.Name, tr, jsonOutput, &totals)
			jsonEntry.Targets = append(jsonEntry.Targets, extraTargetJSON(tr, tr.Target.Path))
		}
		jsonEntries = append(jsonEntries, jsonEntry)
	}

	// Oplog
	status := "ok"
	if totals.errors > 0 {
		status = "partial"
	}
	e := oplog.NewEntry("sync-extras", status, time.Since(start))
	e.Args = map[string]any{
		"extras_count": len(cfg.Extras),
		"synced":       totals.synced,
		"skipped":      totals.skipped,
		"pruned":       totals.pruned,
		"errors":       totals.errors,
		"dry_run":      dryRun,
		"force":        force,
	}
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	if jsonOutput {
		output := syncExtrasJSONOutput{
			Extras:   jsonEntries,
			Duration: formatDuration(start),
		}
		if err := writeJSON(&output); err != nil {
			return err
		}
		if totals.errors > 0 {
			return &jsonSilentError{cause: fmt.Errorf("%d extras sync error(s)", totals.errors)}
		}
		return nil
	}

	printExtrasDone(len(cfg.Extras), totals, dryRun, time.Since(start))

	if totals.errors > 0 {
		return fmt.Errorf("%d extras sync error(s)", totals.errors)
	}
	return nil
}

func cmdSyncExtrasProject(cwd string, dryRun, force, jsonOutput, partOfAll bool, start time.Time) error {
	projCfg, err := config.LoadProject(cwd)
	if err != nil {
		return err
	}

	// Target problems fail only those targets' skills and agents, not extras.
	if _, _, err := config.ValidateProjectConfigForSync(projCfg, cwd); err != nil {
		return err
	}
	if len(projCfg.Extras) == 0 {
		// Clean up empty extras directory
		removeEmptyDir(config.ExtrasParentDirProject(projCfg.EffectiveExtrasSource(cwd)))

		if jsonOutput {
			return writeJSON(&syncExtrasJSONOutput{Extras: []syncExtrasJSONEntry{}, Duration: formatDuration(start)})
		}
		if partOfAll {
			return nil
		}
		ui.Info("No extras configured in project.")
		ui.Info("Run 'skillshare extras init <name> --target <path> -p' to add one.")
		return nil
	}

	// Detect overlap between extras "agents" and the agents sync system
	var agentTargetPaths map[string]bool
	for _, extra := range projCfg.Extras {
		if extra.Name == extrasAgentsName {
			agentTargetPaths = collectAgentTargetPathsProject(cwd)
			break
		}
	}

	totals := extrasSyncTotals{width: extrasRowWidth(projCfg.Extras)}
	var jsonEntries []syncExtrasJSONEntry

	if !jsonOutput {
		ui.Section("Extras")
	}

	opts := sync.ExtraRunOptions{
		DryRun:      dryRun,
		Force:       force,
		ProjectRoot: cwd,
		// Expand ~ and resolve relative paths against project root
		ResolvePath: func(path string) string { return resolveProjectPath(cwd, path) },
		ResolveExtension: func(ext string) (*sync.ExtensionSpec, error) {
			return resolveExtension(ext, projectExtensionsDir(cwd))
		},
		AgentTargetPaths: agentTargetPaths,
	}

	for _, extra := range projCfg.Extras {
		extraSource := config.ResolveExtrasSourceDirProject(extra, projCfg.EffectiveExtrasSource(cwd), cwd)

		run := sync.RunExtraTargets(extra, extraSource, opts)
		if run.SourceMissing {
			printMissingExtraSource(extra.Name, extraSource, jsonOutput)
			if jsonOutput {
				jsonEntries = append(jsonEntries, syncExtrasJSONEntry{Name: extra.Name, Targets: []syncExtrasJSONTarget{}})
			}
			continue
		}

		jsonEntry := syncExtrasJSONEntry{Name: extra.Name}
		for _, tr := range run.Targets {
			reportExtraTarget(extra.Name, tr, jsonOutput, &totals)
			// Targets that reached the sync report the resolved path.
			path := tr.Target.Path
			if tr.SkippedBy == "" && tr.ModeErr == nil && tr.ExtensionErr == nil {
				path = tr.Path
			}
			jsonEntry.Targets = append(jsonEntry.Targets, extraTargetJSON(tr, path))
		}
		jsonEntries = append(jsonEntries, jsonEntry)
	}

	status := "ok"
	if totals.errors > 0 {
		status = "partial"
	}
	e := oplog.NewEntry("sync-extras", status, time.Since(start))
	e.Args = map[string]any{
		"extras_count": len(projCfg.Extras),
		"synced":       totals.synced,
		"skipped":      totals.skipped,
		"pruned":       totals.pruned,
		"errors":       totals.errors,
		"dry_run":      dryRun,
		"force":        force,
		"scope":        "project",
	}
	oplog.WriteWithLimit(config.ProjectConfigPath(cwd), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	if jsonOutput {
		output := syncExtrasJSONOutput{
			Extras:   jsonEntries,
			Duration: formatDuration(start),
		}
		if err := writeJSON(&output); err != nil {
			return err
		}
		if totals.errors > 0 {
			return &jsonSilentError{cause: fmt.Errorf("%d extras sync error(s)", totals.errors)}
		}
		return nil
	}

	printExtrasDone(len(projCfg.Extras), totals, dryRun, time.Since(start))

	if totals.errors > 0 {
		return fmt.Errorf("%d extras sync error(s)", totals.errors)
	}
	return nil
}

// extrasRowWidth lines up the rows of every extra.
func extrasRowWidth(extras []config.ExtraConfig) int {
	names := make([]string, len(extras))
	for i, e := range extras {
		names[i] = e.Name
	}
	return ui.RowWidth(names...)
}

// printExtrasDone closes an extras sync.
func printExtrasDone(extras int, totals extrasSyncTotals, dryRun bool, took time.Duration) {
	fmt.Println()
	switch {
	case totals.errors > 0:
		ui.Done(ui.MarkFail, plural(totals.errors, "extras error"), took)
	case dryRun:
		ui.Done(ui.MarkOK, fmt.Sprintf("Would sync %s to %s", plural(extras, "extra"), plural(totals.targets, "folder")), took)
		ui.DryRun()
	default:
		ui.Done(ui.MarkOK, fmt.Sprintf("Synced %s to %s", plural(extras, "extra"), plural(totals.targets, "folder")), took)
	}
}

// syncVerb returns a user-facing verb for the given sync mode.
func syncVerb(mode string) string {
	switch mode {
	case "copy":
		return "copied"
	case "symlink":
		return "linked"
	case "import":
		return "imported"
	default:
		return "synced"
	}
}

// runExtrasSync runs extras sync and returns JSON entries without printing,
// plus an error counting failures as `sync extras` does.
// Used by sync --all --json to merge extras into the skills JSON output.
// agentTargetPaths is used to skip extras "agents" targets that overlap with the agents sync system.
func runExtrasSyncEntries(extras []config.ExtraConfig, sourceFunc func(config.ExtraConfig) string, dryRun, force bool, projectRoot string, agentTargetPaths map[string]bool) ([]syncExtrasJSONEntry, error) {
	// Resolve the per-target transform extension so --all --json applies
	// it like a normal sync instead of copying files verbatim.
	extDir := globalExtensionsDir()
	resolvePath := config.ExpandPath
	if projectRoot != "" {
		extDir = projectExtensionsDir(projectRoot)
		resolvePath = func(path string) string { return resolveProjectPath(projectRoot, path) }
	}
	opts := sync.ExtraRunOptions{
		DryRun:      dryRun,
		Force:       force,
		ProjectRoot: projectRoot,
		ResolvePath: resolvePath,
		ResolveExtension: func(ext string) (*sync.ExtensionSpec, error) {
			return resolveExtension(ext, extDir)
		},
		AgentTargetPaths: agentTargetPaths,
	}

	var totals extrasSyncTotals
	entries := make([]syncExtrasJSONEntry, 0, len(extras))
	for _, extra := range extras {
		entry := syncExtrasJSONEntry{Name: extra.Name}
		run := sync.RunExtraTargets(extra, sourceFunc(extra), opts)
		if run.SourceMissing {
			entry.Targets = []syncExtrasJSONTarget{}
		}
		for _, tr := range run.Targets {
			reportExtraTarget(extra.Name, tr, true, &totals)
			entry.Targets = append(entry.Targets, extraTargetJSON(tr, tr.Path))
		}
		entries = append(entries, entry)
	}
	if totals.errors > 0 {
		return entries, fmt.Errorf("%d extras sync error(s)", totals.errors)
	}
	return entries, nil
}

// printMissingExtraSource tells the user, unless jsonOutput, that an extra was
// skipped because its source directory does not exist.
func printMissingExtraSource(name, sourceDir string, jsonOutput bool) {
	if jsonOutput {
		return
	}
	ui.Row(ui.MarkWarn, name, "source folder not found: "+shortenPath(sourceDir), ui.RowWidth(name))
	ui.Note("Create it to start syncing " + name)
}

// extrasSyncTotals accumulates target outcomes for the summary and oplog.
type extrasSyncTotals struct {
	synced, skipped, pruned, errors, targets int
	width                                    int // label width of the printed rows
}

// reportExtraTarget adds one target's outcome to totals and, unless
// jsonOutput, prints it.
func reportExtraTarget(extraName string, tr sync.ExtraTargetRun, jsonOutput bool, totals *extrasSyncTotals) {
	totals.targets++
	shortTarget := shortenPath(tr.Path)

	if tr.SkippedBy != "" {
		if !jsonOutput {
			ui.Row(ui.MarkWarn, extraName, shortTarget+" "+theme.Dim().Render("skipped — already managed by agents sync"), totals.width)
		}
		return
	}
	if err := extraTargetFailure(tr); err != nil {
		if !jsonOutput {
			ui.Row(ui.MarkFail, extraName, shortTarget+" "+err.Error(), totals.width)
		}
		totals.errors++
		return
	}

	result := tr.Result
	totals.synced += result.Synced
	totals.skipped += result.Skipped
	totals.pruned += result.Pruned
	totals.errors += len(result.Errors)
	if jsonOutput {
		return
	}

	verb := syncVerb(tr.Mode)
	preserved := countPart{result.Preserved, "local preserved"}
	switch {
	case result.Synced > 0:
		ui.Row(ui.MarkOK, extraName, shortTarget+"  "+syncCounts("",
			countPart{result.Synced, "files " + verb}, countPart{result.Pruned, "pruned"}, preserved), totals.width)
	case result.Skipped > result.Preserved:
		ui.Row(ui.MarkWarn, extraName, fmt.Sprintf("%s  %d files skipped (use --force to override)", shortTarget, result.Skipped-result.Preserved), totals.width)
	default:
		ui.Row(ui.MarkOK, extraName, shortTarget+"  "+syncCounts("up to date", preserved), totals.width)
	}

	for _, e := range result.Errors {
		fmt.Printf("  %s %s\n", theme.Warning().Render(ui.MarkWarn), e)
	}
	for _, w := range result.Warnings {
		ui.Note(w)
	}
}

// extraTargetFailure returns the error that stopped a target, checking the
// mode before the extension.
func extraTargetFailure(tr sync.ExtraTargetRun) error {
	switch {
	case tr.ModeErr != nil:
		return tr.ModeErr
	case tr.ExtensionErr != nil:
		return tr.ExtensionErr
	default:
		return tr.Err
	}
}

// extraTargetJSON returns the JSON form of one target, reported at path.
func extraTargetJSON(tr sync.ExtraTargetRun, path string) syncExtrasJSONTarget {
	jt := syncExtrasJSONTarget{Path: path, Mode: tr.Mode, SkippedBy: tr.SkippedBy}
	if err := extraTargetFailure(tr); err != nil {
		jt.Error = err.Error()
		return jt
	}
	if tr.Result != nil {
		jt.Synced = tr.Result.Synced
		jt.Skipped = tr.Result.Skipped
		jt.Pruned = tr.Result.Pruned
		jt.Warnings = tr.Result.Warnings
		if len(tr.Result.Errors) > 0 {
			jt.Error = strings.Join(tr.Result.Errors, "; ")
		}
	}
	return jt
}

// cachedHome caches the home directory for shortenPath.
var cachedHome = func() string {
	h, _ := os.UserHomeDir()
	return h
}()

// shortenPath replaces the home directory prefix with ~.
func shortenPath(p string) string {
	if cachedHome != "" && strings.HasPrefix(p, cachedHome) {
		return "~" + p[len(cachedHome):]
	}
	return p
}
