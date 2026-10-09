//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// targetFlagScope runs `target claude <args>` in global or project mode.
type targetFlagScope struct {
	name       string
	run        func(args ...string) *testutil.Result
	configPath string
}

func targetFlagScopes(t *testing.T, sb *testutil.Sandbox) []targetFlagScope {
	t.Helper()
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)
	projectRoot := sb.SetupProjectDir("claude")
	return []targetFlagScope{
		{"global", func(args ...string) *testutil.Result {
			return sb.RunCLI(append([]string{"target", "claude"}, args...)...)
		}, sb.ConfigPath},
		{"project", func(args ...string) *testutil.Result {
			return sb.RunCLIInDir(projectRoot, append([]string{"target", "claude"}, append(args, "-p")...)...)
		}, filepath.Join(projectRoot, ".skillshare", "config.yaml")},
	}
}

func TestTarget_SettingFlagsAreAllApplied(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, sc := range targetFlagScopes(t, sb) {
		t.Run(sc.name, func(t *testing.T) {
			result := sc.run("--mode", "copy", "--agent-mode", "symlink", "--target-naming", "prefixed")
			result.AssertSuccess(t)
			result.AssertAnyOutputContains(t, "agent mode: merge -> symlink")
			cfg := sb.ReadFile(sc.configPath)
			for _, want := range []string{"mode: copy", "mode: symlink", "target_naming: prefixed"} {
				if !strings.Contains(cfg, want) {
					t.Errorf("config is missing %q:\n%s", want, cfg)
				}
			}
		})
	}
}

func TestTarget_RejectedSettingsLeaveConfigUntouched(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, sc := range targetFlagScopes(t, sb) {
		t.Run(sc.name, func(t *testing.T) {
			sc.run().AssertSuccess(t) // loading migrates a legacy config once; compare after that
			before := sb.ReadFile(sc.configPath)
			for name, args := range map[string][]string{
				"invalid agent mode": {"--mode", "copy", "--agent-mode", "bogus"},
				"naming needs copy":  {"--agent-mode", "copy", "--target-naming", "prefixed"},
				"with filters":       {"--mode", "copy", "--add-exclude", "x"},
				"with skills switch": {"--agent-mode", "copy", "--skills=false"},
				"dry run":            {"--mode", "copy", "--dry-run"},
				"empty value":        {"--mode", ""},
			} {
				sc.run(args...).AssertFailure(t)
				if after := sb.ReadFile(sc.configPath); after != before {
					t.Errorf("%s changed the config:\n%s", name, after)
				}
			}
		})
	}
}

func TestTarget_ValueFlagDoesNotSwallowNextFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, sc := range targetFlagScopes(t, sb) {
		t.Run(sc.name, func(t *testing.T) {
			result := sc.run("--target-naming", "-p")
			result.AssertFailure(t)
			result.AssertAnyOutputContains(t, "--target-naming requires a value")
		})
	}
}

func TestTarget_SameValueIsReportedUnchanged(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, sc := range targetFlagScopes(t, sb) {
		t.Run(sc.name, func(t *testing.T) {
			sc.run("--mode", "copy", "--agent-mode", "copy", "--target-naming", "prefixed").AssertSuccess(t)
			// Save would drop the comment, so it proves the config was not rewritten.
			f, err := os.OpenFile(sc.configPath, os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.WriteString("# keep\n"); err != nil {
				t.Fatal(err)
			}
			f.Close()

			result := sc.run("--mode", "copy", "--agent-mode", "copy", "--target-naming", "prefixed")
			result.AssertSuccess(t)
			result.AssertAnyOutputContains(t, "agent mode unchanged")
			result.AssertOutputNotContains(t, "Changed")
			if !strings.Contains(sb.ReadFile(sc.configPath), "# keep") {
				t.Error("an unchanged setting must not rewrite the config")
			}
		})
	}
}

// A target with skills off may keep a prefixed naming its mode cannot sync, so
// an agents-only change must not trip over that skills pair.
func TestTarget_AgentModeIgnoresSkillsPairOfSkillsOffTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	scopes := targetFlagScopes(t, sb)
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
      mode: merge
      target_naming: prefixed
      enabled: false
`)
	if err := os.WriteFile(scopes[1].configPath, []byte(`targets:
  - name: claude
    skills:
      mode: merge
      target_naming: prefixed
      enabled: false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, sc := range scopes {
		t.Run(sc.name, func(t *testing.T) {
			sc.run("--agent-mode", "copy").AssertSuccess(t)
		})
	}
}
