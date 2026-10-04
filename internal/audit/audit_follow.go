package audit

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// ScanResolvedSkill scans an explicitly followed root and keeps logical paths in
// the result. The coverage check makes an unexpectedly empty scan fail closed.
func ScanResolvedSkill(logicalPath string, scan func(string) (*Result, error)) (*Result, error) {
	root, err := sourcewalk.Canonicalize(logicalPath)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve followed audit root: %w", err)
	}
	result, err := scan(root)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("followed audit returned no result for %s", logicalPath)
	}
	if err := verifyFollowedScan(root, result); err != nil {
		return nil, err
	}
	result.ScanTarget = logicalPath
	result.SkillName = filepath.Base(logicalPath)
	return result, nil
}

// MarkFollowedInputs flags the skill inputs under a followed entry so ParallelScan
// resolves their root and enforces the coverage check. A nil follow marks none.
func MarkFollowedInputs(inputs []SkillInput, source string, follow *sourcewalk.FollowSet) {
	for i := range inputs {
		inputs[i].Followed = false
		if follow == nil {
			continue
		}
		rel, err := filepath.Rel(source, inputs[i].Path)
		if err != nil {
			continue
		}
		_, inputs[i].Followed = follow.InFollowed(rel)
	}
}

func verifyFollowedScan(root string, result *Result) error {
	if result.scannedFiles > 0 {
		return nil
	}
	// Eligibility matches scanSkillImpl: visible paths within its depth/size
	// limits, excluding metadata, links, binary content and non-scannable files.
	// .git-only and ignored (hidden/depth-limited) trees therefore remain valid.
	eligible := false
	err := sourcewalk.Walk(root, sourcewalk.Options{}, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		depth := relDepth(rel)
		if info.IsDir() {
			if path != root && (utils.IsHidden(info.Name()) || depth > maxScanDepth) {
				return filepath.SkipDir
			}
			return nil
		}
		if depth > maxScanDepth || !info.Mode().IsRegular() || info.Name() == ".skillshare-meta.json" || info.Size() > maxScanFileSize || !isScannable(info.Name()) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot read eligible file %s: %w", rel, err)
		}
		if !isBinaryContent(data) {
			eligible = true
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("cannot verify followed audit coverage: %w", err)
	}
	if eligible {
		return fmt.Errorf("followed audit read zero eligible files although the resolved directory contains eligible content")
	}
	return nil
}
