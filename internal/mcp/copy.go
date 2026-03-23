package mcp

import (
	"bytes"
	"os"
	"path/filepath"
)

// CopyToTarget copies the generated JSON file to the target path.
// Returns SyncResult with status:
//   - "copied"  — new file created
//   - "updated" — existing file content changed
//   - "ok"      — content unchanged, skipped
//   - "error"   — failed
func CopyToTarget(name, generatedPath, targetPath string, dryRun bool) SyncResult {
	generated, err := os.ReadFile(generatedPath)
	if err != nil {
		return SyncResult{Target: name, Status: "error", Path: targetPath, Error: err.Error()}
	}

	existing, err := os.ReadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return SyncResult{Target: name, Status: "error", Path: targetPath, Error: err.Error()}
	}

	fileExists := err == nil

	if fileExists && bytes.Equal(existing, generated) {
		return SyncResult{Target: name, Status: "ok", Path: targetPath}
	}

	status := "copied"
	if fileExists {
		status = "updated"
	}

	if dryRun {
		return SyncResult{Target: name, Status: status, Path: targetPath}
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return SyncResult{Target: name, Status: "error", Path: targetPath, Error: err.Error()}
	}
	if err := os.WriteFile(targetPath, generated, 0644); err != nil {
		return SyncResult{Target: name, Status: "error", Path: targetPath, Error: err.Error()}
	}

	return SyncResult{Target: name, Status: status, Path: targetPath}
}
