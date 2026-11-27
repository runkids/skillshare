package sync

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
)

// Restore writes where another tool's link leads, even before that directory
// exists, so the link is kept rather than replaced.
func TestResolveTargetRoot_BrokenLinkLeadsToItsDestination(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "dotfiles", "skills")
	link := filepath.Join(tmp, "target")
	if err := os.Symlink(dest, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if got, want := ResolveTargetRoot(link, filepath.Join(tmp, "source")), sourcewalk.Canonical(dest); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// A folder reached through another tool's link is the directory it leads to,
// so removing the target keeps the links another target still uses there.
func TestSkillsFolderKeptBy_FollowsExternalLink(t *testing.T) {
	tmp := t.TempDir()
	shared, link := filepath.Join(tmp, "agents-skills"), filepath.Join(tmp, "gemini-skills")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	targets := map[string]config.TargetConfig{
		"universal": {Skills: &config.ResourceTargetConfig{Path: shared}},
		"gemini":    {Skills: &config.ResourceTargetConfig{Path: link}},
	}
	if got := SkillsFolderKeptBy(targets, "gemini", nil, filepath.Join(tmp, "source")); got != "universal" {
		t.Errorf("got %q, want universal", got)
	}
}

// Symlink-mode folders all lead to the source; each is its own link, so
// none keeps another.
func TestSkillsFolderKeptBy_SourceLinksAreNotShared(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	one, two := filepath.Join(tmp, "one"), filepath.Join(tmp, "two")
	for _, link := range []string{one, two} {
		if err := os.Symlink(source, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	targets := map[string]config.TargetConfig{
		"one": {Skills: &config.ResourceTargetConfig{Path: one}},
		"two": {Skills: &config.ResourceTargetConfig{Path: two}},
	}
	if got := SkillsFolderKeptBy(targets, "one", nil, source); got != "" {
		t.Errorf("got %q, want none", got)
	}
}

// The other way round: name's folder is plain, and another target's folder
// links to it, so removing name must keep that target's links.
func TestSkillsFolderKeptBy_PlainFolderReachedByLink(t *testing.T) {
	tmp := t.TempDir()
	shared, link := filepath.Join(tmp, "agents-skills"), filepath.Join(tmp, "gemini-skills")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	targets := map[string]config.TargetConfig{
		"universal": {Skills: &config.ResourceTargetConfig{Path: shared}},
		"gemini":    {Skills: &config.ResourceTargetConfig{Path: link}},
	}
	if got := SkillsFolderKeptBy(targets, "universal", nil, filepath.Join(tmp, "source")); got != "gemini" {
		t.Errorf("got %q, want gemini", got)
	}
}
