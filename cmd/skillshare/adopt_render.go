package main

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"

	"skillshare/internal/adopt"
	"skillshare/internal/ui"
)

// adoptOptions holds parsed CLI flags for `skillshare adopt`.
type adoptOptions struct {
	dryRun     bool
	force      bool
	all        bool
	jsonOutput bool
	targetName string
}

// lockWarning describes a skill that was adopted but is still referenced in the
// external tool's lockfile. We never write the lockfile — we only warn.
type lockWarning struct {
	Name       string `json:"name"`
	SourceTool string `json:"source_tool"`
}

// adoptResult is the outcome of an adopt run, used for rendering and oplog.
type adoptResult struct {
	Adopted      []string // skill names migrated into source
	Skipped      []string // skills skipped (conflict, no --force)
	Failed       map[string]string
	Trashed      int           // originals moved to trash
	Pruned       int           // orphan symlinks removed across targets
	LockWarnings []lockWarning // adopted skills still present in the lockfile
	DryRun       bool
}

// adoptJSONOutput is the JSON shape for `skillshare adopt --json`.
type adoptJSONOutput struct {
	Adopted      []string          `json:"adopted"`
	Skipped      []string          `json:"skipped"`
	Failed       map[string]string `json:"failed"`
	Trashed      int               `json:"trashed"`
	Pruned       int               `json:"pruned"`
	LockWarnings []lockWarning     `json:"lock_warnings"`
	DryRun       bool              `json:"dry_run"`
	Duration     string            `json:"duration"`
}

func newAdoptResult(dryRun bool) *adoptResult {
	return &adoptResult{Failed: make(map[string]string), DryRun: dryRun}
}

// renderAdoptPreview lists the detected candidates before any changes are made.
func renderAdoptPreview(candidates []adopt.Candidate) {
	ui.Header(ui.WithModeLabel("Adoptable skills found"))
	for _, c := range candidates {
		detail := c.Path
		if c.SourceTool != "" {
			detail = fmt.Sprintf("[%s] %s", c.SourceTool, c.Path)
		}
		status := "info"
		if c.Conflict {
			status = "warning"
			detail = "conflict: already in source — use --force to overwrite · " + detail
		}
		ui.ListItem(status, c.Name, detail)
		if n := len(c.ExternalLinks); n > 0 {
			ui.ListItem("info", "", fmt.Sprintf("  %d orphan symlink(s) to clean", n))
		}
	}
}

// renderAdoptResult prints the outcome of an adopt run in human-readable form.
func renderAdoptResult(res *adoptResult, source string) error {
	ui.Header(ui.WithModeLabel("Adopting skills"))

	for _, name := range res.Adopted {
		ui.StepDone(name, "migrated to source")
	}
	for _, name := range res.Skipped {
		ui.StepSkip(name, "already exists in source, use --force to overwrite")
	}
	for name, msg := range res.Failed {
		ui.StepFail(name, msg)
	}

	ui.OperationSummary("Adopt", 0,
		ui.Metric{Label: "adopted", Count: len(res.Adopted), HighlightColor: pterm.Green},
		ui.Metric{Label: "trashed", Count: res.Trashed, HighlightColor: pterm.Yellow},
		ui.Metric{Label: "pruned", Count: res.Pruned, HighlightColor: pterm.Yellow},
		ui.Metric{Label: "failed", Count: len(res.Failed), HighlightColor: pterm.Red},
	)

	renderLockWarnings(res.LockWarnings)

	if len(res.Adopted) > 0 {
		fmt.Println()
		ui.Info("Synced to all targets. Source of truth: %s", source)
	}

	return nil
}

// renderLockWarnings warns about lingering lockfile entries. We never touch the
// lockfile; the owning tool must release them.
func renderLockWarnings(warnings []lockWarning) {
	if len(warnings) == 0 {
		return
	}
	fmt.Println()
	ui.Warning("Adopted skills are still tracked in %s:", adopt.LockFileName())
	for _, w := range warnings {
		if w.SourceTool != "" {
			ui.ListItem("warning", w.Name, fmt.Sprintf("owned by %s — run its uninstall to release", w.SourceTool))
		} else {
			ui.ListItem("warning", w.Name, "run the owning tool's uninstall to release the lock")
		}
	}
	ui.Info("skillshare never modifies the lockfile; release entries from the owning tool.")
}

// adoptResultToJSON converts an adoptResult to its JSON output form.
func adoptResultToJSON(res *adoptResult, start time.Time) *adoptJSONOutput {
	out := &adoptJSONOutput{
		Failed:       make(map[string]string),
		LockWarnings: []lockWarning{},
		DryRun:       res.DryRun,
		Duration:     formatDuration(start),
	}
	out.Adopted = res.Adopted
	out.Skipped = res.Skipped
	out.Trashed = res.Trashed
	out.Pruned = res.Pruned
	if res.LockWarnings != nil {
		out.LockWarnings = res.LockWarnings
	}
	for k, v := range res.Failed {
		out.Failed[k] = v
	}
	return out
}
