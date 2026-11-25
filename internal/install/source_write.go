package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
)

// removeInSource removes destPath, below the skills source sourceDir, through
// the source-write handle, so a link at or above it is refused instead of
// replaced.
func removeInSource(sourceDir, destPath string, follow *sourcewalk.Follow) error {
	src, err := sourcefs.Open(sourceDir, follow)
	if err != nil {
		return err
	}
	defer src.Close()
	rel, err := src.Rel(destPath)
	if err != nil {
		return err
	}
	return src.RemoveAll(rel)
}

// setAsideInSource renames destPath, below the skills source sourceDir, to a
// hidden sibling so a reinstall over it can be undone. restore puts it back in
// place of whatever the failed install left; discard deletes it.
func setAsideInSource(sourceDir, destPath string, follow *sourcewalk.Follow) (restore, discard func() error, err error) {
	src, err := sourcefs.Open(sourceDir, follow)
	if err != nil {
		return nil, nil, err
	}
	rel, err := src.Rel(destPath)
	if err == nil {
		backup := filepath.Join(filepath.Dir(rel), ".skillshare-"+filepath.Base(rel)+".old."+strconv.FormatInt(time.Now().UnixNano(), 36))
		if err = src.Rename(rel, backup); err == nil {
			restore = func() error {
				defer src.Close()
				if err := src.RemoveAll(rel); err != nil {
					return err
				}
				return src.Rename(backup, rel)
			}
			discard = func() error {
				defer src.Close()
				return src.RemoveAll(backup)
			}
			return restore, discard, nil
		}
	}
	src.Close()
	return nil, nil, err
}

// swapStagedIntoSource replaces destPath, below the skills source sourceDir,
// with the staged directory. The rename crosses the source's edge, so the
// in-source side is checked through the handle first. Only a cross-device
// rename falls back to a copy, and that copy also goes through the handle;
// every other failure, a refused link included, is returned as is. The old
// skill is kept under a sibling name until replacement succeeds and restored
// if copying or moving fails.
func swapStagedIntoSource(sourceDir, staged, destPath string, follow *sourcewalk.Follow) error {
	src, err := sourcefs.Open(sourceDir, follow)
	if err != nil {
		return err
	}
	defer src.Close()
	rel, err := src.Rel(destPath)
	if err != nil {
		return err
	}
	if err := src.CheckNoLink(rel); err != nil {
		return err
	}
	backup := ""
	if _, err := src.Lstat(rel); err == nil {
		// ".old." keeps the backup apart from the ".skillshare-<name>.<time>" temp that
		// sourcefs.MoveIn creates beside it; a coarse clock (Windows) made both names equal,
		// and the failed move then deleted the backup.
		backup = filepath.Join(filepath.Dir(rel), ".skillshare-"+filepath.Base(rel)+".old."+strconv.FormatInt(time.Now().UnixNano(), 36))
		if err := src.Rename(rel, backup); err != nil {
			return fmt.Errorf("failed to preserve existing skill: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	moveErr := src.MoveIn(staged, rel)
	if sourcefs.IsCrossDevice(moveErr) {
		moveErr = src.CopyIn(staged, rel)
	}
	if moveErr != nil {
		if err := src.RemoveAll(rel); err != nil {
			return fmt.Errorf("failed to move updated skill: %w; cleanup failed: %v", moveErr, err)
		}
		if backup != "" {
			if err := src.Rename(backup, rel); err != nil {
				return fmt.Errorf("failed to move updated skill: %w; restore failed: %v", moveErr, err)
			}
		}
		return fmt.Errorf("failed to move updated skill: %w", moveErr)
	}
	if backup != "" {
		return src.RemoveAll(backup)
	}
	return nil
}
