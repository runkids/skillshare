package main

import (
	"fmt"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/projects"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
)

// Keep the CLI contract explicit: ExecuteResult also contains an internal
// discovery snapshot that is neither a stable API nor useful machine output.
type projectSyncJSON struct {
	Results         []ssync.TargetResult           `json:"results"`
	Warnings        []string                       `json:"warnings"`
	Unmatched       []ssync.TargetUnmatchedInclude `json:"unmatched"`
	Failed          []ssync.TargetFailure          `json:"failed"`
	FolderConflicts []config.SkillsFolderConflict  `json:"folder_conflicts"`
	PathOverlap     int                            `json:"path_overlap"`
	ssync.IgnoreReport
}
type projectResultJSON struct {
	Action       string                 `json:"action"`
	Project      *projects.Item         `json:"project"`
	Applied      *bool                  `json:"applied,omitempty"`
	Unchanged    bool                   `json:"unchanged,omitempty"`
	Affected     []string               `json:"affected"`
	Warnings     []string               `json:"warnings"`
	Sync         *projectSyncJSON       `json:"sync,omitempty"`
	Detach       *projects.DetachReport `json:"detach,omitempty"`
	Registration string                 `json:"registration,omitempty"`
	Error        string                 `json:"error,omitempty"`
}

// projectJSONItem preserves the CLI's explicit off state without changing the
// dashboard DTO's existing representation of disabled resource settings.
func projectJSONItem(item projects.Item) projects.Item {
	if item.Skills != nil && item.Skills.Disabled {
		item.Skills = nil
	}
	if item.Agents != nil && item.Agents.Disabled {
		item.Agents = nil
	}
	return item
}

func projectSlice[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
func projectSyncPayload(r *ssync.ExecuteResult, agentsSource string) *projectSyncJSON {
	if r == nil {
		return nil
	}
	// Mirror the dashboard sync envelope from the agents the sync discovered.
	p := &projectSyncJSON{Results: projectSlice(r.Results), Warnings: projectSlice(r.Warnings), Unmatched: projectSlice(r.Unmatched), Failed: projectSlice(r.Failed), FolderConflicts: projectSlice(r.FolderConflicts), PathOverlap: r.PathOverlap, IgnoreReport: ssync.NewIgnoreReport(r.IgnoreStats, agentsSource, r.AllAgents)}
	for i := range p.Results {
		row := &p.Results[i]
		row.Linked = projectSlice(row.Linked)
		row.Updated = projectSlice(row.Updated)
		row.Skipped = projectSlice(row.Skipped)
		row.Pruned = projectSlice(row.Pruned)
	}
	return p
}
func renderProjectResult(o projectOptions, r projectResult, err error, start time.Time) error {
	unchanged := (o.action == "edit" || o.action == "add") && len(r.Affected) == 0
	applied := !o.dryRun && r.Registration == "updated"
	if o.action == "remove" {
		applied = r.Registration == "removed"
	}
	p := projectResultJSON{Action: o.action, Project: r.Item, Applied: &applied, Unchanged: unchanged, Affected: projectSlice(r.Affected), Warnings: projectSlice(r.Warnings), Registration: r.Registration}
	if r.Sync != nil {
		agentsSource := ""
		if r.Expanded != nil {
			agentsSource = r.Expanded.EffectiveAgentsSource()
		}
		p.Sync = projectSyncPayload(r.Sync, agentsSource)
	}
	if p.Project != nil {
		item := projectJSONItem(*p.Project)
		p.Project = &item
	}
	if o.action == "sync" && !o.dryRun {
		p.Applied = nil
	}
	if r.Detach != nil {
		d := *r.Detach
		d.Removed = projectSlice(d.Removed)
		d.CopiedBack = projectSlice(d.CopiedBack)
		d.Retained = projectSlice(d.Retained)
		p.Detach = &d
	}
	if err != nil {
		p.Error = err.Error()
	}
	if o.json {
		return writeJSONResult(p, err)
	}
	if r.Item != nil {
		renderProjectItem(*r.Item)
	}
	for _, root := range r.Affected {
		ui.Row(ui.MarkNone, "Affected", root, ui.RowWidth("Affected"))
	}
	if r.Detach != nil {
		for _, path := range r.Detach.Removed {
			ui.Row(ui.MarkOK, "Removed", path, ui.RowWidth("Copied back"))
		}
		for _, path := range r.Detach.CopiedBack {
			ui.Row(ui.MarkOK, "Copied back", path, ui.RowWidth("Copied back"))
		}
		for _, path := range r.Detach.Retained {
			ui.Row(ui.MarkNone, "Retained", path, ui.RowWidth("Copied back"))
		}
		ui.Row(ui.MarkNone, "Registration", r.Registration, ui.RowWidth("Registration"))
	}
	if r.Sync != nil {
		for _, unmatched := range r.Sync.Unmatched {
			for _, pattern := range unmatched.Patterns {
				ui.Warning("%s: %s", unmatched.Target, (ssync.UnmatchedInclude{Pattern: pattern, Suggestions: unmatched.Suggestions}).Warning())
			}
		}
		var targets map[string]config.TargetConfig
		var managed map[string]config.ManagedProject
		if r.Expanded != nil {
			targets = r.Expanded.Targets
			managed = r.Expanded.Projects
		}
		printSyncFolderConflicts(r.Sync.FolderConflicts, r.Sync.PathOverlap, false, targets, managed)
		for _, row := range r.Sync.Results {
			ui.Row(ui.MarkOK, row.Target, fmt.Sprintf("%d linked, %d updated, %d skipped, %d pruned", len(row.Linked), len(row.Updated), len(row.Skipped), len(row.Pruned)), ui.RowWidth(row.Target))
		}
		for _, failure := range r.Sync.Failed {
			ui.Row(ui.MarkFail, failure.Target, failure.Error, ui.RowWidth(failure.Target))
		}
	}
	for _, warning := range r.Warnings {
		ui.Warning("%s", warning)
	}
	if o.dryRun {
		ui.DryRun()
	} else if unchanged {
		ui.Done(ui.MarkNone, "Project unchanged", time.Since(start))
	} else if err == nil {
		ui.Done(ui.MarkOK, "Project "+o.action+" complete", time.Since(start))
	}
	if err == nil && !o.dryRun {
		ui.Next("skillshare project list", "Inspect registered projects")
	}
	return err
}
func renderProjectItem(item projects.Item) {
	width := ui.RowWidth("Project", "Root", "Targets", "Skills", "Agents")
	ui.Row(ui.MarkNone, "Project", item.Name, width)
	ui.Row(ui.MarkNone, "Root", item.Root, width)
	ui.Row(ui.MarkNone, "Targets", strings.Join(item.Targets, ", "), width)
	for _, kind := range []string{"Skills", "Agents"} {
		r := item.Skills
		if kind == "Agents" {
			r = item.Agents
		}
		value := "off"
		if r != nil && !r.Disabled {
			value = "on"
			if r.Mode != "" {
				value += " (" + r.Mode + ")"
			}
		}
		ui.Row(ui.MarkNone, kind, value, width)
	}
	if item.Missing {
		ui.Warning("Project folder is missing: %s", item.Path)
	}
}
