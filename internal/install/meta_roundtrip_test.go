package install

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteMeta_ReadMeta_Roundtrip(t *testing.T) {
	dir := t.TempDir()

	original := &SkillMeta{
		Source:      "github.com/user/repo",
		Type:        "github",
		InstalledAt: time.Now(),
		RepoURL:     "https://github.com/user/repo.git",
		Version:     "abc1234",
		FileHashes: map[string]string{
			"SKILL.md": "sha256:deadbeef",
		},
	}

	if err := WriteMeta(dir, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := ReadMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil meta")
	}
	if loaded.Source != original.Source {
		t.Errorf("Source mismatch: %q vs %q", loaded.Source, original.Source)
	}
	if loaded.Type != original.Type {
		t.Errorf("Type mismatch: %q vs %q", loaded.Type, original.Type)
	}
	if loaded.RepoURL != original.RepoURL {
		t.Errorf("RepoURL mismatch: %q vs %q", loaded.RepoURL, original.RepoURL)
	}
	if loaded.Version != original.Version {
		t.Errorf("Version mismatch: %q vs %q", loaded.Version, original.Version)
	}
	if loaded.InstalledAt.IsZero() {
		t.Error("expected InstalledAt to be non-zero")
	}
	if len(loaded.FileHashes) != 1 {
		t.Errorf("expected 1 file hash, got %d", len(loaded.FileHashes))
	}
}

func TestReadMeta_NoFile(t *testing.T) {
	dir := t.TempDir()

	meta, err := ReadMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	if meta != nil {
		t.Error("expected nil meta when no file exists")
	}
}

func TestReadMeta_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, metaFileName), []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadMeta(dir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHasMeta_True(t *testing.T) {
	dir := t.TempDir()
	meta := &SkillMeta{Source: "test", Type: "local"}
	if err := WriteMeta(dir, meta); err != nil {
		t.Fatal(err)
	}

	if !HasMeta(dir) {
		t.Error("expected HasMeta true after writing meta")
	}
}

func TestHasMeta_False(t *testing.T) {
	dir := t.TempDir()
	if HasMeta(dir) {
		t.Error("expected HasMeta false for empty dir")
	}
}
