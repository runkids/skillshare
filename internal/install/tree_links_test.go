package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTreeLinks(t *testing.T) {
	links := map[string]string{
		".agents/skills/foo": "../../store/foo",
		"pack":               "store",
		"store/chain":        "foo",
		"abs":                "/etc",
		"up":                 "../outside",
		"loop/a":             "b",
		"loop/b":             "a",
		"publish":            "alias/../foo",
		"alias":              "store/nested",
	}
	tests := []struct {
		path, want string
		ok         bool
	}{
		{"store/foo", "store/foo", true},
		{".agents/skills/foo", "store/foo", true},
		{"pack/foo/refs", "store/foo/refs", true},
		{"pack/chain", "store/foo", true},
		{"abs", "", false},
		{"up", "", false},
		{"loop/a", "", false},
		// ".." after a link leaves the folder the link leads to, not the link.
		{"publish", "store/foo", true},
	}
	for _, tt := range tests {
		got, ok := ResolveTreeLinks(tt.path, func(p string) (string, bool, error) {
			text, isLink := links[p]
			return text, isLink, nil
		})
		if got != tt.want || ok != tt.ok {
			t.Errorf("ResolveTreeLinks(%q) = %q, %v; want %q, %v", tt.path, got, ok, tt.want, tt.ok)
		}
	}
}

func TestGetSubdirTreeHash_LinkSubdirHashesTarget(t *testing.T) {
	repo := initTestRepo(t)
	writeCommit(t, repo, "store/foo", "# foo\n")
	if err := os.MkdirAll(filepath.Join(repo, ".agents", "skills"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "store", "foo"), filepath.Join(repo, ".agents", "skills", "foo")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "publish foo")

	want := getSubdirTreeHash(repo, "store/foo")
	if got := getSubdirTreeHash(repo, ".agents/skills/foo"); want == "" || got != want {
		t.Fatalf("tree hash through the link = %q, want the target's %q", got, want)
	}
}

func TestGetSubdirTreeHash_LinkedParentFolder(t *testing.T) {
	repo := initTestRepo(t)
	writeCommit(t, repo, "store/foo", "# foo\n")
	if err := os.Symlink("store", filepath.Join(repo, "pack")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "link pack")

	want := getSubdirTreeHash(repo, "store/foo")
	if got := getSubdirTreeHash(repo, "pack/foo"); want == "" || got != want {
		t.Fatalf("tree hash through the linked parent = %q, want %q", got, want)
	}
}

func TestGetSubdirTreeHash_LinkTargetHoldingLinkComparesByCommit(t *testing.T) {
	repo := initTestRepo(t)
	writeCommit(t, repo, "store/foo", "# foo\n")
	writeCommit(t, repo, "shared", "# shared\n")
	if err := os.MkdirAll(filepath.Join(repo, ".agents", "skills"), 0755); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		".agents/skills/foo": filepath.Join("..", "..", "store", "foo"),
		"store/foo/ref.md":   filepath.Join("..", "..", "shared", "SKILL.md"),
	} {
		if err := os.Symlink(target, filepath.Join(repo, link)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "publish foo")

	if got := getSubdirTreeHash(repo, ".agents/skills/foo"); got != "" {
		t.Fatalf("tree hash = %q, want empty: the target holds a link", got)
	}
}

func TestGetSubdirTreeHash_DotDotAfterLinkLeavesItsTarget(t *testing.T) {
	repo := initTestRepo(t)
	writeCommit(t, repo, "store/nested", "# nested\n")
	writeCommit(t, repo, "store/foo", "# foo\n")
	writeCommit(t, repo, "foo", "# top\n")
	for link, target := range map[string]string{
		"alias":   "store/nested",
		"publish": "alias/../foo", // literal: filepath.Join would clean it to "foo"
	} {
		if err := os.Symlink(target, filepath.Join(repo, link)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "publish")

	want := getSubdirTreeHash(repo, "store/foo")
	if got := getSubdirTreeHash(repo, "publish"); want == "" || got != want {
		t.Fatalf("tree hash of publish = %q, want store/foo's %q", got, want)
	}
}
