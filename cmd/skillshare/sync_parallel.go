package main

import (
	"fmt"
	"path/filepath"
	"strings"
	gosync "sync"
	"time"

	"github.com/pterm/pterm"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
)

// syncTargetResult captures all output data for one target's sync operation.
// Collected during parallel sync, rendered after all targets finish.
type syncTargetResult struct {
	name     string
	mode     string // "merge", "copy", "symlink"
	stats    syncModeStats
	message  string   // primary success message
	include  []string // target filter display
	exclude  []string
	warnings []string // prune warnings etc.
	infos    []string // extra info lines (symlink mode hints)
	errMsg   string   // non-empty if target failed
	// skillsOff marks a target with skills switched off: nothing was synced.
	skillsOff bool
}

const syncMaxWorkers = 8

// syncProgress displays multi-target sync progress.
// Modeled on diffProgress from diff.go.
type syncProgress struct {
	names   []string
	states  []string // "queued", "syncing", "done", "error"
	details []string
	area    *pterm.AreaPrinter
	mu      gosync.Mutex
	stopCh  chan struct{}
	frames  []string
	frame   int
	isTTY   bool
}

func newSyncProgress(names []string) *syncProgress {
	sp := &syncProgress{
		names:   names,
		states:  make([]string, len(names)),
		details: make([]string, len(names)),
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		isTTY:   ui.IsTTY(),
	}
	for i := range sp.states {
		sp.states[i] = "queued"
	}
	if !sp.isTTY {
		return sp
	}
	area, _ := pterm.DefaultArea.WithRemoveWhenDone(true).Start()
	sp.area = area
	sp.stopCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-sp.stopCh:
				return
			case <-ticker.C:
				sp.mu.Lock()
				sp.frame = (sp.frame + 1) % len(sp.frames)
				sp.render()
				sp.mu.Unlock()
			}
		}
	}()
	sp.render()
	return sp
}

func (sp *syncProgress) render() {
	if sp.area == nil {
		return
	}
	var lines []string
	for i, name := range sp.names {
		var line string
		switch sp.states[i] {
		case "queued":
			line = "  " + theme.Dim().Render(name)
		case "syncing":
			line = fmt.Sprintf("%s %s  %s", theme.Accent().Render(sp.frames[sp.frame]), name, theme.Dim().Render(sp.details[i]))
		case "done":
			line = fmt.Sprintf("%s %s  %s", theme.Success().Render(ui.MarkOK), name, theme.Dim().Render(sp.details[i]))
		case "error":
			line = fmt.Sprintf("%s %s  %s", theme.Danger().Render(ui.MarkFail), name, theme.Dim().Render(sp.details[i]))
		}
		lines = append(lines, line)
	}
	sp.area.Update(strings.Join(lines, "\n"))
}

func (sp *syncProgress) startTarget(name string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	for i, n := range sp.names {
		if n == name {
			sp.states[i] = "syncing"
			sp.details[i] = "syncing"
			break
		}
	}
}

func (sp *syncProgress) updateTarget(name, detail string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	for i, n := range sp.names {
		if n == name {
			sp.details[i] = detail
			break
		}
	}
}

func (sp *syncProgress) doneTarget(name string, r syncTargetResult) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	for i, n := range sp.names {
		if n != name {
			continue
		}
		if r.errMsg != "" {
			sp.states[i] = "error"
			sp.details[i] = r.errMsg
		} else {
			sp.states[i] = "done"
			sp.details[i] = r.message
		}
		break
	}
}

func (sp *syncProgress) stop() {
	if sp.stopCh != nil {
		close(sp.stopCh)
	}
	if sp.area != nil {
		sp.area.Stop() //nolint:errcheck
	}
}

// syncTargetEntry holds a pre-resolved target for parallel sync dispatch.
type syncTargetEntry struct {
	name   string
	target config.TargetConfig
	mode   string
	// configErr fails the target without syncing it: its settings are invalid.
	configErr error
	// sourceIncomplete means a followed source link is unavailable: link and
	// copy, but prune nothing.
	sourceIncomplete bool
}

