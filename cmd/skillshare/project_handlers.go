package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/projects"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
)

func runProject(o projectOptions) error {
	start := time.Now()
	path := config.ReadOnlyConfigPath()
	var release func()
	confirmBeforeLock := o.action == "remove" && !o.dryRun && !o.yes
	if !confirmBeforeLock && !o.dryRun && slices.Contains([]string{"add", "edit", "remove"}, o.action) {
		var err error
		release, err = projects.LockConfig(context.Background(), path)
		if err != nil {
			return err
		}
		defer release()
	}
	cfg, err := config.LoadReadOnlyPath(path)
	if err != nil {
		return err
	}
	service := projects.NewService(path, func() (*config.Config, error) { return cfg, nil })
	if o.action == "list" {
		items, err := service.List()
		if err != nil {
			return err
		}
		if o.json {
			for i := range items {
				items[i] = projectJSONItem(items[i])
			}
			return writeJSON(struct {
				Projects []projects.Item `json:"projects"`
			}{items})
		}
		for _, item := range items {
			renderProjectItem(item)
		}
		if len(items) == 0 {
			ui.Note("No projects registered. Use skillshare project add <root> --target T.")
		}
		return nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	var root string
	if o.action == "add" {
		root, err = projects.StoredKey(o.token, cwd, home)
		if err == nil {
			found, findErr := projects.FindRoot(cfg.Projects, root, cwd, home)
			if findErr == nil {
				root = found
			} else if !errors.Is(findErr, projects.ErrNotFound) {
				return findErr
			}
		}
	} else {
		root, err = projects.Find(cfg.Projects, o.token, cwd, home)
	}
	if err != nil {
		return err
	}

	if o.action == "show" {
		item, err := service.Show(root)
		if err != nil {
			return err
		}
		if o.json {
			item = projectJSONItem(item)
			return writeJSON(struct {
				Project projects.Item `json:"project"`
			}{item})
		}
		renderProjectItem(item)
		return nil
	}
	if confirmBeforeLock {
		confirmedEntry := cfg.Projects[root]
		ok, err := ui.ConfirmAction("Remove project "+root+"?", false)
		if err != nil {
			return err
		}
		if !ok {
			ui.Cancelled("removed")
			return nil
		}
		// A person can leave the prompt open indefinitely. Hold the shared lock
		// only after consent, then re-resolve and reject a changed registration.
		release, err = projects.LockConfig(context.Background(), path)
		if err != nil {
			return err
		}
		defer release()
		cfg, err = config.LoadReadOnlyPath(path)
		if err != nil {
			return err
		}
		latestRoot, err := projects.Find(cfg.Projects, o.token, cwd, home)
		if err != nil || latestRoot != root || !reflect.DeepEqual(confirmedEntry, cfg.Projects[latestRoot]) {
			return fmt.Errorf("project changed while confirming; retry project remove")
		}
	}
	var result projectResult
	var opErr error
	switch o.action {
	case "add", "edit":
		entry := cfg.Projects[root]
		if o.action == "add" {
			entry = config.ManagedProject{Skills: &config.ResourceTargetConfig{}}
		}
		entry, err = patchProject(entry, o)
		if err != nil {
			return err
		}
		create := o.action == "add"
		if _, exists := cfg.Projects[root]; create && exists {
			if info, err := os.Stat(config.ExpandPath(root)); err != nil || !info.IsDir() {
				return fmt.Errorf("%w: %s", projects.ErrFolderNotFound, root)
			}
			candidate, err := projects.PrepareProjects(path, cfg, func(entries map[string]config.ManagedProject) error { entries[root] = entry; return nil })
			if err != nil {
				return err
			}
			if len(candidate.Affected) > 0 {
				return fmt.Errorf("%w: %s; use skillshare project edit", projects.ErrExists, root)
			}
			item, err := service.Show(root)
			if err != nil {
				return err
			}
			result.Result = projects.Result{Item: &item, Affected: candidate.Affected, Expanded: candidate.Save.Expanded, Registration: "kept", Detach: &projects.DetachReport{}}
		} else {
			result.Result, opErr = service.Put(root, entry, projects.PutOptions{Create: create, DryRun: o.dryRun, Detach: true})
		}
		if release != nil {
			release()
		}
		if opErr != nil {
			changed := result.Detach != nil && (len(result.Detach.Removed) > 0 || len(result.Detach.CopiedBack) > 0)
			if !changed {
				return opErr
			}
			if !o.dryRun {
				logProject(cfg, "project", o.action, root, "partial", opErr, start)
			}
			return renderProjectResult(o, result, opErr, start)
		}
		if !o.dryRun && len(result.Affected) > 0 {
			logProject(cfg, "project", o.action, root, "ok", nil, start)
		}
		if o.sync {
			// Dry-run sync must use the unsaved candidate, including renamed targets.
			cfg = result.Expanded
			synced, syncErr := syncProject(cfg, root, o.dryRun)
			result.Sync = synced
			if synced != nil {
				result.Warnings = append(result.Warnings, synced.Warnings...)
			}
			opErr = projectSyncError(synced, syncErr, o.dryRun)
			if !o.dryRun {
				logProject(cfg, "sync", "sync", root, projectSyncStatus(result, opErr), opErr, start)
			}
		}
	case "sync":
		item, err := service.Show(root)
		if err != nil {
			return err
		}
		result.Item, result.Expanded = &item, cfg
		result.Sync, opErr = syncProject(cfg, root, o.dryRun)
		if result.Sync != nil {
			result.Warnings = result.Sync.Warnings
		}
		opErr = projectSyncError(result.Sync, opErr, o.dryRun)
		if !o.dryRun {
			logProject(cfg, "sync", "sync", root, projectSyncStatus(result, opErr), opErr, start)
		}
		if opErr != nil && result.Sync == nil {
			return opErr
		}
	case "remove":
		result.Result, opErr = service.Remove(root, projects.RemoveOptions{DryRun: o.dryRun})
		changed := result.Detach != nil && (len(result.Detach.Removed) > 0 || len(result.Detach.CopiedBack) > 0)
		if opErr != nil && !changed {
			return opErr
		}
		if !o.dryRun && (opErr == nil || changed) {
			status := "ok"
			if opErr != nil {
				status = "partial"
			}
			logProject(cfg, "project", "remove", root, status, opErr, start)
		}
	}
	return renderProjectResult(o, result, opErr, start)
}

func patchProject(entry config.ManagedProject, o projectOptions) (config.ManagedProject, error) {
	entry.Targets = slices.Clone(entry.Targets)
	if values, ok := o.values["--target"]; ok {
		entry.Targets = nil
		for _, value := range values {
			for _, target := range strings.Split(value, ",") {
				target = strings.TrimSpace(target)
				if target != "" {
					entry.Targets = append(entry.Targets, target)
				}
			}
		}
	}
	if o.action == "add" && len(entry.Targets) == 0 {
		return entry, fmt.Errorf("project add requires at least one --target")
	}
	if values, ok := o.values["--skills-naming"]; ok && !config.IsValidTargetNaming(values[len(values)-1]) {
		return entry, fmt.Errorf("invalid skills naming %q (valid: %s)", values[len(values)-1], strings.Join(config.ValidTargetNamings, ", "))
	}
	if values, ok := o.values["--name"]; ok {
		entry.Name = values[len(values)-1]
		if err := projects.ValidateName(entry.Name); err != nil {
			return entry, err
		}
	}
	for _, kind := range []string{"skills", "agents"} {
		slot := &entry.Skills
		if kind == "agents" {
			slot = &entry.Agents
		}
		if *slot != nil {
			copy := **slot
			copy.Include = slices.Clone(copy.Include)
			copy.Exclude = slices.Clone(copy.Exclude)
			*slot = &copy
		}
		if o.toggles["--no-"+kind] {
			*slot = nil
		}
		if o.toggles["--"+kind] {
			if *slot == nil {
				*slot = &config.ResourceTargetConfig{}
			}
			(*slot).SetEnabled(true)
		}
		for _, field := range []string{"mode", "naming", "include", "exclude"} {
			values, ok := o.values["--"+kind+"-"+field]
			if !ok {
				continue
			}
			if *slot == nil {
				*slot = &config.ResourceTargetConfig{}
			}
			(*slot).SetEnabled(true)
			switch field {
			case "mode":
				(*slot).Mode = values[len(values)-1]
			case "naming":
				(*slot).TargetNaming = values[len(values)-1]
			case "include", "exclude":
				var patterns []string
				for _, p := range values {
					if p == "" {
						patterns = nil
					} else {
						patterns = append(patterns, p)
					}
				}
				if field == "include" {
					(*slot).Include = patterns
				} else {
					(*slot).Exclude = patterns
				}
			}
		}
	}
	// Reject only a conflict these flags create, so a registration saved with
	// both elsewhere stays editable.
	_, mode := o.values["--agents-mode"]
	_, include := o.values["--agents-include"]
	_, exclude := o.values["--agents-exclude"]
	if (mode || include || exclude) && entry.Agents != nil && entry.Agents.Mode == "symlink" && (len(entry.Agents.Include) > 0 || len(entry.Agents.Exclude) > 0) {
		return entry, fmt.Errorf("agent include/exclude filters are ignored in symlink mode; use --agents-mode merge or copy")
	}
	return entry, nil
}

func projectSyncError(synced *ssync.ExecuteResult, err error, preview bool) error {
	if err != nil || synced == nil || len(synced.Failed) == 0 {
		return err
	}
	if preview && !slices.ContainsFunc(synced.Failed, func(f ssync.TargetFailure) bool { return !f.Conflict }) {
		return nil
	}
	return fmt.Errorf("sync failed for %d target resources", len(synced.Failed))
}

// Count each target once, even when both its skills and agents fail, just as
// skillshare sync and the dashboard do.
func projectSyncStatus(result projectResult, err error) string {
	if result.Sync != nil && result.Item != nil && result.Expanded != nil {
		total := 0
		skills := result.Item.Skills != nil && !result.Item.Skills.Disabled
		for _, group := range result.Item.Groups {
			target, ok := result.Expanded.Targets[group.Target]
			if ok && (skills || (target.Agents != nil && target.Agents.IsEnabled())) {
				total++
			}
		}
		failed := map[string]bool{}
		for _, failure := range result.Sync.Failed {
			failed[failure.Target] = true
		}
		if len(failed) > 0 && len(failed) < total {
			return "partial"
		}
	}
	return statusFromErr(err)
}

func logProject(cfg *config.Config, command, action, root, status string, err error, start time.Time) {
	if action == "edit" {
		action = "update"
	}
	e := oplog.NewEntry(command, status, time.Since(start))
	e.Args = map[string]any{"action": action, "root": root, "scope": "cli"}
	if err != nil {
		e.Message = err.Error()
	}
	limit := config.DefaultLogMaxEntries
	if cfg.Log.MaxEntries != nil && *cfg.Log.MaxEntries >= 0 {
		limit = *cfg.Log.MaxEntries
	}
	_ = oplog.WriteWithLimit(config.ReadOnlyConfigPath(), oplog.OpsFile, e, limit)
}
