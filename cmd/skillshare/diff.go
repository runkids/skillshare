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

// diffRenderOpts controls diff output behavior.
type diffRenderOpts struct {
	noTUI     bool
	showPatch bool
	showStat  bool
}

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
	var opts diffRenderOpts
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--help", "-h":
			printDiffHelp()
			return nil
		case "--no-tui":
			opts.noTUI = true
		case "--patch":
			opts.showPatch = true
			opts.noTUI = true // --patch implies no TUI
		case "--stat":
			opts.showStat = true
			opts.noTUI = true // --stat implies no TUI
		default:
			targetName = rest[i]
		}
	}

	var cmdErr error
	if mode == modeProject {
		cmdErr = cmdDiffProject(cwd, targetName, opts)
	} else {
		cmdErr = cmdDiffGlobal(targetName, opts)
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
	srcMtime   time.Time // newest file mtime across source skills
	dstMtime   time.Time // newest file mtime in target dir
}

type copyDiffEntry struct {
	action string // "add", "modify", "remove"
	name   string
	reason string
	isSync bool            // true = needs sync, false = local-only
	files  []fileDiffEntry // file-level diffs (nil until populated)
	srcDir string          // source directory path (for lazy diff)
	dstDir string          // target directory path (for lazy diff)
}

// ensureFiles lazily populates file-level diffs on first access.
// Works for items with srcDir (full diff) or only dstDir (file listing).
func (e *copyDiffEntry) ensureFiles() {
	if e.files != nil {
		return
	}
	if e.srcDir == "" && e.dstDir == "" {
		return
	}
	e.files = diffSkillFiles(e.srcDir, e.dstDir)
}

