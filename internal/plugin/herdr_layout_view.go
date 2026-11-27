package plugin

import (
	"cmp"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
)

// Herdr keeps rendering a previously loaded mod while its layout still
// references it. Switching layouts releases those views without editing the copy.
type herdrLayoutView struct {
	Copy     string
	From     string
	To       string
	Resume   bool
	Remember bool
	// Suspend and Left are what the copy's receipt records once the switch is applied.
	Suspend bool
	Left    string `json:",omitempty"`
	// Drop lists returns this change abandons because the window was moved since.
	Drop []string `json:",omitempty"`
}

// herdrLayoutPlan follows one request's layout switches in apply order, so each
// change starts from the layout the previous change left the window on.
type herdrLayoutPlan struct {
	home   string
	parent map[string]string // recorded copy -> the layout it was made from
	owner  map[string]string // recorded copy -> the package whose mod it docks
	// installed tells whether a package's mod is installed at this point of the apply.
	installed map[string]bool
	active    string
	// left maps a layout a switch in this request reached to the layout the window started on.
	left map[string]string
	// waiting lists layouts a return stopped short of, recorded earlier or in this request,
	// so the change that installs the missing mod continues it.
	waiting []string
}

func newHerdrLayoutPlan(d *document) *herdrLayoutPlan {
	home, _ := filepath.EvalSymlinks(herdrHome())
	p := &herdrLayoutPlan{home: home, parent: map[string]string{}, owner: map[string]string{}, installed: map[string]bool{}, left: map[string]string{}}
	for name, bindings := range d.state.Bindings {
		local := bindings["herdr-app"]
		p.installed[name] = local.Applied != nil && local.Applied.Binding.Selected()
		for _, rec := range local.HerdrLayouts {
			if rec.valid() && rec.Home == home {
				p.parent[rec.Name], p.owner[rec.Name] = rec.From, name
				// An installed mod's suspended copy stopped short; a paused mod's waits for that mod.
				if p.installed[name] && rec.Suspended && rec.Left != "" {
					p.waiting = append(p.waiting, rec.Left)
				}
			}
		}
	}
	return p
}

// owns tells whether a package made a recorded copy.
func (p *herdrLayoutPlan) owns(name string) bool {
	for _, owner := range p.owner {
		if owner == name {
			return true
		}
	}
	return false
}

// depth counts the recorded copies that a package's copies were made from.
func (p *herdrLayoutPlan) depth(name string) int {
	most := 0
	for layout, owner := range p.owner {
		if owner == name {
			most = max(most, len(p.chain(layout))-2)
		}
	}
	return most
}

// chain lists layout, then each recorded copy it was made from, ending with an original layout.
// A copy made from another copy keeps that copy's panels.
func (p *herdrLayoutPlan) chain(layout string) []string {
	out := []string{layout}
	for from, ok := p.parent[layout]; ok && len(out) <= len(p.parent); from, ok = p.parent[from] {
		out = append(out, from)
	}
	return out
}

// deepest returns the newest layout in chain whose docked mods are all installed.
func (p *herdrLayoutPlan) deepest(chain []string) string {
	at := chain[len(chain)-1]
	for _, layout := range slices.Backward(chain[:len(chain)-1]) {
		if !p.installed[p.owner[layout]] {
			break
		}
		if _, err := readHerdrLayout(p.home, layout); err != nil {
			break
		}
		at = layout
	}
	return at
}

var errHerdrLayoutSwitch = agentError{
	key:     "plugins.error.herdrLayoutSwitch",
	message: "Open Herdr with the destination layout available before switching layouts.",
}

func (r herdrLayoutReceipt) showsPanel(home string) bool {
	if r.Home != home || !r.valid() {
		return false
	}
	data, err := readHerdrLayout(home, r.Name)
	if err != nil {
		return false
	}
	// An edited layout can still be used, but only if it still contains the
	// two owned panel nodes. A replaced layout with the same name isn't ours.
	var clone struct {
		ClonedFrom struct{ Name string } `json:"cloned_from"`
	}
	if json.Unmarshal(data, &clone) != nil || clone.ClonedFrom.Name != r.From {
		return false
	}
	if r.DockMod != "" {
		var root map[string]any
		if json.Unmarshal(data, &root) != nil {
			return false
		}
		found := false
		walkHerdrNodes(root, func(node map[string]any) {
			content, _ := node["content"].(map[string]any)
			found = found || node["id"] == herdrDockID(r.DockMod) && content["mod"] == r.DockMod
		})
		return found
	}
	patch, err := patchHerdrPanel(data)
	return err == nil && patch == nil
}

