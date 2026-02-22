package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckStatus_NotExist(t *testing.T) {
	status := CheckStatus("/nonexistent/target", "/nonexistent/source")
	if status != StatusNotExist {
		t.Errorf("expected StatusNotExist, got %s", status)
	}
}

func TestCheckStatus_Linked(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, tgt); err != nil {
		t.Fatal(err)
	}

	status := CheckStatus(tgt, src)
	if status != StatusLinked {
		t.Errorf("expected StatusLinked, got %s", status)
	}
}

func TestCheckStatus_Broken(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")

	// Create symlink to source, then remove source
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, tgt); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}

	status := CheckStatus(tgt, src)
	if status != StatusBroken {
		t.Errorf("expected StatusBroken, got %s", status)
	}
}

func TestCheckStatus_Conflict(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	other := filepath.Join(tmp, "other")
	tgt := filepath.Join(tmp, "target")

	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatal(err)
	}
	// Symlink points to "other", not "source"
	if err := os.Symlink(other, tgt); err != nil {
		t.Fatal(err)
	}

	status := CheckStatus(tgt, src)
	if status != StatusConflict {
		t.Errorf("expected StatusConflict, got %s", status)
	}
}

func TestCheckStatus_HasFiles(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")

	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tgt, 0755); err != nil {
		t.Fatal(err)
	}
	// Put a file so it's a directory with files
	if err := os.WriteFile(filepath.Join(tgt, "file.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	status := CheckStatus(tgt, src)
	if status != StatusHasFiles {
		t.Errorf("expected StatusHasFiles, got %s", status)
	}
}

func TestCheckStatusMerge_NotExist(t *testing.T) {
	status, linked, local := CheckStatusMerge("/nonexistent", "/nonexistent/src")
	if status != StatusNotExist {
		t.Errorf("expected StatusNotExist, got %s", status)
	}
	if linked != 0 || local != 0 {
		t.Errorf("expected 0/0 counts, got %d/%d", linked, local)
	}
}

func TestCheckStatusMerge_Merged(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")
	skillSrc := filepath.Join(src, "my-skill")

	if err := os.MkdirAll(skillSrc, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tgt, 0755); err != nil {
		t.Fatal(err)
	}
	// Create a symlink in target pointing to source skill
	if err := os.Symlink(skillSrc, filepath.Join(tgt, "my-skill")); err != nil {
		t.Fatal(err)
	}

	status, linked, local := CheckStatusMerge(tgt, src)
	if status != StatusMerged {
		t.Errorf("expected StatusMerged, got %s", status)
	}
	if linked != 1 {
		t.Errorf("expected 1 linked, got %d", linked)
	}
	if local != 0 {
		t.Errorf("expected 0 local, got %d", local)
	}
}

func TestCheckStatusMerge_HasFiles(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")

	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tgt, 0755); err != nil {
		t.Fatal(err)
	}
	// A local (non-symlinked) skill directory
	localSkill := filepath.Join(tgt, "local-skill")
	if err := os.MkdirAll(localSkill, 0755); err != nil {
		t.Fatal(err)
	}

	status, linked, local := CheckStatusMerge(tgt, src)
	if status != StatusHasFiles {
		t.Errorf("expected StatusHasFiles, got %s", status)
	}
	if linked != 0 {
		t.Errorf("expected 0 linked, got %d", linked)
	}
	if local != 1 {
		t.Errorf("expected 1 local, got %d", local)
	}
}

func TestCheckStatusMerge_MixedLinkedLocal(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")
	skillSrc := filepath.Join(src, "synced")

	if err := os.MkdirAll(skillSrc, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tgt, 0755); err != nil {
		t.Fatal(err)
	}
	// One symlinked skill
	if err := os.Symlink(skillSrc, filepath.Join(tgt, "synced")); err != nil {
		t.Fatal(err)
	}
	// One local skill
	if err := os.MkdirAll(filepath.Join(tgt, "local"), 0755); err != nil {
		t.Fatal(err)
	}

	status, linked, local := CheckStatusMerge(tgt, src)
	if status != StatusMerged {
		t.Errorf("expected StatusMerged, got %s", status)
	}
	if linked != 1 {
		t.Errorf("expected 1 linked, got %d", linked)
	}
	if local != 1 {
		t.Errorf("expected 1 local, got %d", local)
	}
}

func TestTargetStatus_String(t *testing.T) {
	tests := []struct {
		status TargetStatus
		want   string
	}{
		{StatusLinked, "linked"},
		{StatusNotExist, "not exist"},
		{StatusHasFiles, "has files"},
		{StatusConflict, "conflict"},
		{StatusBroken, "broken"},
		{StatusMerged, "merged"},
		{StatusCopied, "copied"},
		{StatusUnknown, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
