package sync

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"skillshare/internal/sourcefs"
)

// linkedSource builds a skills source with linked, a link to an external
// skill, and returns the source and the external tree's contents.
func linkedSource(t *testing.T) (string, string, map[string]string) {
	t.Helper()
	source, external := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(external, "SKILL.md"), []byte("external"), 0o644)
	if err := os.Symlink(external, filepath.Join(source, "linked")); err != nil {
		t.Fatal(err)
	}
	return source, external, dirContents(t, external)
}

func dirContents(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		data, _ := os.ReadFile(p)
		out[rel] = string(data)
		return nil
	})
	return out
}

func TestPullSkill_ForceRefusesLinkedSkill(t *testing.T) {
	source, external, before := linkedSource(t)
	local := filepath.Join(t.TempDir(), "linked")
	os.MkdirAll(local, 0o755)
	os.WriteFile(filepath.Join(local, "SKILL.md"), []byte("collected"), 0o644)

	err := PullSkill(LocalSkillInfo{Name: "linked", Path: local}, source, true)
	if !errors.Is(err, sourcefs.ErrLink) {
		t.Fatalf("force collect onto a linked skill: got %v, want ErrLink", err)
	}
	if info, err := os.Lstat(filepath.Join(source, "linked")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was replaced: %v %v", info, err)
	}
	if after := dirContents(t, external); !reflect.DeepEqual(after, before) {
		t.Fatalf("external tree changed: %v -> %v", before, after)
	}
}

func TestMigrateToSource_MergeRefusesLinkInSource(t *testing.T) {
	source, external, before := linkedSource(t)
	target := t.TempDir()
	os.MkdirAll(filepath.Join(target, "linked"), 0o755)
	os.WriteFile(filepath.Join(target, "linked", "extra.md"), []byte("from target"), 0o644)

	if err := MigrateToSource(target, source); !errors.Is(err, sourcefs.ErrLink) {
		t.Fatalf("merge through a link in the source: got %v, want ErrLink", err)
	}
	if after := dirContents(t, external); !reflect.DeepEqual(after, before) {
		t.Fatalf("external tree changed: %v -> %v", before, after)
	}
	if _, err := os.Stat(filepath.Join(target, "linked", "extra.md")); err != nil {
		t.Fatalf("target was removed after a failed merge: %v", err)
	}
}
