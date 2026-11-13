package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExtraJournal_AtomicRoundTrip(t *testing.T) {
	root := t.TempDir()
	journal := extraJournal{Version: 1, Name: "rules", Remote: "https://example.com/org/repo", Branch: "main", Commit: strings.Repeat("a", 40), HadPrevious: true, Old: map[string]string{"file": "sha256:old"}}
	if err := writeExtraJournal(root, journal); err != nil {
		t.Fatal(err)
	}
	got, err := readExtraJournal(root, "rules")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != journal.Name || got.Commit != journal.Commit || got.Old["file"] != "sha256:old" {
		t.Fatalf("journal: %+v", got)
	}
}

func TestExtraJournal_InvalidStaysInPlace(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, extraStagingDir)
	if err := os.Mkdir(stage, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stage, "publishing-rules.json")
	original := []byte(`{"version":1,"name":"rules","remote":"https://u:secret@example.com/repo","commit":"bad"}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readExtraJournal(root, "rules"); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe journal: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(original) {
		t.Fatalf("journal changed: %q %v", got, err)
	}
}

func TestExtraFingerprint_DetectsEditsWithoutFollowingLinks(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	first, err := fingerprintExtraTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(file, 0755); err != nil {
			t.Fatal(err)
		}
		same, err := fingerprintExtraTree(dir)
		if err != nil || !sameExtraFingerprint(first, same) {
			t.Fatalf("mode-only change: %v", err)
		}
	}
	if err := os.WriteFile(file, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	next, err := fingerprintExtraTree(dir)
	if err != nil || sameExtraFingerprint(first, next) {
		t.Fatalf("missed change: %v", err)
	}
}

func TestExtraSnapshot_CompareBeforeRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("before"), 0640); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotExtraFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("ours"), 0600); err != nil {
		t.Fatal(err)
	}
	written, err := snapshotExtraFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	if ok, err := restoreExtraSnapshotIfUnchanged(path, before, written); err != nil || ok {
		t.Fatalf("overwrote another writer: %v %v", ok, err)
	}
	if err := os.WriteFile(path, []byte("ours"), 0600); err != nil {
		t.Fatal(err)
	}
	if ok, err := restoreExtraSnapshotIfUnchanged(path, before, written); err != nil || !ok {
		t.Fatalf("restore: %v %v", ok, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "before" {
		t.Fatalf("restore: %q %v", data, err)
	}
}

func TestExtraJournal_RejectsMarkerSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires symlink privilege")
	}
	root := t.TempDir()
	stage := filepath.Join(root, extraStagingDir)
	if err := os.Mkdir(stage, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "sentinel")
	if err := os.WriteFile(outside, []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, extraJournalPath(root, "rules")); err != nil {
		t.Fatal(err)
	}
	if _, err := readExtraJournal(root, "rules"); err == nil {
		t.Fatal("followed marker link")
	}
	j := extraJournal{Version: 1, Name: "rules", Remote: "https://example.com/repo", Commit: strings.Repeat("a", 40)}
	if err := writeExtraJournal(root, j); err == nil {
		t.Fatal("overwrote marker link")
	}
	got, err := os.ReadFile(outside)
	if err != nil || string(got) != "sentinel" {
		t.Fatalf("sentinel changed: %q %v", got, err)
	}
}
