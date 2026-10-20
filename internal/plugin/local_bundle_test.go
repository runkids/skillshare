package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalInventoryUsesTargetVersion(t *testing.T) {
	for _, target := range []string{"cursor", "antigravity"} {
		t.Run(target, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			s := &Service{}
			root, _, err := s.localRoot(target)
			if err != nil {
				t.Fatal(err)
			}
			manifest := "plugin.json"
			if target == "cursor" {
				manifest = ".cursor-plugin/plugin.json"
			}
			writePluginFile(t, root, "demo/"+manifest, `{"name":"demo","version":"1.0.0"}`)
			writePluginFile(t, root, "demo/package.json", `{"name":"demo","version":"2.0.0","pi":{"extensions":["./index.ts"]}}`)
			installed, _, err := s.localInventory(target)
			if err != nil || len(installed) != 1 || installed[0].Version != "1.0.0" {
				t.Fatalf("lost native target version: %+v %v", installed, err)
			}
		})
	}
}

// A config root reached through a linked ancestor (macOS /var, a linked home or
// CLAUDE_CONFIG_DIR) is the user's choice; links inside the root are refused.
func TestNoSymlinkTrustsOnlyTheRootAndAbove(t *testing.T) {
	base := t.TempDir()
	volume := filepath.Join(base, "volume")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{filepath.Join(volume, "agent", "real"), outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := func(target, name string) {
		t.Helper()
		if err := os.Symlink(target, name); err != nil {
			t.Fatal(err)
		}
	}
	linked := filepath.Join(base, "linked")
	link(volume, linked)
	root := filepath.Join(linked, "agent")
	link(filepath.Join(outside, "settings.json"), filepath.Join(root, "settings.json"))
	link(outside, filepath.Join(root, "plugins"))
	linkedRoot := filepath.Join(base, "linked-agent")
	link(filepath.Join(volume, "agent"), linkedRoot)

	for _, tc := range []struct {
		name, root, path string
		ok               bool
	}{
		{"linked ancestor", root, filepath.Join(root, "real", "config.json"), true},
		{"linked root", linkedRoot, filepath.Join(linkedRoot, "real", "config.json"), true},
		{"missing path", root, filepath.Join(root, "new", "config.json"), true},
		{"linked config file", root, filepath.Join(root, "settings.json"), false},
		{"linked folder inside root", root, filepath.Join(root, "plugins", "demo"), false},
		{"outside root", root, filepath.Join(base, "volume", "other.json"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := noSymlink(tc.root, tc.path); (err == nil) != tc.ok {
				t.Fatalf("noSymlink(%s, %s) = %v", tc.root, tc.path, err)
			}
		})
	}
}
