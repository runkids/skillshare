package install

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
	"skillshare/internal/validate"
)

// configSkillEntry pairs a SkillEntryDTO with its parsed Source.
type configSkillEntry struct {
	dto    SkillEntryDTO
	source *Source
	relock bool // already installed, but at another commit than the lockfile's
}

// optsFor stages a relock like an update, so a blocked audit keeps the
// installed copy.
func (e configSkillEntry) optsFor(opts InstallOptions) InstallOptions {
	if e.relock {
		opts.Force, opts.Update = true, true
	}
	return opts
}

// configSkillGroup holds config skills sharing the same CloneURL.
// The group is cloned once; each skill is then installed from the local clone.
type configSkillGroup struct {
	cloneURL string
	skills   []configSkillEntry
}

// groupConfigSkillsByRepo partitions parsed config entries by CloneURL + Branch.
// Entries that share the same git repo AND branch (2+ skills with Subdir) are grouped
// for a single clone. Entries that cannot be grouped (local path, no subdir,
// or sole skill from a repo) are returned in the singles slice.
func groupConfigSkillsByRepo(entries []configSkillEntry) (groups []configSkillGroup, singles []configSkillEntry) {
	buckets := make(map[string]*configSkillGroup)
	var order []string

	for _, e := range entries {
		if !e.source.IsGit() || e.source.Subdir == "" || e.source.HasAmbiguousWebRef() {
			singles = append(singles, e)
			continue
		}
		// Include branch in key so same repo on different branches are not grouped together
		key := e.source.CloneURL
		if e.source.Branch != "" {
			key += "\t" + e.source.Branch
		}
		// Skills of one repo installed at different times are locked apart.
		key += "\t" + e.source.Commit
		if g, ok := buckets[key]; ok {
			g.skills = append(g.skills, e)
		} else {
			buckets[key] = &configSkillGroup{
				cloneURL: e.source.CloneURL,
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

// MetadataSkillEntries lists the entries of a metadata store as config skills,
// sorted by key; it is the global-mode source of InstallContext.ConfigSkills.
func MetadataSkillEntries(store *MetadataStore) []SkillEntryDTO {
	names := store.List() // sorted
	dtos := make([]SkillEntryDTO, 0, len(names))
	for _, name := range names {
		entry := store.Get(name)
		if entry == nil {
			continue
		}
		group, bareName := splitTrackedRelPath(filepath.ToSlash(KeyToRelPath(name, entry)))
		dtos = append(dtos, SkillEntryDTO{
			Name:    bareName,
			Source:  entry.Source,
			Tracked: entry.Tracked,
			Group:   group,
			Branch:  entry.Branch,
		})
	}
	return dtos
}

// MissingFromConfig returns the config skills InstallFromConfig would install:
// entries with a name whose directory does not exist on disk.
func MissingFromConfig(ctx InstallContext) []SkillEntryDTO {
	sourcePath := ctx.SourcePath()
	var missing []SkillEntryDTO
	for _, skill := range ctx.ConfigSkills() {
		if _, bareName := skill.EffectiveParts(); strings.TrimSpace(bareName) == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(sourcePath, filepath.FromSlash(skill.FullName()))); err != nil {
			missing = append(missing, skill)
		}
	}
	return missing
}

// InstallFromConfig iterates over the remote skills listed in the config
// (via ctx.ConfigSkills) and installs each one that is not already present.
// It handles both tracked repos and plain skills, delegates per-skill hooks
// to ctx.PostInstallSkill, and calls ctx.Reconcile when at least one skill
// was installed or found moved, so the moved skill's record follows it.
//
// The caller is responsible for UI chrome (logo, spinner, next-steps).
func InstallFromConfig(ctx InstallContext, opts InstallOptions) (ConfigInstallResult, error) {
	result := ConfigInstallResult{
		InstalledSkills: make([]string, 0),
		FailedSkills:    make([]string, 0),
	}

	sourcePath := ctx.SourcePath()
	// A grouped entry installs to sourcePath/<group>/<name> without opts.Into, so
	// the skills root cannot be derived from the destination path.
	opts.SourceDir = sourcePath

	parseOpts := ParseOptions{
		GitLabHosts: ctx.GitLabHosts(),
		AzureHosts:  ctx.AzureHosts(),
		CNBHosts:    ctx.CNBHosts(),
		GiteaHosts:  ctx.GiteaHosts(),
	}

	// ── Classify skills: tracked / plain (groupable vs singles) ──
	var tracked []SkillEntryDTO
	var plain []configSkillEntry

	store := LoadMetadataOrNew(sourcePath)
	moved := 0

	for _, skill := range ctx.ConfigSkills() {
		_, bareName := skill.EffectiveParts()
		if strings.TrimSpace(bareName) == "" {
			continue
		}

		displayName := skill.FullName()
		destPath := filepath.Join(sourcePath, filepath.FromSlash(displayName))
		locked := opts.Lock.CommitFor(displayName, skill.Source)

		// Skip skills that already exist on disk, unless the lockfile moved on
		// (a teammate updated) and this copy has to follow.
		relock := false
		if _, err := os.Stat(destPath); err == nil {
			if locked == "" || InstalledCommit(destPath, store.GetByPath(displayName)) == locked {
				result.Skipped++
				if skill.Tracked {
					result.SkippedRepos++
				}
				if !opts.Quiet {
					ui.StepDone(displayName, "skipped (already exists)")
				}
				continue
			}
			if opts.DryRun {
				if !opts.Quiet {
					ui.StepDone(displayName, "would move to locked commit "+shortHash(locked))
				}
				continue
			}
			if IsGitRepo(destPath) {
				if err := relockGitRepo(destPath, locked, opts); err != nil {
					if !opts.Quiet {
						ui.StepFail(displayName, err.Error())
					}
					result.FailedSkills = append(result.FailedSkills, displayName)
					continue
				}
				if !opts.Quiet {
					ui.StepDone(displayName, "moved to locked commit "+shortHash(locked))
				}
				result.InstalledSkills = append(result.InstalledSkills, displayName)
				result.Installed++
				continue
			}
			relock = true
		} else if to := movedCopyOf(store, sourcePath, skill, opts.SourceFollow, locked); to != "" {
			// Reconcile moves the record to the copy; installing would duplicate it.
			result.Skipped++
			if !opts.DryRun {
				moved++
			}
			if !opts.Quiet {
				ui.StepDone(displayName, "moved to "+to+", record follows")
			}
			continue
		}

		source, err := ParseSourceWithOptions(skill.Source, parseOpts)
		if err != nil {
			if !opts.Quiet {
				ui.StepFail(displayName, fmt.Sprintf("invalid source: %v", err))
			}
			result.FailedSkills = append(result.FailedSkills, displayName)
			continue
		}
		source.Name = bareName
		source.ApplyRecordedBranch(skill.Branch)
		source.Commit = locked

		if skill.Tracked {
			tracked = append(tracked, skill)
		} else {
			plain = append(plain, configSkillEntry{dto: skill, source: source, relock: relock})
		}
	}

	groups, singles := groupConfigSkillsByRepo(plain)

	// ── Phase 1: tracked repos (unchanged — each does its own clone+discover) ──
	for _, skill := range tracked {
		groupDir, bareName := skill.EffectiveParts()
		displayName := skill.FullName()

		source, err := ParseSourceWithOptions(skill.Source, parseOpts)
		if err != nil {
			if !opts.Quiet {
				ui.StepFail(displayName, fmt.Sprintf("invalid source: %v", err))
			}
			result.FailedSkills = append(result.FailedSkills, displayName)
			continue
		}
		source.Name = bareName
		source.Branch = skill.Branch
		source.Commit = opts.Lock.CommitFor(displayName, skill.Source)

		installed := installTrackedFromConfig(source, sourcePath, displayName, groupDir, opts)
		if installed.failed {
			result.FailedSkills = append(result.FailedSkills, displayName)
			continue
		}
		if opts.DryRun {
			if !opts.Quiet {
				ui.StepDone(displayName, installed.action)
			}
			continue
		}
		result.InstalledSkills = append(result.InstalledSkills, installed.skills...)
		result.Installed++
		result.InstalledRepos++
		result.InstalledRepoSkills += installed.skillCount
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
				if err := sourcefs.MkdirAllIn(sourcePath, filepath.FromSlash(groupDir), opts.SourceFollow); err != nil {
					if !opts.Quiet {
						ui.StepFail(displayName, fmt.Sprintf("failed to create group directory: %v", err))
					}
					result.FailedSkills = append(result.FailedSkills, displayName)
					continue
				}
			}

			_, err := InstallFromDiscovery(discovery, skill, destPath, e.optsFor(opts))
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

		ok := installPlainFromConfig(ctx, e.source, sourcePath, destPath, displayName, bareName, groupDir, e.optsFor(opts))
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

	// ── Phase 4: Reconcile config after installs and moves ──
	if (result.Installed > 0 || moved > 0) && !opts.DryRun {
		if err := ctx.Reconcile(); err != nil {
			return result, err
		}
	}

	return result, nil
}

// movedCopyOf returns the source-relative path the recorded skill was moved
// to (see MetadataStore.MovedEntryKey), or "" unless exactly one copy matches.
// The record must still install what skill declares, and a copy at another
// commit than locked does not count, so the lockfile's pin is kept.
func movedCopyOf(store *MetadataStore, sourcePath string, skill SkillEntryDTO, follow *sourcewalk.Follow, locked string) string {
	displayName := skill.FullName()
	entry := store.GetByPath(displayName)
	if entry == nil || len(entry.FileHashes) == 0 || !entry.MatchesDeclaration(skill) {
		return ""
	}
	var found []string
	walkFailed := false // an unreadable directory may hide a copy
	// The walk reports paths under the resolved root, as in reconcile.
	root := utils.ResolveSymlink(sourcePath)
	_ = sourcewalk.WalkDir(root, sourcewalk.Options{Follow: follow}, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			walkFailed = true
			return nil
		}
		if !d.IsDir() || p == root {
			return nil
		}
		if utils.IsHidden(d.Name()) {
			return filepath.SkipDir
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return nil
		}
		// Reconcile does not look inside an installed skill or tracked checkout,
		// so a copy there could not take the record.
		if e := store.GetByPath(rel); IsTrackedCheckout(p) || e != nil && e != entry && e.Source != "" {
			return filepath.SkipDir
		}
		if d.Name() == path.Base(displayName) {
			key, hashErr := store.MovedEntryKey(root, rel, p, follow)
			if hashErr != nil {
				walkFailed = true
			}
			if key != "" && store.Entries[key] == entry {
				found = append(found, filepath.ToSlash(rel))
				return filepath.SkipDir
			}
		}
		return nil
	})
	// An unreadable source link may hide another copy.
	if len(found) != 1 || walkFailed || follow.Incomplete() || locked != "" && InstalledCommit(filepath.Join(root, filepath.FromSlash(found[0])), entry) != locked {
		return ""
	}
	return found[0]
}

// trackedInstallOutcome captures the result of a single tracked-repo install
// within the config loop.
type trackedInstallOutcome struct {
	failed     bool
	action     string   // only meaningful on dry-run
	skills     []string // skills names to record
	skillCount int      // skills discovered in the repo
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
	trackOpts.Name = source.Name // keep the name recorded in config
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
	return trackedInstallOutcome{skills: skills, skillCount: trackedResult.SkillCount}
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
		if err := sourcefs.MkdirAllIn(sourcePath, filepath.FromSlash(groupDir), opts.SourceFollow); err != nil {
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
