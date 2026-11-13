package profileapply

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/profile"
	ssync "skillshare/internal/sync"
)

var ErrRefused = errors.New("profile switch refused")
var ErrIncomplete = errors.New("profile switch not fully applied")

type RefusalCode string

const (
	RefusalLocked  RefusalCode = "profile_locked"
	RefusalEmpty   RefusalCode = "profile_empty"
	RefusalBlocked RefusalCode = "profile_blocked"
	RefusalGeneric RefusalCode = "profile_refused"
)

type Refusal struct {
	Code   RefusalCode
	Reason string
}

func (e *Refusal) Error() string { return e.Reason }
func (e *Refusal) Unwrap() error { return ErrRefused }

type Request struct {
	Scope                                          Scope
	Name                                           string
	None, DryRun, NoSync, DiscardEdits, AllowEmpty bool
	// Private failure seams keep orchestration tests deterministic without
	// replacing the target writer or adding production recovery state.
	snapshot                   func(string, string, string) (string, error)
	afterSnapshot, afterRecord func()
}

type Selection struct {
	Previous string `json:"previous"`
	Active   string `json:"active"`
	Pending  bool   `json:"pending"`
}

type Target struct {
	Name           string              `json:"name"`
	State          string              `json:"state"`
	Added          []string            `json:"added"`
	Refreshed      []string            `json:"refreshed"`
	Removed        []string            `json:"removed"`
	Converted      []string            `json:"converted"`
	KeptModified   []ssync.EntryChange `json:"kept_modified"`
	Error          string              `json:"error,omitempty"`
	PruneError     string              `json:"prune_error,omitempty"`
	FailedRemovals []string            `json:"failed_removals,omitempty"`
}

type Result struct {
	Profile        *Info               `json:"-"`
	Selection      Selection           `json:"profile"`
	Complete       bool                `json:"complete"`
	Targets        []Target            `json:"targets"`
	Blocked        []ssync.EntryChange `json:"blocked"`
	SymlinkTargets []string            `json:"symlink_targets"`
	Snapshot       []Snapshot          `json:"snapshot"`
	Executed       ssync.ExecuteResult `json:"-"`
}

func refusal(code RefusalCode, reason string) error { return &Refusal{Code: code, Reason: reason} }

func emptyResult() Result {
	return Result{Targets: []Target{}, Blocked: []ssync.EntryChange{}, SymlinkTargets: []string{}, Snapshot: []Snapshot{}}
}

