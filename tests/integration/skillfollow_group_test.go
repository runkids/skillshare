//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestSkillfollowGroupCheckAndUpdate(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			source, projectRoot := sb.SourcePath, ""
			if project {
				projectRoot = sb.SetupProjectDir("claude")
				source = filepath.Join(projectRoot, ".skillshare", "skills")
			} else {
				sb.WriteConfig("source: " + source + "\ntargets: {}\n")
			}
			run := func(args ...string) *testutil.Result {
				if project {
					return sb.RunCLIInDir(projectRoot, append(args, "-p")...)
				}
				return sb.RunCLI(append(args, "-g")...)
			}
			base := t.TempDir()
			remote := testutil.SetupBareRemoteRepo(t, base)
			testutil.SeedRemoteBranch(t, base, remote, "main", map[string]string{"safe/SKILL.md": "# Safe\n"})
			external := filepath.Join(sb.Root, "external")
			testutil.RunGit(t, "", "clone", remote, filepath.Join(external, "sub", "_repo"))
			if err := os.Symlink(external, filepath.Join(source, "group")); err != nil {
				t.Fatal(err)
			}
			sb.WriteFile(filepath.Join(source, ".skillfollow"), "group\n")
			for _, command := range []string{"check", "update"} {
				for _, group := range []string{"group", "group/sub"} {
					for _, positional := range []bool{false, true} {
						args := []string{command}
						if !positional {
							args = append(args, "--group")
						}
						args = append(args, group)
						if command == "update" {
							args = append(args, "--dry-run")
						}
						result := run(args...)
						result.AssertSuccess(t)
						result.AssertAnyOutputContains(t, "group/sub/_repo")
					}
				}
			}
			// A declared first-level link does not authorize a second link hop.
			escape := filepath.Join(sb.Root, "escape")
			sb.WriteFile(filepath.Join(escape, "a", "SKILL.md"), "# Escape\n")
			if err := os.Symlink(escape, filepath.Join(external, "escape")); err != nil {
				t.Fatal(err)
			}
			for _, command := range []string{"check", "update"} {
				args := []string{command, "--group", "group/escape"}
				if command == "update" {
					args = append(args, "--dry-run")
				}
				result := run(args...)
				result.AssertFailure(t)
				result.AssertAnyOutputContains(t, "outside source directory")
			}
			// An active declaration file must not grant access to undeclared groups.
			sb.WriteFile(filepath.Join(source, ".skillfollow"), "# no declared links\n")
			for _, command := range []string{"check", "update"} {
				for _, positional := range []bool{false, true} {
					args := []string{command}
					if !positional {
						args = append(args, "--group")
					}
					args = append(args, "group")
					if command == "update" {
						args = append(args, "--dry-run")
					}
					result := run(args...)
					result.AssertFailure(t)
					result.AssertAnyOutputContains(t, "outside source directory")
				}
			}
		})
	}
}

// An unreadable directory below a selected followed group makes discovery
// incomplete, so group check and update must fail instead of acting on the
// repos that were still readable.
func TestSkillfollowGroupUnreadableSubdirFails(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix permission enforcement")
	}
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			source, projectRoot := sb.SourcePath, ""
			if project {
				projectRoot = sb.SetupProjectDir("claude")
				source = filepath.Join(projectRoot, ".skillshare", "skills")
			} else {
				sb.WriteConfig("source: " + source + "\ntargets: {}\n")
			}
			run := func(args ...string) *testutil.Result {
				if project {
					return sb.RunCLIInDir(projectRoot, append(args, "-p")...)
				}
				return sb.RunCLI(append(args, "-g")...)
			}
			base := t.TempDir()
			remote := testutil.SetupBareRemoteRepo(t, base)
			testutil.SeedRemoteBranch(t, base, remote, "main", map[string]string{"safe/SKILL.md": "# Safe\n"})
			external := filepath.Join(sb.Root, "external")
			testutil.RunGit(t, "", "clone", remote, filepath.Join(external, "other", "_repo"))
			unreadable := filepath.Join(external, "sub")
			sb.WriteFile(filepath.Join(unreadable, "hidden", "SKILL.md"), "# Hidden\n")
			if err := os.Symlink(external, filepath.Join(source, "group")); err != nil {
				t.Fatal(err)
			}
			sb.WriteFile(filepath.Join(source, ".skillfollow"), "group\n")
			if err := os.Chmod(unreadable, 0000); err != nil {
				t.Fatal(err)
			}
			defer os.Chmod(unreadable, 0755)
			for _, args := range [][]string{
				{"update", "--group", "group", "--dry-run"},
				{"update", "group", "--dry-run"},
				{"check", "--group", "group"},
				{"check", "group"},
			} {
				result := run(args...)
				result.AssertFailure(t)
				result.AssertAnyOutputContains(t, "incomplete discovery of group")
				result.AssertAnyOutputContains(t, filepath.Join("group", "sub"))
			}
			result := run("update", "--group", "group", "--dry-run", "--json")
			result.AssertFailure(t)
			result.AssertOutputNotContains(t, "_repo")
		})
	}
}

