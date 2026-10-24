//go:build windows && !online

package integration

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Native Windows coverage complements lexical path and cross-compilation checks.
// The CI step records the runner's elevated/basic token and architecture.
func TestProject_NativeLinksSmoke(t *testing.T) {
	t.Logf("native runtime %s/%s", runtime.GOOS, runtime.GOARCH)
	for _, mode := range []string{"merge", "symlink", "copy"} {
		t.Run(mode, func(t *testing.T) {
			sb, root := projectFixture(t)
			sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
			sb.RunCLI("project", "add", root, "--target", "claude", "--skills-mode", mode, "--sync", "--json").AssertSuccess(t)
			target := filepath.Join(root, ".claude", "skills")
			if _, err := os.Stat(filepath.Join(target, "alpha", "SKILL.md")); err != nil {
				t.Fatal(err)
			}
			sb.RunCLI("project", "remove", root, "--yes", "--json").AssertSuccess(t)
			if _, err := os.Stat(filepath.Join(sb.SourcePath, "alpha", "SKILL.md")); err != nil {
				t.Fatal("source removed", err)
			}
			if mode == "symlink" {
				if _, err := os.Lstat(target); !os.IsNotExist(err) {
					t.Fatal("whole-folder link not detached", err)
				}
			} else {
				if _, err := os.Stat(filepath.Join(target, "alpha", "SKILL.md")); err != nil {
					t.Fatal("local files not retained", err)
				}
			}
		})
	}
}
