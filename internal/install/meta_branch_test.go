package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetaBranchRoundTrip(t *testing.T) {
	dir := t.TempDir()
	meta := &SkillMeta{
		Source: "github.com/owner/repo",
		Type:   "github",
		Branch: "develop",
	}
	if err := WriteMeta(dir, meta); err != nil {
		t.Fatalf("WriteMeta: %v", err)
	}
	loaded, err := ReadMeta(dir)
	if err != nil {
		t.Fatalf("ReadMeta: %v", err)
	}
	if loaded.Branch != "develop" {
		t.Errorf("Branch = %q, want %q", loaded.Branch, "develop")
	}
}

func TestMetaBranchOmittedWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	meta := &SkillMeta{
		Source: "github.com/owner/repo",
		Type:   "github",
	}
	if err := WriteMeta(dir, meta); err != nil {
		t.Fatalf("WriteMeta: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, MetaFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), "branch") {
		t.Errorf("empty branch should be omitted from JSON, got: %s", data)
	}
}
