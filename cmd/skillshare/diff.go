package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	gosync "sync"
	"time"

	"github.com/pterm/pterm"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

func cmdDiff(args []string) error {
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

	scope := "global"
	cfgPath := config.ConfigPath()
	if mode == modeProject {
		scope = "project"
		cfgPath = config.ProjectConfigPath(cwd)
	}

	var targetName string
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--target" || rest[i] == "-t" {
			if i+1 < len(rest) {
				targetName = rest[i+1]
				i++
			}
		} else {
			targetName = rest[i]
		}
	}

	var cmdErr error
	if mode == modeProject {
		cmdErr = cmdDiffProject(cwd, targetName)
	} else {
		cmdErr = cmdDiffGlobal(targetName)
	}
	logDiffOp(cfgPath, targetName, scope, 0, start, cmdErr)
	return cmdErr
}

func logDiffOp(cfgPath string, targetName, scope string, targetsShown int, start time.Time, cmdErr error) {
	e := oplog.NewEntry("diff", statusFromErr(cmdErr), time.Since(start))
	a := map[string]any{"scope": scope}
	if targetName != "" {
		a["target"] = targetName
	}
	if targetsShown > 0 {
		a["targets_shown"] = targetsShown
	}
	e.Args = a
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

// targetDiffResult holds the diff outcome for one target.
type targetDiffResult struct {
	name       string
	mode       string          // "merge", "copy", "symlink"
	items      []copyDiffEntry // reuse existing struct
	syncCount  int
	localCount int
	synced     bool   // true if fully synced
	errMsg     string // non-empty if target inaccessible
	include    []string
	exclude    []string
}

type copyDiffEntry struct {
	action string // "add", "modify", "remove"
	name   string
	reason string
	isSync bool // true = needs sync, false = local-only
}

// diffProgress displays multi-target scanning progress.
// Target list (spinner/queued/done) + overall progress bar at the bottom.
type diffProgress struct {
	names           []string
	states          []string // "queued", "scanning", "done", "error"
	details         []string
	totalSkills     int
	processedSkills int
	area            *pterm.AreaPrinter
	mu              gosync.Mutex
	stopCh          chan struct{}
	frames          []string
	frame           int
	isTTY           bool
}

func newDiffProgress(names []string, totalSkills int) *diffProgress {
	dp := &diffProgress{
		names:       names,
		states:      make([]string, len(names)),
		details:     make([]string, len(names)),
		totalSkills: totalSkills,
		frames:      []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		isTTY:       ui.IsTTY(),
	}
	for i := range dp.states {
		dp.states[i] = "queued"
	}
	if !dp.isTTY {
		return dp
	}
	area, _ := pterm.DefaultArea.WithRemoveWhenDone(true).Start()
	dp.area = area
	dp.stopCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-dp.stopCh:
				return
			case <-ticker.C:
				dp.mu.Lock()
				dp.frame = (dp.frame + 1) % len(dp.frames)
				dp.render()
				dp.mu.Unlock()
			}
		}
	}()
	dp.render()
	return dp
}

func (dp *diffProgress) render() {
	if dp.area == nil {
		return
	}
	var lines []string
	for i, name := range dp.names {
		var line string
		switch dp.states[i] {
		case "queued":
			line = fmt.Sprintf("  %s  %s", pterm.Gray(name), pterm.Gray("queued"))
		case "scanning":
			spin := pterm.Cyan(dp.frames[dp.frame])
			line = fmt.Sprintf("  %s %s  %s", spin, pterm.Cyan(name), pterm.Gray(dp.details[i]))
		case "done":
			line = fmt.Sprintf("  %s %s  %s", pterm.Green("✓"), name, pterm.Gray(dp.details[i]))
		case "error":
			line = fmt.Sprintf("  %s %s  %s", pterm.Red("✗"), name, pterm.Gray(dp.details[i]))
		}
		lines = append(lines, line)
	}
	// Progress bar at bottom
	if dp.totalSkills > 0 {
		lines = append(lines, "  "+dp.renderBar())
	}
	dp.area.Update(strings.Join(lines, "\n"))
}

func (dp *diffProgress) renderBar() string {
	const barWidth = 30
	current := dp.processedSkills
	total := dp.totalSkills
	filled := current * barWidth / total
	if filled > barWidth {
		filled = barWidth
	}
	pct := int(math.Round(float64(current) * 100 / float64(total)))
	filledBar := pterm.Cyan(strings.Repeat("█", filled))
	emptyBar := pterm.Gray(strings.Repeat("█", barWidth-filled))
	count := fmt.Sprintf("%d/%d", current, total)
	return fmt.Sprintf("%s%s %s %d%%", filledBar, emptyBar, pterm.Gray(count), pct)
}

