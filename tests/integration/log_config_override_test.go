//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// A global config selected with SKILLSHARE_CONFIG may live in a directory
// named like a project directory; its operation log still belongs in the
// global state directory, not next to the config.
func TestLog_GlobalConfigOverrideUsesStateDir(t *testing.T) {
	for _, dir := range []string{"skillshare", ".skillshare"} {
		t.Run(dir, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()

			sb.CreateSkill("test-skill", map[string]string{"SKILL.md": "# Test\n\nTest."})
			targetPath := sb.CreateTarget("claude")
			sb.ConfigPath = filepath.Join(sb.Home, "shared", dir, "config.yaml")
			sb.WriteConfig("source: " + sb.SourcePath + "\nmode: merge\ntargets:\n  claude:\n    path: " + targetPath + "\n")

			sb.RunCLI("sync", "-g").AssertSuccess(t)

			raw, err := os.ReadFile(filepath.Join(sb.Home, ".local", "state", "skillshare", "logs", "operations.log"))
			if err != nil || !strings.Contains(string(raw), `"cmd":"sync"`) {
				t.Fatalf("sync not logged to global state: %q %v", raw, err)
			}
			for _, name := range []string{"logs", ".gitignore"} {
				if _, err := os.Stat(filepath.Join(filepath.Dir(sb.ConfigPath), name)); !os.IsNotExist(err) {
					t.Fatalf("global config treated as a project: %s exists (%v)", name, err)
				}
			}

			read := sb.RunCLI("log", "-g", "--cmd", "sync", "--json")
			read.AssertSuccess(t)
			if !strings.Contains(read.Stdout, `"cmd":"sync"`) {
				t.Fatalf("global log reader missed sync: %s", read.Output())
			}
			list := sb.RunCLI("log", "-g", "--no-tui")
			list.AssertSuccess(t)
			list.AssertOutputNotContains(t, "· project")
		})
	}
}
