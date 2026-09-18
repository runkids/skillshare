package plugin

import (
	"os"
	"path/filepath"
	"slices"
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
