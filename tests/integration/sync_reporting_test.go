//go:build !online

package integration

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// keptLocalSandbox has a source skill "alpha" whose name a folder the user
// made already holds in the target.
func keptLocalSandbox(t *testing.T, mode string) (*testutil.Sandbox, string) {
	t.Helper()
	sb := testutil.NewSandbox(t)
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha"})
	targetPath := sb.CreateTarget("claude")
	sb.WriteFile(filepath.Join(targetPath, "alpha", "mine.txt"), "mine")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: ` + mode + `
`)
	return sb, targetPath
}

func TestSync_KeptLocalFolder_IsReported(t *testing.T) {
	for _, mode := range []string{"copy", "merge"} {
		t.Run(mode, func(t *testing.T) {
			sb, targetPath := keptLocalSandbox(t, mode)
			defer sb.Cleanup()

			result := sb.RunCLI("sync")
			result.AssertSuccess(t)
			result.AssertOutputContains(t, "kept local: alpha")
			result.AssertOutputContains(t, "sync --force")
			result.AssertOutputNotContains(t, "up to date")
			if !sb.FileExists(filepath.Join(targetPath, "alpha", "mine.txt")) {
				t.Fatal("the user's folder was replaced without --force")
			}
		})
	}
}

func TestStatusDoctor_SkippedSkill_IsNotCountedAsNotSynced(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha"})
	sb.CreateNestedSkill("frontend/dev", map[string]string{"SKILL.md": "---\nname: wrong-name\n---\n# Wrong"})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
target_naming: standard
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)
	sb.RunCLI("sync").AssertSuccess(t)

	// Sync skips the invalid skill on purpose, so running it again cannot help.
	status := sb.RunCLI("status")
	status.AssertSuccess(t)
	status.AssertOutputNotContains(t, "not synced")
	sb.RunCLI("doctor").AssertOutputNotContains(t, "not synced")
}

func TestDiff_LocalOnlyTarget_IsInSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha"})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)
	sb.RunCLI("sync").AssertSuccess(t)
	sb.WriteFile(filepath.Join(targetPath, "user-own", "SKILL.md"), "# Mine")

	plain := sb.RunCLI("diff", "--no-tui")
	plain.AssertSuccess(t)
	plain.AssertOutputContains(t, "user-own")
	plain.AssertOutputNotContains(t, "to sync")

	js := sb.RunCLI("diff", "--json")
	js.AssertSuccess(t)
	var out struct {
		Targets []struct {
			Synced bool `json:"synced"`
			Items  []struct {
				Name string `json:"name"`
			} `json:"items"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(js.Stdout), &out); err != nil {
		t.Fatalf("diff --json: %v\n%s", err, js.Stdout)
	}
	if len(out.Targets) != 1 || !out.Targets[0].Synced {
		t.Fatalf("a target with only local-only folders must be synced:\n%s", js.Stdout)
	}
	if len(out.Targets[0].Items) != 1 || out.Targets[0].Items[0].Name != "user-own" {
		t.Fatalf("local-only folder is no longer listed:\n%s", js.Stdout)
	}
}
