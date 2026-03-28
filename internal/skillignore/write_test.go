package skillignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddPattern_CreatesFileIfMissing(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, ".skillignore")

	if _, err := AddPattern(fp, "my-skill"); err != nil {
		t.Fatalf("AddPattern: %v", err)
	}

	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "my-skill\n" {
		t.Errorf("got %q, want %q", string(data), "my-skill\n")
	}
}

func TestAddPattern_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, ".skillignore")
	os.WriteFile(fp, []byte("existing-skill\n"), 0644)

	if _, err := AddPattern(fp, "new-skill"); err != nil {
		t.Fatalf("AddPattern: %v", err)
	}

	data, _ := os.ReadFile(fp)
	want := "existing-skill\nnew-skill\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}

func TestAddPattern_NoDuplicate(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, ".skillignore")
	os.WriteFile(fp, []byte("my-skill\n"), 0644)

	if _, err := AddPattern(fp, "my-skill"); err != nil {
		t.Fatalf("AddPattern: %v", err)
	}

	data, _ := os.ReadFile(fp)
	if string(data) != "my-skill\n" {
		t.Errorf("got %q, want %q — duplicate was added", string(data), "my-skill\n")
	}
}

func TestAddPattern_PreservesCommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, ".skillignore")
	os.WriteFile(fp, []byte("# comment\n\nexisting\n"), 0644)

	if _, err := AddPattern(fp, "new-skill"); err != nil {
		t.Fatalf("AddPattern: %v", err)
	}

	data, _ := os.ReadFile(fp)
	want := "# comment\n\nexisting\nnew-skill\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}

func TestAddPattern_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, ".skillignore")
	os.WriteFile(fp, []byte("existing"), 0644)

	if _, err := AddPattern(fp, "new-skill"); err != nil {
		t.Fatalf("AddPattern: %v", err)
	}

	data, _ := os.ReadFile(fp)
	want := "existing\nnew-skill\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}

func TestRemovePattern_RemovesExact(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, ".skillignore")
	os.WriteFile(fp, []byte("skill-a\nskill-b\nskill-c\n"), 0644)

	removed, err := RemovePattern(fp, "skill-b")
	if err != nil {
		t.Fatalf("RemovePattern: %v", err)
	}
	if !removed {
		t.Error("expected removed=true")
	}

	data, _ := os.ReadFile(fp)
	want := "skill-a\nskill-c\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}

func TestRemovePattern_NotFound(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, ".skillignore")
	os.WriteFile(fp, []byte("skill-a\n"), 0644)

	removed, err := RemovePattern(fp, "nonexistent")
	if err != nil {
		t.Fatalf("RemovePattern: %v", err)
	}
	if removed {
		t.Error("expected removed=false for nonexistent pattern")
	}
}

func TestRemovePattern_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, ".skillignore")

	removed, err := RemovePattern(fp, "anything")
	if err != nil {
		t.Fatalf("RemovePattern: %v", err)
	}
	if removed {
		t.Error("expected removed=false for missing file")
	}
}

func TestRemovePattern_PreservesOtherContent(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, ".skillignore")
	os.WriteFile(fp, []byte("# header\n\nskill-a\ntarget-skill\nskill-c\n"), 0644)

	RemovePattern(fp, "target-skill")

	data, _ := os.ReadFile(fp)
	want := "# header\n\nskill-a\nskill-c\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}

func TestRemovePattern_GlobPattern(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, ".skillignore")
	os.WriteFile(fp, []byte("draft-*\nskill-a\n"), 0644)

	removed, _ := RemovePattern(fp, "draft-*")
	if !removed {
		t.Error("expected removed=true for glob pattern")
	}

	data, _ := os.ReadFile(fp)
	want := "skill-a\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}

func TestHasPattern_Exact(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, ".skillignore")
	os.WriteFile(fp, []byte("skill-a\nskill-b\n"), 0644)

	if !HasPattern(fp, "skill-a") {
		t.Error("expected true for existing pattern")
	}
	if HasPattern(fp, "nonexistent") {
		t.Error("expected false for missing pattern")
	}
}

func TestHasPattern_MissingFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, ".skillignore")

	if HasPattern(fp, "anything") {
		t.Error("expected false for missing file")
	}
}