func (dp *diffProgress) startTarget(name string) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	for i, n := range dp.names {
		if n == name {
			dp.states[i] = "scanning"
			dp.details[i] = "comparing..."
			break
		}
	}
	if !dp.isTTY {
		fmt.Printf("  %s: scanning...\n", name)
	}
}

func (dp *diffProgress) update(targetName, skillName string) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.processedSkills++
	for i, n := range dp.names {
		if n == targetName {
			dp.details[i] = skillName
			break
		}
	}
}

func (dp *diffProgress) add(n int) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.processedSkills += n
}

func (dp *diffProgress) doneTarget(name string, r targetDiffResult) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	for i, n := range dp.names {
		if n != name {
			continue
		}
		if r.errMsg != "" {
			dp.states[i] = "error"
			dp.details[i] = r.errMsg
		} else if r.synced {
			dp.states[i] = "done"
			dp.details[i] = "fully synced"
		} else {
			dp.states[i] = "done"
			dp.details[i] = fmt.Sprintf("%d difference(s)", r.syncCount+r.localCount)
		}
		break
	}
	if !dp.isTTY {
		for i, n := range dp.names {
			if n == name {
				fmt.Printf("  %s: %s\n", name, dp.details[i])
				break
			}
		}
	}
}

func (dp *diffProgress) stop() {
	if dp.stopCh != nil {
		close(dp.stopCh)
	}
	if dp.area != nil {
		dp.area.Stop() //nolint:errcheck
	}
}

