//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestVariants_SharedFolderSyncStatusAndDiff(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"merge", "copy"} {
			scope := "global"
			if project {
				scope = "project"
			}
			t.Run(scope+"/"+mode, func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				root, source := sb.Root, sb.SourcePath
				dest := filepath.Join(sb.Home, ".agents", "skills")
				flag := "-g"
				if project {
					root = sb.SetupProjectDir("codex", "universal")
					source = filepath.Join(root, ".skillshare", "skills")
					dest = filepath.Join(root, ".agents", "skills")
					flag = "-p"
					sb.WriteProjectConfig(root, "targets:\n  - name: codex\n    mode: "+mode+"\n  - name: universal\n    mode: "+mode+"\n")
				} else {
					sb.WriteConfig("source: " + source + "\nmode: " + mode + "\ntargets:\n  codex: {}\n  universal: {}\n")
				}
				for _, agent := range []string{"claude", "codex"} {
					dir := filepath.Join(source, "_repo/internal/skills", agent, "fix")
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Instructions for "+agent), 0644); err != nil {
						t.Fatal(err)
					}
				}
				initGitRepo(t, filepath.Join(source, "_repo"))
				for i := 0; i < 2; i++ {
					sb.RunCLIInDir(root, "sync", flag, "--json").AssertSuccess(t)
					got, err := os.ReadFile(filepath.Join(dest, "_repo__internal__skills__codex__fix", "SKILL.md"))
					if err != nil || string(got) != "# Instructions for codex" {
						t.Fatalf("variant lost after sync %d: %q, %v", i, got, err)
					}
				}
				status := sb.RunCLIInDir(root, "status", flag, "--json")
				status.AssertSuccess(t)
				var counts struct {
					Skills   int `json:"skill_count"`
					Variants int `json:"variant_count"`
				}
				if err := json.Unmarshal([]byte(status.Stdout), &counts); err != nil {
					t.Fatal(err)
				}
				if counts.Skills != 1 || counts.Variants != 2 {
					t.Fatalf("inventory: %s", status.Stdout)
				}
				diff := sb.RunCLIInDir(root, "diff", flag, "--json")
				diff.AssertSuccess(t)
				var preview struct {
					Targets []struct {
						Synced bool `json:"synced"`
					} `json:"targets"`
				}
				if err := json.Unmarshal([]byte(diff.Stdout), &preview); err != nil {
					t.Fatal(err)
				}
				if len(preview.Targets) != 2 {
					t.Fatalf("diff targets: %s", diff.Stdout)
				}
				for _, target := range preview.Targets {
					if !target.Synced {
						t.Fatalf("shared folder disagrees with sync: %s", diff.Stdout)
					}
				}
				// A target-only run still respects its configured co-writer.
				sb.RunCLIInDir(root, "sync", flag, "--json", "universal").AssertSuccess(t)
				if _, err := os.Stat(filepath.Join(dest, "_repo__internal__skills__codex__fix", "SKILL.md")); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestVariants_CategoryAndNamingWarningsDoNotCreatePendingDiff(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("_repo/skills/agents/planner", map[string]string{"SKILL.md": "---\nname: planner\n---\n# Planner"})
	sb.CreateSkill("invalid", map[string]string{"SKILL.md": "---\nname: wrong-name\n---\n# Invalid"})
	target := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + "\ntarget_naming: standard\ntargets:\n  claude:\n    path: " + target + "\n")
	sb.RunCLI("sync", "-g").AssertSuccess(t)
	if _, err := os.Stat(filepath.Join(target, "planner", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	result := sb.RunCLI("diff", "-g", "--json")
	result.AssertSuccess(t)
	var preview struct {
		Targets []struct {
			Synced bool `json:"synced"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Targets) != 1 || !preview.Targets[0].Synced {
		t.Fatalf("naming warning becomes pending work: %s", result.Stdout)
	}
}

func TestVariants_AccountSyncAndPrune(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		t.Run(mode, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			for _, agent := range []string{"codex", "claude"} {
				sb.CreateNestedSkill("_repo/skills/"+agent+"/fix", map[string]string{"SKILL.md": "# " + agent})
			}
			dest := filepath.Join(sb.Home, "account", "skills")
			config := "source: " + sb.SourcePath + "\nmode: " + mode + "\ntargets:\n  codex-work:\n    agent: codex\n    config_dir: " + filepath.Join(sb.Home, "account") + "\n    path: " + dest + "\n"
			sb.WriteConfig(config)
			for i := 0; i < 2; i++ {
				sb.RunCLI("sync", "-g").AssertSuccess(t)
			}
			selected := filepath.Join(dest, "_repo__skills__codex__fix", "SKILL.md")
			if got := sb.ReadFile(selected); got != "# codex" {
				t.Fatalf("account bytes = %q", got)
			}
			if sb.FileExists(filepath.Join(dest, "_repo__skills__claude__fix")) {
				t.Fatal("account received other agent")
			}
			sb.WriteConfig(config + "    exclude: ['*codex*']\n")
			sb.RunCLI("sync", "-g").AssertSuccess(t)
			if sb.FileExists(selected) {
				t.Fatal("excluded account variant was not pruned")
			}
		})
	}
}

func TestVariants_ProjectAmbiguityAndOverrides(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		t.Run(mode, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			root := sb.SetupProjectDir("claude")
			source := filepath.Join(root, ".skillshare", "skills")
			cfg := "targets:\n  - name: claude\n    mode: " + mode + "\n"
			sb.WriteProjectConfig(root, cfg)
			for _, agent := range []string{"claude", "claude-code", "codex"} {
				dir := filepath.Join(source, "_repo", "skills", agent, "fix")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+agent), 0644); err != nil {
					t.Fatal(err)
				}
			}
			dest := filepath.Join(root, ".claude", "skills")
			sb.RunCLIInDir(root, "sync", "-p").AssertSuccess(t)
			for _, agent := range []string{"claude", "claude-code"} {
				if sb.FileExists(filepath.Join(dest, "_repo__skills__"+agent+"__fix")) {
					t.Fatal("ambiguous variant synced")
				}
			}
			sb.WriteProjectConfig(root, cfg+"    include: ['_repo__skills__claude__fix']\n")
			sb.RunCLIInDir(root, "sync", "-p").AssertSuccess(t)
			if !sb.FileExists(filepath.Join(dest, "_repo__skills__claude__fix", "SKILL.md")) {
				t.Fatal("include did not resolve ambiguity")
			}
			sb.WriteProjectConfig(root, cfg)
			store := install.NewMetadataStore()
			store.SetTargetOverride("_repo/skills/codex/fix", []string{"claude"})
			if err := store.Save(source); err != nil {
				t.Fatal(err)
			}
			sb.RunCLIInDir(root, "sync", "-p").AssertSuccess(t)
			if got := sb.ReadFile(filepath.Join(dest, "_repo__skills__codex__fix", "SKILL.md")); got != "# codex" {
				t.Fatalf("override lost bytes: %q", got)
			}
			if sb.FileExists(filepath.Join(dest, "_repo__skills__claude__fix")) {
				t.Fatal("newly ambiguous variant was not pruned")
			}
		})
	}
}

func TestVariants_InitAndStatusAgree(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	if err := os.Remove(sb.ConfigPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	remote := createSkillsRemote(t, sb.Root, map[string]string{
		"_repo/skills/codex/fix": "# codex fix", "_repo/skills/claude/fix": "# claude fix",
		"_repo/skills/codex/review": "# codex review", "_repo/skills/claude/review": "# claude review",
	})
	result := sb.RunCLI("init", "--remote", remote, "--no-skill")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "2 skills")
	result.AssertOutputNotContains(t, "4 skills")
	status := sb.RunCLI("status", "-g", "--json")
	status.AssertSuccess(t)
	var counts struct {
		Skills   int `json:"skill_count"`
		Variants int `json:"variant_count"`
	}
	if err := json.Unmarshal([]byte(status.Stdout), &counts); err != nil {
		t.Fatal(err)
	}
	if counts.Skills != 2 || counts.Variants != 4 {
		t.Fatalf("status: %s", status.Stdout)
	}
}
