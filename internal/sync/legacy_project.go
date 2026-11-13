package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/utils"
)

// MovedTarget describes one configured project target for legacy-directory cleanup.
type MovedTarget struct {
	Name         string // canonical target name, used for the also_scans lookup
	Path         string // resolved skills path this target writes to
	Pinned       bool   // the project config pinned an explicit path
	TargetNaming string
	// Disabled marks a target with skills off: its path stays protected from
	// other targets' sweeps, but it starts none of its own.
	Disabled bool
}

// MovedProjectTargets pairs each project config entry with its resolved target,
// which is how CleanMovedProjectDirs learns both the path a target writes to and
// whether the config pinned it.
func MovedProjectTargets(cfg *config.ProjectConfig, resolved map[string]config.TargetConfig) []MovedTarget {
	if cfg == nil {
		return nil
	}
	targets := make([]MovedTarget, 0, len(cfg.Targets))
	for _, entry := range cfg.Targets {
		target, ok := resolved[entry.Name]
		if !ok {
			continue
		}
		sc := target.SkillsConfig()
		targets = append(targets, MovedTarget{
			Name:         entry.Name,
			Path:         sc.Path,
			Pinned:       strings.TrimSpace(entry.SkillsConfig().Path) != "",
			TargetNaming: sc.TargetNaming,
			Disabled:     !sc.IsEnabled(),
		})
	}
	return targets
}

// CleanMovedProjectDirs removes skillshare-managed entries that stayed behind in
// a target's also_scans directories after its default project path moved (e.g.
// goose from .goose/skills to .agents/skills). Without this the runtime reads
// both directories and shows every skill twice.
//
// Only directories no configured target writes to are considered, and a target
// with a pinned path is skipped entirely — the user chose that path, so nothing
// moved under them. Inside a directory the prune functions run with an empty
// skills list, so they remove exactly the entries skillshare owns (symlinks into
// the project source and manifest-tracked copies) and leave local folders and
// external symlinks alone.
//
// Returns one message per cleaned directory, for the caller to render.
func CleanMovedProjectDirs(root, sourcePath string, targets []MovedTarget, dryRun bool) []string {
	return cleanMovedProjectDirsWithPolicy(root, sourcePath, targets, dryRun, false, nil).Messages
}

type movedCleanupResult struct {
	Messages []string
	Outcomes []TargetResult
}

func cleanMovedProjectDirsWithPolicy(root, sourcePath string, targets []MovedTarget, dryRun, profileAware bool, discard []DiscardEntry, verification ...PruneOptions) movedCleanupResult {
	var inputs PruneOptions
	if len(verification) > 0 {
		inputs = verification[0]
	}
	owned := make(map[string]bool, len(targets))
	for _, t := range targets {
		if t.Path != "" {
			owned[filepath.Clean(t.Path)] = true
		}
	}

	var result movedCleanupResult
	cleaned := make(map[string]bool)

	for _, t := range targets {
		if t.Pinned || t.Disabled {
			continue
		}
		for _, rel := range config.AlsoScansProject(t.Name) {
			dir := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
			if owned[dir] || cleaned[dir] {
				continue
			}
			// Lstat, never Stat: if the directory is itself a symlink, symlink
			// mode pointed it at the source and pruning would walk into the
			// source skills.
			// ponytail: symlink-mode leftovers are left in place; neither prune
			// function can remove a whole-directory link safely.
			info, err := os.Lstat(dir)
			if err != nil || !info.IsDir() || utils.IsSymlinkOrJunction(dir) {
				continue
			}
			cleaned[dir] = true

			// A dry run leaves both passes reporting the same entry, so count names.
			removed := make(map[string]bool)
			outcome := TargetResult{Target: t.Name}
			seen := make(map[string]bool)
			collect := func(prune *PruneResult, err error) {
				if err != nil && !profileAware {
					return
				}
				if err != nil && profileAware {
					if outcome.PruneError != "" {
						outcome.PruneError += "; "
					}
					outcome.PruneError += fmt.Sprintf("%s: %v", dir, err)
				}
				if prune == nil {
					return
				}
				for _, name := range prune.Removed {
					removed[name] = true
				}
				for _, change := range prune.KeptModified {
					if !seen["kept:"+change.Entry] {
						change.Reason = fmt.Sprintf("%s (in %s)", change.Reason, dir)
						outcome.KeptModified = append(outcome.KeptModified, change)
						seen["kept:"+change.Entry] = true
					}
				}
				for _, change := range prune.Blocked {
					if !seen["blocked:"+change.Entry] {
						change.Reason = fmt.Sprintf("%s (in %s)", change.Reason, dir)
						outcome.Blocked = append(outcome.Blocked, change)
						seen["blocked:"+change.Entry] = true
					}
				}
				for _, name := range prune.FailedRemovals {
					if !seen["failed:"+name] {
						outcome.FailedRemovals = append(outcome.FailedRemovals, filepath.Join(dir, name))
						seen["failed:"+name] = true
					}
				}
			}
			// ManagedOnly: this directory belongs to another tool, so a folder or
			// live link is ours only if the manifest tracks it; broken links into
			// the source are also removed.
			r, err := PruneOrphanLinksWithSkills(PruneOptions{
				ProfileAware:       profileAware,
				Skills:             []DiscoveredSkill{},
				VerificationSkills: inputs.Skills,
				IgnorePatterns:     inputs.IgnorePatterns,
				Discard:            discard,
				TargetPath:         dir,
				SourcePath:         sourcePath,
				TargetNaming:       t.TargetNaming,
				TargetName:         t.Name,
				DryRun:             dryRun,
				ManagedOnly:        true,
			})
			collect(r, err)
			// Sweeps manifest entries whose directory is already gone.
			sc := config.ResourceTargetConfig{Path: dir, TargetNaming: t.TargetNaming}
			r, err = pruneOrphanCopiesWithPolicy(t.Name, sc, nil, dryRun, sourcePath, profileAware, discard, inputs.IgnorePatterns, inputs.Skills)
			collect(r, err)
			if profileAware && (len(outcome.KeptModified) > 0 || len(outcome.Blocked) > 0 || len(outcome.FailedRemovals) > 0 || outcome.PruneError != "") {
				result.Outcomes = append(result.Outcomes, outcome)
			}

			if len(removed) > 0 {
				verb := "Cleaned"
				if dryRun {
					verb = "Would clean"
				}
				noun := "skills"
				if len(removed) == 1 {
					noun = "skill"
				}
				result.Messages = append(result.Messages, fmt.Sprintf(
					"%s %d leftover %s from %s: the default path for '%s' moved to %s",
					verb, len(removed), noun, rel, t.Name, defaultProjectSkillsPath(t.Name)))
			}
		}
	}

	return result
}

func defaultProjectSkillsPath(name string) string {
	if known, ok := config.LookupProjectTarget(name); ok {
		return known.Path
	}
	return ""
}
