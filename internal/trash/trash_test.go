package trash

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMoveToTrash(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")
	srcDir := filepath.Join(tmpDir, "source", "my-skill")

	// Create source skill
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("# Test"), 0644)

	trashPath, err := MoveToTrash(srcDir, "my-skill", trashBase)
	if err != nil {
		t.Fatalf("MoveToTrash failed: %v", err)
	}

	// Source should be gone
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Error("source should be removed after MoveToTrash")
	}

	// Trash should contain the skill
	if _, err := os.Stat(filepath.Join(trashPath, "SKILL.md")); err != nil {
		t.Error("trashed skill should contain SKILL.md")
	}

	// Trash path should be under trashBase
	if rel, err := filepath.Rel(trashBase, trashPath); err != nil || rel == "" {
		t.Error("trash path should be under trashBase")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")

	// Create two trashed items
	os.MkdirAll(filepath.Join(trashBase, "skill-a_2026-01-01_10-00-00"), 0755)
	os.WriteFile(filepath.Join(trashBase, "skill-a_2026-01-01_10-00-00", "SKILL.md"), []byte("a"), 0644)
	os.MkdirAll(filepath.Join(trashBase, "skill-b_2026-01-02_10-00-00"), 0755)
	os.WriteFile(filepath.Join(trashBase, "skill-b_2026-01-02_10-00-00", "SKILL.md"), []byte("b"), 0644)

	items := List(trashBase)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Newest first
	if items[0].Name != "skill-b" {
		t.Errorf("expected newest first, got %s", items[0].Name)
	}
	if items[1].Name != "skill-a" {
		t.Errorf("expected skill-a second, got %s", items[1].Name)
	}
}

func TestList_Empty(t *testing.T) {
	items := List("/nonexistent/path")
	if len(items) != 0 {
		t.Errorf("expected 0 items for nonexistent path, got %d", len(items))
	}
}

func TestList_SkillNameWithUnderscore(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")

	// Tracked repo with _ prefix
	os.MkdirAll(filepath.Join(trashBase, "_team-repo_2026-01-01_10-00-00"), 0755)

	items := List(trashBase)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Name != "_team-repo" {
		t.Errorf("expected _team-repo, got %s", items[0].Name)
	}
}

func TestCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")

	// Create an old item (8 days ago)
	old := time.Now().Add(-8 * 24 * time.Hour).Format("2006-01-02_15-04-05")
	os.MkdirAll(filepath.Join(trashBase, "old-skill_"+old), 0755)

	// Create a recent item (1 day ago)
	recent := time.Now().Add(-1 * 24 * time.Hour).Format("2006-01-02_15-04-05")
	os.MkdirAll(filepath.Join(trashBase, "new-skill_"+recent), 0755)

	removed, err := Cleanup(trashBase, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	items := List(trashBase)
	if len(items) != 1 {
		t.Fatalf("expected 1 item remaining, got %d", len(items))
	}
	if items[0].Name != "new-skill" {
		t.Errorf("expected new-skill to remain, got %s", items[0].Name)
	}
}

func TestTotalSize(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")

	os.MkdirAll(filepath.Join(trashBase, "skill_2026-01-01_10-00-00"), 0755)
	os.WriteFile(filepath.Join(trashBase, "skill_2026-01-01_10-00-00", "SKILL.md"), []byte("hello"), 0644)

	size := TotalSize(trashBase)
	if size != 5 {
		t.Errorf("expected size 5, got %d", size)
	}
}

func TestFindByName(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")

	os.MkdirAll(filepath.Join(trashBase, "my-skill_2026-01-01_10-00-00"), 0755)
	os.MkdirAll(filepath.Join(trashBase, "my-skill_2026-01-02_10-00-00"), 0755)
	os.MkdirAll(filepath.Join(trashBase, "other_2026-01-01_10-00-00"), 0755)

	// Should return the newest match
	entry := FindByName(trashBase, "my-skill")
	if entry == nil {
		t.Fatal("expected to find my-skill")
	}
	if entry.Timestamp != "2026-01-02_10-00-00" {
		t.Errorf("expected newest entry, got %s", entry.Timestamp)
	}

	// Not found
	if FindByName(trashBase, "nonexistent") != nil {
		t.Error("expected nil for nonexistent skill")
	}
}

func TestRestore(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")
	destDir := filepath.Join(tmpDir, "skills")
	os.MkdirAll(destDir, 0755)

	// Trash a skill first
	srcDir := filepath.Join(tmpDir, "src", "restore-me")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("# Restore"), 0644)
	MoveToTrash(srcDir, "restore-me", trashBase)

	entry := FindByName(trashBase, "restore-me")
	if entry == nil {
		t.Fatal("expected to find restore-me in trash")
	}

	if err := Restore(entry, destDir); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Should be back in dest
	restored := filepath.Join(destDir, "restore-me", "SKILL.md")
	if _, err := os.Stat(restored); err != nil {
		t.Error("restored skill should contain SKILL.md")
	}

	// Trash entry should be gone
	if FindByName(trashBase, "restore-me") != nil {
		t.Error("trash entry should be removed after restore")
	}
}

