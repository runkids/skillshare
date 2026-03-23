package mcp

import (
	"os"
	"path/filepath"
	"strings"
)

// SyncResult reports what happened for a single target.
type SyncResult struct {
	Target string `json:"target"`
	Status string `json:"status"` // "linked", "updated", "ok", "skipped", "error"
	Path   string `json:"path"`
	Error  string `json:"error,omitempty"`
}

// SyncTarget creates a symlink from targetPath → generatedPath.
// Non-destructive behavior:
//   - Target doesn't exist → create symlink → "linked"
//   - Target is symlink pointing into generatedDir → update if different → "updated", else "ok"
//   - Target is regular file or symlink elsewhere → skip → "skipped"
//   - dryRun=true → compute status without creating/modifying anything
func SyncTarget(name, generatedPath, targetPath string, dryRun bool) SyncResult {
	generatedDir := filepath.Dir(generatedPath)

	info, err := os.Lstat(targetPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return SyncResult{Target: name, Status: "error", Path: targetPath, Error: err.Error()}
		}
		// Target doesn't exist → would link
		if dryRun {
			return SyncResult{Target: name, Status: "linked", Path: targetPath}
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return SyncResult{Target: name, Status: "error", Path: targetPath, Error: err.Error()}
		}
		if err := os.Symlink(generatedPath, targetPath); err != nil {
			return SyncResult{Target: name, Status: "error", Path: targetPath, Error: err.Error()}
		}
		return SyncResult{Target: name, Status: "linked", Path: targetPath}
	}

	// Target exists — check if it's a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		// Regular file (or directory) — skip
		return SyncResult{Target: name, Status: "skipped", Path: targetPath}
	}

	// It's a symlink — check if it's ours (points into generatedDir)
	link, err := os.Readlink(targetPath)
	if err != nil {
		return SyncResult{Target: name, Status: "error", Path: targetPath, Error: err.Error()}
	}

	if !strings.HasPrefix(link, generatedDir) {
		// Symlink points somewhere else — skip
		return SyncResult{Target: name, Status: "skipped", Path: targetPath}
	}

	// Our symlink — check if it already points to the right file
	if link == generatedPath {
		return SyncResult{Target: name, Status: "ok", Path: targetPath}
	}

	// Points to a different generated file — update
	if dryRun {
		return SyncResult{Target: name, Status: "updated", Path: targetPath}
	}
	if err := os.Remove(targetPath); err != nil {
		return SyncResult{Target: name, Status: "error", Path: targetPath, Error: err.Error()}
	}
	if err := os.Symlink(generatedPath, targetPath); err != nil {
		return SyncResult{Target: name, Status: "error", Path: targetPath, Error: err.Error()}
	}
	return SyncResult{Target: name, Status: "updated", Path: targetPath}
}

// UnsyncTarget removes a symlink if it points into generatedDir.
// Returns true if removed. Never removes regular files.
func UnsyncTarget(targetPath, generatedDir string) bool {
	info, err := os.Lstat(targetPath)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false
	}

	link, err := os.Readlink(targetPath)
	if err != nil {
		return false
	}

	if !strings.HasPrefix(link, generatedDir) {
		return false
	}

	if err := os.Remove(targetPath); err != nil {
		return false
	}
	return true
}

// CleanupStaleLinks removes symlinks for targets that no longer have generated files.
// Returns list of target names that had stale symlinks removed.
func CleanupStaleLinks(targets []MCPTargetSpec, generatedDir string, resolveTargetPath func(MCPTargetSpec) string, generatedFiles map[string]string) []string {
	var removed []string
	for _, spec := range targets {
		if _, hasGenerated := generatedFiles[spec.Name]; hasGenerated {
			continue
		}
		targetPath := resolveTargetPath(spec)
		if targetPath == "" {
			continue
		}
		if UnsyncTarget(targetPath, generatedDir) {
			removed = append(removed, spec.Name)
		}
	}
	return removed
}
