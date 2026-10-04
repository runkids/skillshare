package install

import (
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
// in-source side is checked through the handle first. Only a cross-device
// rename falls back to a copy, and that copy also goes through the handle;
// every other failure, a refused link included, is returned as is.
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
		if !sourcefs.IsCrossDevice(err) {
			return fmt.Errorf("failed to move updated skill: %w", err)
		}
		if err := src.CopyIn(staged, rel); err != nil {
			return fmt.Errorf("failed to copy updated skill: %w", err)
		}
	}
	return nil
}
