package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// A layout copy is owned only by the local installation that made it. It
// survives sync exclusion, but is never shared in the plugin declaration.
type herdrLayoutReceipt struct {
	Home      string `json:"home"`
	From      string `json:"from"`
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	Clicks    string `json:"clicks,omitempty"`
	DockMod   string `json:"dockMod,omitempty"`
	Suspended bool   `json:"suspended,omitempty"`
	// Left is the layout the window was on when Skillshare suspended this copy:
	// this copy or a later copy made from it.
	Left string `json:"left,omitempty"`
}

func (r herdrLayoutReceipt) valid() bool {
	return filepath.IsAbs(r.Home) && herdrLayoutName.MatchString(r.From) && herdrLayoutName.MatchString(r.Name) &&
		strings.HasPrefix(r.Name, r.From+"-skillshare") && r.Digest != "" && (r.DockMod == "" || namePattern.MatchString(r.DockMod))
}

type herdrLayoutCleanup struct {
	herdrLayoutReceipt
	Keep bool
}

var errHerdrLayoutRemoveOpen = agentError{
	key:     "plugins.error.herdrLayoutRemoveOpen",
	message: "Open Herdr with the original layout available before removing the Skillshare layout copy.",
}

func herdrLayoutClicks(home, name string) (digest string, present bool, err error) {
	raw, err := readHerdrLayoutFile(home, filepath.Join("layouts", ".state", name+".json"))
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(raw, &state); err != nil {
		return "", false, err
	}
	// Herdr rewrites this snapshot of the source tree when it saves the same click overrides.
	delete(state, "file")
	if len(state) == 0 {
		return "", true, nil
	}
	data, err := json.Marshal(state)
	if err != nil {
		return "", false, err
	}
	return hash(data), true, nil
}

// inspect keeps copies that changed, are linked, or belong to another app home.
func (r herdrLayoutReceipt) inspect(home string) (layout, state, safe bool) {
	if !r.valid() || r.Home != home {
		return false, false, false
	}
	data, err := readHerdrLayout(home, r.Name)
	layout = err == nil
	if layout && hash(data) != r.Digest || !layout && !os.IsNotExist(err) {
		return false, false, false
	}
	clicks, state, err := herdrLayoutClicks(home, r.Name)
	if err != nil || state && clicks != r.Clicks || layout && !state && r.Clicks != "" {
		return false, false, false
	}
	return layout, state, true
}

// orderHerdrLayoutChanges orders the picked Herdr app changes by how their layout copies chain.
// Packages leaving go first, a package whose copy was made from another's before that package,
// so the window steps back one copy at a time while each layout it returns to still exists.
// Packages returning go after, in the opposite order, so a copy is reached only once every mod
// it docks is installed; those already installed go last, after every mod this request installs.
func orderHerdrLayoutChanges(changes []Change, plan *herdrLayoutPlan, picked, leaving func(Change) bool) {
	tier := len(plan.parent) + 1
	key := func(c Change) int {
		switch {
		case leaving(c):
			return -plan.depth(c.Name)
		case c.Action == "noop":
			return 2*tier + plan.depth(c.Name)
		}
		return tier + plan.depth(c.Name)
	}
	var at []int
	for i, c := range changes {
		if c.Target == "herdr-app" && picked(c) {
			at = append(at, i)
		}
	}
	picks := make([]Change, len(at))
	for j, i := range at {
		picks[j] = changes[i]
	}
	slices.SortStableFunc(picks, func(a, b Change) int { return key(a) - key(b) })
	for j, i := range at {
		changes[i] = picks[j]
	}
}

