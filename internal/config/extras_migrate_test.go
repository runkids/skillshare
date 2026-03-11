package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateExtras_MovesOldToNew(t *testing.T) {
	tmp := t.TempDir()

	// Create old path: configDir/rules/ with a file inside
	oldDir := filepath.Join(tmp, "rules")
	if err := os.MkdirAll(oldDir, 0755); err != nil {
		t.Fatalf("create old dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "my-rule.md"), []byte("# rule"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	extras := []ExtraConfig{
		{Name: "rules", Targets: []ExtraTargetConfig{{Path: "/some/target"}}},
	}

	warnings := MigrateExtrasDir(tmp, extras)

	// No warnings on clean migration
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}

	// New path should exist with the file
	newDir := filepath.Join(tmp, "extras", "rules")
	data, err := os.ReadFile(filepath.Join(newDir, "my-rule.md"))
	if err != nil {
		t.Fatalf("file not in new location: %v", err)
	}
	if string(data) != "# rule" {
		t.Errorf("file content mismatch: got %q", string(data))
	}

	// Old path should be gone
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("old dir should be removed after migration")
	}
}

func TestMigrateExtras_BothExist_Warns(t *testing.T) {
	tmp := t.TempDir()

	// Create both old and new directories
	oldDir := filepath.Join(tmp, "rules")
	newDir := filepath.Join(tmp, "extras", "rules")

	if err := os.MkdirAll(oldDir, 0755); err != nil {
		t.Fatalf("create old dir: %v", err)
	}
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatalf("create new dir: %v", err)
	}
	os.WriteFile(filepath.Join(oldDir, "old.md"), []byte("old"), 0644)
	os.WriteFile(filepath.Join(newDir, "new.md"), []byte("new"), 0644)

	extras := []ExtraConfig{
		{Name: "rules", Targets: []ExtraTargetConfig{{Path: "/some/target"}}},
	}

	warnings := MigrateExtrasDir(tmp, extras)

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

func TestMigrateExtras_NothingToMigrate(t *testing.T) {
	tmp := t.TempDir()

	extras := []ExtraConfig{
		{Name: "rules", Targets: []ExtraTargetConfig{{Path: "/some/target"}}},
	}

	warnings := MigrateExtrasDir(tmp, extras)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestMigrateExtras_AlreadyMigrated(t *testing.T) {
	tmp := t.TempDir()

	// Only new directory exists
	newDir := filepath.Join(tmp, "extras", "rules")
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatalf("create new dir: %v", err)
	}
	os.WriteFile(filepath.Join(newDir, "rule.md"), []byte("# rule"), 0644)

	extras := []ExtraConfig{
		{Name: "rules", Targets: []ExtraTargetConfig{{Path: "/some/target"}}},
	}

	warnings := MigrateExtrasDir(tmp, extras)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestMigrateExtras_OnlyConfiguredNames(t *testing.T) {
	tmp := t.TempDir()

	// Create an old dir for "commands" which is NOT in extras config
	unconfiguredOld := filepath.Join(tmp, "commands")
	if err := os.MkdirAll(unconfiguredOld, 0755); err != nil {
		t.Fatalf("create unconfigured old dir: %v", err)
	}
	os.WriteFile(filepath.Join(unconfiguredOld, "cmd.md"), []byte("cmd"), 0644)

	// Only "rules" is in extras config
	extras := []ExtraConfig{
		{Name: "rules", Targets: []ExtraTargetConfig{{Path: "/some/target"}}},
	}

	warnings := MigrateExtrasDir(tmp, extras)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}

	// "commands" dir should NOT have been moved
	if _, err := os.Stat(unconfiguredOld); os.IsNotExist(err) {
		t.Error("unconfigured dir should not be moved")
	}

	// extras/commands should NOT exist
	unexpectedNew := filepath.Join(tmp, "extras", "commands")
	if _, err := os.Stat(unexpectedNew); !os.IsNotExist(err) {
		t.Error("extras/commands should not have been created")
	}
}
