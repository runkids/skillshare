package plugin

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFilesListsTheSnapshotAndReadsOnlyInsideIt(t *testing.T) {
	home := t.TempDir()
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	writePluginFile(t, home, "config.yaml", "plugins:\n  packages:\n    demo:\n      bindings:\n        claude:\n          id: demo@skillshare-abc\n          source: /src/demo\n          plugin: demo\n    imported:\n      bindings:\n        claude:\n          id: other@market\n")
	content := filepath.Join(s.snapshotPath(Binding{ID: "demo@skillshare-abc", Source: "/src/demo", Plugin: "demo"}, "claude"), "content")
	for path, body := range map[string]string{"skills/hi/SKILL.md": "# hi", "node_modules/dep/index.js": "x", "../secret": "no"} {
		full := filepath.Join(content, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	b := Binding{ID: "demo@skillshare-abc", Source: "/src/demo", Plugin: "demo"}
	b.Digest, _ = treeDigest(content)
	writePluginFile(t, filepath.Dir(content), ".skillshare-owner", b.Source+"\n"+b.Plugin)
	writePluginFile(t, filepath.Dir(content), ".skillshare-digest", b.Digest)
	seedLocalBinding(t, s, "demo", "claude", b)
	files, err := s.Files("demo")
	if err != nil || !slices.Equal(files, []string{"skills/hi/SKILL.md"}) {
		t.Fatalf("files = %v, %v", files, err)
	}
	if data, err := s.ReadFile("demo", "skills/hi/SKILL.md"); err != nil || string(data) != "# hi" {
		t.Fatalf("read = %q, %v", data, err)
	}
	if _, err := s.ReadFile("demo", "../secret"); err == nil {
		t.Fatal("a path outside the snapshot was read")
	}
	if files, err := s.Files("imported"); err != nil || len(files) != 0 {
		t.Fatalf("an imported plugin has no snapshot: %v, %v", files, err)
	}
}

func TestSaveKeepsTwoSpaceIndent(t *testing.T) {
	s := &Service{ConfigPath: filepath.Join(t.TempDir(), "config.yaml")}
	raw := []byte("ignore:\n  - '**/.git/**'\n")
	if err := os.WriteFile(s.ConfigPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	d, err := decodeDocument(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.intent["demo"] = packageDeclaration{Bindings: map[string]declaration{"claude": {Imported: true}}}
	if err := s.save(d); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(s.ConfigPath)
	want := "ignore:\n  - '**/.git/**'\nplugins:\n  version: 2\n  packages:\n    demo:\n      bindings:\n        claude:\n          imported: true\n"
	if !strings.HasPrefix(string(data), want) {
		t.Errorf("got:\n%s\nwant prefix:\n%s", data, want)
	}
}

// Opening a managed plugin asks which other Agents can take it. Its reviewed snapshot answers,
// so nothing is cloned: with no git on PATH, a clone would fail.
func TestDiscoverManagedReadsTheSnapshotWithoutGit(t *testing.T) {
	var calls []string
	s := accountPluginService(t, &calls)
	source := "https://example.invalid/demo.git"
	digest, err := treeDigest(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	b := Binding{ID: "demo@skillshare-abc", Source: source, Plugin: "demo", Digest: digest, Commit: "abc123"}
	if err := copyTree(fixture(t), filepath.Join(s.snapshotPath(b, "claude"), "content")); err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, filepath.Dir(s.ConfigPath), "config.yaml", "plugins:\n  packages:\n    demo:\n      bindings:\n        claude:\n          id: demo@skillshare-abc\n          source: "+source+"\n          plugin: demo\n          commit: abc123\n          digest: "+digest+"\n")
	writePluginFile(t, s.snapshotPath(b, "claude"), ".skillshare-owner", b.Source+"\n"+b.Plugin)
	writePluginFile(t, s.snapshotPath(b, "claude"), ".skillshare-digest", b.Digest)
	seedLocalBinding(t, s, "demo", "claude", b)
	t.Setenv("PATH", t.TempDir())
	d, err := s.DiscoverManaged(context.Background(), "demo", source, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Source != source || d.Commit != "abc123" || d.Digest != digest || len(d.Candidates) != 1 || !slices.Contains(d.Candidates[0].Targets, "claude-work") || len(calls) != 0 {
		t.Fatalf("discovery = %+v, calls = %v", d, calls)
	}
}
