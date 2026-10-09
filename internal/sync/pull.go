package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// ErrAlreadyExists is returned when a skill or agent already exists in source
// and --force was not specified.
var ErrAlreadyExists = errors.New("already exists in source")

// ErrFollowedLink prevents collect from replacing a user's source link.
var ErrFollowedLink = errors.New("followed source link preserved; collect cannot replace it")

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
	Walk   sourcewalk.Options
}

// PullResult describes the result of a pull operation
type PullResult struct {
	Pulled   []string
	Skipped  []string
	Warnings map[string]string
	Failed   map[string]error
}

// HasSkillFile reports whether dir holds a regular SKILL.md file; a folder
// without one (a scratch dir) is not a skill, and a FIFO would block a copy.
func HasSkillFile(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil && info.Mode().IsRegular()
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

		if !HasSkillFile(skillPath) {
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
func PullSkill(skill LocalSkillInfo, sourcePath string, force bool, walks ...sourcewalk.Options) error {
	destPath := filepath.Join(sourcePath, skill.Name)
	if force && len(walks) > 0 {
		if _, ok := walks[0].Follow.Resolve(destPath); ok && utils.IsSymlinkOrJunction(destPath) {
			return ErrFollowedLink
		}
	}

	// Check if skill already exists in source
	if _, err := os.Stat(destPath); err == nil {
		if !force {
			return ErrAlreadyExists
		}
		// Remove existing to overwrite
		if err := os.RemoveAll(destPath); err != nil {
			return fmt.Errorf("failed to remove existing: %w", err)
		}
	}

	// Copy skill to source, skipping .git directories (collect brings
	// user content, not repository metadata).
	return copyDirectorySkipGit(skill.Path, destPath)
}

// PullSkills pulls multiple skills to source
func PullSkills(skills []LocalSkillInfo, sourcePath string, opts PullOptions) (*PullResult, error) {
	result := &PullResult{
		Failed:   make(map[string]error),
		Warnings: make(map[string]string),
	}

	for _, skill := range skills {
		if opts.Force {
			dest := filepath.Join(sourcePath, skill.Name)
			if _, ok := opts.Walk.Follow.Resolve(dest); ok && utils.IsSymlinkOrJunction(dest) {
				result.Skipped = append(result.Skipped, skill.Name)
				result.Warnings[skill.Name] = ErrFollowedLink.Error()
				continue
			}
		}
		if opts.DryRun {
			result.Pulled = append(result.Pulled, skill.Name)
			continue
		}

		err := PullSkill(skill, sourcePath, opts.Force, opts.Walk)
		if err != nil {
			if errors.Is(err, ErrFollowedLink) {
				result.Skipped = append(result.Skipped, skill.Name)
				result.Warnings[skill.Name] = err.Error()
			} else if errors.Is(err, ErrAlreadyExists) {
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
