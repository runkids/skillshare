//go:build !online

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestInstallExtra_EarlyErrorsNeverEchoCredentials(t *testing.T) {
	credential := "https://user:SYNTHETIC@example.com/repo"
	cases := map[string][]string{
		"positional":    {"example-org/ai-hub", credential},
		"name":          {"example-org/ai-hub", "--name", credential},
		"option":        {"example-org/ai-hub", "--" + credential},
		"kind":          {"example-org/ai-hub", "--kind", credential},
		"threshold":     {"example-org/ai-hub", "--audit-threshold", credential},
		"source-parser": {"unrecognized-source-SYNTHETIC"},
	}
	for _, project := range []bool{false, true} {
		for _, output := range []string{"text", "json", "dry-run"} {
			for name, input := range cases {
				t.Run(fmt.Sprintf("project=%t/%s/%s", project, output, name), func(t *testing.T) {
					sb := testutil.NewSandbox(t)
					defer sb.Cleanup()
					sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
					cwd, mode := sb.Root, "-g"
					if project {
						cwd, mode = sb.SetupProjectDir("claude"), "-p"
					}
					before := projectTree(t, sb.Root)
					// Put --kind after malformed inputs for half the paths: the
					// boundary must recognize extra intent before formatting errors.
					args := append([]string{"install"}, input...)
					args = append(args, "--kind", "extra", mode)
					if name == "kind" {
						args = append([]string{"install", "--kind", "extra"}, input...)
						args = append(args, mode)
					}
					if output == "json" {
						args = append(args, "--json")
					} else if output == "dry-run" {
						args = append(args, "--dry-run")
					}
					result := sb.RunCLIInDir(cwd, args...)
					result.AssertFailure(t)
					if strings.Contains(result.Stdout+result.Stderr, "SYNTHETIC") {
						t.Errorf("input disclosed: stdout=%s stderr=%s", result.Stdout, result.Stderr)
					}
					if output == "json" && !json.Valid([]byte(result.Stdout)) {
						t.Errorf("early error stdout is not JSON: %s", result.Stdout)
					}
					if after := projectTree(t, sb.Root); !reflect.DeepEqual(before, after) {
						t.Error("refusal changed fixture files, including logs/config/metadata/pins")
					}
				})
			}
		}
	}
}

func TestInstallExtra_WebSlashRefResolvesDirectoryAndBranch(t *testing.T) {
	providers := map[string]string{
		"github.com":    "tree/",
		"gitlab.com":    "-/tree/",
		"bitbucket.org": "src/",
	}
	for host, marker := range providers {
		for _, project := range []bool{false, true} {
			for _, explicit := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/project=%t/explicit=%t", host, project, explicit), func(t *testing.T) {
					sb := testutil.NewSandbox(t)
					defer sb.Cleanup()
					sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
					bare := testutil.SetupBareRemoteRepo(t, sb.Root)
					files := map[string]string{"rules/rule.md": "correct slash-ref directory", "x/rules/rule.md": "WRONG directory from first-segment ref split"}
					testutil.SeedRemoteBranch(t, sb.Root, bare, "feature/x", files)
					head := strings.TrimSpace(testutil.RunGit(t, bare, "rev-parse", "refs/heads/feature/x"))
					gitConfig := filepath.Join(sb.Root, "fixture.gitconfig")
					repoURL := "https://" + host + "/example-org/ai-hub"
					testutil.RunGit(t, sb.Root, "config", "--file", gitConfig, "url.file://"+filepath.ToSlash(bare)+".insteadOf", repoURL+".git")
					sb.SetEnv("GIT_CONFIG_GLOBAL", gitConfig)
					sb.SetEnv("GIT_CONFIG_NOSYSTEM", "1")
					sb.SetEnv("GIT_ALLOW_PROTOCOL", "file")
					cwd, mode := sb.Root, "-g"
					root := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
					if project {
						cwd, mode = sb.SetupProjectDir("claude"), "-p"
						root = filepath.Join(cwd, ".skillshare", "extras")
					}
					raw := repoURL + "/" + marker + "feature/x/rules"
					args := []string{"install", raw, "--kind", "extra", "--name", "rules", "--skip-audit", "--json", mode}
					if explicit {
						args = append(args, "--branch", "feature/x")
					}
					result := sb.RunCLIInDir(cwd, args...)
					result.AssertSuccess(t)
					var out struct{ Branch, Commit string }
					if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
						t.Fatal(err)
					}
					if out.Branch != "feature/x" || out.Commit != head {
						t.Errorf("effective ref lost: %+v want feature/x at%s", out, head)
					}
					if data, err := os.ReadFile(filepath.Join(root, "rules", "rule.md")); err != nil || string(data) != files["rules/rule.md"] {
						t.Errorf("wrong selected directory: %s %v", data, err)
					}
					store, _, err := install.ReadMetadata(root)
					if err != nil {
						t.Fatal(err)
					}
					entry := store.GetExact("rules")
					if entry == nil || entry.Branch != "feature/x" || entry.Commit != head || entry.Source != raw {
						t.Errorf("metadata ref/identity lost: %+v", entry)
					}
					var extras []config.ExtraConfig
					if project {
						cfg, err := config.LoadProjectReadOnly(cwd)
						if err != nil {
							t.Fatal(err)
						}
						extras = cfg.Extras
						if pin := readLock(t, cwd).Extras["rules"]; pin.Source != raw || pin.Commit != head {
							t.Errorf("project pin lost: %+v", pin)
						}
					} else {
						data, err := os.ReadFile(sb.ConfigPath)
						if err != nil {
							t.Fatal(err)
						}
						extras, err = config.RawExtras(data)
						if err != nil {
							t.Fatal(err)
						}
					}
					if len(extras) != 1 || extras[0].Branch != "feature/x" || extras[0].Remote != raw {
						t.Errorf("saved ref/identity lost: %+v", extras)
					}
					// Omitted branch on a later install must preserve the resolved
					// declaration and the same exact pin, even when the URL is ambiguous.
					sb.RunCLIInDir(cwd, "install", raw, "--kind", "extra", "--name", "rules", "--skip-audit", "--json", mode).AssertSuccess(t)
					if data, err := os.ReadFile(filepath.Join(root, "rules", "rule.md")); err != nil || string(data) != files["rules/rule.md"] {
						t.Errorf("reinstall changed selected directory: %s %v", data, err)
					}
				})
			}
		}
	}
}
