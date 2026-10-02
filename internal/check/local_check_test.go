package check

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
)

// localEntry writes a skill at a fresh local source dir and returns a
// metadata entry whose file hashes match it, as `install <path>` records.
func localEntry(t *testing.T) (*install.MetadataEntry, string) {
	t.Helper()
	src := filepath.Join(t.TempDir(), "my-skill")
	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# v1"), 0644)

	hashes, err := install.ComputeFileHashes(src)
	if err != nil {
		t.Fatal(err)
	}
	return &install.MetadataEntry{Source: src, Type: "local", FileHashes: hashes}, src
}

func TestLocalSourceStatus_Unchanged(t *testing.T) {
	entry, _ := localEntry(t)

	if status, _ := LocalSourceStatus(entry); status != "up_to_date" {
		t.Errorf("status = %q, want up_to_date", status)
	}
}

func TestLocalSourceStatus_FileChanged(t *testing.T) {
	entry, src := localEntry(t)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# v2"), 0644)

	if status, _ := LocalSourceStatus(entry); status != "update_available" {
		t.Errorf("status = %q, want update_available", status)
	}
}

func TestLocalSourceStatus_FileAdded(t *testing.T) {
	entry, src := localEntry(t)
	os.WriteFile(filepath.Join(src, "reference.md"), []byte("new"), 0644)

	if status, _ := LocalSourceStatus(entry); status != "update_available" {
		t.Errorf("status = %q, want update_available", status)
	}
}

func TestLocalSourceStatus_SourceMissing(t *testing.T) {
	entry, src := localEntry(t)
	os.RemoveAll(src)

	status, msg := LocalSourceStatus(entry)
	if status != "error" || msg == "" {
		t.Errorf("got (%q, %q), want error with a message", status, msg)
	}
}

func TestLocalSourceStatus_NoRecordedHashes(t *testing.T) {
	entry, _ := localEntry(t)
	entry.FileHashes = nil

	if status, _ := LocalSourceStatus(entry); status != "local" {
		t.Errorf("status = %q, want local", status)
	}
}

func TestLocalSourceStatus_NotLocalType(t *testing.T) {
	entry, _ := localEntry(t)
	entry.Type = "github"

	if status, _ := LocalSourceStatus(entry); status != "local" {
		t.Errorf("status = %q, want local", status)
	}
}
