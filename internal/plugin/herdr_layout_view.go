package plugin

import (
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

func (s *Service) prepareHerdrLayoutView(ctx context.Context, c *Change, records []herdrLayoutReceipt, resume, remember bool) error {
	if len(records) == 0 {
		return nil
	}
	home, _ := filepath.EvalSymlinks(herdrHome())
	active := ""
	for _, rec := range slices.Backward(records) {
		if resume && !rec.Suspended || !rec.showsPanel(home) {
			continue
		}
		if active == "" {
			var err error
			active, err = s.currentHerdrLayout(ctx)
			if err != nil {
				return errHerdrLayoutSwitch
			}
		}
		view := herdrLayoutView{Copy: rec.Name, From: rec.Name, To: rec.From, Resume: resume, Remember: remember}
		if resume {
			view.From, view.To = view.To, view.From
		}
		if active != view.From {
			continue
		}
		if _, err := readHerdrLayout(home, view.To); err != nil {
			return errHerdrLayoutSwitch
		}
		c.herdrLayoutView = &view
		if remember {
			c.Message, c.MessageKey = "Switch the Herdr window to "+view.To+".", "plugins.note.herdrLayoutSwitch"
			c.MessageArgs = map[string]string{"layout": view.To}
		}
		return nil
	}
	return nil
}

func (s *Service) applyHerdrLayoutView(ctx context.Context, view *herdrLayoutView, local *localBinding) error {
	home, _ := filepath.EvalSymlinks(herdrHome())
	idx := slices.IndexFunc(local.HerdrLayouts, func(r herdrLayoutReceipt) bool {
		return r.Name == view.Copy && r.showsPanel(home)
	})
	if idx < 0 {
		return errHerdrLayoutSwitch
	}
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
	if view.Remember {
		local.HerdrLayouts[idx].Suspended = !view.Resume
	}
	return nil
}

func clearHerdrLayoutSuspension(local *localBinding) bool {
	cleared := false
	for i := range local.HerdrLayouts {
		if local.HerdrLayouts[i].Suspended {
			local.HerdrLayouts[i].Suspended = false
			cleared = true
		}
	}
	return cleared
}