func cmdDiffGlobal(targetName string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	spinner := ui.StartSpinner("Discovering skills")
	discovered, err := sync.DiscoverSourceSkills(cfg.Source)
	if err != nil {
		spinner.Fail("Discovery failed")
		return fmt.Errorf("failed to discover skills: %w", err)
	}
	spinner.Success(fmt.Sprintf("Discovered %d skills", len(discovered)))

	targets := cfg.Targets
	if targetName != "" {
		if t, exists := cfg.Targets[targetName]; exists {
			targets = map[string]config.TargetConfig{targetName: t}
		} else {
			return fmt.Errorf("target '%s' not found", targetName)
		}
	}

	// Build sorted target list for deterministic progress display
	type targetEntry struct {
		name   string
		target config.TargetConfig
		mode   string
	}
	var entries []targetEntry
	for name, target := range targets {
		mode := target.Mode
		if mode == "" {
			mode = cfg.Mode
			if mode == "" {
				mode = "merge"
			}
		}
		entries = append(entries, targetEntry{name, target, mode})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	// Pre-filter skills per target and compute total for progress bar
	type filteredEntry struct {
		targetEntry
		filtered []sync.DiscoveredSkill
	}
	var fentries []filteredEntry
	totalSkills := 0
	for _, e := range entries {
		filtered, err := sync.FilterSkills(discovered, e.target.Include, e.target.Exclude)
		if err != nil {
			return fmt.Errorf("target %s has invalid include/exclude config: %w", e.name, err)
		}
		fentries = append(fentries, filteredEntry{e, filtered})
		totalSkills += len(filtered)
	}

	names := make([]string, len(fentries))
	for i, fe := range fentries {
		names[i] = fe.name
	}
	progress := newDiffProgress(names, totalSkills)

	var results []targetDiffResult
	for _, fe := range fentries {
		progress.startTarget(fe.name)
		r := collectTargetDiff(fe.name, fe.target, cfg.Source, fe.mode, fe.filtered, progress)
		progress.doneTarget(fe.name, r)
		results = append(results, r)
	}

	progress.stop()
	renderGroupedDiffs(results)
	return nil
}

func collectTargetDiff(name string, target config.TargetConfig, source, mode string, filtered []sync.DiscoveredSkill, dp *diffProgress) targetDiffResult {
	r := targetDiffResult{
		name:    name,
		mode:    mode,
		include: target.Include,
		exclude: target.Exclude,
	}

	// Check if target is accessible
	_, err := os.Lstat(target.Path)
	if err != nil {
		r.errMsg = fmt.Sprintf("Cannot access target: %v", err)
		dp.add(len(filtered))
		return r
	}

	sourceSkills := make(map[string]bool, len(filtered))
	for _, skill := range filtered {
		sourceSkills[skill.FlatName] = true
	}

	if utils.IsSymlinkOrJunction(target.Path) {
		r.mode = "symlink"
		collectSymlinkDiff(&r, target.Path, source)
		dp.add(len(filtered))
		return r
	}

	if mode == "copy" {
		manifest, _ := sync.ReadManifest(target.Path)
		collectCopyDiff(&r, name, target.Path, filtered, sourceSkills, manifest, dp)
		return r
	}

	// Merge mode (instant)
	collectMergeDiff(&r, target.Path, sourceSkills)
	dp.add(len(filtered))
	return r
}

func collectSymlinkDiff(r *targetDiffResult, targetPath, source string) {
	absLink, err := utils.ResolveLinkTarget(targetPath)
	if err != nil {
		r.errMsg = fmt.Sprintf("Unable to resolve symlink target: %v", err)
		return
	}
	absSource, _ := filepath.Abs(source)
	if utils.PathsEqual(absLink, absSource) {
		r.synced = true
	} else {
		r.errMsg = fmt.Sprintf("Symlink points to different location: %s", absLink)
	}
}

func collectCopyDiff(r *targetDiffResult, targetName, targetPath string, filtered []sync.DiscoveredSkill, sourceSkills map[string]bool, manifest *sync.Manifest, dp *diffProgress) {
	for _, skill := range filtered {
		dp.update(targetName, skill.FlatName)
		oldChecksum, isManaged := manifest.Managed[skill.FlatName]
		targetSkillPath := filepath.Join(targetPath, skill.FlatName)
		if !isManaged {
			if info, err := os.Stat(targetSkillPath); err == nil {
				if info.IsDir() {
					r.items = append(r.items, copyDiffEntry{"modify", skill.FlatName, "local copy (sync --force to replace)", true})
				} else {
					r.items = append(r.items, copyDiffEntry{"modify", skill.FlatName, "target entry is not a directory", true})
				}
			} else if os.IsNotExist(err) {
				r.items = append(r.items, copyDiffEntry{"add", skill.FlatName, "source only", true})
			} else {
				r.items = append(r.items, copyDiffEntry{"modify", skill.FlatName, "cannot access target entry", true})
			}
			continue
		}
		targetInfo, err := os.Stat(targetSkillPath)
		if os.IsNotExist(err) {
			r.items = append(r.items, copyDiffEntry{"add", skill.FlatName, "deleted from target", true})
			continue
		}
		if err != nil {
			r.items = append(r.items, copyDiffEntry{"modify", skill.FlatName, "cannot access target entry", true})
			continue
		}
		if !targetInfo.IsDir() {
			r.items = append(r.items, copyDiffEntry{"modify", skill.FlatName, "target entry is not a directory", true})
			continue
		}
		// mtime fast-path
		oldMtime := manifest.Mtimes[skill.FlatName]
		currentMtime, mtimeErr := sync.DirMaxMtime(skill.SourcePath)
		if mtimeErr == nil && oldMtime > 0 && currentMtime == oldMtime {
			continue
		}
		srcChecksum, err := sync.DirChecksum(skill.SourcePath)
		if err != nil {
			r.items = append(r.items, copyDiffEntry{"modify", skill.FlatName, "cannot compute checksum", true})
			continue
		}
		if srcChecksum != oldChecksum {
			r.items = append(r.items, copyDiffEntry{"modify", skill.FlatName, "content changed", true})
		}
	}

	// Orphan managed copies
	for name := range manifest.Managed {
		if !sourceSkills[name] {
			r.items = append(r.items, copyDiffEntry{"remove", name, "orphan (will be pruned)", true})
		}
	}

	// Local directories
	entries, _ := os.ReadDir(targetPath)
	for _, e := range entries {
		if utils.IsHidden(e.Name()) || !e.IsDir() {
			continue
		}
		if sourceSkills[e.Name()] {
			continue
		}
		if _, isManaged := manifest.Managed[e.Name()]; isManaged {
			continue
		}
		r.items = append(r.items, copyDiffEntry{"remove", e.Name(), "local only", false})
	}

	// Compute counts
	for _, item := range r.items {
		if item.isSync {
			r.syncCount++
		} else {
			r.localCount++
		}
	}
	r.synced = r.syncCount == 0 && r.localCount == 0
}

func collectMergeDiff(r *targetDiffResult, targetPath string, sourceSkills map[string]bool) {
	targetSkills := make(map[string]bool)
	targetSymlinks := make(map[string]bool)
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		r.errMsg = fmt.Sprintf("Cannot read target: %v", err)
		return
	}

	for _, e := range entries {
		if utils.IsHidden(e.Name()) {
			continue
		}
		skillPath := filepath.Join(targetPath, e.Name())
		if utils.IsSymlinkOrJunction(skillPath) {
			targetSymlinks[e.Name()] = true
		}
		targetSkills[e.Name()] = true
	}

	// Skills only in source (not synced)
	for skill := range sourceSkills {
		if !targetSkills[skill] {
			r.items = append(r.items, copyDiffEntry{"add", skill, "source only", true})
			r.syncCount++
		} else if !targetSymlinks[skill] {
			r.items = append(r.items, copyDiffEntry{"modify", skill, "local copy (sync --force to replace)", true})
			r.syncCount++
		}
	}

	// Skills only in target (local only)
	for skill := range targetSkills {
		if !sourceSkills[skill] && !targetSymlinks[skill] {
			r.items = append(r.items, copyDiffEntry{"remove", skill, "local only", false})
			r.localCount++
		}
	}

	r.synced = r.syncCount == 0 && r.localCount == 0
}

