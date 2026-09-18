package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, root, name, data string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSourceInternalLinksSurviveSnapshot(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, "CLAUDE.md", "instructions")
	for name, target := range map[string]string{"AGENTS.md": "CLAUDE.md", "instructions": "skills", "nested": "AGENTS.md"} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "copy")
	if err := copyTree(root, dest); err != nil {
		t.Fatal(err)
	}
	if link, err := os.Readlink(filepath.Join(dest, "AGENTS.md")); err != nil || link != "CLAUDE.md" {
		t.Fatalf("link lost: %s %v", link, err)
	}
	if digest, err := treeDigest(dest); err != nil || digest != d.Digest {
		t.Fatalf("copy digest differs: %s %v", digest, err)
	}
	writeFile(t, root, "CLAUDE.md", "changed")
	if digest, _ := treeDigest(root); digest == d.Digest {
		t.Fatal("target edit did not change digest")
	}
}

func TestSourceRejectsUnsafeLinks(t *testing.T) {
	for _, target := range []string{"/etc/passwd", "../outside", "missing", "link", ".git/config", "."} {
		t.Run(target, func(t *testing.T) {
			root := fixture(t)
			writeFile(t, root, ".git/config", "private")
			if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
				t.Fatal(err)
			}
			if _, err := treeDigest(root); err == nil {
				t.Fatal("unsafe link accepted")
			}
			if err := copyTree(root, filepath.Join(t.TempDir(), "copy")); err == nil {
				t.Fatal("unsafe copy accepted")
			}
		})
	}
}

func TestDiscoverAllCatalogsAndRelativeURL(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, ".agents/plugins/marketplace.json", `{"name":"codex-market","plugins":[{"name":"demo","source":{"source":"url","url":"./"}}]}`)
	writeFile(t, root, ".claude-plugin/marketplace.json", `{"name":"claude-market","plugins":[{"name":"demo","source":"./"},{"name":"second","source":"./second"}]}`)
	writeFile(t, root, "second/.claude-plugin/plugin.json", `{"name":"second"}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 2 || d.Candidates[0].Problem != "" || len(d.Candidates[0].Targets) != 5 {
		t.Fatalf("catalogs not merged: %+v", d)
	}
}

func TestSourceRejectsCrossDirectoryLinkCycle(t *testing.T) {
	root := fixture(t)
	for _, dir := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("../b", filepath.Join(root, "a", "next")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../a", filepath.Join(root, "b", "next")); err != nil {
		t.Fatal(err)
	}
	if _, err := treeDigest(root); err == nil {
		t.Fatal("directory cycle accepted")
	}
}

func TestSourceLinkParentAfterDirectoryLink(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "nested/deep/keep", "data")
	writeFile(t, root, "nested/target", "data")
	if err := os.Symlink("nested/deep", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("alias/../target", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveSourcePath(root, filepath.Join(root, "link"))
	if err != nil || resolved != filepath.Join(root, "nested/target") {
		t.Fatalf("wrong link resolution: %s %v", resolved, err)
	}
}
