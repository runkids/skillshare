package main

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"skillshare/internal/profile"
	"skillshare/internal/profileapply"
	"skillshare/internal/ui"
)

func cmdProfile(args []string) error {
	o, err := parseProfileArgs(args)
	if err != nil {
		return err
	}
	if o.action == "help" {
		printProfileHelp()
		return nil
	}
	scope, err := profileScope(o)
	if err != nil {
		return err
	}
	if o.action == "list" || o.action == "show" {
		return profileRead(scope, o)
	}
	if o.action == "use" {
		return profileUse(scope, o)
	}
	o.mutation.Op, o.mutation.Name = o.action, o.names[0]
	if o.action == "rename" {
		o.mutation.NewName = o.names[1]
	}
	change, err := profileapply.Mutate(context.Background(), scope, o.mutation)
	if err != nil {
		return err
	}
	if o.json {
		return writeJSON(change)
	}
	message := map[string]string{"create": "created", "edit": "edited", "rename": "renamed", "delete": "deleted"}[o.action]
	if !change.Changed {
		message = "unchanged"
	}
	ui.Row(ui.MarkOK, o.names[0], message, ui.RowWidth(o.names[0]))
	if o.action == "create" && len(o.mutation.AddInclude) == 0 {
		ui.Note("Empty profile: selects no skills.")
	}
	if change.NeedsSync {
		ui.Next("skillshare sync", "apply the profile change")
	}
	return nil
}

func profileUse(scope profileapply.Scope, o profileOptions) error {
	name := ""
	if len(o.names) == 1 {
		name = o.names[0]
	}
	if name == "" && !o.none {
		if !ui.IsTTY() || o.json {
			return fmt.Errorf("profile use requires a name or --none outside an interactive terminal")
		}
		v, err := profileapply.Load(scope)
		if err != nil {
			return err
		}
		options := []ui.Option{{Label: "No profile", Value: ""}}
		for _, key := range slices.Sorted(maps.Keys(v.Catalog.Definitions)) {
			if v.Catalog.Definitions[key].Usable {
				options = append(options, ui.Option{Label: key, Value: key})
			}
		}
		name, err = ui.Select("Active skill profile", options, v.State.Active)
		if err != nil {
			return err
		}
		o.none = name == ""
	}
	out, err := profileapply.Apply(context.Background(), profileapply.Request{Scope: scope, Name: name, None: o.none, DryRun: o.dryRun, NoSync: o.noSync, DiscardEdits: o.discard, AllowEmpty: o.allowEmpty})
	if o.json {
		return writeJSONResult(out, err)
	}
	printProfileInfo(out.Profile)
	for _, target := range out.Targets {
		mark := ui.MarkOK
		if target.State != "applied" {
			mark = ui.MarkWarn
		}
		ui.Row(mark, target.Name, fmt.Sprintf("%s: %d added, %d refreshed, %d removed, %d converted", target.State, len(target.Added), len(target.Refreshed), len(target.Removed), len(target.Converted)), ui.RowWidth(target.Name))
		if target.Error != "" {
			ui.Note(target.Error)
		}
		if target.PruneError != "" {
			ui.Note(target.PruneError)
		}
	}
	for _, entry := range out.Blocked {
		ui.Row(ui.MarkWarn, entry.Target+"/"+entry.Entry, entry.Reason+"; use skillshare collect", ui.RowWidth(entry.Target+"/"+entry.Entry))
	}
	for _, target := range out.SymlinkTargets {
		ui.Note(target + ": symlink target unchanged; use skillshare target " + target + " --mode merge")
	}
	printProfileSnapshots(out.Snapshot)
	if o.dryRun {
		ui.DryRun()
	}
	if o.noSync || out.Selection.Pending {
		ui.Next("skillshare sync", "apply the pending profile switch")
	}
	return err
}

type profileRow struct {
	Name          string                  `json:"name"`
	Description   string                  `json:"description"`
	Skills        *profile.SkillSelection `json:"skills"`
	Selected      []string                `json:"selected"`
	Unmatched     []string                `json:"unmatched"`
	InertExcludes []string                `json:"inert_excludes,omitempty"`
	Usable        bool                    `json:"usable"`
	Reason        string                  `json:"reason,omitempty"`
	Targets       map[string]int          `json:"targets,omitempty"`
}