// diffFingerprint generates a grouping key from diff items.
// Results with the same fingerprint are displayed together.
func diffFingerprint(items []copyDiffEntry) string {
	sorted := make([]copyDiffEntry, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].name != sorted[j].name {
			return sorted[i].name < sorted[j].name
		}
		return sorted[i].action < sorted[j].action
	})
	var b strings.Builder
	for _, item := range sorted {
		fmt.Fprintf(&b, "%s|%s|%s\n", item.action, item.name, item.reason)
	}
	return b.String()
}

// renderGroupedDiffs groups targets with identical diff results and renders
// merged output. Targets with errors are always shown individually.
func renderGroupedDiffs(results []targetDiffResult) {
	// Sort results by name for deterministic output
	sort.Slice(results, func(i, j int) bool {
		return results[i].name < results[j].name
	})

	// Separate error targets from normal targets
	var errorResults []targetDiffResult
	var syncedNames []string
	type diffGroup struct {
		names   []string
		result  targetDiffResult // representative
		include []string
		exclude []string
	}
	groups := make(map[string]*diffGroup)
	var groupOrder []string

	for _, r := range results {
		if r.errMsg != "" {
			errorResults = append(errorResults, r)
			continue
		}
		if r.synced {
			syncedNames = append(syncedNames, r.name)
			continue
		}
		fp := diffFingerprint(r.items)
		if g, exists := groups[fp]; exists {
			g.names = append(g.names, r.name)
			// Merge include/exclude for display if they differ
			if len(r.include) > 0 || len(r.exclude) > 0 {
				g.include = nil
				g.exclude = nil
			}
		} else {
			groups[fp] = &diffGroup{
				names:   []string{r.name},
				result:  r,
				include: r.include,
				exclude: r.exclude,
			}
			groupOrder = append(groupOrder, fp)
		}
	}

	// Render error targets individually
	for _, r := range errorResults {
		ui.Header(r.name)
		ui.Warning("%s", r.errMsg)
	}

	// Render grouped diff results
	for _, fp := range groupOrder {
		g := groups[fp]
		sort.Strings(g.names)
		ui.Header(strings.Join(g.names, ", "))

		// Show include/exclude only if all targets in group share the same filter
		if len(g.names) == 1 {
			if len(g.include) > 0 {
				ui.Info("  include: %s", strings.Join(g.include, ", "))
			}
			if len(g.exclude) > 0 {
				ui.Info("  exclude: %s", strings.Join(g.exclude, ", "))
			}
		}

		// Sort items by name for stable output
		items := make([]copyDiffEntry, len(g.result.items))
		copy(items, g.result.items)
		sort.Slice(items, func(i, j int) bool {
			return items[i].name < items[j].name
		})

		var syncCount, localCount int
		for _, item := range items {
			ui.DiffItem(item.action, item.name, item.reason)
			if item.isSync {
				syncCount++
			} else {
				localCount++
			}
		}

		fmt.Println()
		if syncCount > 0 {
			if g.result.mode == "copy" {
				ui.Info("Run 'sync' to copy missing, 'sync --force' to replace local copies")
			} else {
				ui.Info("Run 'sync' to add missing, 'sync --force' to replace local copies")
			}
		}
		if localCount > 0 {
			if len(g.names) == 1 {
				ui.Info("Run 'collect %s' to import local-only skills to source", g.names[0])
			} else {
				ui.Info("Run 'collect' to import local-only skills to source")
			}
		}
	}

	// Render fully synced targets as a single line
	if len(syncedNames) > 0 {
		sort.Strings(syncedNames)
		ui.Success("%s: fully synced", strings.Join(syncedNames, ", "))
	}
}
