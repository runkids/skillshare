package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"skillshare/internal/plugin"
)

func pluginChoose(title string, items []checklistItemData, multiple bool, kinds ...string) ([]int, error) {
	kind := "plugin"
	if len(kinds) > 0 {
		kind = kinds[0]
	}
	selected, err := runChecklistTUI(checklistConfig{title: title, items: items, singleSelect: !multiple, itemName: kind})
	if err == nil && len(selected) == 0 {
		return nil, errMCPCancelled
	}
	return selected, err
}

func runPluginManager(s *plugin.Service) error {
	for {
		if err := executePlugin(s, pluginOptions{request: plugin.Request{Action: "list"}, noTUI: true}); err != nil {
			return err
		}
		actions := []string{"add", "import", "inspect", "sync", "check", "update", "enable", "disable", "remove"}
		labels := []string{"Add a plugin", "Import an existing install", "Inspect a plugin", "Sync / retry pending operations", "Check for source updates", "Update a plugin", "Include a target in sync", "Exclude a target from sync", "Remove a plugin"}
		items := make([]checklistItemData, len(actions))
		for i, label := range labels {
			items[i] = checklistItemData{label: label}
		}
		selected, err := pluginChoose("Plugins · Esc to close", items, false, "action")
		if err != nil {
			return err
		}
		if err := pluginWizard(s, pluginOptions{request: plugin.Request{Action: actions[selected[0]]}}); err != nil {
			if err == errMCPCancelled {
				continue
			}
			fmt.Println(err)
		}
	}
}

func pluginWizard(s *plugin.Service, o pluginOptions) error {
	ctx := context.Background()
	r := o.request
	switch r.Action {
	case "add":
		if r.Source == "" {
			v, err := promptMCPText("1/3 · Paste a GitHub repository or local plugin directory", "")
			if err != nil {
				return err
			}
			r.Source = v
		}
		d, err := plugin.DiscoverOptions(ctx, r.Source, r.SourceRef, r.Entry)
		if err != nil {
			return err
		}
		candidates := []plugin.Candidate{}
		items := []checklistItemData{}
		for _, c := range d.Candidates {
			if c.Problem == "" {
				candidates = append(candidates, c)
				items = append(items, checklistItemData{label: c.Name, desc: c.Description + " · " + strings.Join(c.Components, ", ")})
			}
		}
		if len(candidates) == 0 {
			return fmt.Errorf("no locally packaged plugins in this source; install external catalog entries with their native client and import them")
		}
		var chosen *plugin.Candidate
		for _, c := range candidates {
			if c.Name == r.Plugin {
				cc := c
				chosen = &cc
			}
		}
		if chosen == nil {
			idx, err := pluginChoose("Choose a plugin", items, false)
			if err != nil {
				return err
			}
			chosen = &candidates[idx[0]]
		}
		r.Plugin = chosen.Name
		if len(r.Targets) == 0 {
			items = nil
			targets := []string{}
			for _, definition := range plugin.TargetDefinitions() {
				target := definition.Target
				if !slices.Contains(chosen.Targets, target) || !slices.Contains(definition.Operations, "add") {
					continue
				}
				if s.ProjectRoot != "" && !plugin.ProjectSupported(target) {
					continue
				}
				targets = append(targets, target)
				items = append(items, checklistItemData{label: definition.Label, desc: strings.Join(chosen.TargetInfo[target].Components, ", ")})
			}
			if len(items) == 0 {
				return fmt.Errorf("no compatible native clients in this scope")
			}
			idx, err := pluginChoose("2/3 · Which tools should receive this plugin?", items, true, "target")
			if err != nil {
				return err
			}
			for _, i := range idx {
				r.Targets = append(r.Targets, targets[i])
			}
		}
	case "import":
		inventory, err := s.Inventory(ctx)
		if err != nil {
			return err
		}
		type selection struct{ target, id string }
		choices := []selection{}
		items := []checklistItemData{}
		for _, h := range inventory.Hosts {
			supported := false
			for _, definition := range inventory.TargetDefinitions {
				if definition.Target == h.Target && slices.Contains(definition.Operations, "import") {
					supported = true
				}
			}
			if !supported || h.Error != "" {
				continue
			}
			if r.From != "" && r.From != h.Target {
				continue
			}
			for _, i := range h.Installed {
				if i.Filtered {
					continue
				}
				if r.Plugin != "" && r.Plugin != i.ID {
					continue
				}
				choices = append(choices, selection{h.Target, i.ID})
				items = append(items, checklistItemData{label: i.ID, desc: h.Target + " · " + i.Version})
			}
		}
		if len(items) == 0 {
			return fmt.Errorf("no native installations found in this scope; check that the client is installed and signed in")
		}
		idx, err := pluginChoose("Import without reinstalling", items, false)
		if err != nil {
			return err
		}
		r.From = choices[idx[0]].target
		r.Plugin = choices[idx[0]].id
	default:
		if r.Name == "" && r.Action != "sync" && r.Action != "check" {
			inventory, err := s.Inventory(ctx)
			if err != nil {
				return err
			}
			names := []string{}
			for name := range inventory.Packages {
				names = append(names, name)
			}
			slices.Sort(names)
			if len(names) == 0 {
				return fmt.Errorf("no managed plugins; add or import one first")
			}
			items := make([]checklistItemData, len(names))
			for i, name := range names {
				items[i] = checklistItemData{label: name}
			}
			idx, err := pluginChoose("Choose a plugin", items, false)
			if err != nil {
				return err
			}
			r.Name = names[idx[0]]
		}
	}
	o.request = r
	if len(r.Targets) == 0 && (r.Action == "enable" || r.Action == "disable" || r.Action == "update" || r.Action == "remove") {
		inventory, err := s.Inventory(ctx)
		if err != nil {
			return err
		}
		targets := []string{}
		items := []checklistItemData{}
		for _, definition := range inventory.TargetDefinitions {
			target := definition.Target
			if !slices.Contains(definition.Operations, r.Action) {
				continue
			}
			if b, ok := inventory.Packages[r.Name].Bindings[target]; ok {
				targets = append(targets, target)
				items = append(items, checklistItemData{label: definition.Label, desc: b.ID})
			}
		}
		if len(items) == 0 {
			return fmt.Errorf("no managed targets support %s; inspect plugin capabilities and use the native client", r.Action)
		}
		selected, err := pluginChoose("Which targets?", items, true, "target")
		if err != nil {
			return err
		}
		for _, i := range selected {
			r.Targets = append(r.Targets, targets[i])
		}
		o.request = r
	}
	o.noTUI = true
	if r.Action == "inspect" || r.Action == "check" {
		return executePlugin(s, o)
	}
	p, err := s.Preview(ctx, r)
	if err != nil {
		return err
	}
	printPluginPlan(p)
	if p.Blocked {
		return fmt.Errorf("resolve blocked targets before continuing")
	}
	if len(p.Changes) == 0 {
		return nil
	}
	_, err = pluginChoose("3/3 · Apply these changes? Esc cancels", []checklistItemData{{label: "Apply", desc: "Use native plugin management; login and hook trust remain in each Agent"}}, false)
	if err != nil {
		return err
	}
	o.revision = p.Revision
	return executePlugin(s, o)
}
