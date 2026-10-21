package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

// DetachOptions controls target removal independently of target configuration.
type DetachOptions struct {
	Source string
	DryRun bool
}

// DetachReport lists target paths removed, restored as copies, and left in place.
// Removed includes links replaced by CopiedBack entries. Retained lists immediate
// target entries left untouched. A dry run predicts these operations; otherwise,
// on error the report contains only work completed before the failure.
type DetachReport struct {
	Removed    []string `json:"removed"`
	CopiedBack []string `json:"copiedBack"`
	Retained   []string `json:"retained"`
}

var removeTargetPath = os.Remove

// DetachSkillsTarget removes what sync owns in a skills folder before its target leaves
// the config: the folder symlink, or the manifest and the symlinks into the source.
// Copies and local skills stay.
func DetachSkillsTarget(path string, opts DetachOptions) (DetachReport, error) {
	report := DetachReport{}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return report, nil
	}
	if err != nil {
		return report, fmt.Errorf("failed to inspect target: %w", err)
	}
	if utils.IsLinkMode(path, info.Mode()) {
		// Symlink mode: entire directory is a symlink
		if !opts.DryRun {
			if err := removeTargetPath(path); err != nil {
				return report, fmt.Errorf("failed to remove target symlink: %w", err)
			}
		}
		report.Removed = append(report.Removed, path)
	} else if info.IsDir() {
		// Remove manifest if present (merge/copy mode)
		manifestPath := filepath.Join(path, ManifestFile)
		if opts.DryRun {
			_, err = os.Lstat(manifestPath)
		} else {
			err = removeTargetPath(manifestPath)
		}
		if err == nil {
			report.Removed = append(report.Removed, manifestPath)
		} else if !os.IsNotExist(err) {
			return report, fmt.Errorf("failed to remove target manifest: %w", err)
		}
		// Merge mode: remove individual skill symlinks pointing to source
		if err := unlinkMergeSymlinks(path, opts, &report); err != nil {
			return report, fmt.Errorf("failed to clean target symlinks: %w", err)
		}
	} else {
		report.Retained = append(report.Retained, path)
	}
	return report, nil
}

// unlinkMergeSymlinks removes symlinks in targetPath that point under the
// source directory and copies the skill contents back as real files.
func unlinkMergeSymlinks(targetPath string, opts DetachOptions, report *DetachReport) error {
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return err
	}

	absSource, err := filepath.Abs(opts.Source)
	if err != nil {
		return err
	}
	absSourcePrefix := absSource + string(filepath.Separator)

	for _, entry := range entries {
		// The manifest is already accounted for, but remains present in previews.
		if entry.Name() == ManifestFile {
			continue
		}
		skillPath := filepath.Join(targetPath, entry.Name())

		if !utils.IsSymlinkOrJunction(skillPath) {
			report.Retained = append(report.Retained, skillPath)
			continue
		}

		absLink, err := utils.ResolveLinkTarget(skillPath)
		if err != nil {
			report.Retained = append(report.Retained, skillPath)
			continue
		}

		if !utils.PathHasPrefix(absLink, absSourcePrefix) {
			report.Retained = append(report.Retained, skillPath)
			continue
		}

		// Remove symlink and copy the skill back if source still exists
		if !opts.DryRun {
			if err := removeTargetPath(skillPath); err != nil {
				return fmt.Errorf("remove %s: %w", skillPath, err)
			}
		}
		report.Removed = append(report.Removed, skillPath)
		if _, statErr := os.Stat(absLink); statErr == nil {
			if !opts.DryRun {
				if err := copySkillDir(absLink, skillPath); err != nil {
					return fmt.Errorf("restore %s from %s: %w", skillPath, absLink, err)
				}
			}
			report.CopiedBack = append(report.CopiedBack, skillPath)
		}
	}
	return nil
}

// copySkillDir copies a directory tree from src to dst.
func copySkillDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to compute relative path from %s to %s: %w", src, path, err)
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
