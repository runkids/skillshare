package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	_, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return skills, nil // Empty list if target doesn't exist
		}
		return nil, err
	}

	// If target is a symlink pointing to source, it's using symlink mode — no local skills.
	// If it's an external symlink (e.g., dotfiles manager), follow it and scan.
	if utils.IsSymlinkOrJunction(targetPath) {
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
		if utils.IsSymlinkOrJunction(skillPath) {
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

// PullSkill copies a single skill from target to source.
func PullSkill(skill LocalSkillInfo, sourcePath string, force bool) (retErr error) {
	destPath := filepath.Join(sourcePath, skill.Name)

	// Check if skill already exists in source
	destExists := false
	if _, err := os.Stat(destPath); err == nil {
		destExists = true
		if !force {
			return ErrAlreadyExists
		}
	}

	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		return fmt.Errorf("failed to create source directory: %w", err)
	}

	// Stage beside the destination so every rename stays on one filesystem.
	// The destination is untouched until the complete copy has succeeded.
	stagingDir, err := os.MkdirTemp(sourcePath, ".skillshare-pull-")
	if err != nil {
		return fmt.Errorf("failed to create pull staging directory: %w", err)
	}
	preserveStaging := false
	defer func() {
		if preserveStaging {
			return
		}
		if cleanupErr := os.RemoveAll(stagingDir); cleanupErr != nil {
			cleanupErr = fmt.Errorf("failed to clean pull staging directory: %w", cleanupErr)
			if retErr == nil {
				retErr = cleanupErr
			} else {
				retErr = errors.Join(retErr, cleanupErr)
			}
		}
	}()

	stagedPath := filepath.Join(stagingDir, "replacement")
	// Collect brings user content, not repository metadata.
	if err := copyDirectorySkipGit(skill.Path, stagedPath); err != nil {
		return fmt.Errorf("failed to stage skill copy: %w", err)
	}

	if !destExists {
		if err := os.Rename(stagedPath, destPath); err != nil {
			return fmt.Errorf("failed to install staged skill: %w", err)
		}
		return nil
	}

	backupPath := filepath.Join(stagingDir, "original")
	if err := os.Rename(destPath, backupPath); err != nil {
		return fmt.Errorf("failed to stage existing skill for replacement: %w", err)
	}
	if err := os.Rename(stagedPath, destPath); err != nil {
		if rollbackErr := os.Rename(backupPath, destPath); rollbackErr != nil {
			// Keep the staging directory: it still contains the original skill.
			preserveStaging = true
			return fmt.Errorf("failed to install staged skill: %v; failed to restore existing skill: %w (original preserved at %s)", err, rollbackErr, backupPath)
		}
		return fmt.Errorf("failed to install staged skill: %w", err)
	}

	return nil
}

// PullSkills pulls multiple skills to source
func PullSkills(skills []LocalSkillInfo, sourcePath string, opts PullOptions) (*PullResult, error) {
	result := &PullResult{
		Failed: make(map[string]error),
	}

	for _, skill := range skills {
		if opts.DryRun {
			// Classify exactly as a real pull would: a same-name skill already
			// in source is skipped unless --force, so the preview must not
			// report it as pulled.
			if _, statErr := os.Stat(filepath.Join(sourcePath, skill.Name)); statErr == nil && !opts.Force {
				result.Skipped = append(result.Skipped, skill.Name)
			} else {
				result.Pulled = append(result.Pulled, skill.Name)
			}
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