// A tracked repo nested in a followed group applies its own .skillignore, so
// sync links only the skills that repo keeps.
func TestSkillfollowGroupNestedRepoSkillignore(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	target := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + "\nmode: merge\ntargets:\n  claude:\n    path: " + target + "\n")
	external := filepath.Join(sb.Root, "external")
	repo := filepath.Join(external, "sub", "_repo")
	for _, skill := range []string{"keep", "drop"} {
		sb.WriteFile(filepath.Join(repo, skill, "SKILL.md"), "---\nname: "+skill+"\n---\n# "+skill+"\n")
	}
	sb.WriteFile(filepath.Join(repo, ".skillignore"), "drop\n")
	testutil.RunGit(t, "", "init", repo)
	if err := os.Symlink(external, filepath.Join(sb.SourcePath, "group")); err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(filepath.Join(sb.SourcePath, ".skillfollow"), "group\n")

	sb.RunCLI("sync", "-g").AssertSuccess(t)
	if _, err := os.Lstat(filepath.Join(target, "group__sub___repo__keep")); err != nil {
		t.Fatalf("kept skill not linked: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(target, "group__sub___repo__drop")); !os.IsNotExist(err) {
		t.Fatalf("ignored skill linked: %v", err)
	}
}

// A skill in a tracked repo below the first path component, whether installed
// there with --into or reached through a followed group, belongs to that repo:
// list and status report the repo, and its target override decides the sync.
func TestSkillfollowGroupNestedRepoOwnership(t *testing.T) {
	for _, followed := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "followed"}[followed], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			claude, cursor := sb.CreateTarget("claude"), sb.CreateTarget("cursor")
			sb.WriteConfig("source: " + sb.SourcePath + "\nmode: merge\ntargets:\n  claude:\n    path: " + claude + "\n  cursor:\n    path: " + cursor + "\n")
			group := filepath.Join(sb.SourcePath, "group")
			if followed {
				group = filepath.Join(sb.Root, "external")
			}
			repo := filepath.Join(group, "sub", "_repo")
			sb.WriteFile(filepath.Join(repo, "a", "SKILL.md"), "---\nname: a\n---\n# a\n")
			testutil.RunGit(t, "", "init", repo)
			if followed {
				if err := os.Symlink(group, filepath.Join(sb.SourcePath, "group")); err != nil {
					t.Fatal(err)
				}
				sb.WriteFile(filepath.Join(sb.SourcePath, ".skillfollow"), "group\n")
			}
			store := install.NewMetadataStore()
			store.SetTargetOverride("group/sub/_repo/a", []string{"cursor"})
			if err := store.Save(sb.SourcePath); err != nil {
				t.Fatal(err)
			}

			list := sb.RunCLI("list", "-g", "--json")
			list.AssertSuccess(t)
			var skills []struct {
				RelPath  string `json:"relPath"`
				RepoName string `json:"repoName"`
			}
			if err := json.Unmarshal([]byte(list.Stdout), &skills); err != nil {
				t.Fatalf("invalid list JSON: %v\n%s", err, list.Stdout)
			}
			if len(skills) != 1 || skills[0].RepoName != "group/sub/_repo" {
				t.Errorf("list: want repoName group/sub/_repo, got %+v", skills)
			}

			status := sb.RunCLI("status", "-g", "--json")
			status.AssertSuccess(t)
			var st struct {
				TrackedRepos []struct {
					Name       string `json:"name"`
					SkillCount int    `json:"skill_count"`
				} `json:"tracked_repos"`
			}
			if err := json.Unmarshal([]byte(status.Stdout), &st); err != nil {
				t.Fatalf("invalid status JSON: %v\n%s", err, status.Stdout)
			}
			if len(st.TrackedRepos) != 1 || st.TrackedRepos[0].Name != "group/sub/_repo" || st.TrackedRepos[0].SkillCount != 1 {
				t.Errorf("status: want group/sub/_repo with 1 skill, got %+v", st.TrackedRepos)
			}

			sb.RunCLI("sync", "-g").AssertSuccess(t)
			if _, err := os.Lstat(filepath.Join(cursor, "group__sub___repo__a")); err != nil {
				t.Errorf("override target cursor not linked: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(claude, "group__sub___repo__a")); !os.IsNotExist(err) {
				t.Errorf("skill linked to claude despite override: %v", err)
			}
		})
	}
}
