package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"skillshare/internal/config"
)

func writeIgnoreTestFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// setupNegationCopy creates a skill with docs/keep.md re-included under the
// ignored docs/ folder and syncs it once in copy mode.
func setupNegationCopy(t *testing.T) (src, tgt string, resync func() *CopyResult) {
	t.Helper()
	src, tgt = t.TempDir(), t.TempDir()
	writeIgnoreTestFiles(t, filepath.Join(src, "skill-a"), map[string]string{
		"SKILL.md":     "---\nname: skill-a\n---\n# A",
		"docs/keep.md": "keep",
		"docs/drop.md": "drop",
	})
	opts := CopyOptions{IgnorePatterns: EffectiveFileIgnorePatterns([]string{"docs/", "!docs/keep.md"})}
	resync = func() *CopyResult {
		t.Helper()
		res, err := SyncTargetCopyWithOptions("test", config.TargetConfig{Path: tgt, Mode: "copy"}, src, false, false, opts)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	resync()
	return src, tgt, resync
}

func TestSyncTargetCopy_NegationReincludesFileUnderIgnoredDir(t *testing.T) {
	_, tgt, _ := setupNegationCopy(t)

	if _, err := os.Stat(filepath.Join(tgt, "skill-a", "docs", "keep.md")); err != nil {
		t.Errorf("re-included docs/keep.md should be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tgt, "skill-a", "docs", "drop.md")); !os.IsNotExist(err) {
		t.Errorf("ignored docs/drop.md should not be copied, stat err = %v", err)
	}
}

func TestSyncTargetCopy_NegationReincludedFileChangeIsSynced(t *testing.T) {
	src, tgt, resync := setupNegationCopy(t)

	keep := filepath.Join(src, "skill-a", "docs", "keep.md")
	if err := os.WriteFile(keep, []byte("keep v2"), 0644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(keep, later, later); err != nil {
		t.Fatal(err)
	}

	resync()
	got, err := os.ReadFile(filepath.Join(tgt, "skill-a", "docs", "keep.md"))
	if err != nil || string(got) != "keep v2" {
		t.Errorf("changed re-included file should be synced, got %q, err %v", got, err)
	}
}

func TestCopySkillToTarget_IgnoredDirWithoutKeptFilesNotCreated(t *testing.T) {
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "skill-a")
	writeIgnoreTestFiles(t, src, map[string]string{
		"SKILL.md":                    "# A",
		"tmp/note.txt":                "temporary",
		"scripts/run.sh":              "run",
		"scripts/__pycache__/run.pyc": "cache",
	})
	// "!*.md" can match anywhere, so the walk must enter tmp/ and __pycache__/.
	patterns := EffectiveFileIgnorePatterns([]string{"tmp/", "!*.md"})
	if err := copySkillToTarget(src, dst, "", patterns); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"tmp", "scripts/__pycache__"} {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("ignored %s has no kept files and should not be created, stat err = %v", rel, err)
		}
	}
}

func TestCopySkillToTarget_NegationReincludesDirUnderIgnoredDir(t *testing.T) {
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "skill-a")
	writeIgnoreTestFiles(t, src, map[string]string{
		"SKILL.md":      "# A",
		"docs/api/x.md": "keep",
		"docs/other.md": "drop",
	})
	patterns := EffectiveFileIgnorePatterns([]string{"docs/", "!docs/api/"})
	if err := copySkillToTarget(src, dst, "", patterns); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dst, "docs", "api", "x.md")); err != nil {
		t.Errorf("file in re-included docs/api should be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "docs", "other.md")); !os.IsNotExist(err) {
		t.Errorf("ignored docs/other.md should not be copied, stat err = %v", err)
	}
}

func TestCopiedFiles_ListsWhatTheCopyCopies(t *testing.T) {
	src, dst, outside := t.TempDir(), filepath.Join(t.TempDir(), "skill-a"), t.TempDir()
	writeIgnoreTestFiles(t, src, map[string]string{
		"SKILL.md":                "# a",
		"notes.md":                "ignored",
		"docs/drop.md":            "ignored dir",
		"docs/keep.md":            "re-included",
		".DS_Store":               "default ignore",
		"__pycache__/x.pyc":       "default ignore",
		".git/HEAD":               "never copied",
		"scripts/run.sh":          "kept",
		"scripts/notes.md":        "ignored at any depth",
		"scripts/nested/deep.txt": "kept",
	})
	writeIgnoreTestFiles(t, outside, map[string]string{"linked.md": "through a link", "notes.md": "ignored through a link"})
	if err := os.Symlink(outside, filepath.Join(src, "shared")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	patterns := EffectiveFileIgnorePatterns([]string{"notes.md", "docs/", "!docs/keep.md"})
	if err := copySkillToTarget(src, dst, "", patterns); err != nil {
		t.Fatal(err)
	}
	copied := map[string]bool{}
	if err := filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dst, path)
		copied[filepath.ToSlash(rel)] = true
		return err
	}); err != nil {
		t.Fatal(err)
	}

	listed, err := CopiedFiles(src, patterns)
	if err != nil {
		t.Fatal(err)
	}
	for rel := range listed {
		if !copied[rel] {
			t.Errorf("CopiedFiles lists %s, which the copy left out", rel)
		}
	}
	for rel := range copied {
		if _, ok := listed[rel]; !ok {
			t.Errorf("the copy made %s, which CopiedFiles leaves out", rel)
		}
	}
}
