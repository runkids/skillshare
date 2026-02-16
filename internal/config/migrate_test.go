package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMigrateXDGDirs_MovesLegacyDirs(t *testing.T) {
	tmp := t.TempDir()
	configHome := filepath.Join(tmp, "config")
	dataHome := filepath.Join(tmp, "data")
	stateHome := filepath.Join(tmp, "state")

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_STATE_HOME", stateHome)

	// Create legacy directories with content
	legacyBackups := filepath.Join(configHome, "skillshare", "backups")
	legacyTrash := filepath.Join(configHome, "skillshare", "trash")
	legacyLogs := filepath.Join(configHome, "skillshare", "logs")

	os.MkdirAll(legacyBackups, 0755)
	os.WriteFile(filepath.Join(legacyBackups, "backup1.tar"), []byte("data"), 0644)

	os.MkdirAll(legacyTrash, 0755)
	os.WriteFile(filepath.Join(legacyTrash, "trashed"), []byte("data"), 0644)

	os.MkdirAll(legacyLogs, 0755)
	os.WriteFile(filepath.Join(legacyLogs, "ops.log"), []byte("data"), 0644)

	results := MigrateXDGDirs()

	// Verify moved to new locations
	newBackups := filepath.Join(dataHome, "skillshare", "backups")
	newTrash := filepath.Join(dataHome, "skillshare", "trash")
	newLogs := filepath.Join(stateHome, "skillshare", "logs")

	if _, err := os.Stat(filepath.Join(newBackups, "backup1.tar")); err != nil {
		t.Errorf("backups not migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newTrash, "trashed")); err != nil {
		t.Errorf("trash not migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newLogs, "ops.log")); err != nil {
		t.Errorf("logs not migrated: %v", err)
	}

	// Verify legacy dirs removed
	if _, err := os.Stat(legacyBackups); !os.IsNotExist(err) {
		t.Error("legacy backups dir should be removed after migration")
	}
	if _, err := os.Stat(legacyTrash); !os.IsNotExist(err) {
		t.Error("legacy trash dir should be removed after migration")
	}
	if _, err := os.Stat(legacyLogs); !os.IsNotExist(err) {
		t.Error("legacy logs dir should be removed after migration")
	}

	assertNoMigrationFailures(t, results)
}

func TestMigrateXDGDirs_NoopWhenNoLegacyDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))

	// No legacy dirs exist — should not panic or error
	results := MigrateXDGDirs()
	assertNoMigrationFailures(t, results)
}

func TestMigrateXDGDirs_SkipsWhenDestExists(t *testing.T) {
	tmp := t.TempDir()
	configHome := filepath.Join(tmp, "config")
	dataHome := filepath.Join(tmp, "data")
	stateHome := filepath.Join(tmp, "state")

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_STATE_HOME", stateHome)

	// Create both legacy and new backups dir
	legacyBackups := filepath.Join(configHome, "skillshare", "backups")
	newBackups := filepath.Join(dataHome, "skillshare", "backups")

	os.MkdirAll(legacyBackups, 0755)
	os.WriteFile(filepath.Join(legacyBackups, "old.tar"), []byte("old"), 0644)

	os.MkdirAll(newBackups, 0755)
	os.WriteFile(filepath.Join(newBackups, "new.tar"), []byte("new"), 0644)

	results := MigrateXDGDirs()

	// New dir should be untouched
	data, err := os.ReadFile(filepath.Join(newBackups, "new.tar"))
	if err != nil || string(data) != "new" {
		t.Error("existing new dir should not be overwritten")
	}

	// Legacy dir should still exist (not removed since dest already existed)
	if _, err := os.Stat(legacyBackups); os.IsNotExist(err) {
		t.Error("legacy dir should remain when dest already exists")
	}

	result := findMigrationResultByFrom(results, legacyBackups)
	if result == nil {
		t.Fatalf("expected migration result for %s", legacyBackups)
	}
	if result.Status != MigrationSkippedDestinationExists {
		t.Fatalf("unexpected migration status for backup dir: got %q, want %q", result.Status, MigrationSkippedDestinationExists)
	}
}

