package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/ui"
	"skillshare/internal/validate"
)

// configSkillEntry pairs a SkillEntryDTO with its parsed Source.
type configSkillEntry struct {
	dto    SkillEntryDTO
	source *Source
}

// configSkillGroup holds config skills sharing the same CloneURL.
// The group is cloned once; each skill is then installed from the local clone.
type configSkillGroup struct {
	cloneURL string
	skills   []configSkillEntry
}

// groupConfigSkillsByRepo partitions parsed config entries by CloneURL.
// Entries that share the same git repo (2+ skills with Subdir) are grouped
// for a single clone. Entries that cannot be grouped (local path, no subdir,
// or sole skill from a repo) are returned in the singles slice.
func groupConfigSkillsByRepo(entries []configSkillEntry) (groups []configSkillGroup, singles []configSkillEntry) {
	buckets := make(map[string]*configSkillGroup)
	var order []string

	for _, e := range entries {
		if !e.source.IsGit() || e.source.Subdir == "" {
			singles = append(singles, e)
			continue
		}
		key := e.source.CloneURL
		if g, ok := buckets[key]; ok {
			g.skills = append(g.skills, e)
		} else {
			buckets[key] = &configSkillGroup{
				cloneURL: key,
				skills:   []configSkillEntry{e},
			}
			order = append(order, key)
		}
	}

	for _, key := range order {
		g := buckets[key]
		if len(g.skills) < 2 {
			singles = append(singles, g.skills[0])
		} else {
			groups = append(groups, *g)
		}
	}
	return groups, singles
}

// repoSourceForConfigGroup converts a subdir source into a repo-root source
// for whole-repo cloning. Similar to repoSourceForGroupedClone in search_batch.go.
func repoSourceForConfigGroup(src *Source) Source {
	repoSource := *src
	repoSource.Subdir = ""
	repoSource.Raw = repoSource.CloneURL

	if root, err := ParseSource(repoSource.CloneURL); err == nil {
		repoSource.Type = root.Type
		repoSource.Raw = root.Raw
		repoSource.Name = root.Name
	}

	return repoSource
}

// matchDiscoveredSkillBySubdir finds a SkillInfo in the discovery result that
// matches the given subdir path.
func matchDiscoveredSkillBySubdir(discovery *DiscoveryResult, subdir string) (SkillInfo, bool) {
	for _, skill := range discovery.Skills {
		if skill.Path == subdir {
			return skill, true
		}
	}
	return SkillInfo{}, false
}

// InstallFromConfig iterates over the remote skills listed in the config
// (via ctx.ConfigSkills) and installs each one that is not already present.
// It handles both tracked repos and plain skills, delegates per-skill hooks
// to ctx.PostInstallSkill, and calls ctx.Reconcile when at least one skill
// was installed.
//
// The caller is responsible for UI chrome (logo, spinner, next-steps).
func InstallFromConfig(ctx InstallContext, opts InstallOptions) (ConfigInstallResult, error) {
	result := ConfigInstallResult{
		InstalledSkills: make([]string, 0),
		FailedSkills:    make([]string, 0),
	}

	sourcePath := ctx.SourcePath()

	// ── Classify skills: tracked / plain (groupable vs singles) ──
	var tracked []SkillEntryDTO
	var plain []configSkillEntry

	for _, skill := range ctx.ConfigSkills() {
		_, bareName := skill.EffectiveParts()
		if strings.TrimSpace(bareName) == "" {
			continue
		}

		displayName := skill.FullName()
		destPath := filepath.Join(sourcePath, filepath.FromSlash(displayName))

		// Skip skills that already exist on disk.
		if _, err := os.Stat(destPath); err == nil {
			result.Skipped++
			if !opts.Quiet {
				ui.StepDone(displayName, "skipped (already exists)")
			}
			continue
		}

		source, err := ParseSource(skill.Source)
		if err != nil {
			if !opts.Quiet {
				ui.StepFail(displayName, fmt.Sprintf("invalid source: %v", err))
			}
			result.FailedSkills = append(result.FailedSkills, displayName)
			continue
		}
		source.Name = bareName

		if skill.Tracked {
			tracked = append(tracked, skill)
		} else {
			plain = append(plain, configSkillEntry{dto: skill, source: source})
		}
	}

	groups, singles := groupConfigSkillsByRepo(plain)

	// ── Phase 1: tracked repos (unchanged — each does its own clone+discover) ──
	for _, skill := range tracked {
		groupDir, bareName := skill.EffectiveParts()
		displayName := skill.FullName()

		source, err := ParseSource(skill.Source)
		if err != nil {
			if !opts.Quiet {
				ui.StepFail(displayName, fmt.Sprintf("invalid source: %v", err))
			}
			result.FailedSkills = append(result.FailedSkills, displayName)
			continue
		}
		source.Name = bareName

		installed := installTrackedFromConfig(source, sourcePath, displayName, groupDir, opts)
		if installed.failed {
			result.FailedSkills = append(result.FailedSkills, displayName)
			continue
		}
		if opts.DryRun {
			ui.StepDone(displayName, installed.action)
			continue
		}
		result.InstalledSkills = append(result.InstalledSkills, installed.skills...)
		result.Installed++
	}

	// ── Phase 2: grouped plain skills (clone once per repo) ──
	for _, group := range groups {
		repoSource := repoSourceForConfigGroup(group.skills[0].source)
		discovery, cloneErr := DiscoverFromGitWithProgress(&repoSource, opts.OnProgress)

		if cloneErr != nil {
			// Clone failed — mark all skills in group as failed.
			for _, e := range group.skills {
				displayName := e.dto.FullName()
				if !opts.Quiet {
					ui.StepFail(displayName, fmt.Sprintf("clone failed: %v", cloneErr))
				}
				result.FailedSkills = append(result.FailedSkills, displayName)
			}
			continue
		}

		var fallback []configSkillEntry
		for _, e := range group.skills {
			groupDir, bareName := e.dto.EffectiveParts()
			displayName := e.dto.FullName()
			destPath := filepath.Join(sourcePath, filepath.FromSlash(displayName))

			skill, found := matchDiscoveredSkillBySubdir(discovery, e.source.Subdir)
			if !found {
				// Can't match — fall back to Phase 3 individual install.
				fallback = append(fallback, e)
				continue
			}

			if err := validate.SkillName(bareName); err != nil {
				if !opts.Quiet {
					ui.StepFail(displayName, fmt.Sprintf("invalid name: %v", err))
				}
				result.FailedSkills = append(result.FailedSkills, displayName)
				continue
			}

			if groupDir != "" {
				if err := os.MkdirAll(filepath.Join(sourcePath, filepath.FromSlash(groupDir)), 0o755); err != nil {
					if !opts.Quiet {
						ui.StepFail(displayName, fmt.Sprintf("failed to create group directory: %v", err))
					}
					result.FailedSkills = append(result.FailedSkills, displayName)
					continue
				}
			}

			_, err := InstallFromDiscovery(discovery, skill, destPath, opts)
			if err != nil {
				if !opts.Quiet {
					ui.StepFail(displayName, err.Error())
				}
				result.FailedSkills = append(result.FailedSkills, displayName)
				continue
			}

			if opts.DryRun {
				if !opts.Quiet {
					ui.StepDone(displayName, "would install (grouped)")
				}
				continue
			}

			if err := ctx.PostInstallSkill(displayName); err != nil {
				ui.Warning("post-install hook failed for %s: %v", displayName, err)
			}
			if !opts.Quiet {
				ui.StepDone(displayName, "installed")
			}
			result.InstalledSkills = append(result.InstalledSkills, displayName)
			result.Installed++
		}

		CleanupDiscovery(discovery)
		singles = append(singles, fallback...)
	}

	// ── Phase 3: singles (unchanged per-skill flow) ──
	for _, e := range singles {
		groupDir, bareName := e.dto.EffectiveParts()
		displayName := e.dto.FullName()
		destPath := filepath.Join(sourcePath, filepath.FromSlash(displayName))

		ok := installPlainFromConfig(ctx, e.source, sourcePath, destPath, displayName, bareName, groupDir, opts)
		if !ok {
			result.FailedSkills = append(result.FailedSkills, displayName)
			continue
		}
		if opts.DryRun {
			continue // StepDone already called inside installPlainFromConfig
		}
		result.InstalledSkills = append(result.InstalledSkills, displayName)
		result.Installed++
	}

	// ── Phase 4: Reconcile config after successful installs ──
	if result.Installed > 0 && !opts.DryRun {
		if err := ctx.Reconcile(); err != nil {
			return result, err
		}
	}

	return result, nil
}

