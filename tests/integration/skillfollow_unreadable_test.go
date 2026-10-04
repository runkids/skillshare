//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestSkillfollowSyncUnreadableDeclarationPreservesTargets(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix permission enforcement")
	}
	for _, file := range []string{".skillfollow", ".skillfollow.local"} {
		for _, mode := range []string{"merge", "copy"} {
			t.Run(file+"/"+mode, func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				target := sb.CreateTarget("claude")
				sb.WriteConfig("source: " + sb.SourcePath + "\nmode: " + mode + "\ntargets:\n  claude:\n    path: " + target + "\n")
				external := filepath.Join(sb.Root, "external")
				sb.WriteFile(filepath.Join(external, "a", "SKILL.md"), "# Preserved\n")
				if err := os.Symlink(external, filepath.Join(sb.SourcePath, "group")); err != nil {
					t.Fatal(err)
				}
				declaration := filepath.Join(sb.SourcePath, file)
				sb.WriteFile(declaration, "group\n")
				sb.RunCLI("sync", "-g").AssertSuccess(t)
				managed := filepath.Join(target, "group__a")
				if _, err := os.Lstat(managed); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(declaration, 0000); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(declaration, 0600)
				result := sb.RunCLI("sync", "--force", "-g")
				if _, err := os.Lstat(managed); err != nil {
					t.Fatalf("managed target removed after declaration read failure: %v", err)
				}
				result.AssertFailure(t)
				result.AssertAnyOutputContains(t, file)
				result.AssertAnyOutputContains(t, "permission denied")
				if data, err := os.ReadFile(filepath.Join(managed, "SKILL.md")); err != nil || string(data) != "# Preserved\n" {
					t.Fatalf("managed content changed: %q, %v", data, err)
				}
			})
		}
	}
}

// check and status must report incomplete discovery, not an empty source,
// when a declaration exists but cannot be read.
func TestSkillfollowCheckUnreadableDeclarationFails(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix permission enforcement")
	}
	commands := [][]string{{"check", "--json"}, {"status"}, {"status", "--json"}}
	for _, project := range []bool{false, true} {
		for _, command := range commands {
			t.Run(map[bool]string{false: "global", true: "project"}[project]+"/"+strings.Join(command, " "), func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				source, projectRoot := sb.SourcePath, ""
				if project {
					projectRoot = sb.SetupProjectDir("claude")
					source = filepath.Join(projectRoot, ".skillshare", "skills")
				} else {
					sb.WriteConfig("source: " + source + "\ntargets: {}\n")
				}
				external := filepath.Join(sb.Root, "external")
				sb.WriteFile(filepath.Join(external, "a", "SKILL.md"), "# A\n")
				if err := os.Symlink(external, filepath.Join(source, "group")); err != nil {
					t.Fatal(err)
				}
				declaration := filepath.Join(source, ".skillfollow")
				sb.WriteFile(declaration, "group\n")
				if err := os.Chmod(declaration, 0000); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(declaration, 0600)
				var result *testutil.Result
				if project {
					result = sb.RunCLIInDir(projectRoot, append(command, "-p")...)
				} else {
					result = sb.RunCLI(append(command, "-g")...)
				}
				result.AssertFailure(t)
				result.AssertAnyOutputContains(t, ".skillfollow")
				result.AssertAnyOutputContains(t, "permission denied")
				if strings.Contains(result.Stdout, "skill_count") || strings.Contains(result.Stdout, "0 entries") {
					t.Fatalf("incomplete discovery reported as a count: %s", result.Stdout)
				}
			})
		}
	}
}

// An unreadable declaration hides which repositories are followed, so updates
// must refuse instead of treating a followed checkout as an installed repo.
func TestSkillfollowUpdateUnreadableDeclarationRefuses(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix permission enforcement")
	}
	routes := map[string]func(remote string) []string{
		"update --force": func(string) []string { return []string{"update", "_dev", "--force", "--skip-audit", "-g"} },
		"install --update": func(remote string) []string {
			return []string{"install", "file://" + remote, "--track", "--name", "dev", "--update", "--skip-audit", "-g"}
		},
	}
	for name, args := range routes {
		t.Run(name, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			base := t.TempDir()
			remote := testutil.SetupBareRemoteRepo(t, base)
			testutil.SeedRemoteBranch(t, base, remote, "main", map[string]string{"safe/SKILL.md": "# Safe\n"})
			external := filepath.Join(sb.Root, "external")
			testutil.RunGit(t, "", "clone", remote, external)
			if err := os.Symlink(external, filepath.Join(sb.SourcePath, "_dev")); err != nil {
				t.Fatal(err)
			}
			seed := filepath.Join(base, "seed-main")
			sb.WriteFile(filepath.Join(seed, "remote.txt"), "new\n")
			testutil.RunGit(t, seed, "add", ".")
			testutil.RunGit(t, seed, "commit", "-m", "remote")
			testutil.RunGit(t, seed, "push", "origin", "HEAD:main")
			before := testutil.RunGit(t, external, "rev-parse", "HEAD")
			declaration := filepath.Join(sb.SourcePath, ".skillfollow")
			sb.WriteFile(declaration, "_dev\n")
			if err := os.Chmod(declaration, 0000); err != nil {
				t.Fatal(err)
			}
			defer os.Chmod(declaration, 0600)
			result := sb.RunCLI(args(remote)...)
			result.AssertFailure(t)
			result.AssertAnyOutputContains(t, "read skillfollow declaration")
			if testutil.RunGit(t, external, "rev-parse", "HEAD") != before {
				t.Fatal("update moved the followed checkout")
			}
		})
	}
}
