//go:build !online

package integration

import (
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

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
				// Plain file:// skills use the existing collision error; tracked
				// repositories and agents use the existing skip decisions.
				if kind == "local-skill" || kind == "git-skill" {
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
