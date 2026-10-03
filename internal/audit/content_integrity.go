package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/utils"
)

// checkContentIntegrity compares files on disk against pinned hashes in the
// centralized .metadata.json store. Backward-compatible: skips silently when
// metadata or file_hashes is absent. cache holds file contents already read
// during the walk phase; files not in cache are read from disk as fallback.
// allFiles (if non-nil) is the set of file relPaths collected during the main
// walk, used to detect unexpected files without a second filepath.Walk.
func checkContentIntegrity(skillPath string, cache map[string][]byte, allFiles map[string]bool) []Finding {
	fileHashes := readMetaFileHashes(skillPath)
	if len(fileHashes) == 0 {
		return nil
	}

	var findings []Finding

	// Check pinned files: missing or tampered
	for rel, expected := range fileHashes {
		normalizedRel := filepath.FromSlash(rel)
		// Reject absolute keys in metadata (e.g. "/etc/passwd").
		// file_hashes must always be skill-relative paths.
		// filepath.IsAbs is platform-dependent, so a leading separator is also
		// rejected: on Windows "/etc/passwd" has no volume and looks relative.
		if filepath.IsAbs(normalizedRel) || strings.HasPrefix(normalizedRel, string(filepath.Separator)) {
			continue
		}

		absPath := filepath.Clean(filepath.Join(skillPath, normalizedRel))
		// Containment check: reject keys that escape the skill directory
		if !strings.HasPrefix(absPath, filepath.Clean(skillPath)+string(filepath.Separator)) {
			continue
		}
		info, err := os.Stat(absPath)
		if err != nil {
			findings = append(findings, Finding{
				Severity:   SeverityLow,
				Pattern:    "content-missing",
				Message:    fmt.Sprintf("pinned file missing: %s", rel),
				File:       rel,
				Line:       0,
				RuleID:     "content-missing",
				Analyzer:   AnalyzerIntegrity,
				Category:   CategoryIntegrity,
				Confidence: 1.0,
			})
			continue
		}
		if info.IsDir() {
			continue
		}
		if info.Size() > maxScanFileSize {
			findings = append(findings, Finding{
				Severity:   SeverityMedium,
				Pattern:    "content-oversize",
				Message:    fmt.Sprintf("pinned file exceeds scan size limit (%d bytes): %s", info.Size(), rel),
				File:       rel,
				Line:       0,
				RuleID:     "content-oversize",
				Analyzer:   AnalyzerIntegrity,
				Category:   CategoryIntegrity,
				Confidence: 1.0,
			})
			continue
		}
		// Use cached content when available to avoid re-reading from disk.
		normalized := filepath.ToSlash(rel)
		var actual string
		if cached, ok := cache[normalized]; ok {
			sum := sha256.Sum256(cached)
			actual = hex.EncodeToString(sum[:])
		} else {
			var hashErr error
			actual, hashErr = utils.FileHash(absPath)
			if hashErr != nil {
				continue
			}
		}
		if "sha256:"+actual != expected {
			findings = append(findings, Finding{
				Severity:   SeverityMedium,
				Pattern:    "content-tampered",
				Message:    fmt.Sprintf("file hash mismatch: %s", rel),
				File:       rel,
				Line:       0,
				RuleID:     "content-tampered",
				Analyzer:   AnalyzerIntegrity,
				Category:   CategoryIntegrity,
				Confidence: 1.0,
			})
		}
	}

	// Check for unexpected files not in the pinned set.
	if allFiles != nil {
		// Use pre-collected file set from the main walk (no second walk needed).
		for relPath := range allFiles {
			if _, ok := fileHashes[relPath]; !ok {
				findings = append(findings, Finding{
					Severity:   SeverityLow,
					Pattern:    "content-unexpected",
					Message:    fmt.Sprintf("file not in pinned hashes: %s", relPath),
					File:       relPath,
					Line:       0,
					RuleID:     "content-unexpected",
					Analyzer:   AnalyzerIntegrity,
					Category:   CategoryIntegrity,
					Confidence: 1.0,
				})
			}
		}
	} else {
		// Fallback: walk the directory (used by single-file scan paths).
		filepath.Walk(skillPath, func(path string, fi os.FileInfo, walkErr error) error { //nolint:errcheck
			if walkErr != nil {
				return nil
			}
			if fi.IsDir() {
				if fi.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if fi.Name() == ".skillshare-meta.json" {
				return nil
			}
			rel, relErr := filepath.Rel(skillPath, path)
			if relErr != nil {
				return nil
			}
			normalized := filepath.ToSlash(rel)
			if _, ok := fileHashes[normalized]; !ok {
				findings = append(findings, Finding{
					Severity:   SeverityLow,
					Pattern:    "content-unexpected",
					Message:    fmt.Sprintf("file not in pinned hashes: %s", normalized),
					File:       normalized,
					Line:       0,
					RuleID:     "content-unexpected",
					Analyzer:   AnalyzerIntegrity,
					Category:   CategoryIntegrity,
					Confidence: 1.0,
				})
			}
			return nil
		})
	}

	return findings
}
