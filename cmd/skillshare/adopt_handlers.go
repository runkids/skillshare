package main

import (
	"fmt"
	"time"

	"skillshare/internal/adopt"
	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

// adoptContext carries the mode-specific inputs for the adopt flow so that the
// global and project handlers share a single orchestration core.
type adoptContext struct {
	agentsPath  string                         // universal/agents target skills dir (~/.agents/skills)
	sourcePath  string                         // skillshare source of truth
	syncMode    string                         // agents target sync mode
	defaultMode string                         // config-level sync mode (cfg.Mode); used when a target sets no mode
	projectRoot string                         // project root, empty in global mode
	allTargets  map[string]string              // name -> skills dir, for orphan-link pruning + re-sync
	targets     map[string]config.TargetConfig // resolved targets, for re-sync (optional)
	trashBase   string                         // trash dir (global or project)
	configPath  string                         // config path for oplog
}

// runAdoptCommand wires an adoptContext through detection, confirmation,
// migration, and rendering. It owns the oplog entry.
func runAdoptCommand(actx adoptContext, opts adoptOptions, start time.Time) error {
	candidates, err := adopt.DetectAdoptable(actx.agentsPath, actx.sourcePath, actx.syncMode, actx.allTargets)
	if err != nil {
		logAdoptOp(actx.configPath, start, err, nil)
		return adoptCommandError(err, opts.jsonOutput)
	}

	// Annotate provenance from the lockfile (read-only).
	lockEntries, _ := adopt.ReadLock(actx.agentsPath)
	for i := range candidates {
		candidates[i].SourceTool = adopt.Provenance(lockEntries, candidates[i].Name)
	}

	if len(candidates) == 0 {
		if opts.jsonOutput {
			err = adoptOutputJSON(newAdoptResult(opts.dryRun), start, nil)
			logAdoptOp(actx.configPath, start, err, nil)
			return err
		}
		ui.Header(ui.WithModeLabel("Adopt"))
		ui.Info("No adoptable skills found in %s", actx.agentsPath)
		logAdoptOp(actx.configPath, start, nil, nil)
		return nil
	}

	if !opts.jsonOutput {
		ui.Header(ui.WithModeLabel("Adopt"))
		renderAdoptPreview(candidates)
	}

	// Selection: confirm interactively unless --all + --force (or JSON).
	selected, cancelled := selectAdoptCandidates(candidates, opts)
	if cancelled {
		ui.Info("Cancelled")
		logAdoptOp(actx.configPath, start, nil, nil)
		return nil
	}
	if len(selected) == 0 {
		ui.Info("Nothing selected")
		logAdoptOp(actx.configPath, start, nil, nil)
		return nil
	}

	res, err := applyAdopt(actx, selected, opts)
	if opts.jsonOutput {
		jsonErr := adoptOutputJSON(res, start, err)
		logAdoptOp(actx.configPath, start, err, res)
		if err != nil {
			return jsonErr
		}
		return jsonErr
	}
	logAdoptOp(actx.configPath, start, err, res)
	if err != nil {
		return err
	}
	return renderAdoptResult(res, actx.sourcePath)
}

// selectAdoptCandidates resolves which candidates to adopt. With --all+--force
// (or JSON output) all are selected without prompting; otherwise an interactive
// checklist is shown. Returns (selected, cancelled).
func selectAdoptCandidates(candidates []adopt.Candidate, opts adoptOptions) ([]adopt.Candidate, bool) {
	// Explicit non-interactive opt-in: JSON output or --all --force.
	if opts.jsonOutput || (opts.all && opts.force) {
		return candidates, false
	}
	// Non-interactive terminal (CI, pipe): an explicit selection flag (--all,
	// --dry-run) proceeds — with --all but no --force, conflicts are still
	// skipped downstream. A bare run must NOT silently migrate + trash
	// originals, so refuse and point at an explicit flag. Preview already shown.
	if !shouldLaunchTUI(false, nil) {
		if opts.all || opts.dryRun {
			return candidates, false
		}
		ui.Warning("Non-interactive terminal: pass --all to adopt (add --force to overwrite conflicts), or --dry-run to preview.")
		return nil, true
	}

	items := make([]checklistItemData, len(candidates))
	for i, c := range candidates {
		desc := c.Path
		if c.SourceTool != "" {
			desc = fmt.Sprintf("[%s] %s", c.SourceTool, c.Path)
		}
		if c.Conflict {
			desc = "conflict — overwrites source · " + desc
		}
		items[i] = checklistItemData{label: c.Name, desc: desc, preSelected: !c.Conflict}
	}

	idxs, err := runChecklistTUI(checklistConfig{
		title:    "Adopt skills into skillshare",
		itemName: "skill",
		items:    items,
	})
	if err != nil || idxs == nil {
		return nil, true
	}

	picked := make([]adopt.Candidate, 0, len(idxs))
	for _, i := range idxs {
		picked = append(picked, candidates[i])
	}
	return picked, false
}

// applyAdopt performs the migration via the shared adopt.Apply orchestration
// (copy into source, trash the original, prune orphan symlinks, re-sync to all
// targets), then adapts the result to the CLI's render/oplog shape.
func applyAdopt(actx adoptContext, selected []adopt.Candidate, opts adoptOptions) (*adoptResult, error) {
	names := make([]string, len(selected))
	for i, c := range selected {
		names[i] = c.Name
	}
	out, err := adopt.Apply(selected, adopt.Request{
		AgentsPath:  actx.agentsPath,
		SourcePath:  actx.sourcePath,
		SyncMode:    actx.syncMode,
		DefaultMode: actx.defaultMode,
		ProjectRoot: actx.projectRoot,
		TrashBase:   actx.trashBase,
		AllTargets:  actx.allTargets,
		Targets:     actx.targets,
		DryRun:      opts.dryRun,
		Force:       opts.force,
		Selected:    names,
	})
	return adoptResultFromApply(out, opts.dryRun), err
}

// adoptResultFromApply converts a shared adopt.Result into the CLI's adoptResult.
func adoptResultFromApply(out *adopt.Result, dryRun bool) *adoptResult {
	res := newAdoptResult(dryRun)
	if out == nil {
		return res
	}
	res.Adopted = out.Adopted
	res.Skipped = out.Skipped
	res.Trashed = out.Trashed
	res.Pruned = out.PrunedLinks
	res.DryRun = out.DryRun
	for k, v := range out.Failed {
		res.Failed[k] = v
	}
	for _, w := range out.LockWarnings {
		res.LockWarnings = append(res.LockWarnings, lockWarning{Name: w.Name, SourceTool: w.SourceTool})
	}
	return res
}

// runAdopt is a thin testable wrapper around the migration core (no UI/oplog).
func runAdopt(actx adoptContext, opts adoptOptions) (*adoptResult, error) {
	candidates, err := adopt.DetectAdoptable(actx.agentsPath, actx.sourcePath, actx.syncMode, actx.allTargets)
	if err != nil {
		return newAdoptResult(opts.dryRun), err
	}
	lockEntries, _ := adopt.ReadLock(actx.agentsPath)
	for i := range candidates {
		candidates[i].SourceTool = adopt.Provenance(lockEntries, candidates[i].Name)
	}
	if len(candidates) == 0 {
		return newAdoptResult(opts.dryRun), nil
	}
	return applyAdopt(actx, candidates, opts)
}

func adoptCommandError(err error, jsonOutput bool) error {
	if err == nil {
		return nil
	}
	if jsonOutput {
		return writeJSONError(err)
	}
	return err
}

func adoptOutputJSON(res *adoptResult, start time.Time, adoptErr error) error {
	return writeJSONResult(adoptResultToJSON(res, start), adoptErr)
}

func logAdoptOp(cfgPath string, start time.Time, cmdErr error, res *adoptResult) {
	status := statusFromErr(cmdErr)
	if cmdErr == nil && res != nil && len(res.Failed) > 0 {
		status = "partial"
	}

	e := oplog.NewEntry("adopt", status, time.Since(start))
	args := map[string]any{
		"adopted": 0,
		"trashed": 0,
		"pruned":  0,
	}
	if res != nil {
		args["adopted"] = len(res.Adopted)
		args["trashed"] = res.Trashed
		args["pruned"] = res.Pruned
		args["dry_run"] = res.DryRun
	}
	e.Args = args
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}
