package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
	"skillshare/internal/trash"
)

// identicalCopySetup makes a source skill and a byte-identical folder in a
// merge-mode target, with the trash under a temporary data home.
func identicalCopySetup(t *testing.T) (cfg *config.Config, skills []ssync.DiscoveredSkill, tgt string) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	src := filepath.Join(t.TempDir(), "alpha")
	tgt = t.TempDir()
	for _, dir := range []string{src, filepath.Join(tgt, "alpha")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: alpha\n---\nsame"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg = &config.Config{Mode: "merge", Targets: map[string]config.TargetConfig{"claude": {Path: tgt}}}
	return cfg, []ssync.DiscoveredSkill{{SourcePath: src, FlatName: "alpha"}}, tgt
}

// init's import replaces a target folder that matches the source byte for byte
// with a link. The folder must go to the trash, not be deleted, so it can be
// copied back.
func TestReplaceIdenticalCopiesMovesCopiesToTrash(t *testing.T) {
	cfg, skills, tgt := identicalCopySetup(t)

	if notTrashed := replaceIdenticalCopies(cfg, skills); len(notTrashed) != 0 {
		t.Errorf("identical copy reported as not trashed: %v", notTrashed)
	}
	if _, err := os.Lstat(filepath.Join(tgt, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("identical copy still in the target: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(trash.TrashDir(), trash.ReplacedByInitDir, "claude", "alpha_*", "SKILL.md"))
	if len(matches) != 1 {
		t.Fatalf("want the copy in the trash, found %v", matches)
	}
}

// A folder the trash cannot take stays where it is, and is reported with the
// error instead of as a copy that differs from the source.
func TestReplaceIdenticalCopiesReportsFailedMove(t *testing.T) {
	cfg, skills, tgt := identicalCopySetup(t)
	// A file where the trash folder belongs makes the move fail.
	if err := os.MkdirAll(trash.TrashDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trash.TrashDir(), trash.ReplacedByInitDir), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	notTrashed := replaceIdenticalCopies(cfg, skills)

	if len(notTrashed) != 1 || !strings.HasPrefix(notTrashed[0], "claude/alpha: ") {
		t.Errorf("want one failed move for claude/alpha, got %v", notTrashed)
	}
	if _, err := os.Stat(filepath.Join(tgt, "alpha", "SKILL.md")); err != nil {
		t.Errorf("the folder must stay in the target: %v", err)
	}
}

// The first sync keeps a link another tool put on a skill's name. init must
// say so instead of only reporting the skill as synced.
func TestFirstSyncReportsLinkKeptFromAnotherTool(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	source, tgt, other := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "alpha", "SKILL.md"), []byte("---\nname: alpha\n---\nours"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(tgt, "alpha")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	cfg := &config.Config{Source: source, Mode: "merge", Targets: map[string]config.TargetConfig{"claude": {Path: tgt}}}

	var warnings []string
	captureStdout(t, func() {
		var err error
		if _, warnings, err = firstSync(cfg); err != nil {
			t.Fatal(err)
		}
	})

	if len(warnings) != 1 || !strings.Contains(warnings[0], "claude: kept local: alpha") {
		t.Fatalf("want the kept link reported, got %v", warnings)
	}
}
