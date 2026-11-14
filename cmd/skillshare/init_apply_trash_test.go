package main

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
	"skillshare/internal/trash"
)

// init's import replaces a target folder that matches the source byte for byte
// with a link. The folder must go to the trash, not be deleted, so it can be
// recovered.
func TestReplaceIdenticalCopiesMovesCopiesToTrash(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	src := filepath.Join(t.TempDir(), "alpha")
	tgt := t.TempDir()
	for _, dir := range []string{src, filepath.Join(tgt, "alpha")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: alpha\n---\nsame"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{Mode: "merge", Targets: map[string]config.TargetConfig{"claude": {Path: tgt}}}

	kept := replaceIdenticalCopies(cfg, []ssync.DiscoveredSkill{{SourcePath: src, FlatName: "alpha"}})

	if len(kept) != 0 {
		t.Errorf("identical copy reported as kept: %v", kept)
	}
	if _, err := os.Lstat(filepath.Join(tgt, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("identical copy still in the target: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(trash.TrashDir(), "replaced-by-init", "claude", "alpha_*", "SKILL.md"))
	if len(matches) != 1 {
		t.Fatalf("want the copy in the trash, found %v", matches)
	}
}
