//go:build !windows

package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateLink_Relative(t *testing.T) {
	tmp := t.TempDir()

	srcDir := filepath.Join(tmp, "project", ".skillshare", "skills", "my-skill")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tgtDir := filepath.Join(tmp, "project", ".claude", "skills")
	if err := os.MkdirAll(tgtDir, 0755); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(tgtDir, "my-skill")
	if err := createLink(linkPath, srcDir, true); err != nil {
		t.Fatalf("createLink with relative=true failed: %v", err)
	}

	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	if filepath.IsAbs(target) {
		t.Errorf("expected relative symlink, got absolute: %s", target)
	}

	content, err := os.ReadFile(filepath.Join(linkPath, "SKILL.md"))
	if err != nil {
		t.Fatalf("cannot read through relative symlink: %v", err)
	}
	if string(content) != "test" {
		t.Errorf("content = %q, want %q", string(content), "test")
	}
}

func TestCreateLink_Absolute(t *testing.T) {
	tmp := t.TempDir()

	srcDir := filepath.Join(tmp, "source", "skill")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	tgtDir := filepath.Join(tmp, "target")
	if err := os.MkdirAll(tgtDir, 0755); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(tgtDir, "skill")
	if err := createLink(linkPath, srcDir, false); err != nil {
		t.Fatalf("createLink with relative=false failed: %v", err)
	}

	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	if !filepath.IsAbs(target) {
		t.Errorf("expected absolute symlink, got relative: %s", target)
	}
}
