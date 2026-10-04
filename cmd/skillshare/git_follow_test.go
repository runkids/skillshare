package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
	gitops "skillshare/internal/git"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/testutil"
	"skillshare/internal/ui"
)

func TestFollowedStagingCLISurfaces(t *testing.T) {
	for _, scope := range []string{"skills", "root"} {
		for _, route := range []string{"commit", "push", "stage", "init"} {
			t.Run(scope+"/"+route, func(t *testing.T) {
				xdg := t.TempDir()
				t.Setenv("XDG_CONFIG_HOME", xdg)
				base := filepath.Join(xdg, "skillshare")
				if err := os.MkdirAll(base, 0755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("SKILLSHARE_CONFIG", filepath.Join(base, "config.yaml"))
				source := filepath.Join(base, "skills")
				if err := os.Mkdir(source, 0755); err != nil {
					t.Fatal(err)
				}
				cfg := &config.Config{Source: source, GitRoot: scope}
				if err := cfg.Save(); err != nil {
					t.Fatal(err)
				}
				staging := cfg.EffectiveGitRoot()
				testutil.RunGit(t, staging, "init")
				testutil.ConfigureGitUser(t, staging)
				testutil.RunGit(t, staging, "remote", "add", "origin", filepath.Join(t.TempDir(), "remote.git"))
				if err := os.Symlink(t.TempDir(), filepath.Join(source, "_dev")); err != nil {
					t.Skip(err)
				}
				if err := os.WriteFile(filepath.Join(source, ".skillfollow"), []byte("_dev\n"), 0644); err != nil {
					t.Fatal(err)
				}
				follow := globalSkillFollowSet(cfg)
				var err error
				switch route {
				case "commit":
					err = cmdCommit([]string{"--dry-run"})
				case "push":
					err = cmdPush([]string{"--dry-run"})
				case "stage":
					spinner := ui.StartSpinner("test")
					err = stageAndCommit(staging, "test", spinner, follow)
					spinner.Stop()
				case "init":
					err = commitSourceFiles(staging, follow)
				}
				if err == nil || !strings.Contains(err.Error(), "not-ignored") {
					t.Fatalf("missing refusal: %v", err)
				}
				if got := testutil.RunGit(t, staging, "ls-files"); got != "" {
					t.Fatalf("guard staged files: %s", got)
				}
			})
		}
	}
}

func TestFollowedStagingUnreadableDeclaration(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix permission enforcement")
	}
	for _, route := range []string{"commit", "push", "stage", "init"} {
		t.Run(route, func(t *testing.T) {
			xdg := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", xdg)
			base := filepath.Join(xdg, "skillshare")
			source := filepath.Join(base, "skills")
			if err := os.MkdirAll(source, 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("SKILLSHARE_CONFIG", filepath.Join(base, "config.yaml"))
			cfg := &config.Config{Source: source}
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			testutil.RunGit(t, source, "init")
			testutil.ConfigureGitUser(t, source)
			testutil.RunGit(t, source, "remote", "add", "origin", filepath.Join(t.TempDir(), "remote.git"))
			if err := os.Symlink(t.TempDir(), filepath.Join(source, "_dev")); err != nil {
				t.Skip(err)
			}
			if err := os.WriteFile(filepath.Join(source, ".gitignore"), []byte("/.skillfollow.local\n"), 0644); err != nil {
				t.Fatal(err)
			}
			local := filepath.Join(source, ".skillfollow.local")
			if err := os.WriteFile(local, []byte("_dev\n"), 0000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(local, 0600) })
			follow := globalSkillFollowSet(cfg)
			var err error
			switch route {
			case "commit":
				err = cmdCommit([]string{"--dry-run"})
			case "push":
				err = cmdPush([]string{"--dry-run"})
			case "stage":
				spinner := ui.StartSpinner("test")
				err = stageAndCommit(source, "test", spinner, follow)
				spinner.Stop()
			case "init":
				err = commitSourceFiles(source, follow)
			}
			if err == nil || !strings.Contains(err.Error(), "read skillfollow declaration") {
				t.Fatalf("missing refusal: %v", err)
			}
			if got := testutil.RunGit(t, source, "ls-files"); got != "" {
				t.Fatalf("guard staged files: %s", got)
			}
		})
	}
}