// removing names the packages this request also takes off Herdr app.
func (s *Service) prepareHerdrLayoutCleanup(ctx context.Context, c *Change, d *document, removing map[string]bool) error {
	records := d.state.Bindings[c.Name][c.Target].HerdrLayouts
	if len(records) == 0 {
		return nil
	}
	home, _ := filepath.EvalSymlinks(herdrHome())
	// Another managed copy may still need this original as its return layout.
	needed := map[string]bool{}
	for name, bindings := range d.state.Bindings {
		if name == c.Name {
			continue
		}
		for _, other := range bindings[c.Target].HerdrLayouts {
			if other.valid() && other.Home == home {
				// A copy this request removes as well no longer needs its original.
				if _, _, safe := other.inspect(home); removing[name] && safe {
					continue
				}
				if _, err := readHerdrLayout(home, other.Name); err == nil {
					needed[other.From] = true
				}
			}
		}
	}
	var remove, keep []string
	active := ""
	// A later copy can have an earlier copy as its original. Remove it first.
	for _, rec := range slices.Backward(records) {
		layout, state, safe := rec.inspect(home)
		keepCopy := !safe || needed[rec.Name]
		c.herdrLayoutCleanup = append(c.herdrLayoutCleanup, herdrLayoutCleanup{rec, keepCopy})
		if keepCopy {
			keep = append(keep, rec.Name)
			continue
		}
		if !layout && !state {
			continue
		}
		if active == "" {
			var err error
			active, err = s.currentHerdrLayout(ctx)
			if err != nil {
				return errHerdrLayoutRemoveOpen
			}
		}
		if active == rec.Name {
			if _, err := readHerdrLayout(home, rec.From); err != nil {
				return errHerdrLayoutRemoveOpen
			}
		}
		remove = append(remove, rec.Name)
	}
	switch {
	case len(remove) != 0 && len(keep) != 0:
		c.Message, c.MessageKey = "Also remove unchanged Herdr layouts: "+strings.Join(remove, ", ")+". Keep edited or unverified layouts: "+strings.Join(keep, ", ")+".", "plugins.note.herdrLayoutDeleteKeep"
		c.MessageArgs = map[string]string{"remove": strings.Join(remove, ", "), "keep": strings.Join(keep, ", ")}
	case len(remove) != 0:
		c.Message, c.MessageKey = "Also remove the Skillshare layout copies: "+strings.Join(remove, ", ")+". If a copy is active, switch to its original first.", "plugins.note.herdrLayoutDelete"
		c.MessageArgs = map[string]string{"remove": strings.Join(remove, ", ")}
	case len(keep) != 0:
		c.Message, c.MessageKey = "Keep edited or unverified Herdr layouts: "+strings.Join(keep, ", ")+".", "plugins.note.herdrLayoutKeep"
		c.MessageArgs = map[string]string{"keep": strings.Join(keep, ", ")}
	}
	return nil
}

func (s *Service) removeHerdrLayout(ctx context.Context, rec herdrLayoutReceipt) error {
	home, err := filepath.EvalSymlinks(herdrHome())
	if err != nil {
		return err
	}
	layout, state, safe := rec.inspect(home)
	if !safe {
		return errHerdrLayoutStale
	}
	if !layout && !state {
		return nil
	}
	active, err := s.currentHerdrLayout(ctx)
	if err != nil {
		return errHerdrLayoutRemoveOpen
	}
	if active == rec.Name {
		if _, err := readHerdrLayout(home, rec.From); err != nil {
			return errHerdrLayoutRemoveOpen
		}
		args, _ := json.Marshal(map[string]string{"name": rec.From})
		if _, err := s.run(ctx, "herdr-app", "do", "layout.switch", string(args)); err != nil {
			return err
		}
		if active, err := s.currentHerdrLayout(ctx); err != nil || active != rec.From {
			return errHerdrLayoutRemoveOpen
		}
	}
	layout, state, safe = rec.inspect(home)
	if !safe {
		return errHerdrLayoutStale
	}
	root, err := os.OpenRoot(filepath.Join(home, "layouts"))
	if err != nil {
		return err
	}
	defer root.Close()
	if layout {
		if err := root.Remove(rec.Name + ".json"); err != nil {
			return fmt.Errorf("remove Herdr layout copy: %w", err)
		}
	}
	if state {
		if err := root.Remove(filepath.Join(".state", rec.Name+".json")); err != nil {
			return fmt.Errorf("remove Herdr layout copy state: %w", err)
		}
	}
	return nil
}