// prepareHerdrLayoutView plans the switch that keeps the window off panels of mods
// that are not installed. Leaving moves the window up the copy chain to the newest
// layout without the mod; returning moves it back down toward where it left from.
func (s *Service) prepareHerdrLayoutView(ctx context.Context, c *Change, plan *herdrLayoutPlan, records []herdrLayoutReceipt, resume, remember bool) error {
	before := plan.installed[c.Name]
	plan.installed[c.Name] = resume
	// Only installing a mod lets the window move further down a copy chain.
	if resume && before {
		return nil
	}
	for _, rec := range slices.Backward(records) {
		if !rec.showsPanel(plan.home) {
			continue
		}
		var targets []string
		if resume {
			if rec.Suspended {
				target := rec.Left
				if target == "" || !slices.Contains(plan.chain(target), rec.Name) {
					target = rec.Name
				}
				targets = append(targets, target)
			}
			// Another copy's return may be waiting for this mod.
			for _, target := range plan.waiting {
				if slices.Contains(plan.chain(target), rec.Name) {
					targets = append(targets, target)
				}
			}
			if len(targets) == 0 {
				continue
			}
		}
		if plan.active == "" {
			active, err := s.currentHerdrLayout(ctx)
			if err != nil {
				return errHerdrLayoutSwitch
			}
			plan.active = active
		}
		view := herdrLayoutView{Copy: rec.Name, From: plan.active, Resume: resume, Remember: remember}
		if resume {
			// Go as far down toward a target as installed mods allow, but only from
			// where Skillshare left the window; a window the user moved stays put.
			best := -1
			for _, target := range targets {
				chain := plan.chain(target)
				plan.installed[c.Name] = before
				left := plan.deepest(chain)
				plan.installed[c.Name] = resume
				if left != plan.active {
					view.Drop = append(view.Drop, target)
					continue
				}
				to := plan.deepest(chain)
				if n := len(plan.chain(to)); n > best {
					best, view.To = n, to
					// A mod docked in a later copy is still off: stop short, and return later.
					view.Suspend, view.Left = to != target, ""
					if view.Suspend {
						view.Left = target
					}
				}
			}
			plan.waiting = slices.DeleteFunc(plan.waiting, func(t string) bool { return slices.Contains(view.Drop, t) })
			if best < 0 {
				if len(view.Drop) == 0 {
					continue
				}
				view.To = view.From
			} else if view.To == view.From && !(rec.Suspended && view.Suspend) && len(view.Drop) == 0 {
				continue
			}
		} else {
			chain := plan.chain(plan.active)
			if !slices.Contains(chain, rec.Name) {
				continue
			}
			view.To = plan.deepest(chain)
			view.Suspend, view.Left = true, cmp.Or(plan.left[plan.active], plan.active)
			plan.left[view.To] = view.Left
		}
		if _, err := readHerdrLayout(plan.home, view.To); err != nil {
			return errHerdrLayoutSwitch
		}
		plan.active = view.To
		if view.Suspend && resume {
			plan.waiting = append(plan.waiting, view.Left)
		}
		c.herdrLayoutView = &view
		if remember && view.From != view.To {
			c.Message, c.MessageKey = "Switch the Herdr window to "+view.To+".", "plugins.note.herdrLayoutSwitch"
			c.MessageArgs = map[string]string{"layout": view.To}
		}
		return nil
	}
	return nil
}

func (s *Service) applyHerdrLayoutView(ctx context.Context, d *document, view *herdrLayoutView, local *localBinding) error {
	home, _ := filepath.EvalSymlinks(herdrHome())
	idx := slices.IndexFunc(local.HerdrLayouts, func(r herdrLayoutReceipt) bool {
		return r.Name == view.Copy && r.showsPanel(home)
	})
	if idx < 0 {
		return errHerdrLayoutSwitch
	}
	done := view.Drop
	if view.From != view.To {
		active, err := s.currentHerdrLayout(ctx)
		if err != nil || active != view.From {
			return errHerdrLayoutSwitch
		}
		if _, err := readHerdrLayout(home, view.To); err != nil {
			return errHerdrLayoutSwitch
		}
		args, _ := json.Marshal(map[string]string{"name": view.To})
		if _, err := s.run(ctx, "herdr-app", "do", "layout.switch", string(args)); err != nil {
			return err
		}
		if active, err := s.currentHerdrLayout(ctx); err != nil || active != view.To {
			return errHerdrLayoutSwitch
		}
		done = append(done, view.To)
	}
	// Returns any package recorded for a reached or abandoned layout are over. Receipts
	// share their backing arrays with the bindings being applied, so this updates those too.
	for _, bindings := range d.state.Bindings {
		for i, rec := range bindings["herdr-app"].HerdrLayouts {
			if rec.Suspended && slices.Contains(done, rec.Left) && rec.Name != view.Copy {
				bindings["herdr-app"].HerdrLayouts[i].Suspended, bindings["herdr-app"].HerdrLayouts[i].Left = false, ""
			}
		}
	}
	if view.Remember {
		local.HerdrLayouts[idx].Suspended, local.HerdrLayouts[idx].Left = view.Suspend, view.Left
	}
	return nil
}

// clearHerdrLayoutSuspension forgets where to return once the mod is synced on,
// except for the copy the view left suspended because a later copy's mod is still off.
func clearHerdrLayoutSuspension(local *localBinding, view *herdrLayoutView) {
	for i, rec := range local.HerdrLayouts {
		if rec.Suspended && (view == nil || !view.Suspend || view.Copy != rec.Name) {
			local.HerdrLayouts[i].Suspended, local.HerdrLayouts[i].Left = false, ""
		}
	}
}