func TestDoctorFollowedIgnoreDiagnostics(t *testing.T) {
	source := t.TempDir()
	testutil.RunGit(t, source, "init")
	if err := os.Symlink(t.TempDir(), filepath.Join(source, "_dev")); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".skillfollow.local"), []byte("_dev\n"), 0644); err != nil {
		t.Fatal(err)
	}
	follow := sourcewalk.Follow(source, sourcewalk.FollowOptions{})
	result := &doctorResult{}
	checkSkillfollow(result, &follow)
	var messages []string
	for _, check := range result.checks {
		messages = append(messages, check.Message)
	}
	joined := strings.Join(messages, "\n")
	if !strings.Contains(joined, "_dev: not-ignored") || !strings.Contains(joined, "/.skillfollow.local") || !strings.Contains(joined, filepath.Join(source, ".gitignore")) {
		t.Fatalf("missing exact ignore guidance: %s", joined)
	}
	if _, err := os.Stat(filepath.Join(source, ".gitignore")); !os.IsNotExist(err) {
		t.Fatal("doctor wrote .gitignore")
	}
	testutil.RunGit(t, source, "add", "--", ".skillfollow.local")
	result = &doctorResult{}
	checkSkillfollow(result, &follow)
	found := false
	for _, check := range result.checks {
		found = found || strings.Contains(check.Message, ".skillfollow.local: tracked")
	}
	if !found {
		t.Fatal("tracked local declaration was not reported")
	}
}

func TestSourceMutationCLIAndInitRefuse(t *testing.T) {
	for _, route := range []string{"pull", "init-reset", "init-setup"} {
		for _, condition := range []string{"indexed", "declared", "undeclared"} {
			t.Run(route+"/"+condition, func(t *testing.T) {
				base := t.TempDir()
				remote := testutil.SetupBareRemoteRepo(t, base)
				files := map[string]string{"README.md": "# Source\n", ".gitignore": "/_dev\n"}
				if condition != "undeclared" {
					files[".skillfollow"] = "_dev\n"
				}
				testutil.SeedRemoteBranch(t, base, remote, "main", files)
				source := filepath.Join(base, "source")
				testutil.RunGit(t, "", "clone", remote, source)
				testutil.ConfigureGitUser(t, source)
				target := t.TempDir()
				if err := os.Symlink(target, filepath.Join(source, "_dev")); err != nil {
					t.Skip(err)
				}
				if condition == "indexed" {
					testutil.RunGit(t, source, "add", "-f", "_dev")
					testutil.RunGit(t, source, "commit", "-m", "indexed")
				}
				before := testutil.RunGit(t, source, "rev-parse", "HEAD")
				seed := filepath.Join(base, "seed-main")
				rel := "_dev/SKILL.md"
				if condition == "indexed" {
					rel = "safe/SKILL.md"
				}
				path := filepath.Join(seed, rel)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("# Incoming"), 0644); err != nil {
					t.Fatal(err)
				}
				testutil.RunGit(t, seed, "add", "-f", "--", rel)
				testutil.RunGit(t, seed, "commit", "-m", "incoming")
				testutil.RunGit(t, seed, "push", "origin", "HEAD:main")
				cfg := &config.Config{Source: source}
				follow := globalSkillFollowSet(cfg)
				var err error
				switch route {
				case "pull":
					err = pullFromRemote(cfg, false, false)
				case "init-reset":
					if err = gitops.Fetch(source); err == nil {
						err = resetToRemoteBranch(source, "main", follow)
					}
				case "init-setup":
					if !tryPullAfterRemoteSetup(source, remote, follow) {
						t.Fatal("init did not report remote/refusal")
					}
				}
				if route != "init-setup" && (err == nil || !strings.Contains(err.Error(), "refusing incoming commit")) {
					t.Fatalf("missing guard refusal: %v", err)
				}
				if testutil.RunGit(t, source, "rev-parse", "HEAD") != before {
					t.Fatal("refusal changed source HEAD")
				}
				if got, err := os.Readlink(filepath.Join(source, "_dev")); err != nil || got != target {
					t.Fatalf("link changed: %s %v", got, err)
				}
			})
		}
	}
}
