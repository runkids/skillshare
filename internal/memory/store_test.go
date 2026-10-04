package memory

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	syncpkg "skillshare/internal/sync"
)

func TestStoreNotesAndExternalEdit(t *testing.T) {
	root := filepath.Join(t.TempDir(), "memory")
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	note, err := Write(root, "projects/build.md", "---\ntype: project\n---\n# Build notes\nUse the devcontainer.\n", "")
	if err != nil {
		t.Fatal(err)
	}
	if note.Title != "Build notes" || note.Version == "" {
		t.Fatalf("unexpected note: %+v", note)
	}
	found, err := List(root, "DEVCONTAINER")
	if err != nil || len(found) != 1 || found[0].Path != note.Path {
		t.Fatalf("search: %+v, %v", found, err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects/build.md"), []byte("# External edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root, note.Path, "lost update", note.Version); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, note.Path))
	if string(data) != "# External edit\n" {
		t.Fatal("external edit overwritten")
	}
}

func TestStoreRejectsUnsafePathsAndSkipsEditorState(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "private.md"), []byte("private"), 0644)
	paths := []string{"../private.md", "/absolute.md", `..\private.md`, "C:/private.md", ".obsidian/hidden.md", "not-markdown.txt"}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err == nil {
		paths = append(paths, "linked/private.md")
	} else {
		t.Logf("directory symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, "private.md"), filepath.Join(root, "link.md")); err == nil {
		paths = append(paths, "link.md")
	} else {
		t.Logf("file symlinks unavailable: %v", err)
	}
	os.MkdirAll(filepath.Join(root, ".obsidian"), 0755)
	os.WriteFile(filepath.Join(root, ".obsidian/hidden.md"), []byte("hidden"), 0644)
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			if _, err := Write(root, path, "changed", ""); err == nil {
				t.Fatal("unsafe path accepted")
			}
		})
	}
	entries, err := List(root, "")
	if err != nil || len(entries) != 0 {
		t.Fatalf("listed hidden or linked files: %+v, %v", entries, err)
	}
	data, _ := os.ReadFile(filepath.Join(outside, "private.md"))
	if string(data) != "private" {
		t.Fatal("outside file modified")
	}
}

func TestInitPreservesExistingIndex(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "INDEX.md"), []byte("# My index\n"), 0644)
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "INDEX.md"))
	if string(data) != "# My index\n" {
		t.Fatal("existing index overwritten")
	}
	// Instructions renders the folder in slash form, so comparing it against an OS-native
	// Windows path can never match.
	if !strings.Contains(Instructions(root, "", ModePassive), filepath.ToSlash(root)) {
		t.Fatal("loading instructions missing canonical path")
	}
}

func TestInitStarterNotes(t *testing.T) {
	for _, existing := range [][]string{nil, {"INDEX.md"}, {"LEARNED.md"}, {"INDEX.md", "LEARNED.md"}} {
		t.Run(strings.Join(existing, "+"), func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "memory")
			for _, path := range existing {
				if _, err := Write(root, path, "# My "+path+"\n", ""); err != nil {
					t.Fatal(err)
				}
			}
			if err := Init(root); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"INDEX.md", "LEARNED.md"} {
				note, err := Read(root, path)
				if err != nil {
					t.Fatal(err)
				}
				preserved := false
				for _, prior := range existing {
					if path == prior {
						preserved = true
					}
				}
				if preserved {
					if note.Content != "# My "+path+"\n" {
						t.Fatalf("existing %s overwritten", path)
					}
				} else if path == "INDEX.md" {
					if !strings.Contains(note.Content, "[Lessons learned](LEARNED.md)") {
						t.Fatal("starter index missing lessons link")
					}
				} else {
					for _, field := range []string{"Date:", "Context:", "Conclusion:", "Evidence:"} {
						if !strings.Contains(note.Content, field) {
							t.Fatalf("lessons template missing %s", field)
						}
					}
				}
				if err := Init(root); err != nil {
					t.Fatal(err)
				}
				after, err := Read(root, path)
				if err != nil || after.Version != note.Version {
					t.Fatalf("reinitialization changed %s: %v", path, err)
				}
			}
		})
	}
}

func TestWriteBacksUpAndPreservesMode(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	root := t.TempDir()
	before, err := Write(root, "note.md", "# Before\n", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, before.Path), 0600); err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(filepath.Join(root, before.Path))
	if err != nil {
		t.Fatal(err)
	}
	after, err := Write(root, before.Path, "# After\n", before.Version)
	if err != nil || after.Version == before.Version {
		t.Fatalf("updated note: %+v, %v", after, err)
	}
	info, err := os.Stat(filepath.Join(root, before.Path))
	if err != nil || info.Mode().Perm() != beforeInfo.Mode().Perm() {
		t.Fatalf("mode not preserved: %v, %v", info, err)
	}
	versions, err := syncpkg.FileBackupVersions(filepath.Join(root, before.Path))
	if err != nil || len(versions) != 1 {
		t.Fatalf("backups: %+v, %v", versions, err)
	}
	data, _, err := syncpkg.ReadFileBackupVersion(filepath.Join(root, before.Path), versions[0].ID)
	if err != nil || string(data) != before.Content {
		t.Fatalf("backup: %q, %v", data, err)
	}
	if _, err := Write(root, before.Path, "too large"+strings.Repeat("x", MaxNoteBytes), after.Version); err == nil {
		t.Fatal("oversized edit accepted")
	}
	current, err := Read(root, before.Path)
	if err != nil || current.Version != after.Version {
		t.Fatal("rejected edit changed note")
	}
}