// TestMigrateWindowsLegacyDir_UsesMigrateDir tests the underlying migrateDir
// logic that MigrateWindowsLegacyDir delegates to. We can't test the actual
// function on non-Windows (it early-returns), so we test the migration path directly.
func TestMigrateWindowsLegacyDir_MigrationPath(t *testing.T) {
	tmp := t.TempDir()
	oldDir := filepath.Join(tmp, ".config", "skillshare")
	newDir := filepath.Join(tmp, "AppData", "skillshare")

	// Create legacy dir with config and skills
	os.MkdirAll(filepath.Join(oldDir, "skills"), 0755)
	os.WriteFile(filepath.Join(oldDir, "config.yaml"), []byte("source: skills"), 0644)
	os.WriteFile(filepath.Join(oldDir, "skills", "SKILL.md"), []byte("# skill"), 0644)

	// Simulate what MigrateWindowsLegacyDir does: migrateDir(old, new)
	result := migrateDir(oldDir, newDir)

	// Verify moved
	data, err := os.ReadFile(filepath.Join(newDir, "config.yaml"))
	if err != nil || string(data) != "source: skills" {
		t.Errorf("config.yaml not migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDir, "skills", "SKILL.md")); err != nil {
		t.Errorf("skills dir not migrated: %v", err)
	}

	// Verify old dir removed
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("legacy dir should be removed after migration")
	}
	if result.Status != MigrationMoved {
		t.Fatalf("unexpected migration status: got %q, want %q", result.Status, MigrationMoved)
	}
}

func TestMigrateWindowsLegacyDir_SkipsOnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test verifies non-Windows no-op behavior")
	}
	// Should not panic or do anything on non-Windows
	results := MigrateWindowsLegacyDir()
	assertNoMigrationFailures(t, results)
}

func TestMigrateWindowsLegacyDir_SkipsWhenDestExists(t *testing.T) {
	tmp := t.TempDir()
	oldDir := filepath.Join(tmp, ".config", "skillshare")
	newDir := filepath.Join(tmp, "AppData", "skillshare")

	os.MkdirAll(oldDir, 0755)
	os.WriteFile(filepath.Join(oldDir, "config.yaml"), []byte("old"), 0644)

	os.MkdirAll(newDir, 0755)
	os.WriteFile(filepath.Join(newDir, "config.yaml"), []byte("new"), 0644)

	result := migrateDir(oldDir, newDir)

	// New dir untouched
	data, _ := os.ReadFile(filepath.Join(newDir, "config.yaml"))
	if string(data) != "new" {
		t.Error("existing dest should not be overwritten")
	}

	// Old dir still exists
	if _, err := os.Stat(oldDir); os.IsNotExist(err) {
		t.Error("legacy dir should remain when dest already exists")
	}
	if result.Status != MigrationSkippedDestinationExists {
		t.Fatalf("unexpected migration status: got %q, want %q", result.Status, MigrationSkippedDestinationExists)
	}
}

func findMigrationResultByFrom(results []MigrationResult, from string) *MigrationResult {
	for i := range results {
		if results[i].From == from {
			return &results[i]
		}
	}
	return nil
}

func assertNoMigrationFailures(t *testing.T, results []MigrationResult) {
	t.Helper()
	for _, r := range results {
		if r.Status == MigrationFailed {
			t.Fatalf("unexpected migration failure for %s -> %s: %v", r.From, r.To, r.Err)
		}
	}
}

func TestMigrateConfigSourcePath_RewritesLegacySource(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	oldRoot := filepath.Join(tmp, "old", "skillshare")
	newRoot := filepath.Join(tmp, "new", "skillshare")
	oldSource := filepath.Join(oldRoot, "skills")
	newSource := filepath.Join(newRoot, "skills")

	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	cfg := &Config{
		Source:  oldSource,
		Targets: map[string]TargetConfig{},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	result := migrateConfigSourcePath(oldRoot, newRoot)
	if result.Status != MigrationMoved {
		t.Fatalf("unexpected migration status: got %q, want %q", result.Status, MigrationMoved)
	}
	if result.From != oldSource || result.To != newSource {
		t.Fatalf("unexpected migration result: from=%q to=%q", result.From, result.To)
	}

	updated, err := Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if updated.Source != newSource {
		t.Fatalf("source not rewritten: got %q, want %q", updated.Source, newSource)
	}
}

func TestMigrateConfigSourcePath_SkipsWhenUnrelatedSource(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	oldRoot := filepath.Join(tmp, "old", "skillshare")
	newRoot := filepath.Join(tmp, "new", "skillshare")
	customSource := filepath.Join(tmp, "custom", "skills")

	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	cfg := &Config{
		Source:  customSource,
		Targets: map[string]TargetConfig{},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	result := migrateConfigSourcePath(oldRoot, newRoot)
	if result.Status != MigrationSkippedNoChange {
		t.Fatalf("unexpected migration status: got %q, want %q", result.Status, MigrationSkippedNoChange)
	}

	updated, err := Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if updated.Source != customSource {
		t.Fatalf("source should remain unchanged: got %q, want %q", updated.Source, customSource)
	}
}
