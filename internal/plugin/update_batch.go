package plugin

import (
	"context"
	"errors"
)

// ErrSourceMoved means a source changed after the check the user approved: nothing was
// applied, and only a new check can show what an update would bring.
var ErrSourceMoved = errors.New("source changed since the check; check again")

// Seen is what a check showed for one Agent of a plugin.
type Seen struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Digest  string `json:"digest,omitempty"`
}

// movedTo says whether the source an update would install is not the one the check showed.
func (seen Seen) movedTo(b Binding) bool {
	// An npm package (no source) has no reviewed snapshot to move: Pi installs whatever npm names latest.
	if b.Source == "" {
		return false
	}
	// A local directory has no commit; its digest is what moves.
	return b.Version != seen.Version || seen.Commit != "" && b.Commit != seen.Commit || seen.Digest != "" && b.Digest != seen.Digest
}

// UpdateItem is one plugin of a batch update and, per Agent, what the check showed.
type UpdateItem struct {
	Name    string          `json:"name"`
	Targets []string        `json:"targets"`
	Seen    map[string]Seen `json:"seen,omitempty"`
}

// Batch is a batch update and how its caller follows it.
type Batch struct {
	Items []UpdateItem
	// Stopped is asked before each plugin; the one under way always finishes.
	Stopped func() bool
	// Around runs each plugin's update, so the caller can hold its own lock for just that long.
	Around  func(update func())
	Started func(name string)
	Planned func(name string, p *Plan)
	Ended   func(name string, p *Plan, result *Result, err error)
}

// UpdateBatch updates b.Items one after another under the Lock its caller holds, so one
// failure stops nothing else, and returns how many did not complete. Plugins from one
// source share a single read of it for this batch only.
func (s *Service) UpdateBatch(ctx context.Context, b Batch) (failed int) {
	s.discoveries = map[string]*Discovery{}
	defer func() { s.discoveries = nil }()
	for _, item := range b.Items {
		if b.Stopped() {
			break
		}
		b.Started(item.Name)
		var p *Plan
		var result *Result
		var err error
		b.Around(func() { p, result, err = s.updateLocked(ctx, item, func(p *Plan) { b.Planned(item.Name, p) }) })
		if err != nil {
			failed++
		}
		b.Ended(item.Name, p, result, err)
	}
	return failed
}

// updateLocked previews and applies one plugin's update. planned sees the plan before it
// is applied. A blocked plan is returned with the error that stops it.
func (s *Service) updateLocked(ctx context.Context, item UpdateItem, planned func(*Plan)) (*Plan, *Result, error) {
	r := Request{Action: "update", Name: item.Name, Targets: item.Targets}
	p, err := s.Preview(ctx, r)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range p.Changes {
		if c.Action != "update" {
			continue
		}
		// An Agent no check showed has no version the user approved.
		seen, ok := item.Seen[c.Target]
		if !ok || seen.movedTo(c.Binding) {
			return p, nil, ErrSourceMoved
		}
	}
	planned(p)
	result, err := s.applyPlan(ctx, r, p)
	return p, result, err
}
