package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

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