// collectSyncResult runs sync for one target and returns a result struct.
// Does NOT print any UI output — all output data is captured in the result.
func collectSyncResult(entry syncTargetEntry, source string, skills []sync.DiscoveredSkill, ignorePatterns []string, dryRun, force bool, progress *syncProgress, projectRoot string) syncTargetResult {
	name, target, mode := entry.name, entry.target, entry.mode
	sc := target.SkillsConfig()
	r := syncTargetResult{
		name:    name,
		mode:    mode,
		include: sc.Include,
		exclude: sc.Exclude,
	}
	if !sc.IsEnabled() {
		r.skillsOff = true
		r.message = "skills off"
		r.include, r.exclude = nil, nil
		return r
	}

	opts := sync.SkillRunOptions{
		Source: source, ProjectRoot: projectRoot, IgnorePatterns: ignorePatterns,
		DryRun: dryRun, Force: force, SourceIncomplete: entry.sourceIncomplete,
	}
	if progress != nil {
		opts.OnProgress = func(cur, total int, skill string) {
			progress.updateTarget(name, fmt.Sprintf("%d/%d %s", cur, total, skill))
		}
	}
	run := sync.SyncSkillTarget(sync.SkillTarget{Name: name, Target: target, Mode: mode}, skills, opts)
	if run.Err != nil {
		r.errMsg = run.Err.Error()
		return r
	}

	switch mode {
	case "merge":
		collectMergeSyncResult(&r, run, dryRun)
	case "copy":
		collectCopySyncResult(&r, run, dryRun)
	default:
		collectSymlinkSyncResult(&r, run, sc.Path)
	}

	return r
}

func collectMergeSyncResult(r *syncTargetResult, run sync.SkillTargetResult, dryRun bool) {
	linkedCount := len(run.Linked)
	updatedCount := len(run.Updated)
	skippedCount := len(run.Skipped) + len(run.LocalDirs)
	removedCount := len(run.Pruned)
	r.stats = syncModeStats{linked: linkedCount, local: skippedCount, updated: updatedCount, pruned: removedCount}

	r.message = syncCounts("no skills",
		countPart{linkedCount, "linked"}, countPart{skippedCount, "local"},
		countPart{updatedCount, "updated"}, countPart{removedCount, "pruned"})

	r.infos = append(r.infos, dirCreatedInfos(run.DirCreated, dryRun)...)
	r.warnings = append(r.warnings, run.UnmatchedWarnings()...)
	r.warnings = append(r.warnings, run.Warnings...)
}

func collectCopySyncResult(r *syncTargetResult, run sync.SkillTargetResult, dryRun bool) {
	copiedCount := len(run.Linked)
	updatedCount := len(run.Updated)
	skippedCount := len(run.Skipped)
	keptCount := len(run.KeptLocal)
	removedCount := len(run.Pruned)
	r.stats = syncModeStats{linked: copiedCount, local: skippedCount, updated: updatedCount, pruned: removedCount}

	r.message = syncCounts("no skills",
		countPart{copiedCount, "copied"}, countPart{skippedCount - keptCount, "up to date"}, countPart{keptCount, "local"},
		countPart{updatedCount, "updated"}, countPart{removedCount, "pruned"})

	r.infos = append(r.infos, dirCreatedInfos(run.DirCreated, dryRun)...)
	r.warnings = append(r.warnings, run.UnmatchedWarnings()...)
	r.warnings = append(r.warnings, run.Warnings...)
}

type countPart struct {
	n     int
	label string
}

// syncCounts describes a target's result by its non-zero counts, the first
// in full and the rest dimmed: "4 linked · 1 pruned". With no counts it is empty.
func syncCounts(empty string, parts ...countPart) string {
	var out []string
	for _, p := range parts {
		if p.n > 0 {
			out = append(out, fmt.Sprintf("%d %s", p.n, p.label))
		}
	}
	if len(out) == 0 {
		return empty
	}
	if len(out) == 1 {
		return out[0]
	}
	return out[0] + " " + theme.Dim().Render("· "+strings.Join(out[1:], " · "))
}

// dirCreatedInfos notes a target directory the sync created or would create.
func dirCreatedInfos(dir string, dryRun bool) []string {
	if dir == "" {
		return nil
	}
	verb := "Created"
	if dryRun {
		verb = "Will create"
	}
	return []string{fmt.Sprintf("%s target directory: %s", verb, dir)}
}

func collectSymlinkSyncResult(r *syncTargetResult, run sync.SkillTargetResult, path string) {
	switch run.SymlinkStatus {
	case sync.StatusLinked:
		r.message = "already linked"
	case sync.StatusNotExist:
		r.message = "symlink created"
		r.warnings = append(r.warnings, fmt.Sprintf("Symlink mode: deleting files in %s will delete from source!", path))
		r.infos = append(r.infos, fmt.Sprintf("Use 'skillshare target remove %s' to safely unlink", r.name))
	case sync.StatusHasFiles:
		r.message = "files migrated and linked"
		r.warnings = append(r.warnings, fmt.Sprintf("Symlink mode: deleting files in %s will delete from source!", path))
		r.infos = append(r.infos, fmt.Sprintf("Use 'skillshare target remove %s' to safely unlink", r.name))
	case sync.StatusBroken:
		r.message = "broken link fixed"
	case sync.StatusConflict:
		r.message = "conflict resolved (forced)"
	}
}