func profileRead(scope profileapply.Scope, o profileOptions) error {
	v, err := profileapply.Load(scope)
	if err != nil {
		return err
	}
	discovery, err := v.Discover()
	if err != nil {
		return err
	}
	names := make([]string, 0, len(discovery.Skills))
	for _, sk := range discovery.Skills {
		if !sk.Disabled {
			names = append(names, sk.FlatName)
		}
	}
	slices.Sort(names)
	keys := slices.Sorted(maps.Keys(v.Catalog.Definitions))
	if o.action == "show" {
		keys = o.names
		if _, ok := v.Catalog.Definitions[keys[0]]; !ok {
			return fmt.Errorf("profile %q does not exist", keys[0])
		}
	}
	rows := make([]profileRow, 0, len(keys))
	for _, key := range keys {
		d := v.Catalog.Definitions[key]
		p, _ := profile.Preview(d, names)
		row := profileRow{Name: key, Description: d.Description, Skills: d.Skills, Selected: p.Selected, Unmatched: p.Unmatched, InertExcludes: p.InertExcludes, Usable: d.Usable, Reason: d.Reason}
		if o.action == "show" && d.Usable {
			row.Targets, err = v.Candidate(key).TargetCounts(discovery.Skills)
			if err != nil {
				return err
			}
		}
		rows = append(rows, row)
	}
	info := v.Info(discovery.Skills)
	if o.json {
		return writeJSON(struct {
			Profile  *profileapply.Info `json:"profile"`
			Profiles []profileRow       `json:"profiles"`
		}{info, rows})
	}
	printProfileInfo(info)
	if v.Catalog.Reason != "" {
		ui.Warning("%s", v.Catalog.Reason)
	}
	for _, row := range rows {
		mark, text := ui.MarkNone, fmt.Sprintf("%d skills", len(row.Selected))
		if row.Name == v.State.Active {
			mark = ui.MarkOK
			text += " (active)"
		}
		if !row.Usable {
			mark, text = ui.MarkWarn, row.Reason
		}
		ui.Row(mark, row.Name, text, ui.RowWidth(keys...))
		if row.Description != "" {
			ui.Note(row.Description)
		}
		if o.action == "show" {
			if row.Skills == nil {
				ui.Note("Skills: unrestricted")
			} else {
				ui.Note(fmt.Sprintf("Include: %v; exclude: %v", row.Skills.Include, row.Skills.Exclude))
			}
			for _, name := range row.Selected {
				ui.Note(name)
			}
			for _, pattern := range row.Unmatched {
				ui.Note("Unmatched include: " + pattern)
			}
			if len(row.InertExcludes) > 0 {
				ui.Note(fmt.Sprintf("Inert excludes (include selects nothing): %v", row.InertExcludes))
			}
			for _, target := range slices.Sorted(maps.Keys(row.Targets)) {
				ui.Note(fmt.Sprintf("%s: %d selected", target, row.Targets[target]))
			}
		}
	}
	return nil
}

func printProfileInfo(info *profileapply.Info) {
	if info == nil {
		return
	}
	name := info.Name
	if name == "" {
		name = "none"
	}
	value := fmt.Sprintf("%s (%d of %d skills)", name, info.Selected, info.Total)
	mark := ui.MarkNone
	if info.Pending {
		value += "; switch not fully applied"
		mark = ui.MarkWarn
	}
	if info.Reason != "" {
		value += "; " + info.Reason
		mark = ui.MarkWarn
	}
	ui.Row(mark, "Profile", value, ui.RowWidth("Profile"))
}

func printProfileSnapshots(snapshots []profileapply.Snapshot) {
	for _, saved := range snapshots {
		ui.Note("Snapshot: " + saved.Path)
		ui.Note(saved.Retention)
		ui.Next(saved.Restore, "restore the saved edits")
	}
}
