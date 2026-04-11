package apply

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"skillshare/internal/resources/adapters"
)

func TestCompiledFiles_SortsUpdatedAndSkipped(t *testing.T) {
	dir := t.TempDir()
	samePath := filepath.Join(dir, "a.json")
	updatePath := filepath.Join(dir, "b.json")

	if err := os.WriteFile(samePath, []byte("same"), 0o644); err != nil {
		t.Fatalf("seed samePath: %v", err)
	}
	if err := os.WriteFile(updatePath, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed updatePath: %v", err)
	}

	updated, skipped, err := CompiledFiles([]adapters.CompiledFile{
		{Path: updatePath, Content: "new"},
		{Path: samePath, Content: "same"},
	}, false)
	if err != nil {
		t.Fatalf("CompiledFiles() error = %v", err)
	}

	if !reflect.DeepEqual(updated, []string{updatePath}) {
		t.Fatalf("updated = %v, want [%s]", updated, updatePath)
	}
	if !reflect.DeepEqual(skipped, []string{samePath}) {
		t.Fatalf("skipped = %v, want [%s]", skipped, samePath)
	}

	got, err := os.ReadFile(updatePath)
	if err != nil {
		t.Fatalf("read updated path: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("updated file content = %q, want %q", string(got), "new")
	}
}

func TestCompiledFiles_LeavesDestinationUntouchedWhenAtomicWriteFails(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(targetPath, []byte("original"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	originalWriter := writeFileAtomically
	writeFileAtomically = func(path string, data []byte, perm os.FileMode) error {
		return errors.New("boom")
	}
	t.Cleanup(func() {
		writeFileAtomically = originalWriter
	})

	_, _, err := CompiledFiles([]adapters.CompiledFile{
		{Path: targetPath, Content: "replacement"},
	}, false)
	if err == nil {
		t.Fatal("CompiledFiles() error = nil, want write failure")
	}

	got, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("read target after failure: %v", readErr)
	}
	if string(got) != "original" {
		t.Fatalf("target content after failed write = %q, want %q", string(got), "original")
	}
}
