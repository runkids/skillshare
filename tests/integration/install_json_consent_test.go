//go:build !online

package integration

import (
	"fmt"
	"path/filepath"
	"testing"

	"skillshare/internal/oplog"
	"skillshare/internal/testutil"
)

// Reinstalling a repo keeps its skills and the agents that come with them,
// and the kept names reach the JSON result and the operation log.
func TestInstall_JSON_ReportsKeptAgentsOfSkillRepo(t *testing.T) {
	for _, mode := range []string{"global", "project"} {
		t.Run(mode, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			cwd, skillsDir, configPath, flag := sb.Root, sb.SourcePath, sb.ConfigPath, "-g"
			if mode == "project" {
				cwd = sb.SetupProjectDir("claude")
				skillsDir, configPath, flag = filepath.Join(cwd, ".skillshare", "skills"), projectConfigPath(cwd), "-p"
			}
			repo := filepath.Join(sb.Root, "mixed")
			for _, name := range []string{"alpha", "beta"} {
				sb.WriteFile(filepath.Join(repo, "skills", name, "SKILL.md"), "# "+name)
			}
			sb.WriteFile(filepath.Join(repo, "agents", "helper.md"), "# Helper")
			initGitRepo(t, repo)
			sb.RunCLIInDir(cwd, "install", "file://"+repo, "--json", flag).AssertSuccess(t)
			installed := filepath.Join(filepath.Dir(skillsDir), "agents", "helper.md")
			sb.WriteFile(installed, "# local edit")

			result := sb.RunCLIInDir(cwd, "install", "file://"+repo, "--json", flag)
			if got := sb.ReadFile(installed); got != "# local edit" {
				t.Fatalf("--json overwrote the installed agent: %q", got)
			}
			result.AssertSuccess(t)
			if got := fmt.Sprint(parseJSON(t, result.Stdout)["skipped"]); got != "[alpha beta helper]" {
				t.Errorf("skipped = %s, want [alpha beta helper]: %s", got, result.Stdout)
			}
			entries, err := oplog.Read(configPath, oplog.OpsFile, 1)
			if err != nil || len(entries) != 1 || entries[0].Command != "install" {
				t.Fatalf("last oplog entry = %+v, %v", entries, err)
			}
			if got := fmt.Sprint(entries[0].Args["skipped_skills"]); got != "[alpha beta helper]" {
				t.Errorf("oplog skipped_skills = %s, want [alpha beta helper]", got)
			}
		})
	}
}

// Skipping the only skill of a repo must not skip the agents requested with it.
func TestInstall_SkippedSingleSkillStillInstallsRequestedAgent(t *testing.T) {
	for _, subdir := range []string{"", "pack"} {
		t.Run("subdir="+subdir, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			repo := filepath.Join(sb.Root, "single")
			sb.WriteFile(filepath.Join(repo, subdir, "skills", "alpha", "SKILL.md"), "# alpha")
			sb.WriteFile(filepath.Join(repo, subdir, "agents", "helper.md"), "# Helper")
			initGitRepo(t, repo)
			source := "file://" + repo
			if subdir != "" {
				source += "//" + subdir
			}
			sb.RunCLI("install", source, "-s", "alpha", "--json", "-g").AssertSuccess(t)

			result := sb.RunCLI("install", source, "-s", "alpha", "-a", "helper", "--json", "-g")
			result.AssertSuccess(t)
			if !sb.FileExists(filepath.Join(filepath.Dir(sb.SourcePath), "agents", "helper.md")) {
				t.Errorf("requested agent helper was not installed: %s", result.Stdout)
			}
		})
	}
}

// JSON output must never grant permission to replace locally edited installs.
func TestInstall_JSON_OverwriteConsent(t *testing.T) {
	for _, mode := range []string{"global", "project"} {
		for _, kind := range []string{"local-skill", "git-skill", "tracked-skill", "agent"} {
			t.Run(mode+"/"+kind, func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
				cwd, skillsDir, flag := sb.Root, sb.SourcePath, "-g"
				if mode == "project" {
					cwd = sb.SetupProjectDir("claude")
					skillsDir, flag = filepath.Join(cwd, ".skillshare", "skills"), "-p"
				}
				repo := filepath.Join(sb.Root, "consent-source")
				name, source, content := "consent-source", repo, "# Original skill\n"
				installedFile := filepath.Join(skillsDir, name, "SKILL.md")
				args := []string{"install", "--json", flag}
				if kind == "tracked-skill" {
					name = "_" + name
					installedFile = filepath.Join(skillsDir, name, "SKILL.md")
					args = append(args, "--track")
				}
				if kind == "agent" {
					name, content = "reviewer", "# Original reviewer\n"
					sb.WriteFile(filepath.Join(repo, "reviewer.md"), content)
					installedFile = filepath.Join(filepath.Dir(skillsDir), "agents", "reviewer.md")
					args = append(args, "--kind", "agent")
				} else {
					sb.WriteFile(filepath.Join(repo, "SKILL.md"), content)
				}
				if kind != "local-skill" {
					initGitRepo(t, repo)
					source = "file://" + repo
				}
				args = append(args, source)
				sb.RunCLIInDir(cwd, args...).AssertSuccess(t)
				const edited = "# Locally edited copy\n"
				sb.WriteFile(installedFile, edited)

				result := sb.RunCLIInDir(cwd, args...)
				if got := sb.ReadFile(installedFile); got != edited {
					t.Errorf("--json overwrote the local edit: %q", got)
				}
				output := parseJSON(t, result.Stdout)
				if installed, ok := output["skills"].([]any); !ok || len(installed) != 0 {
					t.Errorf("existing item reported as installed: %s", result.Stdout)
				}
				// A local folder records no repo, so it keeps the collision error;
				// items installed from the same repo are skipped.
				if kind == "local-skill" {
					result.AssertFailure(t)
					if output["error"] == nil || output["error"] == "" {
						t.Errorf("missing collision error: %s", result.Stdout)
					}
				} else {
					result.AssertSuccess(t)
					skipped, ok := output["skipped"].([]any)
					if !ok || len(skipped) != 1 || skipped[0] != name {
						t.Errorf("missing skipped item %q: %s", name, result.Stdout)
					}
				}

				forced := sb.RunCLIInDir(cwd, append(args, "--force")...)
				forced.AssertSuccess(t)
				output = parseJSON(t, forced.Stdout)
				installed, ok := output["skills"].([]any)
				if !ok || len(installed) != 1 || installed[0] != name {
					t.Errorf("forced overwrite not reported: %s", forced.Stdout)
				}
				if skipped, ok := output["skipped"].([]any); !ok || len(skipped) != 0 {
					t.Errorf("forced overwrite reported as skipped: %s", forced.Stdout)
				}
				if got := sb.ReadFile(installedFile); got != content {
					t.Errorf("--json --force did not replace local edit: %q", got)
				}
			})
		}
	}
}
