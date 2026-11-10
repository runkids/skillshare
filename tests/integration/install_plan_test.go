//go:build !online

package integration

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"skillshare/internal/testutil"
)

func TestInstall_JSON_PlannedOmitted(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, tracked := range []bool{false, true} {
			mode := "global"
			if project {
				mode = "project"
			}
			kind := "config"
			if tracked {
				kind = "tracked"
			}
			t.Run(mode+"/"+kind, func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
				cwd := sb.Root
				args := []string{"install", "--json", "--dry-run"}
				if project {
					cwd = sb.SetupProjectDir("claude")
					args = append(args, "-p")
				} else {
					args = append(args, "-g")
				}
				if tracked {
					repo := createMultiSkillGitRepo(t, sb, "tracked", []string{"design"})
					args = append(args, "file://"+repo, "--track")
				}
				result := sb.RunCLIInDir(cwd, args...)
				result.AssertSuccess(t)
				var out map[string]any
				if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
					t.Fatal(err)
				}
				if _, ok := out["planned"]; ok {
					t.Fatalf("unexpected plan for %s install: %s", kind, result.Stdout)
				}
			})
		}
	}
}

func TestInstall_JSON_Planned(t *testing.T) {
	type skill struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	for _, project := range []bool{false, true} {
		mode := "global"
		if project {
			mode = "project"
		}
		for _, layout := range []string{"collection", "fallback", "root", "local", "local-frontmatter", "local-renamed", "renamed", "empty"} {
			t.Run(mode+"/"+layout, func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
				cwd, dest := sb.Root, sb.SourcePath
				args := []string{"install", "--json", "--dry-run", "--kind", "skill"}
				if project {
					cwd = sb.SetupProjectDir("claude")
					dest = filepath.Join(cwd, ".skillshare", "skills")
					args = append(args, "-p")
				} else {
					args = append(args, "-g")
				}
				repo := filepath.Join(sb.Root, "collection")
				source := "file://" + repo
				want := []skill{}
				sb.WriteFile(filepath.Join(repo, "README.md"), "# Collection")
				sb.WriteFile(filepath.Join(repo, ".claude/skills/phantom/SKILL.md"), "# Target copy")
				switch layout {
				case "collection":
					sb.WriteFile(filepath.Join(repo, "cli/assets/design/SKILL.md"), "# Design")
					sb.WriteFile(filepath.Join(repo, ".claude/skills/design/SKILL.md"), "# Target copy")
					sb.WriteFile(filepath.Join(repo, "agents/critic.md"), "---\nname: critic\ndescription: Review code\n---\n# Critic")
					sb.WriteFile(filepath.Join(repo, "skills/SKILL.md"), "---\nname: Custom Tool\n---\n# Tool")
					sb.WriteFile(filepath.Join(repo, "ignored/SKILL.md"), "# Ignored")
					sb.WriteFile(filepath.Join(repo, ".agents/skills/copy/SKILL.md"), "# Copy")
					sb.WriteFile(filepath.Join(repo, ".skillignore"), "ignored/\n")
					want = []skill{{"design", "cli/assets/design"}, {"custom-tool", "skills"}}
				case "fallback":
					sb.WriteFile(filepath.Join(repo, ".agents/skills/only/SKILL.md"), "# Authored")
					want = []skill{{"only", ".agents/skills/only"}}
				case "local-frontmatter":
					sb.WriteFile(filepath.Join(repo, "skills/SKILL.md"), "---\nname: Custom Tool\n---\n# Tool")
					source = filepath.Join(repo, "skills")
					want = []skill{{"custom-tool", "."}}
				case "root", "local", "local-renamed", "renamed":
					sb.WriteFile(filepath.Join(repo, "SKILL.md"), "# Root")
					want = []skill{{"collection", "."}}
					if layout == "local" || layout == "local-renamed" {
						source = repo
					}
					if layout == "renamed" || layout == "local-renamed" {
						args = append(args, "--name", "custom-root")
						want[0].Name = "custom-root"
					}
				}
				initGitRepo(t, repo)
				result := sb.RunCLIInDir(cwd, append(args, source)...)
				result.AssertSuccess(t)
				var out struct {
					Planned *[]skill `json:"planned"`
					Skills  []string `json:"skills"`
					Failed  []string `json:"failed"`
				}
				if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
					t.Fatalf("invalid JSON: %v\n%s", err, result.Stdout)
				}
				if out.Planned == nil || !reflect.DeepEqual(*out.Planned, want) {
					t.Fatalf("planned = %v, want %v; output: %s", out.Planned, want, result.Stdout)
				}
				if len(out.Skills) != 0 || len(out.Failed) != 0 {
					t.Fatalf("dry-run reported installed/failed skills: %s", result.Stdout)
				}
				for _, s := range want {
					if sb.FileExists(filepath.Join(dest, s.Name, "SKILL.md")) {
						t.Fatalf("dry-run installed %s", s.Name)
					}
				}
				if layout == "collection" {
					// Preview selection uses the same canonical names as the install.
					selected := sb.RunCLIInDir(cwd, append(args, source, "--skill", "custom-tool")...)
					selected.AssertSuccess(t)
					if err := json.Unmarshal([]byte(selected.Stdout), &out); err != nil {
						t.Fatal(err)
					}
					if out.Planned == nil || !reflect.DeepEqual(*out.Planned, want[1:]) {
						t.Fatalf("selected plan: %s", selected.Stdout)
					}
					// Existing destinations must not prevent listing candidates.
					sb.WriteFile(filepath.Join(dest, "custom-tool/SKILL.md"), "# Existing")
					sb.RunCLIInDir(cwd, append(args, source)...).AssertSuccess(t)
					installArgs := []string{"install", source, "--json", "--kind", "skill", "--skill", "design"}
					if project {
						installArgs = append(installArgs, "-p")
					} else {
						installArgs = append(installArgs, "-g")
					}
					installed := sb.RunCLIInDir(cwd, installArgs...)
					installed.AssertSuccess(t)
					if got := sb.ReadFile(filepath.Join(dest, "design/SKILL.md")); got != "# Design" {
						t.Fatalf("installed a different path: %q", got)
					}
					var actual map[string]any
					if err := json.Unmarshal([]byte(installed.Stdout), &actual); err != nil {
						t.Fatal(err)
					}
					if _, ok := actual["planned"]; ok || !reflect.DeepEqual(actual["skills"], []any{"design"}) {
						t.Fatalf("unexpected actual install result: %s", installed.Stdout)
					}
				}
			})
		}
	}
}
