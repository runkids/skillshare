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
	}
	for _, tt := range tests {
		got, ok := ResolveTreeLinks(tt.path, links)
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