func TestDeleteBacksUpAndRejectsStaleVersions(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	root := t.TempDir()
	note, err := Write(root, "wiki/note.md", "# Keep evidence\n", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"", "stale"} {
		if err := Delete(root, note.Path, version); !errors.Is(err, ErrConflict) {
			t.Fatalf("stale deletion: %v", err)
		}
	}
	if _, err := Read(root, note.Path); err != nil {
		t.Fatalf("rejected deletion changed note: %v", err)
	}
	if err := Delete(root, "../outside.md", note.Version); err == nil {
		t.Fatal("traversal accepted")
	}
	if err := Delete(root, note.Path, note.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, note.Path); !os.IsNotExist(err) {
		t.Fatalf("note still exists: %v", err)
	}
	abs := filepath.Join(root, note.Path)
	versions, err := syncpkg.FileBackupVersions(abs)
	if err != nil || len(versions) != 1 || versions[0].Reason != syncpkg.BackupReasonDelete {
		t.Fatalf("delete backup: %+v, %v", versions, err)
	}
	if _, err := syncpkg.RestoreFileBackup(abs, versions[0].ID, false); err != nil {
		t.Fatal(err)
	}
	restored, err := Read(root, note.Path)
	if err != nil || restored.Content != note.Content {
		t.Fatalf("restored: %+v, %v", restored, err)
	}
}

func TestListConfiguredLinkedRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := Write(root, "note.md", "# Shared\n", ""); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "memory")
	if err := os.Symlink(root, link); err != nil {
		t.Skip(err)
	}
	notes, err := List(link, "")
	if err != nil || len(notes) != 1 {
		t.Fatalf("linked root: %+v, %v", notes, err)
	}
}

func TestMoveNote(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	root := t.TempDir()
	note, err := Write(root, "wiki/note.md", "# Evidence\nKeep this content.\n", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, note.Path), 0600); err != nil {
		t.Fatal(err)
	}
	originalInfo, _ := os.Stat(filepath.Join(root, note.Path))
	renamed, err := Move(root, note.Path, "wiki/renamed.md", note.Version)
	if err != nil {
		t.Fatal(err)
	}
	moved, err := Move(root, renamed.Path, "projects/deep/evidence.md", renamed.Version)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Path != "projects/deep/evidence.md" || moved.Content != note.Content || moved.Version != note.Version {
		t.Fatalf("moved note: %+v", moved)
	}
	info, err := os.Stat(filepath.Join(root, moved.Path))
	if err != nil || info.Mode().Perm() != originalInfo.Mode().Perm() {
		t.Fatalf("mode: %v, %v", info, err)
	}
	for _, path := range []string{note.Path, renamed.Path} {
		if _, err := Read(root, path); !os.IsNotExist(err) {
			t.Fatalf("old path exists: %s: %v", path, err)
		}
		versions, err := syncpkg.FileBackupVersions(filepath.Join(root, path))
		if err != nil || len(versions) != 1 {
			t.Fatalf("backup: %+v, %v", versions, err)
		}
		data, _, err := syncpkg.ReadFileBackupVersion(filepath.Join(root, path), versions[0].ID)
		if err != nil || string(data) != note.Content {
			t.Fatalf("backup content: %q, %v", data, err)
		}
	}
}

func TestMoveRejectsConflictAndUnsafeDestination(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	root := t.TempDir()
	note, err := Write(root, "note.md", "# Original\n", "")
	if err != nil {
		t.Fatal(err)
	}
	existing, err := Write(root, "existing.md", "# Existing\n", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"", "stale"} {
		if _, err := Move(root, note.Path, "new.md", version); !errors.Is(err, ErrConflict) {
			t.Fatalf("version: %v", err)
		}
	}
	paths := []string{"existing.md", "../outside.md", ".hidden/note.md", "note.txt", "wiki/../note.md"}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err == nil {
		paths = append(paths, "linked/note.md")
	}
	if err := os.Symlink(filepath.Join(outside, "absent.md"), filepath.Join(root, "dangling.md")); err == nil {
		paths = append(paths, "dangling.md")
	}
	for _, path := range paths {
		if _, err := Move(root, note.Path, path, note.Version); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	current, err := Read(root, note.Path)
	if err != nil || current.Version != note.Version {
		t.Fatalf("source changed: %+v %v", current, err)
	}
	current, err = Read(root, existing.Path)
	if err != nil || current.Version != existing.Version {
		t.Fatalf("destination changed: %+v %v", current, err)
	}
	if _, err := Move(root, "absent.md", "new.md", note.Version); !os.IsNotExist(err) {
		t.Fatalf("missing source: %v", err)
	}
	same, err := Move(root, note.Path, note.Path, note.Version)
	if err != nil || same.Version != note.Version {
		t.Fatalf("same path: %+v %v", same, err)
	}
}
