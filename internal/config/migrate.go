package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// MigrateWindowsLegacyDir moves the entire ~/.config/skillshare directory to
// %AppData%\skillshare on Windows. This handles upgrades from pre-v0.13.0 where
// Windows incorrectly used the Unix-style ~/.config/ path.
// No-op on non-Windows platforms or if legacy dir doesn't exist.
func MigrateWindowsLegacyDir() {
	if runtime.GOOS != "windows" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	oldDir := filepath.Join(home, ".config", "skillshare")
	newDir := BaseDir() // %AppData%\skillshare on Windows
	if oldDir == newDir {
		return // XDG_CONFIG_HOME points to ~/.config, no migration needed
	}
	migrateDir(oldDir, newDir)
}

// MigrateXDGDirs moves backups/trash/logs from legacy config dir to proper XDG dirs.
// Called once at startup. No-op if already migrated or no legacy data exists.
func MigrateXDGDirs() {
	base := BaseDir()
	moves := []struct{ old, new string }{
		{filepath.Join(base, "backups"), filepath.Join(DataDir(), "backups")},
		{filepath.Join(base, "trash"), filepath.Join(DataDir(), "trash")},
		{filepath.Join(base, "logs"), filepath.Join(StateDir(), "logs")},
	}
	for _, m := range moves {
		if m.old == m.new {
			continue // XDG vars resolve to same location
		}
		migrateDir(m.old, m.new)
	}
}

func migrateDir(oldPath, newPath string) {
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return // nothing to migrate
	}
	if _, err := os.Stat(newPath); err == nil {
		return // destination already exists
	}
	os.MkdirAll(filepath.Dir(newPath), 0755)
	os.Rename(oldPath, newPath)
}
