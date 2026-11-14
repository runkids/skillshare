package main

import (
	"fmt"
	"maps"
	"slices"

	"skillshare/internal/config"
	"skillshare/internal/profile"
	"skillshare/internal/profileapply"
	"skillshare/internal/sync"
)

// Use the loaded scope explicitly: the project doctor's synthetic global config
// does not carry the project's machine-local selector or state.
func profileDoctorChecks(cfg *config.Config, pc *config.ProjectConfig, root string, skills []sync.DiscoveredSkill, discoverErr error) []doctorCheck {
	v := &profileapply.View{Config: cfg, State: cfg.ProfileState(), Catalog: profile.Parse(&cfg.Profiles)}
	if pc != nil {
		v.Project, v.Scope.ProjectRoot = pc, root
		v.State, v.Catalog = pc.ProfileState(), profile.Parse(&pc.Profiles)
	}
	var checks []doctorCheck
	add := func(name, status, message string, details []string) {
		checks = append(checks, doctorCheck{Name: name, Status: status, Message: message, Details: details})
	}
	activeErr := v.ActiveProfileError()
	if activeErr != nil {
		add("profile_unavailable", checkError, activeErr.Error(), nil)
	}
	if v.State.Pending {
		add("profile_pending", checkWarning, "Profile switch not fully applied; run skillshare sync", nil)
	}
	if discoverErr == nil {
		names := profileSkillNames(skills)
		for _, name := range slices.Sorted(maps.Keys(v.Catalog.Definitions)) {
			d := v.Catalog.Definitions[name]
			preview, err := profile.Preview(d, names)
			if err != nil {
				continue // unusable active definitions are reported above
			}
			if len(preview.Unmatched) > 0 {
				add("profile_unmatched", checkWarning, fmt.Sprintf("Profile %s has patterns matching no skills", name), preview.Unmatched)
			}
			if len(preview.InertExcludes) > 0 {
				add("profile_inert_excludes", checkWarning, fmt.Sprintf("Profile %s has inert excludes with an empty include", name), preview.InertExcludes)
			}
		}
	}
	if activeErr != nil || v.State.Active == "" && !v.State.Pending {
		return checks
	}
	targets, symlinks := v.Eligible()
	if v.State.Active != "" && len(symlinks) > 0 {
		add("profile_symlink_target", checkWarning, "Symlink-mode targets ignore the active profile", symlinks)
	}
	if discoverErr != nil {
		return checks
	}
	targets = config.WithSharedSkillsTargets(targets, cfg.Mode)
	ignore := cfg.Ignore
	if pc != nil {
		ignore = pc.Ignore
	}
	var kept []string
	for _, name := range slices.Sorted(maps.Keys(targets)) {
		target := targets[name]
		row := sync.SyncSkillTarget(sync.SkillTarget{Name: name, Target: target, Mode: getTargetMode(target.SkillsConfig().Mode, cfg.Mode)}, skills,
			sync.SkillRunOptions{Source: cfg.EffectiveSkillsSource(), ProjectRoot: v.Scope.ProjectRoot, IgnorePatterns: sync.EffectiveFileIgnorePatterns(ignore), DryRun: true, ProfileAware: true})
		if row.Err != nil {
			add("profile_kept_modified", checkWarning, fmt.Sprintf("Could not inspect profile target %s: %v", name, row.Err), nil)
		}
		for _, entry := range row.KeptModified {
			kept = append(kept, name+"/"+entry.Entry)
		}
	}
	if len(kept) > 0 {
		add("profile_kept_modified", checkWarning, "Modified managed copies will be kept; use skillshare collect", kept)
	}
	return checks
}

func profileSkillNames(skills []sync.DiscoveredSkill) []string {
	var names []string
	for _, skill := range skills {
		if !skill.Disabled {
			names = append(names, skill.FlatName)
		}
	}
	return names
}
