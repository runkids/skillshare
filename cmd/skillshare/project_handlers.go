package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
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
	confirmBeforeLock := !o.dryRun && !o.yes && (o.action == "remove" || o.action == "share" && !o.json)
	if !confirmBeforeLock && !o.dryRun && slices.Contains([]string{"add", "edit", "remove", "localize", "share"}, o.action) {
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
	// A dry run previews the replacement it would otherwise ask about.
	replace := o.yes || o.dryRun
	if confirmBeforeLock {
		confirmedEntry := cfg.Projects[root]
		question, err := projectConfirmation(o, service, root)
		if err != nil {
			return err
		}
		if question != "" {
			ok, err := ui.ConfirmAction(question, false)
			if err != nil {
				return err
			}
			if !ok {
				ui.Cancelled(map[string]string{"remove": "removed", "share": "moved"}[o.action])
				return nil
			}
			replace = true
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
			return fmt.Errorf("project changed while confirming; retry project %s", o.action)
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
		storage := ""
		if o.machineLocal {
			storage = projects.OriginLocal
		} else if o.shared {
			storage = projects.OriginShared
		}
		if _, exists := cfg.Projects[root]; create && exists {
			// Adding the same project again is a no-op, so scripts can rerun it.
			if info, err := os.Stat(config.ExpandPath(root)); err != nil || !info.IsDir() {
				return fmt.Errorf("%w: %s", projects.ErrFolderNotFound, root)
			}
			preview, err := service.Put(root, entry, projects.PutOptions{DryRun: true})
			if err != nil {
				return err
			}
			if len(preview.Affected) > 0 {
				return fmt.Errorf("%w: %s; use skillshare project edit", projects.ErrExists, root)
			}
			if origin, _ := cfg.ProjectOrigin(root); storage != "" && storage != origin {
				return fmt.Errorf("%w: %s is saved %s; use skillshare project %s", projects.ErrExists, root, projectStorageLabel(origin), map[string]string{projects.OriginLocal: "localize", projects.OriginShared: "share"}[storage])
			}
			item, err := service.Show(root)
			if err != nil {
				return err
			}
			result.Result = projects.Result{Item: &item, Expanded: cfg, Registration: "kept", Detach: &projects.DetachReport{}}
		} else {
			result.Result, opErr = service.Put(root, entry, projects.PutOptions{Create: create, DryRun: o.dryRun, Detach: true, Storage: storage})
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
				logProject(cfg, "project", o.action, root, "partial", opErr, start, nil)
			}
			return renderProjectResult(o, result, opErr, start)
		}
		if !o.dryRun && len(result.Affected) > 0 {
			logProject(cfg, "project", o.action, root, "ok", nil, start, nil)
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
				logProject(cfg, "sync", "sync", root, projectSyncStatus(result, opErr), opErr, start, nil)
			}
		}
	case "localize", "share":
		if o.action == "localize" {
			result.Result, opErr = service.Localize(root, o.dryRun)
		} else {
			result.Result, opErr = service.Share(root, projects.ShareOptions{DryRun: o.dryRun, Replace: replace})
		}
		if errors.Is(opErr, projects.ErrSharedDiffers) {
			return fmt.Errorf("%w; rerun with --yes to replace them", opErr)
		}
		if opErr != nil {
			return opErr
		}
		if result.Registration == "updated" {
			logProject(cfg, "project", o.action, root, "ok", nil, start, nil)
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
			logProject(cfg, "sync", "sync", root, projectSyncStatus(result, opErr), opErr, start, nil)
		}
		if opErr != nil && result.Sync == nil {
			return opErr
		}
	case "remove":
		result.Result, opErr = service.Remove(root, projects.RemoveOptions{DryRun: o.dryRun, Shared: o.shared})
		changed := result.Detach != nil && (len(result.Detach.Removed) > 0 || len(result.Detach.CopiedBack) > 0)
		if opErr != nil && !changed {
			return opErr
		}
		if !o.dryRun && (opErr == nil || changed) {
			status := "ok"
			if opErr != nil {
				status = "partial"
			}
			logProject(cfg, "project", "remove", root, status, opErr, start, map[string]any{"shared": o.shared})
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

// extra adds action-specific args, such as whether a remove also dropped config.yaml's copy.
func logProject(cfg *config.Config, command, action, root, status string, err error, start time.Time, extra map[string]any) {
	if action == "edit" {
		action = "update"
	}
	e := oplog.NewEntry(command, status, time.Since(start))
	e.Args = map[string]any{"action": action, "root": root, "scope": "cli"}
	maps.Copy(e.Args, extra)
	if err != nil {
		e.Message = err.Error()
	}
	limit := config.DefaultLogMaxEntries
	if cfg.Log.MaxEntries != nil && *cfg.Log.MaxEntries >= 0 {
		limit = *cfg.Log.MaxEntries
	}
	_ = oplog.WriteWithLimit(config.ReadOnlyConfigPath(), oplog.OpsFile, e, limit)
}
