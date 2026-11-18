package main

import (
	"fmt"
	"slices"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/projects"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
)

// projectScanRow adds a scanned project's sync to its scan result.
type projectScanRow struct {
	projects.ScanRow
	Sync   *projectSyncJSON `json:"sync,omitempty"`
	Error  string           `json:"error,omitempty"`
	synced *ssync.ExecuteResult
}

type projectScanJSON struct {
	projectResultJSON
	Scan []projectScanRow `json:"scan"`
}

// runProjectScan runs project add --scan, then syncs the root and every added
// or kept checkout when --sync is set. release unlocks the config before syncing.
func runProjectScan(o projectOptions, cfg *config.Config, service *projects.Service, root string, entry config.ManagedProject, storage string, release func(), start time.Time) error {
	scan, err := service.Scan(root, entry, o.values["--scan"], projects.PutOptions{DryRun: o.dryRun, Storage: storage})
	if release != nil {
		release()
	}
	if err != nil {
		return err
	}
	result := projectResult{Result: scan.Result}
	for _, w := range scan.Warnings {
		result.Warnings = append(result.Warnings, w.String())
	}
	if !o.dryRun && result.Registration == "updated" {
		logProject(cfg, "project", "add", root, "ok", nil, start, map[string]any{"scan": o.values["--scan"], "added": scan.Added()})
	}

	rows := make([]projectScanRow, len(scan.Rows))
	failed := 0
	for i, scanned := range scan.Rows {
		row := &rows[i]
		row.ScanRow = scanned
		if !o.sync || scanned.Item == nil {
			continue
		}
		synced, err := syncProject(result.Expanded, scanned.Item.Root, o.dryRun)
		err = projectSyncError(synced, err, o.dryRun)
		if synced != nil {
			row.synced = synced
			row.Sync = projectSyncPayload(synced, result.Expanded.EffectiveAgentsSource())
			for _, w := range synced.Warnings {
				if !slices.Contains(result.Warnings, w) {
					result.Warnings = append(result.Warnings, w)
				}
			}
		}
		if err != nil {
			failed++
			row.Error = err.Error()
		}
		if !o.dryRun {
			status := projectSyncStatus(projectResult{Result: projects.Result{Item: scanned.Item, Expanded: result.Expanded}, Sync: synced}, err)
			logProject(result.Expanded, "sync", "sync", scanned.Item.Root, status, err, start, nil)
		}
	}
	var opErr error
	if failed > 0 {
		opErr = fmt.Errorf("sync failed for %d of the scanned projects", failed)
	}
	return renderProjectScan(o, result, rows, opErr, start)
}

func renderProjectScan(o projectOptions, r projectResult, rows []projectScanRow, err error, start time.Time) error {
	unchanged := len(r.Affected) == 0
	if o.json {
		applied := !o.dryRun && r.Registration == "updated"
		p := projectScanJSON{projectResultJSON: projectResultJSON{Action: "add", Project: r.Item, Applied: &applied, Unchanged: unchanged, Affected: projectSlice(r.Affected), Warnings: projectSlice(r.Warnings), Registration: r.Registration}, Scan: rows}
		if p.Project != nil {
			item := projectJSONItem(*p.Project)
			p.Project = &item
		}
		if err != nil {
			p.Error = err.Error()
		}
		return writeJSONResult(p, err)
	}
	width := ui.RowWidth("Skipped")
	for _, row := range rows {
		mark, label := ui.MarkOK, "Added"
		switch row.Result {
		case projects.ScanKept:
			mark, label = ui.MarkNone, "Kept"
		case projects.ScanSkipped:
			mark, label = ui.MarkWarn, "Skipped"
		}
		value := row.Root
		if row.Reason != "" {
			value += "  " + row.Reason
		}
		ui.Row(mark, label, value, width)
		if row.synced != nil {
			renderProjectSync(row.synced, r.Expanded)
		}
	}
	for _, warning := range r.Warnings {
		ui.Warning("%s", warning)
	}
	if o.dryRun {
		ui.DryRun()
	} else if unchanged && !o.sync {
		ui.Done(ui.MarkNone, "Projects unchanged", time.Since(start))
	} else if err == nil {
		ui.Done(ui.MarkOK, "Project add complete", time.Since(start))
	}
	if err == nil && !o.dryRun {
		ui.Next("skillshare project list", "Inspect registered projects")
	}
	return err
}
