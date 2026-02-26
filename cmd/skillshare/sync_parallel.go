package main

import (
	"fmt"
	"os"
	"strings"
	gosync "sync"
	"time"

	"github.com/pterm/pterm"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
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
		frames:  []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
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
			line = fmt.Sprintf("  %s  %s", pterm.Gray(name), pterm.Gray("queued"))
		case "syncing":
			spin := pterm.Cyan(sp.frames[sp.frame])
			detail := sp.details[i]
			if detail == "" {
				detail = "syncing..."
			}
			line = fmt.Sprintf("  %s %s  %s", spin, pterm.Cyan(name), pterm.Gray(detail))
		case "done":
			line = fmt.Sprintf("  %s %s  %s", pterm.Green("✓"), name, pterm.Gray(sp.details[i]))
		case "error":
			line = fmt.Sprintf("  %s %s  %s", pterm.Red("✗"), name, pterm.Gray(sp.details[i]))
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
			sp.details[i] = "syncing..."
			break
		}
	}
	if !sp.isTTY {
		fmt.Printf("  %s: syncing...\n", name)
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
	if !sp.isTTY {
		for i, n := range sp.names {
			if n == name {
				fmt.Printf("  %s: %s\n", name, sp.details[i])
				break
			}
		}
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
}

// collectSyncResult runs sync for one target and returns a result struct.
// Does NOT print any UI output — all output data is captured in the result.
func collectSyncResult(name string, target config.TargetConfig, source, mode string, skills []sync.DiscoveredSkill, dryRun, force bool, progress *syncProgress) syncTargetResult {
	r := syncTargetResult{
		name:    name,
		mode:    mode,
		include: target.Include,
		exclude: target.Exclude,
	}

	switch mode {
	case "merge":
		collectMergeSyncResult(&r, name, target, source, skills, dryRun, force)
	case "copy":
		collectCopySyncResult(&r, name, target, source, skills, dryRun, force, progress)
	default:
		collectSymlinkSyncResult(&r, name, target, source, dryRun, force)
	}

	return r
}

func collectMergeSyncResult(r *syncTargetResult, name string, target config.TargetConfig, source string, skills []sync.DiscoveredSkill, dryRun, force bool) {
	result, err := sync.SyncTargetMergeWithSkills(name, target, skills, dryRun, force)
	if err != nil {
		r.errMsg = err.Error()
		return
	}

	pruneResult, pruneErr := sync.PruneOrphanLinksWithSkills(sync.PruneOptions{
		TargetPath: target.Path, SourcePath: source, Skills: skills,
		Include: target.Include, Exclude: target.Exclude, TargetName: name,
		DryRun: dryRun, Force: force,
	})
	if pruneErr != nil {
		r.warnings = append(r.warnings, fmt.Sprintf("%s: prune failed: %v", name, pruneErr))
	}

	r.stats = mergeStats(result, pruneResult)

	// Build message (same logic as reportMergeResult)
	linkedCount := len(result.Linked)
	updatedCount := len(result.Updated)
	skippedCount := len(result.Skipped)
	removedCount := 0
	if pruneResult != nil {
		removedCount = len(pruneResult.Removed)
		skippedCount += len(pruneResult.LocalDirs)
	}

	if linkedCount > 0 || updatedCount > 0 || removedCount > 0 {
		r.message = fmt.Sprintf("merged (%d linked, %d local, %d updated, %d pruned)",
			linkedCount, skippedCount, updatedCount, removedCount)
	} else if skippedCount > 0 {
		r.message = fmt.Sprintf("merged (%d local skills preserved)", skippedCount)
	} else {
		r.message = "merged (no skills)"
	}

	if pruneResult != nil {
		r.warnings = append(r.warnings, pruneResult.Warnings...)
	}
}

func collectCopySyncResult(r *syncTargetResult, name string, target config.TargetConfig, source string, skills []sync.DiscoveredSkill, dryRun, force bool, progress *syncProgress) {
	onProgress := func(cur, total int, skill string) {
		if progress != nil {
			progress.updateTarget(name, fmt.Sprintf("%d/%d %s", cur, total, skill))
		}
	}

	result, err := sync.SyncTargetCopyWithSkills(name, target, skills, dryRun, force, onProgress)
	if err != nil {
		r.errMsg = err.Error()
		return
	}

	pruneResult, pruneErr := sync.PruneOrphanCopiesWithSkills(target.Path, skills, target.Include, target.Exclude, name, dryRun)
	if pruneErr != nil {
		r.warnings = append(r.warnings, fmt.Sprintf("%s: prune failed: %v", name, pruneErr))
	}

	r.stats = copyStats(result, pruneResult)

	// Build message (same logic as reportCopyResult)
	copiedCount := len(result.Copied)
	updatedCount := len(result.Updated)
	skippedCount := len(result.Skipped)
	removedCount := 0
	if pruneResult != nil {
		removedCount = len(pruneResult.Removed)
	}

	if copiedCount > 0 || updatedCount > 0 || removedCount > 0 {
		r.message = fmt.Sprintf("copied (%d new, %d skipped, %d updated, %d pruned)",
			copiedCount, skippedCount, updatedCount, removedCount)
	} else if skippedCount > 0 {
		r.message = fmt.Sprintf("copied (%d skipped, up to date)", skippedCount)
	} else {
		r.message = "copied (no skills)"
	}

	if pruneResult != nil {
		r.warnings = append(r.warnings, pruneResult.Warnings...)
	}
}

func collectSymlinkSyncResult(r *syncTargetResult, name string, target config.TargetConfig, source string, dryRun, force bool) {
	status := sync.CheckStatus(target.Path, source)

	if status == sync.StatusConflict && !force {
		link, err := utils.ResolveLinkTarget(target.Path)
		if err != nil {
			link = "(unable to resolve target)"
		}
		r.errMsg = fmt.Sprintf("conflict - symlink points to %s (use --force to override)", link)
		return
	}

	if status == sync.StatusConflict && force && !dryRun {
		os.Remove(target.Path)
	}

	if err := sync.SyncTarget(name, target, source, dryRun); err != nil {
		r.errMsg = err.Error()
		return
	}

	switch status {
	case sync.StatusLinked:
		r.message = "already linked"
	case sync.StatusNotExist:
		r.message = "symlink created"
		r.warnings = append(r.warnings, fmt.Sprintf("Symlink mode: deleting files in %s will delete from source!", target.Path))
		r.infos = append(r.infos, fmt.Sprintf("Use 'skillshare target remove %s' to safely unlink", name))
	case sync.StatusHasFiles:
		r.message = "files migrated and linked"
		r.warnings = append(r.warnings, fmt.Sprintf("Symlink mode: deleting files in %s will delete from source!", target.Path))
		r.infos = append(r.infos, fmt.Sprintf("Use 'skillshare target remove %s' to safely unlink", name))
	case sync.StatusBroken:
		r.message = "broken link fixed"
	case sync.StatusConflict:
		r.message = "conflict resolved (forced)"
	}
}

// renderSyncResults renders all collected sync target results after parallel execution.
func renderSyncResults(results []syncTargetResult) {
	for _, r := range results {
		if r.errMsg != "" {
			ui.Error("%s: %s", r.name, r.errMsg)
			continue
		}

		ui.Success("%s: %s", r.name, r.message)

		if len(r.include) > 0 {
			ui.Info("  include: %s", strings.Join(r.include, ", "))
		}
		if len(r.exclude) > 0 {
			ui.Info("  exclude: %s", strings.Join(r.exclude, ", "))
		}
		for _, warn := range r.warnings {
			ui.Warning("  %s", warn)
		}
		for _, info := range r.infos {
			ui.Info("  %s", info)
		}
	}
}

// runParallelSync executes sync for multiple targets in parallel using bounded concurrency.
// Targets sharing the same resolved path are grouped and run sequentially within one
// goroutine to avoid race conditions (e.g., two targets trying to create the same symlink).
// Returns results (indexed by position) and count of failed targets.
func runParallelSync(entries []syncTargetEntry, source string, skills []sync.DiscoveredSkill, dryRun, force bool) ([]syncTargetResult, int) {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	progress := newSyncProgress(names)

	// Group entries by resolved target path so shared-path targets run sequentially.
	type indexedEntry struct {
		idx   int
		entry syncTargetEntry
	}
	groups := make(map[string][]indexedEntry)
	var groupOrder []string
	for i, e := range entries {
		p := e.target.Path
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
			// Run all members of this path group sequentially.
			for _, m := range members {
				progress.startTarget(m.entry.name)
				r := collectSyncResult(m.entry.name, m.entry.target, source, m.entry.mode, skills, dryRun, force, progress)
				progress.doneTarget(m.entry.name, r)
				results[m.idx] = r
			}
		}(group)
	}
	wg.Wait()
	progress.stop()

	// Render results and compute stats
	renderSyncResults(results)

	failedTargets := 0
	for _, r := range results {
		if r.errMsg != "" {
			failedTargets++
		}
	}
	return results, failedTargets
}
