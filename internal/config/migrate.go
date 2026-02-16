package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"skillshare/internal/utils"
)

type MigrationStatus string

const (
	MigrationMoved                    MigrationStatus = "moved"
	MigrationSkippedNoSource          MigrationStatus = "skipped_no_source"
	MigrationSkippedNoChange          MigrationStatus = "skipped_no_change"
	MigrationSkippedDestinationExists MigrationStatus = "skipped_destination_exists"
	MigrationSkippedSamePath          MigrationStatus = "skipped_same_path"
	MigrationFailed                   MigrationStatus = "failed"
)

type MigrationResult struct {
	From   string
	To     string
	Status MigrationStatus
	Err    error
}

// MigrateWindowsLegacyDir moves the entire ~/.config/skillshare directory to
// %AppData%\skillshare on Windows. This handles upgrades from pre-v0.13.0 where
// Windows incorrectly used the Unix-style ~/.config/ path.
// No-op on non-Windows platforms or if legacy dir doesn't exist.
func MigrateWindowsLegacyDir() []MigrationResult {
	if runtime.GOOS != "windows" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return []MigrationResult{{
			Status: MigrationFailed,
			Err:    fmt.Errorf("resolve user home: %w", err),
		}}
	}
	oldDir := filepath.Join(home, ".config", "skillshare")
	newDir := BaseDir() // %AppData%\skillshare on Windows
	if oldDir == newDir {
		return []MigrationResult{{
			From:   oldDir,
			To:     newDir,
			Status: MigrationSkippedSamePath,
		}} // XDG_CONFIG_HOME points to ~/.config, no migration needed
	}
	dirResult := migrateDir(oldDir, newDir)
	results := []MigrationResult{dirResult}

	// Keep config.yaml source in sync after legacy dir move.
	if dirResult.Status == MigrationMoved {
		results = append(results, migrateConfigSourcePath(oldDir, newDir))
	}

	return results
}

// MigrateXDGDirs moves backups/trash/logs from legacy config dir to proper XDG dirs.
// Called once at startup. No-op if already migrated or no legacy data exists.
func MigrateXDGDirs() []MigrationResult {
	base := BaseDir()
	moves := []struct{ old, new string }{
		{filepath.Join(base, "backups"), filepath.Join(DataDir(), "backups")},
		{filepath.Join(base, "trash"), filepath.Join(DataDir(), "trash")},
		{filepath.Join(base, "logs"), filepath.Join(StateDir(), "logs")},
	}
	results := make([]MigrationResult, 0, len(moves))
	for _, m := range moves {
		if m.old == m.new {
			results = append(results, MigrationResult{
				From:   m.old,
				To:     m.new,
				Status: MigrationSkippedSamePath,
			})
			continue
		}
		results = append(results, migrateDir(m.old, m.new))
	}
	return results
}

func migrateDir(oldPath, newPath string) MigrationResult {
	result := MigrationResult{From: oldPath, To: newPath}
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		result.Status = MigrationSkippedNoSource
		return result // nothing to migrate
	}
	if _, err := os.Stat(newPath); err == nil {
		result.Status = MigrationSkippedDestinationExists
		return result // destination already exists
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		result.Status = MigrationFailed
		result.Err = fmt.Errorf("create destination parent dir: %w", err)
		return result
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		result.Status = MigrationFailed
		result.Err = fmt.Errorf("move %s -> %s: %w", oldPath, newPath, err)
		return result
	}
	result.Status = MigrationMoved
	return result
}

func migrateConfigSourcePath(oldRoot, newRoot string) MigrationResult {
	cfg, err := Load()
	if err != nil {
		return MigrationResult{
			From:   ConfigPath(),
			To:     ConfigPath(),
			Status: MigrationFailed,
			Err:    fmt.Errorf("load config for source migration: %w", err),
		}
	}

	oldSource := cfg.Source
	newSource, changed := remapPathPrefix(oldSource, oldRoot, newRoot)
	if !changed {
		return MigrationResult{
			From:   oldSource,
			To:     oldSource,
			Status: MigrationSkippedNoChange,
		}
	}

	cfg.Source = newSource
	if err := cfg.Save(); err != nil {
		return MigrationResult{
			From:   oldSource,
			To:     newSource,
			Status: MigrationFailed,
			Err:    fmt.Errorf("save updated config source: %w", err),
		}
	}

	return MigrationResult{
		From:   oldSource,
		To:     newSource,
		Status: MigrationMoved,
	}
}

func remapPathPrefix(path, oldRoot, newRoot string) (string, bool) {
	cleanPath := filepath.Clean(path)
	cleanOld := filepath.Clean(oldRoot)
	cleanNew := filepath.Clean(newRoot)

	if utils.PathsEqual(cleanPath, cleanOld) {
		return cleanNew, true
	}

	oldPrefix := cleanOld + string(os.PathSeparator)
	if !utils.PathHasPrefix(cleanPath, oldPrefix) {
		return cleanPath, false
	}

	rel, err := filepath.Rel(cleanOld, cleanPath)
	if err != nil {
		return cleanPath, false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return cleanPath, false
	}

	return filepath.Join(cleanNew, rel), true
}