// latestMtime returns the newest file mtime under dir.
func latestMtime(dir string) time.Time {
	var latest time.Time
	filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error { //nolint:errcheck
		if err != nil || info.IsDir() {
			return nil
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest
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

// newDiffProgress creates a progress display for diff scanning.
// When showBar is false (no copy-mode targets), the progress bar is hidden
// because merge/symlink diffs are instant.
func newDiffProgress(names []string, totalSkills int, showBar bool) *diffProgress {
	barTotal := totalSkills
	if !showBar {
		barTotal = 0
	}
	dp := &diffProgress{
		names:       names,
		states:      make([]string, len(names)),
		details:     make([]string, len(names)),
		totalSkills: barTotal,
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
	// Progress bar at bottom (with blank line separator)
	if dp.totalSkills > 0 {
		lines = append(lines, "", "  "+dp.renderBar())
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

func cmdDiffGlobal(targetName string, opts diffRenderOpts) error {
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
	hasCopyMode := false
	for _, e := range entries {
		filtered, err := sync.FilterSkills(discovered, e.target.Include, e.target.Exclude)
		if err != nil {
			return fmt.Errorf("target %s has invalid include/exclude config: %w", e.name, err)
		}
		fentries = append(fentries, filteredEntry{e, filtered})
		totalSkills += len(filtered)
		if e.mode == "copy" {
			hasCopyMode = true
		}
	}

	names := make([]string, len(fentries))
	for i, fe := range fentries {
		names[i] = fe.name
	}
	progress := newDiffProgress(names, totalSkills, hasCopyMode)

	results := make([]targetDiffResult, len(fentries))
	sem := make(chan struct{}, 8)
	var wg gosync.WaitGroup
	for i, fe := range fentries {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, fe filteredEntry) {
			defer wg.Done()
			defer func() { <-sem }()
			progress.startTarget(fe.name)
			r := collectTargetDiff(fe.name, fe.target, cfg.Source, fe.mode, fe.filtered, progress)
			progress.doneTarget(fe.name, r)
			results[idx] = r
		}(i, fe)
	}
	wg.Wait()

	progress.stop()

	if !opts.noTUI && ui.IsTTY() && len(results) > 0 {
		return runDiffTUI(results)
	}
	renderGroupedDiffs(results, opts)
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
	sourceMap := make(map[string]string, len(filtered))
	for _, skill := range filtered {
		sourceSkills[skill.FlatName] = true
		sourceMap[skill.FlatName] = skill.SourcePath
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
	} else {
		// Merge mode (instant)
		collectMergeDiff(&r, target.Path, sourceSkills, sourceMap)
		dp.add(len(filtered))
	}

	// Compute mtime only for copy mode (merge uses symlinks, mtime is misleading)
	if mode == "copy" {
		r.dstMtime = latestMtime(target.Path)
		var srcLatest time.Time
		for _, skill := range filtered {
			mt := latestMtime(skill.SourcePath)
			if mt.After(srcLatest) {
				srcLatest = mt
			}
		}
		r.srcMtime = srcLatest
	}

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
		srcDir := skill.SourcePath
		dstDir := targetSkillPath
		if !isManaged {
			if info, err := os.Stat(targetSkillPath); err == nil {
				if info.IsDir() {
					r.items = append(r.items, copyDiffEntry{action: "modify", name: skill.FlatName, reason: "local copy (sync --force to replace)", isSync: true, srcDir: srcDir, dstDir: dstDir})
				} else {
					r.items = append(r.items, copyDiffEntry{action: "modify", name: skill.FlatName, reason: "target entry is not a directory", isSync: true, srcDir: srcDir, dstDir: dstDir})
				}
			} else if os.IsNotExist(err) {
				r.items = append(r.items, copyDiffEntry{action: "add", name: skill.FlatName, reason: "source only", isSync: true, srcDir: srcDir, dstDir: dstDir})
			} else {
				r.items = append(r.items, copyDiffEntry{action: "modify", name: skill.FlatName, reason: "cannot access target entry", isSync: true, srcDir: srcDir, dstDir: dstDir})
			}
			continue
		}
		targetInfo, err := os.Stat(targetSkillPath)
		if os.IsNotExist(err) {
			r.items = append(r.items, copyDiffEntry{action: "add", name: skill.FlatName, reason: "deleted from target", isSync: true, srcDir: srcDir, dstDir: dstDir})
			continue
		}
		if err != nil {
			r.items = append(r.items, copyDiffEntry{action: "modify", name: skill.FlatName, reason: "cannot access target entry", isSync: true, srcDir: srcDir, dstDir: dstDir})
			continue
		}
		if !targetInfo.IsDir() {
			r.items = append(r.items, copyDiffEntry{action: "modify", name: skill.FlatName, reason: "target entry is not a directory", isSync: true, srcDir: srcDir, dstDir: dstDir})
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
			r.items = append(r.items, copyDiffEntry{action: "modify", name: skill.FlatName, reason: "cannot compute checksum", isSync: true, srcDir: srcDir, dstDir: dstDir})
			continue
		}
		if srcChecksum != oldChecksum {
			r.items = append(r.items, copyDiffEntry{action: "modify", name: skill.FlatName, reason: "content changed", isSync: true, srcDir: srcDir, dstDir: dstDir})
		}
	}

	// Orphan managed copies
	for name := range manifest.Managed {
		if !sourceSkills[name] {
			r.items = append(r.items, copyDiffEntry{action: "remove", name: name, reason: "orphan (will be pruned)", isSync: true, dstDir: filepath.Join(targetPath, name)})
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
		r.items = append(r.items, copyDiffEntry{action: "remove", name: e.Name(), reason: "local only", isSync: false, dstDir: filepath.Join(targetPath, e.Name())})
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

func collectMergeDiff(r *targetDiffResult, targetPath string, sourceSkills map[string]bool, sourceMap map[string]string) {
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
		srcDir := sourceMap[skill]
		dstDir := filepath.Join(targetPath, skill)
		if !targetSkills[skill] {
			r.items = append(r.items, copyDiffEntry{action: "add", name: skill, reason: "source only", isSync: true, srcDir: srcDir, dstDir: dstDir})
			r.syncCount++
		} else if !targetSymlinks[skill] {
			r.items = append(r.items, copyDiffEntry{action: "modify", name: skill, reason: "local copy (sync --force to replace)", isSync: true, srcDir: srcDir, dstDir: dstDir})
			r.syncCount++
		}
	}

	// Skills only in target (local only)
	for skill := range targetSkills {
		if !sourceSkills[skill] && !targetSymlinks[skill] {
			r.items = append(r.items, copyDiffEntry{action: "remove", name: skill, reason: "local only", isSync: false, dstDir: filepath.Join(targetPath, skill)})
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

// actionCategory groups diff items by the user action needed.
type actionCategory struct {
	kind   string // "new", "modified", "restore", "override", "orphan", "local", "warn"
	label  string // e.g. "New", "Modified", "Local Override"
	names  []string
	expand bool // true = list skill names
}

// categorizeItems maps raw diff items to action-oriented categories.
func categorizeItems(items []copyDiffEntry) []actionCategory {
	type bucket struct {
		kind  string
		label string
		names []string
	}
	buckets := map[string]*bucket{}
	var order []string

	add := func(key, kind, label, name string) {
		if b, ok := buckets[key]; ok {
			b.names = append(b.names, name)
		} else {
			buckets[key] = &bucket{kind: kind, label: label, names: []string{name}}
			order = append(order, key)
		}
	}

	for _, item := range items {
		switch {
		case item.reason == "source only":
			add("new", "new", "New", item.name)
		case item.reason == "deleted from target":
			add("restore", "new", "Restore", item.name)
		case item.reason == "content changed":
			add("modified", "modified", "Modified", item.name)
		case strings.Contains(item.reason, "local copy"):
			add("override", "override", "Local Override", item.name)
		case strings.Contains(item.reason, "orphan"):
			add("orphan", "orphan", "Orphan", item.name)
		case item.reason == "local only" || item.reason == "not in source":
			add("local", "local", "Local Only", item.name)
		default:
			add("warn", "warn", item.reason, item.name)
		}
	}

	var cats []actionCategory
	for _, key := range order {
		b := buckets[key]
		expand := len(b.names) <= 15
		cats = append(cats, actionCategory{
			kind:   b.kind,
			label:  b.label,
			names:  b.names,
			expand: expand,
		})
	}
	return cats
}

// renderGroupedDiffs groups targets with identical diff results and renders
// merged output. Targets with errors are always shown individually.
func renderGroupedDiffs(results []targetDiffResult, opts diffRenderOpts) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].name < results[j].name
	})

	var errorResults []targetDiffResult
	var syncedNames []string
	type diffGroup struct {
		names  []string
		result targetDiffResult
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
		} else {
			groups[fp] = &diffGroup{names: []string{r.name}, result: r}
			groupOrder = append(groupOrder, fp)
		}
	}

	// Overall summary (skip when all targets are fully synced — the ✓ line is enough)
	needCount := 0
	for _, g := range groups {
		needCount += len(g.names)
	}
	if needCount > 0 || len(errorResults) > 0 {
		renderOverallSummary(len(errorResults), needCount, len(syncedNames))
	}

	// Error targets
	for _, r := range errorResults {
		ui.Header(r.name)
		ui.Warning("%s", r.errMsg)
	}

	// Grouped diffs
	var anySyncNeeded, anyForceNeeded, anyCollectNeeded bool
	for _, fp := range groupOrder {
		g := groups[fp]
		sort.Strings(g.names)
		ui.Header(strings.Join(g.names, ", "))

		items := make([]copyDiffEntry, len(g.result.items))
		copy(items, g.result.items)
		sort.Slice(items, func(i, j int) bool {
			return items[i].name < items[j].name
		})

		cats := categorizeItems(items)

		// Per-group stat line
		var statParts []string
		for _, cat := range cats {
			n := len(cat.names)
			statParts = append(statParts, fmt.Sprintf("%d %s", n, strings.ToLower(cat.label)))
		}
		if len(statParts) > 0 {
			fmt.Printf("  %s%s%s\n", ui.Gray, strings.Join(statParts, ", "), ui.Reset)
		}

		for _, cat := range cats {
			n := len(cat.names)
			switch cat.kind {
			case "new", "modified", "restore", "orphan":
				anySyncNeeded = true
			case "override":
				anyForceNeeded = true
			case "local":
				anyCollectNeeded = true
			}

			skillWord := "skills"
			if n == 1 {
				skillWord = "skill"
			}
			if cat.expand && n > 0 {
				ui.ActionLine(cat.kind, fmt.Sprintf("%s %d %s:", cat.label, n, skillWord))
				for _, name := range cat.names {
					fmt.Printf("      %s\n", name)
					if opts.showStat || opts.showPatch || cat.kind == "modified" {
						item := findDiffItem(items, name)
						if item != nil {
							item.ensureFiles()
							if len(item.files) > 0 {
								fmt.Print(renderFileStat(item.files))
							}
							if opts.showPatch {
								for _, f := range item.files {
									if f.Action == "modify" && item.srcDir != "" {
										srcFile := filepath.Join(item.srcDir, f.RelPath)
										dstFile := filepath.Join(item.dstDir, f.RelPath)
										diffText := generateUnifiedDiff(srcFile, dstFile)
										if diffText != "" {
											fmt.Printf("      --- %s\n", f.RelPath)
											fmt.Print(colorizePlainDiff(diffText))
										}
									}
								}
							}
						}
					}
				}
			} else {
				ui.ActionLine(cat.kind, fmt.Sprintf("%s %d %s", cat.label, n, skillWord))
			}
		}

		// Time info
		if !g.result.srcMtime.IsZero() || !g.result.dstMtime.IsZero() {
			if !g.result.srcMtime.IsZero() {
				fmt.Printf("  %sSource modified: %s%s\n", ui.Gray, g.result.srcMtime.Format("2006-01-02 15:04"), ui.Reset)
			}
			if !g.result.dstMtime.IsZero() {
				fmt.Printf("  %sTarget modified: %s%s\n", ui.Gray, g.result.dstMtime.Format("2006-01-02 15:04"), ui.Reset)
			}
		}
	}

	// Conditional hints
	if anySyncNeeded || anyForceNeeded || anyCollectNeeded {
		fmt.Println()
	}
	if anySyncNeeded || anyForceNeeded {
		if anyForceNeeded {
			ui.Info("Run 'skillshare sync' to apply changes, 'skillshare sync --force' to also replace local copies")
		} else {
			ui.Info("Run 'skillshare sync' to apply changes")
		}
	}
	if anyCollectNeeded {
		ui.Info("Run 'skillshare collect' to import local skills to source")
	}

	// Fully synced
	if len(syncedNames) > 0 {
		sort.Strings(syncedNames)
		ui.Success("%s: fully synced", strings.Join(syncedNames, ", "))
	}
}

func findDiffItem(items []copyDiffEntry, name string) *copyDiffEntry {
	for i := range items {
		if items[i].name == name {
			return &items[i]
		}
	}
	return nil
}

func colorizePlainDiff(diff string) string {
	if !ui.IsTTY() {
		return diff
	}
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(diff, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "+ "):
			b.WriteString(ui.Green + line + ui.Reset + "\n")
		case strings.HasPrefix(line, "- "):
			b.WriteString(ui.Red + line + ui.Reset + "\n")
		default:
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func renderOverallSummary(errCount, needCount, syncCount int) {
	var parts []string
	total := errCount + needCount + syncCount
	if errCount > 0 {
		parts = append(parts, fmt.Sprintf("%s%d error%s%s", ui.Red, errCount, pluralS(errCount), ui.Reset))
	}
	if needCount > 0 {
		parts = append(parts, fmt.Sprintf("%s%d need sync%s", ui.Yellow, needCount, ui.Reset))
	}
	if syncCount > 0 {
		parts = append(parts, fmt.Sprintf("%s%d synced%s", ui.Green, syncCount, ui.Reset))
	}
	if len(parts) > 0 {
		fmt.Printf("\n%sSummary:%s %d target%s — %s\n", ui.Bold, ui.Reset, total, pluralS(total), strings.Join(parts, ", "))
	}
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func printDiffHelp() {
	fmt.Println(`Usage: skillshare diff [target] [options]

Show differences between source skills and target directories.
Previews what 'sync' would change without modifying anything.

Arguments:
  target               Target name to diff (optional; diffs all if omitted)

Options:
  --project, -p        Diff project-level skills (.skillshare/)
  --global, -g         Diff global skills (~/.config/skillshare)
  --stat               Show file-level changes (implies --no-tui)
  --patch              Show full unified diff (implies --no-tui)
  --no-tui             Plain text output (skip interactive TUI)
  --help, -h           Show this help

Examples:
  skillshare diff                      # Diff all targets
  skillshare diff claude               # Diff a single target
  skillshare diff -p                   # Diff project-mode targets
  skillshare diff --stat               # Show file-level stat
  skillshare diff --patch              # Show full text diff`)
}