func array[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

// lockedView takes the lock before reloading. The initial read only decides
// whether ordinary sync needs a lock; it never supplies execution state.
func lockedView(ctx context.Context, scope Scope, dryRun, required bool) (*View, func(), error) {
	unlock := func() {}
	if !required {
		v, err := Load(scope)
		if err != nil {
			return nil, unlock, err
		}
		required, err = config.ProfileConfigured(scope.Path(), v.Catalog.Present)
		if err != nil {
			return nil, unlock, err
		}
	}
	if !dryRun && required {
		_, lockPath, err := config.ProfilePaths(scope.Path())
		if err != nil {
			return nil, unlock, err
		}
		unlock, err = profile.Lock(ctx, lockPath)
		if err != nil {
			return nil, func() {}, refusal(RefusalLocked, err.Error())
		}
	}
	v, err := Load(scope)
	if err != nil {
		unlock()
		return nil, func() {}, err
	}
	return v, unlock, nil
}

// Eligible freezes the config's own valid, enabled merge/copy skill targets.
// Other targets are reported, never visited by profile application.
func (v *View) Eligible() (map[string]config.TargetConfig, []string) {
	var invalid map[string]error
	if v.Project != nil {
		_, invalid, _ = config.ValidateProjectConfigForSync(v.Project, v.Scope.ProjectRoot)
	} else {
		_, invalid, _ = config.ValidateConfigForSync(v.Config)
	}
	targets := make(map[string]config.TargetConfig)
	symlinks := make([]string, 0)
	for _, name := range slices.Sorted(maps.Keys(v.Config.Targets)) {
		target := v.Config.Targets[name]
		sc := target.SkillsConfig()
		if target.ProjectRoot() != "" || invalid[name] != nil || !sc.IsEnabled() {
			continue
		}
		mode := sc.Mode
		if mode == "" {
			mode = v.Config.Mode
		}
		if mode == "" {
			mode = "merge"
		}
		if mode == "symlink" {
			symlinks = append(symlinks, name)
			continue
		}
		if mode == "merge" || mode == "copy" {
			targets[name] = target
		}
	}
	return targets, symlinks
}

func (v *View) validate() error {
	if v.Project != nil {
		_, _, err := config.ValidateProjectConfigForSync(v.Project, v.Scope.ProjectRoot)
		return err
	}
	_, _, err := config.ValidateConfigForSync(v.Config)
	return err
}

func execute(ctx context.Context, v *View, targets map[string]config.TargetConfig, discovery ssync.TargetDiscovery, dryRun, force, aware bool, discard []ssync.DiscardEntry, ordinary ...bool) (ssync.ExecuteResult, error) {
	// Shared-folder grouping must see exactly this frozen eligible set, not
	// registered projects or a disabled target sharing the folder.
	cfg := *v.Config
	cfg.Targets = targets
	opts := ssync.ExecuteOptions{Kinds: []string{"skill"}, Discovery: discovery,
		ProjectMode: v.Project != nil, ProjectRoot: v.Scope.ProjectRoot, DryRun: dryRun, Force: force, ProfileAware: aware, Discard: discard}
	if len(ordinary) > 0 && ordinary[0] {
		opts.MovedTargets = ssync.MovedProjectTargets(v.Project, v.Config.Targets)
	}
	return ssync.ExecuteTargets(ctx, &cfg, targets, opts)
}

func (r *Result) outcome(executed ssync.ExecuteResult, sourceIncomplete bool) {
	r.Executed = executed
	r.Complete = len(executed.Failed) == 0 && !sourceIncomplete
	r.Targets = make([]Target, 0, len(executed.Results)+len(executed.Failed))
	r.Blocked = make([]ssync.EntryChange, 0)
	for _, row := range executed.Results {
		state := "applied"
		if sourceIncomplete || len(row.KeptModified)+len(row.Blocked)+len(row.FailedRemovals) > 0 || row.PruneError != "" {
			state = "incomplete"
			r.Complete = false
		}
		r.Targets = append(r.Targets, Target{Name: row.Target, State: state, Added: array(row.Linked), Refreshed: array(row.Refreshed), Removed: array(row.Pruned),
			Converted: array(row.Converted), KeptModified: array(row.KeptModified), PruneError: row.PruneError, FailedRemovals: row.FailedRemovals})
		r.Blocked = append(r.Blocked, row.KeptModified...)
		r.Blocked = append(r.Blocked, row.Blocked...)
	}
	for _, failed := range executed.Failed {
		r.Targets = append(r.Targets, Target{Name: failed.Target, State: "failed", Error: failed.Error,
			Added: []string{}, Refreshed: []string{}, Removed: []string{}, Converted: []string{}, KeptModified: []ssync.EntryChange{}})
	}
}

// Apply is the single switch adapter shared by CLI and API. It performs no
// target writes itself; plan and apply both use the existing skills executor.
func Apply(ctx context.Context, req Request) (out Result, retErr error) {
	out = emptyResult()
	start := time.Now()
	if req.None && req.Name != "" || !req.None && req.Name == "" || req.NoSync && req.DiscardEdits {
		return out, refusal(RefusalGeneric, "choose a profile name or --none; --no-sync cannot discard edits")
	}
	v, unlock, err := lockedView(ctx, req.Scope, req.DryRun, true)
	if err != nil {
		return out, err
	}
	defer unlock()
	out.Selection = Selection{Previous: v.State.Active, Active: v.State.Active, Pending: v.State.Pending}
	previous := v.State.Active
	candidate := v.Candidate(req.Name)
	if err := candidate.ActiveProfileError(); err != nil {
		return out, err
	}
	if err := candidate.validate(); err != nil {
		return out, err
	}
	targets, symlinks := candidate.Eligible()
	out.SymlinkTargets = symlinks
	discovery, err := candidate.Discover()
	if err != nil {
		return out, err
	}
	out.Profile = candidate.Info(discovery.Skills)
	out.Selection.Active = req.Name
	planned, err := execute(ctx, candidate, targets, discovery, true, false, true, nil)
	if err != nil {
		return out, err
	}
	out.outcome(planned, discovery.SourceIncomplete)
	if out.Profile != nil && out.Profile.Total > 0 && out.Profile.Selected == 0 && !req.AllowEmpty {
		var unmatched []string
		if d, ok := candidate.Catalog.Definitions[req.Name]; ok {
			p, _ := profile.Preview(d, enabledNames(discovery.Skills))
			unmatched = p.Unmatched
		}
		return out, refusal(RefusalEmpty, fmt.Sprintf("profile %q selects no enabled skills; unmatched patterns: %s; use --allow-empty to confirm", req.Name, strings.Join(unmatched, ", ")))
	}
	var modified []ssync.EntryChange
	for _, entry := range out.Blocked {
		if entry.Class != ssync.EntryModifiedCopy || !req.DiscardEdits {
			return out, refusal(RefusalBlocked, fmt.Sprintf("%s/%s: %s; use skillshare collect, or --discard-edits for modified managed copies", entry.Target, entry.Entry, entry.Reason))
		}
		modified = append(modified, entry)
	}
	if req.DryRun {
		return out, nil
	}
	var discard []ssync.DiscardEntry
	if len(modified) > 0 {
		out.Snapshot, discard, err = snapshots(candidate, targets, modified, req.snapshot)
		if err != nil {
			return out, refusal(RefusalGeneric, err.Error())
		}
	}
	if req.afterSnapshot != nil {
		req.afterSnapshot()
	}
	state := profile.State{Version: 1, Active: req.Name, Pending: true}
	if err := profile.WriteState(v.StatePath, state); err != nil {
		return out, err
	}
	out.Selection = Selection{Previous: previous, Active: req.Name, Pending: true}
	if out.Profile != nil {
		out.Profile.Pending = true
	}
	defer func() {
		status := "ok"
		if retErr != nil {
			status = "partial"
		}
		e := oplog.NewEntry("profile use", status, time.Since(start))
		e.Args = map[string]any{"previous": previous, "active": req.Name, "pending": out.Selection.Pending, "no_sync": req.NoSync}
		oplog.Write(req.Scope.Path(), oplog.OpsFile, e)
	}()
	if req.afterRecord != nil {
		req.afterRecord()
	}
	if req.NoSync {
		out.Complete = false
		return out, nil
	}
	applied, err := execute(ctx, candidate, targets, discovery, false, false, true, discard)
	out.outcome(applied, discovery.SourceIncomplete)
	if err != nil {
		out.Complete = false
		return out, err
	}
	if !out.Complete {
		return out, ErrIncomplete
	}
	state.Pending = false
	if err := profile.WriteState(v.StatePath, state); err != nil {
		out.Complete = false
		return out, err
	}
	out.Selection.Pending = false
	if out.Profile != nil {
		out.Profile.Pending = false
	}
	return out, nil
}

func enabledNames(skills []ssync.DiscoveredSkill) []string {
	names := make([]string, 0, len(skills))
	for _, skill := range skills {
		if !skill.Disabled {
			names = append(names, skill.FlatName)
		}
	}
	slices.Sort(names)
	return names
}

type SyncRequest struct {
	Scope                       Scope
	DryRun, Force, DiscardEdits bool
	Targets                     []string // nil means every eligible target; an explicit subset cannot settle pending
	// Before preserves the caller's existing all-resource preflight: a native
	// settings conflict must refuse before any skill target is written.
	Before func(*View) error
	// After runs the caller's remaining ordinary-sync resources while the
	// same lock remains held. It never runs from a profile switch.
	After func(*View, *Result) error
}

// WithSkillSync reloads under the same lock as Apply, executes skills through
// ExecuteTargets, then holds the lock through the caller's work and settlement.
func WithSkillSync(ctx context.Context, req SyncRequest) (out Result, retErr error) {
	out = emptyResult()
	v, unlock, err := lockedView(ctx, req.Scope, req.DryRun, false)
	if err != nil {
		return out, err
	}
	defer unlock()
	if err := v.ActiveProfileError(); err != nil {
		return out, err
	}
	if err := v.validate(); err != nil {
		return out, err
	}
	aware := v.State.Active != "" || v.State.Pending
	if req.DiscardEdits && !aware {
		return out, refusal(RefusalGeneric, "--discard-edits requires an active profile or a pending profile switch")
	}
	if req.Before != nil {
		if err := req.Before(v); err != nil {
			return out, err
		}
	}
	targets, symlinks := v.Eligible()
	out.SymlinkTargets = symlinks
	if req.Targets != nil {
		for name := range targets {
			if !slices.Contains(req.Targets, name) {
				delete(targets, name)
			}
		}
	}
	discovery, err := v.Discover()
	if err != nil {
		return out, err
	}
	out.Profile = v.Info(discovery.Skills)
	out.Selection = Selection{Previous: v.State.Active, Active: v.State.Active, Pending: v.State.Pending}
	var discard []ssync.DiscardEntry
	if req.DiscardEdits {
		plan, err := execute(ctx, v, targets, discovery, true, req.Force, aware, nil)
		if err != nil {
			return out, err
		}
		var modified []ssync.EntryChange
		for _, row := range plan.Results {
			modified = append(modified, row.KeptModified...)
		}
		if !req.DryRun {
			out.Snapshot, discard, err = snapshots(v, targets, modified, nil)
			if err != nil {
				return out, refusal(RefusalGeneric, err.Error())
			}
		}
	}
	executed, err := execute(ctx, v, targets, discovery, req.DryRun, req.Force, aware, discard, true)
	out.outcome(executed, discovery.SourceIncomplete)
	if err != nil {
		out.Complete = false
		return out, err
	}
	if !req.DryRun && req.Targets == nil && out.Complete && v.State.Pending {
		state := v.State
		state.Pending = false
		if err := profile.WriteState(v.StatePath, state); err != nil {
			return out, err
		}
		out.Selection.Pending = false
		if out.Profile != nil {
			out.Profile.Pending = false
		}
	}
	if req.After != nil {
		if err := req.After(v, &out); err != nil {
			return out, err
		}
	}
	return out, nil
}
