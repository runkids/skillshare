package main

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"skillshare/internal/hooks"
	"skillshare/internal/mcp"
	"skillshare/internal/profileapply"
	"skillshare/internal/sync"
)

type profileSyncRun struct {
	view   *profileapply.View
	result *profileapply.Result
	native *profileSyncNative
}

type profileSyncNative struct {
	mcpResult    *mcp.Result
	mcpService   *mcp.Service
	hooksResult  *hooks.Result
	hooksService *hooks.Service
}

func (p *profileSyncRun) owns(name string) bool {
	for _, row := range p.result.Executed.Results {
		if row.Target == name {
			return true
		}
	}
	for _, row := range p.result.Executed.Failed {
		if row.Target == name {
			return true
		}
	}
	return false
}

func runProfileSyncCLI(args []string, scope profileapply.Scope, dryRun, force, discard, jsonOutput bool, native *profileSyncNative, before func(*profileapply.View) error) error {
	called := false
	_, err := profileapply.WithSkillSync(context.Background(), profileapply.SyncRequest{Scope: scope, DryRun: dryRun, Force: force, DiscardEdits: discard,
		Before: before,
		After: func(v *profileapply.View, result *profileapply.Result) error {
			called = true
			return cmdSyncPrepared(args, &profileSyncRun{view: v, result: result, native: native})
		}})
	var reported *jsonSilentError
	if err != nil && !called && jsonOutput && !errors.As(err, &reported) {
		return writeJSONError(err)
	}
	return err
}

func (p *profileSyncRun) results(dryRun bool) []syncTargetResult {
	var results []syncTargetResult
	for _, row := range p.result.Executed.Results {
		target := p.view.Config.Targets[row.Target]
		sc := target.SkillsConfig()
		mode := getTargetMode(sc.Mode, p.view.Config.Mode)
		r := syncTargetResult{name: row.Target, mode: mode, include: sc.Include, exclude: sc.Exclude,
			keptModified: row.KeptModified}
		run := sync.SkillTargetResult{Linked: row.Linked, Updated: row.Updated, Pruned: row.Pruned, Skipped: row.Skipped, DirCreated: row.DirCreated}
		if mode == "copy" {
			collectCopySyncResult(&r, run, dryRun)
		} else {
			collectMergeSyncResult(&r, run, dryRun)
		}
		for _, entry := range slices.Concat(row.KeptModified, row.Blocked) {
			r.warnings = append(r.warnings, fmt.Sprintf("%s: %s; use skillshare collect", entry.Entry, entry.Reason))
		}
		if row.PruneError != "" {
			r.warnings = append(r.warnings, row.PruneError)
		}
		if len(row.FailedRemovals) > 0 {
			r.warnings = append(r.warnings, fmt.Sprintf("failed removals: %v", row.FailedRemovals))
		}
		results = append(results, r)
	}
	for _, failure := range p.result.Executed.Failed {
		results = append(results, syncTargetResult{name: failure.Target, errMsg: failure.Error})
	}
	// The caller emits profile metadata once, even with no eligible targets.
	return results
}
