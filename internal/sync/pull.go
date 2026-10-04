package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// ErrAlreadyExists is returned when a skill or agent already exists in source
// and --force was not specified.
var ErrAlreadyExists = errors.New("already exists in source")

// LocalSkillInfo describes a local skill in a target directory
type LocalSkillInfo struct {
	Name       string
	Path       string
	TargetName string
	Size       int64
	ModTime    time.Time
}

// PullOptions holds options for pull operation
type PullOptions struct {
	DryRun bool
	Force  bool
	// Follow is the source's declaration snapshot; skills named like a
	// declared entry are refused, including while its link is offline.
	Follow *sourcewalk.FollowSet
}

// PullResult describes the result of a pull operation
type PullResult struct {
	Pulled  []string
	Skipped []string
	Failed  map[string]error
}

// FindLocalSkills finds all local (non-symlinked) skills in a target directory.
// syncMode should be the target's current sync mode ("merge", "copy", or "symlink").
// In copy mode, skills listed in the manifest are considered managed and skipped.
// In merge mode, managed skills are symlinks (already filtered), so the manifest is ignored.
func FindLocalSkills(targetPath, sourcePath, syncMode string) ([]LocalSkillInfo, error) {
	var skills []LocalSkillInfo

	// Check if target exists
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return skills, nil // Empty list if target doesn't exist
		}
		return nil, err
	}

	// If target is a symlink pointing to source, it's using symlink mode — no local skills.
	// If it's an external symlink (e.g., dotfiles manager), follow it and scan.
	if utils.IsLinkMode(targetPath, info.Mode()) {
		absLink, err := utils.ResolveLinkTarget(targetPath)
		if err != nil {
			return nil, err
		}
		absSource, _ := filepath.Abs(sourcePath)
		if utils.PathsEqual(absLink, absSource) {
			return skills, nil
		}
		resolved, statErr := os.Stat(targetPath)
		if statErr != nil || !resolved.IsDir() {
			return skills, nil
		}
		// Fall through to scan the resolved directory
	}

	// Target is a directory (merge or copy mode) - scan for local skills

	// Read manifest to identify copy-mode managed skills
	manifest, _ := ReadManifest(targetPath)

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		// Skip hidden files/directories
		if utils.IsHidden(entry.Name()) {
			continue
		}

		skillPath := filepath.Join(targetPath, entry.Name())
		skillInfo, err := os.Lstat(skillPath)
		if err != nil {
			continue
		}

		// Skip symlinks (these are synced from source)
		if utils.IsLinkMode(skillPath, skillInfo.Mode()) {
			continue
		}

		// Only process directories (skills are directories)
		if !skillInfo.IsDir() {
			continue
		}

		// Skip copy-mode managed skills.
		// Only relevant when the target is still in copy mode; after switching
		// to merge the old manifest entries are stale (the physical copies are
		// no longer managed) and should be treated as local.
		if syncMode == "copy" && manifest != nil {
			if _, isManaged := manifest.Managed[entry.Name()]; isManaged {
				continue
			}
		}

		// This is a local skill
		skills = append(skills, LocalSkillInfo{
			Name:    entry.Name(),
			Path:    skillPath,
			ModTime: skillInfo.ModTime(),
		})
	}

	return skills, nil
}

// PullSkill copies a single skill from target to source
func PullSkill(skill LocalSkillInfo, sourcePath string, force bool) error {
	destPath := filepath.Join(sourcePath, skill.Name)

	// Check if skill already exists in source
	_, statErr := os.Stat(destPath)
	if statErr == nil && !force {
		return ErrAlreadyExists
	}
	src, err := sourcefs.Create(sourcePath)
	if err != nil {
		return err
	}
	defer src.Close()
	if statErr == nil {
		// Remove existing to overwrite
		if err := src.RemoveAll(skill.Name); err != nil {
			return fmt.Errorf("failed to remove existing: %w", err)
		}
	}

	// Copy skill to source through the handle, skipping .git directories
	// (collect brings user content, not repository metadata).
	return copyDirectorySkipGit(src.Writer(), skill.Path, destPath)
}

// PullSkills pulls multiple skills to source
func PullSkills(skills []LocalSkillInfo, sourcePath string, opts PullOptions) (*PullResult, error) {
	result := &PullResult{
		Failed: make(map[string]error),
	}

	for _, skill := range skills {
		if opts.Follow != nil {
			if entry, ok := opts.Follow.InFollowed(skill.Name); ok {
				result.Failed[skill.Name] = &sourcefs.LinkError{Path: filepath.Join(sourcePath, entry.Name)}
				continue
			}
		}
		if opts.DryRun {
			result.Pulled = append(result.Pulled, skill.Name)
			continue
		}

		err := PullSkill(skill, sourcePath, opts.Force)
		if err != nil {
			if errors.Is(err, ErrAlreadyExists) {
				result.Skipped = append(result.Skipped, skill.Name)
			} else {
				result.Failed[skill.Name] = err
			}
			continue
		}

		result.Pulled = append(result.Pulled, skill.Name)
	}

	return result, nil
}

// CalculateDirSize calculates total size of a directory by walking all files.
func CalculateDirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