// trackedInstallOutcome captures the result of a single tracked-repo install
// within the config loop.
type trackedInstallOutcome struct {
	failed bool
	action string   // only meaningful on dry-run
	skills []string // skills names to record
}

// installTrackedFromConfig installs a single tracked repo from config and
// returns a summary used by the caller loop.
func installTrackedFromConfig(
	source *Source,
	sourcePath string,
	displayName string,
	groupDir string,
	opts InstallOptions,
) trackedInstallOutcome {
	trackOpts := opts
	if groupDir != "" {
		trackOpts.Into = groupDir
	}

	trackedResult, err := InstallTrackedRepo(source, sourcePath, trackOpts)
	if err != nil {
		if !opts.Quiet {
			ui.StepFail(displayName, err.Error())
		}
		return trackedInstallOutcome{failed: true}
	}

	if opts.DryRun {
		return trackedInstallOutcome{action: trackedResult.Action}
	}

	if !opts.Quiet {
		ui.StepDone(displayName, fmt.Sprintf("installed (tracked, %d skills)", trackedResult.SkillCount))
	}

	skills := trackedResult.Skills
	if len(skills) == 0 {
		skills = []string{displayName}
	}
	return trackedInstallOutcome{skills: skills}
}

// installPlainFromConfig installs a single non-tracked skill from config.
// Returns true on success, false on failure (UI messages emitted internally).
func installPlainFromConfig(
	ctx InstallContext,
	source *Source,
	sourcePath string,
	destPath string,
	displayName string,
	bareName string,
	groupDir string,
	opts InstallOptions,
) bool {
	if err := validate.SkillName(bareName); err != nil {
		if !opts.Quiet {
			ui.StepFail(displayName, fmt.Sprintf("invalid name: %v", err))
		}
		return false
	}

	// Ensure group directory exists.
	if groupDir != "" {
		if err := os.MkdirAll(filepath.Join(sourcePath, filepath.FromSlash(groupDir)), 0o755); err != nil {
			if !opts.Quiet {
				ui.StepFail(displayName, fmt.Sprintf("failed to create group directory: %v", err))
			}
			return false
		}
	}

	result, err := Install(source, destPath, opts)
	if err != nil {
		if !opts.Quiet {
			ui.StepFail(displayName, err.Error())
		}
		return false
	}

	if opts.DryRun {
		if !opts.Quiet {
			ui.StepDone(displayName, result.Action)
		}
		return true
	}

	if err := ctx.PostInstallSkill(displayName); err != nil {
		ui.Warning("post-install hook failed for %s: %v", displayName, err)
	}

	if !opts.Quiet {
		ui.StepDone(displayName, "installed")
	}
	return true
}