func TestRestore_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")
	destDir := filepath.Join(tmpDir, "skills")

	// Create existing skill in dest
	os.MkdirAll(filepath.Join(destDir, "conflict"), 0755)

	// Create trash entry
	os.MkdirAll(filepath.Join(trashBase, "conflict_2026-01-01_10-00-00"), 0755)

	entry := FindByName(trashBase, "conflict")
	if entry == nil {
		t.Fatal("expected to find conflict in trash")
	}

	err := Restore(entry, destDir)
	if err == nil {
		t.Error("expected error when dest already exists")
	}
}

func TestMoveToTrash_NestedName(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")
	srcDir := filepath.Join(tmpDir, "source", "org", "_team-skills")

	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# Team"), 0644)

	trashPath, err := MoveToTrash(srcDir, "org/_team-skills", trashBase)
	if err != nil {
		t.Fatalf("MoveToTrash failed: %v", err)
	}

	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Error("source should be removed after MoveToTrash")
	}

	if _, err := os.Stat(filepath.Join(trashPath, "README.md")); err != nil {
		t.Error("trashed repo should contain README.md")
	}

	// Trash path should be nested under trashBase/org/
	rel, err := filepath.Rel(trashBase, trashPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		t.Errorf("trash path should be under trashBase, rel=%q err=%v", rel, err)
	}
	if dir := filepath.Dir(rel); dir != "org" {
		t.Errorf("expected parent dir 'org', got %q", dir)
	}
}

func TestList_NestedEntries(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")

	// Flat entry
	os.MkdirAll(filepath.Join(trashBase, "skill-a_2026-01-01_10-00-00"), 0755)
	// Nested entry
	os.MkdirAll(filepath.Join(trashBase, "org", "_team-skills_2026-01-02_10-00-00"), 0755)

	items := List(trashBase)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Newest first
	if items[0].Name != "org/_team-skills" {
		t.Errorf("expected 'org/_team-skills', got %q", items[0].Name)
	}
	if items[1].Name != "skill-a" {
		t.Errorf("expected 'skill-a', got %q", items[1].Name)
	}
}

func TestFindByName_NestedName(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")

	os.MkdirAll(filepath.Join(trashBase, "org", "_team-skills_2026-01-01_10-00-00"), 0755)

	entry := FindByName(trashBase, "org/_team-skills")
	if entry == nil {
		t.Fatal("expected to find org/_team-skills")
	}
	if entry.Name != "org/_team-skills" {
		t.Errorf("expected Name 'org/_team-skills', got %q", entry.Name)
	}
}

func TestRestore_NestedName(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")
	destDir := filepath.Join(tmpDir, "skills")
	// Note: destDir/org/ does NOT exist yet — Restore should create it

	srcDir := filepath.Join(tmpDir, "src", "org", "_team-skills")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# Team"), 0644)
	MoveToTrash(srcDir, "org/_team-skills", trashBase)

	entry := FindByName(trashBase, "org/_team-skills")
	if entry == nil {
		t.Fatal("expected to find org/_team-skills in trash")
	}

	if err := Restore(entry, destDir); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	restored := filepath.Join(destDir, "org", "_team-skills", "README.md")
	if _, err := os.Stat(restored); err != nil {
		t.Error("restored repo should contain README.md at nested path")
	}

	if FindByName(trashBase, "org/_team-skills") != nil {
		t.Error("trash entry should be removed after restore")
	}
}

func TestCleanup_NestedEntry(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")

	// Create an expired nested entry (8 days ago)
	old := time.Now().Add(-8 * 24 * time.Hour).Format("2006-01-02_15-04-05")
	os.MkdirAll(filepath.Join(trashBase, "org", "_team-skills_"+old), 0755)

	removed, err := Cleanup(trashBase, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// Empty parent dir should also be cleaned
	if _, err := os.Stat(filepath.Join(trashBase, "org")); !os.IsNotExist(err) {
		t.Error("expected empty parent dir 'org' to be cleaned up")
	}
}

func TestParseTrashName(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantTS   string
	}{
		{"my-skill_2026-01-01_10-00-00", "my-skill", "2026-01-01_10-00-00"},
		{"_team-repo_2026-01-01_10-00-00", "_team-repo", "2026-01-01_10-00-00"},
		{"a_b_c_2026-12-31_23-59-59", "a_b_c", "2026-12-31_23-59-59"},
		{"short", "", ""},            // too short
		{"no-timestamp_abc", "", ""}, // invalid timestamp
	}

	for _, tt := range tests {
		name, ts := parseTrashName(tt.input)
		if name != tt.wantName || ts != tt.wantTS {
			t.Errorf("parseTrashName(%q) = (%q, %q), want (%q, %q)",
				tt.input, name, ts, tt.wantName, tt.wantTS)
		}
	}
}
