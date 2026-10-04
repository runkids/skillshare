package install

import (
	"errors"
	"fmt"

	"skillshare/internal/sourcefs"
)

// removeInSource removes destPath, below the skills source sourceDir, through
// the source-write handle, so a link at or above it is refused instead of
// replaced.
func removeInSource(sourceDir, destPath string) error {
	src, err := sourcefs.Open(sourceDir)
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

// swapStagedIntoSource replaces destPath, below the skills source sourceDir,
// with the staged directory. The rename crosses the source's edge, so the
// in-source side is checked through the handle first. A refusal never falls
// back to the copy.
func swapStagedIntoSource(sourceDir, staged, destPath string) error {
	src, err := sourcefs.Open(sourceDir)
	if err != nil {
		return err
	}
	defer src.Close()
	rel, err := src.Rel(destPath)
	if err != nil {
		return err
	}
	if err := src.RemoveAll(rel); err != nil {
		return fmt.Errorf("failed to remove existing skill: %w", err)
	}
	if err := src.MoveIn(staged, rel); err != nil {
		if errors.Is(err, sourcefs.ErrLink) {
			return err
		}
		// Rename failed (possibly cross-device), try copy instead
		if err := copyDir(staged, destPath); err != nil {
			return fmt.Errorf("failed to move updated skill: %w", err)
		}
	}
	return nil
}
