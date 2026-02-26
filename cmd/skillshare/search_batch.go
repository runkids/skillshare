package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"
	"time"

	"github.com/pterm/pterm"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/search"
	"skillshare/internal/ui"
)

// searchInstallResult captures the outcome of a single skill install (no UI output).
type searchInstallResult struct {
	name     string
	source   string
	status   string // "installed", "skipped", "failed"
	detail   string
	warnings []string
}

// searchInstallProgress displays batch install progress using pterm.AreaPrinter.
// Follows the same pattern as syncProgress (sync_parallel.go) and diffProgress (diff.go).
type searchInstallProgress struct {
	names   []string
	states  []string // "queued", "installing", "done", "skipped", "error"
	details []string
	total   int
	done    int
	area    *pterm.AreaPrinter
	mu      gosync.Mutex
	stopCh  chan struct{}
	frames  []string
	frame   int
	isTTY   bool
}

func newSearchInstallProgress(names []string) *searchInstallProgress {
	sp := &searchInstallProgress{
		names:   names,
		states:  make([]string, len(names)),
		details: make([]string, len(names)),
		total:   len(names),
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

func (sp *searchInstallProgress) render() {
	if sp.area == nil {
		return
	}
	var lines []string
	for i, name := range sp.names {
		var line string
		switch sp.states[i] {
		case "queued":
			line = fmt.Sprintf("  %s  %s", pterm.Gray(name), pterm.Gray("queued"))
		case "installing":
			spin := pterm.Cyan(sp.frames[sp.frame])
			line = fmt.Sprintf("  %s %s  %s", spin, pterm.Cyan(name), pterm.Gray("installing..."))
		case "done":
			line = fmt.Sprintf("  %s %s  %s", pterm.Green("✓"), name, pterm.Gray(sp.details[i]))
		case "skipped":
			line = fmt.Sprintf("  %s %s  %s", pterm.Yellow("⚠"), name, pterm.Gray(sp.details[i]))
		case "error":
			line = fmt.Sprintf("  %s %s  %s", pterm.Red("✗"), name, pterm.Gray(sp.details[i]))
		}
		lines = append(lines, line)
	}
	// Progress bar at bottom
	lines = append(lines, "", "  "+sp.renderBar())
	sp.area.Update(strings.Join(lines, "\n"))
}

func (sp *searchInstallProgress) renderBar() string {
	const barWidth = 30
	filled := sp.done * barWidth / sp.total
	if filled > barWidth {
		filled = barWidth
	}
	pct := int(math.Round(float64(sp.done) * 100 / float64(sp.total)))
	filledBar := pterm.Cyan(strings.Repeat("█", filled))
	emptyBar := pterm.Gray(strings.Repeat("█", barWidth-filled))
	count := fmt.Sprintf("%d/%d", sp.done, sp.total)
	return fmt.Sprintf("%s%s %s %d%%", filledBar, emptyBar, pterm.Gray(count), pct)
}

func (sp *searchInstallProgress) startSkill(name string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	for i, n := range sp.names {
		if n == name {
			sp.states[i] = "installing"
			break
		}
	}
	if !sp.isTTY {
		fmt.Printf("  %s: installing...\n", name)
	}
}

func (sp *searchInstallProgress) doneSkill(name string, r searchInstallResult) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.done++
	for i, n := range sp.names {
		if n != name {
			continue
		}
		switch r.status {
		case "installed":
			sp.states[i] = "done"
			sp.details[i] = "installed"
		case "skipped":
			sp.states[i] = "skipped"
			sp.details[i] = r.detail
		case "failed":
			sp.states[i] = "error"
			sp.details[i] = r.detail
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

func (sp *searchInstallProgress) stop() {
	if sp.stopCh != nil {
		close(sp.stopCh)
	}
	if sp.area != nil {
		sp.area.Stop() //nolint:errcheck
	}
}

// collectSearchInstallGlobal installs a search result in global mode without UI output.
func collectSearchInstallGlobal(result search.SearchResult, cfg *config.Config) searchInstallResult {
	r := searchInstallResult{
		name:   result.Name,
		source: result.Source,
	}

	source, err := install.ParseSource(result.Source)
	if err != nil {
		r.status = "failed"
		r.detail = fmt.Sprintf("invalid source: %v", err)
		return r
	}

	destPath := filepath.Join(cfg.Source, result.Name)

	// Check if already exists
	if _, err := os.Stat(destPath); err == nil {
		r.status = "skipped"
		r.detail = "already exists"
		return r
	}

	opts := install.InstallOptions{}
	if result.Skill != "" {
		opts.Skills = []string{result.Skill}
	}

	installResult, err := install.Install(source, destPath, opts)
	if err != nil {
		r.status = "failed"
		r.detail = err.Error()
		return r
	}

	r.status = "installed"
	r.warnings = installResult.Warnings

	// Reconcile global config
	reg, _ := config.LoadRegistry(filepath.Dir(config.ConfigPath()))
	if reg == nil {
		reg = &config.Registry{}
	}
	if rErr := config.ReconcileGlobalSkills(cfg, reg); rErr != nil {
		r.warnings = append(r.warnings, fmt.Sprintf("reconcile: %v", rErr))
	}

	return r
}

// collectSearchInstallProject installs a search result in project mode without UI output.
func collectSearchInstallProject(result search.SearchResult, cwd string) searchInstallResult {
	r := searchInstallResult{
		name:   result.Name,
		source: result.Source,
	}

	// Auto-init project if not yet initialized
	if !projectConfigExists(cwd) {
		if err := performProjectInit(cwd, projectInitOptions{}); err != nil {
			r.status = "failed"
			r.detail = fmt.Sprintf("project init: %v", err)
			return r
		}
	}

	runtime, err := loadProjectRuntime(cwd)
	if err != nil {
		r.status = "failed"
		r.detail = fmt.Sprintf("load config: %v", err)
		return r
	}

	source, err := install.ParseSource(result.Source)
	if err != nil {
		r.status = "failed"
		r.detail = fmt.Sprintf("invalid source: %v", err)
		return r
	}

	destPath := filepath.Join(runtime.sourcePath, result.Name)

	// Check if already exists
	if _, err := os.Stat(destPath); err == nil {
		r.status = "skipped"
		r.detail = "already exists"
		return r
	}

	opts := install.InstallOptions{}
	if result.Skill != "" {
		opts.Skills = []string{result.Skill}
	}

	installResult, err := install.Install(source, destPath, opts)
	if err != nil {
		r.status = "failed"
		r.detail = err.Error()
		return r
	}

	r.status = "installed"
	r.warnings = installResult.Warnings

	// Update .gitignore
	if err := install.UpdateGitIgnore(filepath.Join(runtime.root, ".skillshare"), filepath.Join("skills", result.Name)); err != nil {
		r.warnings = append(r.warnings, fmt.Sprintf("gitignore: %v", err))
	}

	// Reconcile project config
	if err := reconcileProjectRemoteSkills(runtime); err != nil {
		r.warnings = append(r.warnings, fmt.Sprintf("reconcile: %v", err))
	}

	return r
}

// batchInstallFromSearchWithProgress installs multiple search results with progress display.
func batchInstallFromSearchWithProgress(selected []search.SearchResult, mode runMode, cwd string) error {
	var cfg *config.Config
	if mode != modeProject {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	names := make([]string, len(selected))
	for i, r := range selected {
		names[i] = r.Name
	}

	// Move cursor up one line to eat the blank line left by bubbletea TUI exit,
	// then print │ + ├─ Installing as a connected tree branch.
	msg := fmt.Sprintf("%d skill(s)", len(selected))
	if ui.IsTTY() {
		fmt.Print("\033[A") // cursor up — overwrite TUI's trailing blank line
		fmt.Printf("%s\n", pterm.Gray(ui.StepLine))
		fmt.Printf("%s %s  %s\n",
			pterm.Gray(ui.StepBranch+"─"), pterm.Gray("Installing"), pterm.White(msg))
	} else {
		fmt.Printf("%s─ %s  %s\n", ui.StepBranch, "Installing", msg)
	}

	batchStart := time.Now()
	progress := newSearchInstallProgress(names)

	results := make([]searchInstallResult, len(selected))
	for i, sr := range selected {
		progress.startSkill(sr.Name)

		start := time.Now()
		var r searchInstallResult
		if mode == modeProject {
			r = collectSearchInstallProject(sr, cwd)
		} else {
			r = collectSearchInstallGlobal(sr, cfg)
		}
		results[i] = r

		// Log per-skill oplog entry
		var opErr error
		if r.status == "failed" {
			opErr = fmt.Errorf("%s", r.detail)
		}
		modeStr := "global"
		if mode == modeProject {
			modeStr = "project"
		}
		logSummary := installLogSummary{
			Source: sr.Source,
			Mode:   modeStr,
		}
		if r.status == "installed" {
			logSummary.SkillCount = 1
			logSummary.InstalledSkills = []string{r.name}
		} else if r.status == "failed" {
			logSummary.FailedSkills = []string{r.name}
		}
		cfgPath := config.ConfigPath()
		if mode == modeProject {
			cfgPath = config.ProjectConfigPath(cwd)
		}
		logInstallOp(cfgPath, []string{sr.Source}, start, opErr, logSummary)

		progress.doneSkill(sr.Name, r)
	}

	progress.stop()
	renderBatchSearchInstallSummary(results, mode, time.Since(batchStart))

	// Return error if any failed
	var failed []string
	for _, r := range results {
		if r.status == "failed" {
			failed = append(failed, r.name)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to install %d skill(s): %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

// renderBatchSearchInstallSummary renders the final summary after batch install.
func renderBatchSearchInstallSummary(results []searchInstallResult, mode runMode, elapsed time.Duration) {
	var installed, skipped, failed int
	var totalWarnings int
	for _, r := range results {
		switch r.status {
		case "installed":
			installed++
		case "skipped":
			skipped++
		case "failed":
			failed++
		}
		totalWarnings += len(r.warnings)
	}

	// Close tree with result: └─ ✓ SUCCESS / ✗ ERROR
	parts := []string{fmt.Sprintf("Installed %d skill(s)", installed)}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	status := "success"
	if failed > 0 {
		status = "error"
	}
	ui.StepResult(status, strings.Join(parts, ", "), elapsed)

	// Per-skill status
	fmt.Println()
	for _, r := range results {
		switch r.status {
		case "installed":
			fmt.Printf("  %s✓%s %s\n", ui.Green, ui.Reset, r.name)
		case "skipped":
			fmt.Printf("  %s⚠%s %-20s %s%s%s\n", ui.Yellow, ui.Reset, r.name, ui.Gray, r.detail, ui.Reset)
		case "failed":
			fmt.Printf("  %s✗%s %-20s %s%s%s\n", ui.Red, ui.Reset, r.name, ui.Gray, r.detail, ui.Reset)
		}
	}

	// Warnings summary
	if totalWarnings > 0 {
		fmt.Println()
		ui.Warning("%d warning(s) during install", totalWarnings)
	}

	// Sync hint
	fmt.Println()
	if mode == modeProject {
		ui.Info("Run 'skillshare sync' to distribute to project targets")
	} else {
		ui.Info("Run 'skillshare sync' to distribute to all targets")
	}
}
