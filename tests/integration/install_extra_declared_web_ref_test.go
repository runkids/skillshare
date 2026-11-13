//go:build !online

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestInstallExtra_DeclaredURLBranchTracksResolvedRef(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, explicit := range []bool{false, true} {
			t.Run(fmt.Sprintf("project=%t/explicit=%t", project, explicit), func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				bare := testutil.SetupBareRemoteRepo(t, sb.Root)
				testutil.SeedRemoteBranch(t, sb.Root, bare, "feature/x", map[string]string{
					"rules/rule.md": "correct", "x/rules/rule.md": "WRONG",
				})
				gitConfig := filepath.Join(sb.Root, "fixture.gitconfig")
				testutil.RunGit(t, sb.Root, "config", "--file", gitConfig, "url.file://"+filepath.ToSlash(bare)+".insteadOf", "https://github.com/example-org/ai-hub.git")
				sb.SetEnv("GIT_CONFIG_GLOBAL", gitConfig)
				sb.SetEnv("GIT_CONFIG_NOSYSTEM", "1")
				sb.SetEnv("GIT_ALLOW_PROTOCOL", "file")
				raw := "https://github.com/example-org/ai-hub/tree/feature/x/rules"
				// Supported raw-editor declarations can select a branch before
				// content/metadata exists; this value needs URL-ref resolution.
				sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nextras:\n  - name: rules\n    remote: " + raw + "\n    branch: feature\n    targets: []\n")
				cwd, mode, cfgPath := sb.Root, "-g", sb.ConfigPath
				root := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
				if project {
					cwd, mode = sb.SetupProjectDir("claude"), "-p"
					cfg, err := config.LoadProjectReadOnly(cwd)
					if err != nil {
						t.Fatal(err)
					}
					cfg.Extras = []config.ExtraConfig{{Name: "rules", Remote: raw, Branch: "feature", Targets: []config.ExtraTargetConfig{}}}
					if err := cfg.Save(cwd); err != nil {
						t.Fatal(err)
					}
					cfgPath = config.ProjectConfigPath(cwd)
					root = cfg.EffectiveExtrasSource(cwd)
				}
				args := []string{"install", raw, "--kind", "extra", "--name", "rules", "--skip-audit", "--json", mode}
				if explicit {
					args = append(args, "--branch", "feature/x")
				}
				result := sb.RunCLIInDir(cwd, args...)
				result.AssertSuccess(t)
				var out struct{ Branch, Commit string }
				if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil || out.Branch != "feature/x" {
					t.Fatalf("effective ref: %+v %v", out, err)
				}
				data, err := os.ReadFile(cfgPath)
				if err != nil {
					t.Fatal(err)
				}
				entries, err := config.RawExtras(data)
				if err != nil || len(entries) != 1 || entries[0].Branch != out.Branch {
					t.Errorf("saved branch disagrees with resolved install: %+v result=%+v error=%v", entries, out, err)
				}
				store, _, err := install.ReadMetadata(root)
				if err != nil || store.GetExact("rules") == nil || store.GetExact("rules").Branch != out.Branch {
					t.Errorf("metadata branch disagrees: %+v %v", store, err)
				}
				data, err = os.ReadFile(filepath.Join(root, "rules", "rule.md"))
				if err != nil || strings.TrimSpace(string(data)) != "correct" {
					t.Errorf("resolved directory disagrees: %s %v", data, err)
				}
			})
		}
	}
}
