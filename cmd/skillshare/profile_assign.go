package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/profile"
	"skillshare/internal/profileapply"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
)

func profileAssign(scope profileapply.Scope, o profileOptions) error {
	v, err := profileapply.Load(scope)
	if err != nil {
		return err
	}
	skills, err := ssync.DiscoverSourceSkillsAll(v.Config.EffectiveSkillsSource(), v.Config.SkillsWalk())
	if err != nil {
		return err
	}
	names, err := resolveSkillFlatNames(skills, o.names[1:])
	if err != nil {
		return err
	}
	change, err := profileapply.Mutate(context.Background(), scope, profile.Mutation{
		Op: o.action + "-skills", Name: o.names[0], Skills: names,
	})
	if err != nil {
		return err
	}
	if o.json {
		return writeJSON(struct {
			Profile string               `json:"profile"`
			Changed bool                 `json:"changed"`
			Skills  []profile.SkillResult `json:"skills"`
		}{o.names[0], change.Changed, change.Skills})
	}
	for _, result := range change.Skills {
		ui.Row(ui.MarkOK, result.Name, strings.ReplaceAll(result.Result, "-", " "), ui.RowWidth(names...))
	}
	if change.NeedsSync {
		ui.Next("skillshare sync", "apply the profile change")
	}
	return nil
}

// Resolve against the complete source catalog, including ignored skills, so
// assigning membership never silently enables a skill or changes ignore rules.
func resolveSkillFlatNames(skills []ssync.DiscoveredSkill, args []string) ([]string, error) {
	names := make([]string, 0, len(args))
	for _, arg := range args {
		var exact string
		var candidates []string
		for _, skill := range skills {
			if arg == skill.FlatName || filepath.ToSlash(arg) == filepath.ToSlash(skill.RelPath) {
				exact = skill.FlatName
				break
			}
			if arg == filepath.Base(skill.RelPath) {
				candidates = append(candidates, skill.FlatName)
			}
		}
		if exact != "" {
			names = append(names, exact)
			continue
		}
		slices.Sort(candidates)
		candidates = slices.Compact(candidates)
		switch len(candidates) {
		case 0:
			return nil, fmt.Errorf("skill %q does not exist in the source", arg)
		case 1:
			names = append(names, candidates[0])
		default:
			return nil, fmt.Errorf("skill %q is ambiguous; use one of: %s", arg, strings.Join(candidates, ", "))
		}
	}
	return names, nil
}
