package sync

import (
	"errors"
	"fmt"
	"os"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// ErrSymlinkConflict marks a symlink-mode target whose link points somewhere
// other than the source. Force replaces the link.
var ErrSymlinkConflict = errors.New("conflict")

// SkillTarget is one target a skill sync visits.
type SkillTarget struct {
	Name   string
	Target config.TargetConfig
	Mode   string // resolved skills mode: merge, copy, or anything else for symlink
}

// SkillRunOptions holds what differs between skill sync callers.
type SkillRunOptions struct {
	Source         string // skills source directory
	ProjectRoot    string // "" in global mode
	IgnorePatterns []string
	DryRun         bool
	Force          bool
	// SourceIncomplete means a followed source link could not be read, so
	// skills are missing from the list without being removed. Nothing is
	// pruned; linking and copying still run.
	SourceIncomplete bool
	// OnProgress reports copy-mode progress; nil reports nothing.
	OnProgress func(current, total int, skill string)
}

// SkillTargetResult is the outcome of a skill sync for one target.
type SkillTargetResult struct {
	Name    string
	Mode    string
	Linked  []string // merge: linked; copy: copied
	Updated []string
	Skipped []string
	// KeptLocal are the skipped entries that are a user's folder on a skill's name.
	KeptLocal []string
	Pruned    []string // nil when no prune ran or it returned nothing
	// LocalDirs are user folders a merge prune kept.
	LocalDirs  []string
	DirCreated string
	// SymlinkStatus is the target path's status before a symlink-mode sync.
	SymlinkStatus TargetStatus
	Warnings      []string // prune failure, the prune's own warnings, then the kept user folders
	// UnmatchedIncludes are the include patterns that select no skill;
	// UnmatchedWarnings renders them for the CLI.
	UnmatchedIncludes []UnmatchedInclude
	Err               error // the target failed to sync; nothing else is set
}

// SyncSkillTarget links, copies, or symlinks skills into one target, then
// prunes orphans left by earlier syncs. It prints nothing and writes no oplog.
func SyncSkillTarget(t SkillTarget, skills []DiscoveredSkill, opts SkillRunOptions) SkillTargetResult {
	res := SkillTargetResult{Name: t.Name, Mode: t.Mode}
	sc := t.Target.SkillsConfig()

	switch t.Mode {
	case "merge":
		result, err := SyncTargetMergeWithSkills(t.Name, t.Target, skills, opts.Source, opts.DryRun, opts.Force, opts.ProjectRoot)
		if err != nil {
			res.Err = err
			return res
		}
		res.Linked, res.Updated, res.Skipped, res.DirCreated = result.Linked, result.Updated, result.Skipped, result.DirCreated
		res.KeptLocal = result.Skipped // merge skips only folders the user made
		// A partial source may hold what the filter names, so only a full one reports unmatched patterns.
		if opts.SourceIncomplete {
			break
		}
		res.UnmatchedIncludes = result.UnmatchedIncludes
		prune, err := PruneOrphanLinksWithSkills(PruneOptions{
			TargetPath: sc.Path, SourcePath: opts.Source, Skills: skills,
			Include: sc.Include, Exclude: sc.Exclude, TargetNaming: sc.TargetNaming, TargetName: t.Name,
			DryRun: opts.DryRun, Force: opts.Force,
		})
		res.addPrune(prune, err)
		if prune != nil {
			res.LocalDirs = prune.LocalDirs
		}

	case "copy":
		result, err := SyncTargetCopyWithSkillsOptions(t.Name, t.Target, skills, opts.Source, opts.DryRun, opts.Force, opts.OnProgress, CopyOptions{IgnorePatterns: opts.IgnorePatterns})
		if err != nil {
			res.Err = err
			return res
		}
		res.Linked, res.Updated, res.Skipped, res.DirCreated = result.Copied, result.Updated, result.Skipped, result.DirCreated
		res.KeptLocal = result.KeptLocal
		// A partial source may hold what the filter names, so only a full one reports unmatched patterns.
		if opts.SourceIncomplete {
			break
		}
		res.UnmatchedIncludes = result.UnmatchedIncludes
		prune, err := PruneOrphanCopiesWithSkills(sc.Path, skills, sc.Include, sc.Exclude, t.Name, sc.TargetNaming, opts.DryRun)
		res.addPrune(prune, err)

	default:
		res.SymlinkStatus = CheckStatus(sc.Path, opts.Source)
		// A symlink pointing elsewhere is replaced only with force.
		if res.SymlinkStatus == StatusConflict {
			if !opts.Force {
				link, err := utils.ResolveLinkTarget(sc.Path)
				if err != nil {
					link = "(unable to resolve target)"
				}
				res.Err = fmt.Errorf("%w - symlink points to %s (use --force to override)", ErrSymlinkConflict, link)
				return res
			}
			if !opts.DryRun {
				os.Remove(sc.Path)
			}
		}
		res.Err = SyncTarget(t.Name, t.Target, opts.Source, opts.DryRun, opts.ProjectRoot)
	}
	if len(res.KeptLocal) > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s: %s", res.Name, keptLocalWarning(res.KeptLocal)))
	}
	return res
}

// SourceLinkWarnings returns one warning per first-level source link the walk
// did not follow. With pruneSkipped, a warning for an unavailable link also
// says that target entries were kept.
func SourceLinkWarnings(walk sourcewalk.Options, pruneSkipped bool) []string {
	var warnings []string
	for _, s := range walk.Follow.Skipped() {
		w := s.String()
		if pruneSkipped && s.Unavailable {
			w += "; kept existing target entries, nothing pruned this run"
		}
		warnings = append(warnings, w)
	}
	return warnings
}

// UnmatchedWarnings renders the unmatched include patterns, one line each, named by target.
func (res SkillTargetResult) UnmatchedWarnings() []string {
	warnings := make([]string, 0, len(res.UnmatchedIncludes))
	for _, u := range res.UnmatchedIncludes {
		warnings = append(warnings, fmt.Sprintf("%s: %s", res.Name, u.Warning()))
	}
	return warnings
}

// addPrune records a prune's removals and reports its failure and warnings.
func (res *SkillTargetResult) addPrune(prune *PruneResult, err error) {
	if err != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s: prune failed: %v", res.Name, err))
	}
	if prune != nil {
		res.Pruned = prune.Removed
		res.Warnings = append(res.Warnings, prune.Warnings...)
	}
}