// renderSyncResults prints one row per target, with its filters, warnings
// and notes under it.
func renderSyncResults(results []syncTargetResult) {
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.name
	}
	width := ui.RowWidth(names...)
	for _, r := range results {
		switch {
		case r.errMsg != "":
			ui.Row(ui.MarkFail, r.name, r.errMsg, width)
			continue
		case r.skillsOff:
			ui.Row(ui.MarkNone, r.name, theme.Dim().Render(r.message), width)
			continue
		}
		ui.Row(ui.MarkOK, r.name, r.message, width)
		if len(r.include) > 0 {
			ui.Note("include: " + strings.Join(r.include, ", "))
		}
		if len(r.exclude) > 0 {
			ui.Note("exclude: " + strings.Join(r.exclude, ", "))
		}
		for _, warn := range r.warnings {
			fmt.Printf("  %s %s\n", theme.Warning().Render(ui.MarkWarn), warn)
		}
		for _, info := range r.infos {
			ui.Note(info)
		}
	}
}

// runParallelSync executes sync for multiple targets in parallel using bounded concurrency.
// Targets sharing the same resolved path are grouped and run sequentially within one
// goroutine to avoid race conditions (e.g., two targets trying to create the same symlink).
// Returns results (indexed by position) and count of failed targets.
func runParallelSync(entries []syncTargetEntry, source string, skills []sync.DiscoveredSkill, ignorePatterns []string, dryRun, force bool, projectRoot string) ([]syncTargetResult, int) {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	progress := newSyncProgress(names)
	results, failedTargets := runParallelSyncCore(entries, source, skills, ignorePatterns, dryRun, force, progress, projectRoot)
	progress.stop()
	renderSyncResults(results)
	return results, failedTargets
}

// runParallelSyncQuiet executes sync for multiple targets without any UI output.
// Used for --json mode where only structured output should go to stdout.
func runParallelSyncQuiet(entries []syncTargetEntry, source string, skills []sync.DiscoveredSkill, ignorePatterns []string, dryRun, force bool, projectRoot string) ([]syncTargetResult, int) {
	return runParallelSyncCore(entries, source, skills, ignorePatterns, dryRun, force, nil, projectRoot)
}

// runParallelSyncCore is the shared implementation for parallel sync.
// When progress is nil, no UI output is produced (quiet/JSON mode).
func runParallelSyncCore(entries []syncTargetEntry, source string, skills []sync.DiscoveredSkill, ignorePatterns []string, dryRun, force bool, progress *syncProgress, projectRoot string) ([]syncTargetResult, int) {
	type indexedEntry struct {
		idx   int
		entry syncTargetEntry
	}
	groups := make(map[string][]indexedEntry)
	var groupOrder []string
	for i, e := range entries {
		p := canonicalPath(e.target.SkillsConfig().Path)
		if _, seen := groups[p]; !seen {
			groupOrder = append(groupOrder, p)
		}
		groups[p] = append(groups[p], indexedEntry{idx: i, entry: e})
	}

	results := make([]syncTargetResult, len(entries))
	sem := make(chan struct{}, syncMaxWorkers)
	var wg gosync.WaitGroup

	for _, p := range groupOrder {
		group := groups[p]
		wg.Add(1)
		sem <- struct{}{}
		go func(members []indexedEntry) {
			defer wg.Done()
			defer func() { <-sem }()
			for _, m := range members {
				if progress != nil {
					progress.startTarget(m.entry.name)
				}
				r := syncTargetResult{name: m.entry.name, mode: m.entry.mode}
				if m.entry.configErr != nil {
					r.errMsg = invalidConfigMessage(m.entry.configErr)
				} else {
					r = collectSyncResult(m.entry, source, skills, ignorePatterns, dryRun, force, progress, projectRoot)
				}
				if progress != nil {
					progress.doneTarget(m.entry.name, r)
				}
				results[m.idx] = r
			}
		}(group)
	}
	wg.Wait()

	failedTargets := 0
	for _, r := range results {
		if r.errMsg != "" {
			failedTargets++
		}
	}
	return results, failedTargets
}

// canonicalPath resolves a target path to a canonical form for grouping.
// This prevents the same physical directory from being placed in different
// groups due to trailing slashes, relative segments, or symlink aliases.
//
// When the full path doesn't exist (e.g. /link/skills where /link is a symlink
// but skills/ hasn't been created yet), we resolve the longest existing ancestor
// and re-append the remaining segments so that symlink aliases still converge.
func canonicalPath(p string) string {
	cleaned := filepath.Clean(p)
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return cleaned
	}
	// Fast path: entire path exists and can be resolved.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	// Slow path: walk up to the nearest existing ancestor, resolve it,
	// then re-append the non-existent tail segments.
	parent, tail := filepath.Dir(abs), filepath.Base(abs)
	for parent != abs {
		if resolved, err := filepath.EvalSymlinks(parent); err == nil {
			return filepath.Join(resolved, tail)
		}
		abs = parent
		tail = filepath.Join(filepath.Base(parent), tail)
		parent = filepath.Dir(parent)
	}
	return abs
}
