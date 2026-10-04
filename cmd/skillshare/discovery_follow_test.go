package main

import (
	"os"
	"path/filepath"
	"testing"
)

// followedGroupSource returns a source whose declared "group" link holds the
// skill group/c, and an undeclared physical "plain" skill beside it.
func followedGroupSource(t *testing.T) string {
	t.Helper()
	source, external := t.TempDir(), t.TempDir()
	for _, dir := range []string{filepath.Join(external, "c"), filepath.Join(source, "plain")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+filepath.Base(dir)+"\n---\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(external, filepath.Join(source, "group")); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".skillfollow"), []byte("group\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return source
}

// The audit TUI scans the other resource kind for its second tab; the skills
// tab must see skills below a followed group like the first scan does.
func TestDiscoverForKindSkillsUsesFollowSnapshot(t *testing.T) {
	source := followedGroupSource(t)
	refs, err := discoverForKind(kindSkills, source, skillFollowSet(source, nil, source))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(source, "group", "c")
	for _, ref := range refs {
		if ref.path == want {
			return
		}
	}
	t.Fatalf("followed skill %s missing from %+v", want, refs)
}
