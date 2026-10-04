package install

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/sourcefs"
)

func TestSwapStagedIntoSource(t *testing.T) {
	source := t.TempDir()
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "SKILL.md"), []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(source, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	stage := func() string {
		dir := filepath.Join(t.TempDir(), "skill")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("staged"), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	for _, dest := range []string{
		filepath.Join(source, "linked"),          // the skill is a link
		filepath.Join(source, "linked", "child"), // the skill is below one
	} {
		if err := swapStagedIntoSource(source, stage(), dest); !errors.Is(err, sourcefs.ErrLink) {
			t.Errorf("swap into %s: got %v, want ErrLink", dest, err)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(external, "SKILL.md")); string(data) != "external" {
		t.Fatalf("external skill changed: %q", data)
	}
	if entries, _ := os.ReadDir(external); len(entries) != 1 {
		t.Fatalf("external tree changed: %v", entries)
	}

	if err := swapStagedIntoSource(source, stage(), filepath.Join(source, "plain")); err != nil {
		t.Fatalf("swap into a real directory: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(source, "plain", "SKILL.md")); string(data) != "staged" {
		t.Fatalf("plain skill = %q, want staged content", data)
	}
}
