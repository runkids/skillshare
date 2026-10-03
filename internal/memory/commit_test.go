package memory

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	syncpkg "skillshare/internal/sync"
)

func TestCommitNoteRechecksPreparedDraft(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	for _, change := range []string{"edited", "deleted", "created"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "note.md")
			version := ""
			if change != "created" {
				note, err := Write(root, "note.md", "reviewed", "")
				if err != nil {
					t.Fatal(err)
				}
				version = note.Version
				if err := syncpkg.BackupFile(path, syncpkg.BackupReasonEdit); err != nil {
					t.Fatal(err)
				}
			}
			draft := filepath.Join(root, ".memory-draft")
			if err := os.WriteFile(draft, []byte("my draft"), 0600); err != nil {
				t.Fatal(err)
			}
			if change == "deleted" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("external edit"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := commitNote(root, "note.md", draft, version); !errors.Is(err, ErrConflict) {
				t.Fatalf("expected conflict before committing %s, got %v", change, err)
			}
			if change == "deleted" {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("deleted note recreated")
				}
			} else if data, err := os.ReadFile(path); err != nil || string(data) != "external edit" {
				t.Fatalf("external edit lost: %q, %v", data, err)
			}
		})
	}
}

func TestRemoveNoteRechecksAfterBackup(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	root := t.TempDir()
	note, err := Write(root, "note.md", "reviewed", "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "note.md")
	if err := syncpkg.BackupFile(path, syncpkg.BackupReasonDelete); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("external edit after backup"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := removeNote(root, note.Path, note.Version); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict before removal, got %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "external edit after backup" {
		t.Fatalf("external edit lost: %q, %v", data, err)
	}
}

func TestCommitNoteConcurrentCreates(t *testing.T) {
	root := t.TempDir()
	drafts := []string{filepath.Join(root, ".memory-first"), filepath.Join(root, ".memory-second")}
	for _, draft := range drafts {
		if err := os.WriteFile(draft, []byte(draft), 0644); err != nil {
			t.Fatal(err)
		}
	}
	type result struct {
		draft string
		err   error
	}
	results := make(chan result, len(drafts))
	start := make(chan struct{})
	for _, draft := range drafts {
		go func() {
			<-start
			results <- result{draft, commitNote(root, "note.md", draft, "")}
		}()
	}
	close(start)
	winner, conflicts := "", 0
	for range drafts {
		res := <-results
		if res.err == nil {
			winner = res.draft
		} else if errors.Is(res.err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(res.err)
		}
	}
	if winner == "" || conflicts != 1 {
		t.Fatalf("winner = %q, conflicts = %d", winner, conflicts)
	}
	data, err := os.ReadFile(filepath.Join(root, "note.md"))
	if err != nil || string(data) != winner {
		t.Fatalf("winner overwritten: %q, %v", data, err)
	}
}
