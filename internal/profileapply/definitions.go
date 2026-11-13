package profileapply

import (
	"context"
	"fmt"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/profile"
)

// Mutate is the shared CLI/API definition transaction. All mutations validate
// against one candidate raw node before a single config save under the lock.
// An active rename follows in local state; a state-save failure is reported
// explicitly, since config and local state cannot be committed atomically.
func Mutate(ctx context.Context, scope Scope, mutations ...profile.Mutation) (profile.Change, error) {
	start := time.Now()
	var total profile.Change
	statePath, lockPath, err := config.ProfilePaths(scope.Path())
	if err != nil {
		return total, err
	}
	unlock, err := profile.Lock(ctx, lockPath)
	if err != nil {
		return total, err
	}
	defer unlock()
	v, err := Load(scope)
	if err != nil {
		return total, err
	}
	state, err := profile.ReadState(statePath)
	if err != nil {
		return total, err
	}
	total.Active = state.Active
	node := v.Definitions()
	for _, mutation := range mutations {
		next, change, err := profile.Mutate(node, total.Active, mutation)
		if err != nil {
			return profile.Change{Active: state.Active}, err
		}
		node = next
		total.Skills = append(total.Skills, change.Skills...)
		total.Active = change.Active
		total.Changed = total.Changed || change.Changed
		total.NeedsSync = total.NeedsSync || change.NeedsSync
	}
	if !total.Changed {
		return total, nil
	}
	*v.Definitions() = *node
	if v.Project != nil {
		err = v.Project.Save(scope.ProjectRoot)
	} else {
		var candidate *config.SaveCandidate
		candidate, err = config.PrepareSave(scope.Path(), v.Config)
		if err == nil {
			err = candidate.Write()
		}
	}
	if err != nil {
		return total, err
	}
	if total.Active != state.Active {
		state.Active = total.Active
		if err := profile.WriteState(statePath, state); err != nil {
			return total, fmt.Errorf("definitions saved, but local active rename failed; repair with skillshare profile use %s: %w", total.Active, err)
		}
	}
	e := oplog.NewEntry("profile definitions", "ok", time.Since(start))
	e.Args = map[string]any{"mutations": len(mutations)}
	oplog.Write(scope.Path(), oplog.OpsFile, e)
	return total, nil
}
