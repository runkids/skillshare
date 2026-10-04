package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestServerFollowedStagingGuards(t *testing.T) {
	for _, route := range []string{"commit", "push"} {
		for _, dryRun := range []bool{false, true} {
			t.Run(route+map[bool]string{false: "/apply", true: "/dry-run"}[dryRun], func(t *testing.T) {
				s, source := newTestServer(t)
				testutil.RunGit(t, source, "init")
				testutil.RunGit(t, source, "remote", "add", "origin", filepath.Join(t.TempDir(), "remote.git"))
				if err := os.Symlink(t.TempDir(), filepath.Join(source, "_dev")); err != nil {
					t.Skip(err)
				}
				if err := os.WriteFile(filepath.Join(source, ".skillfollow"), []byte("_dev\n"), 0644); err != nil {
					t.Fatal(err)
				}
				body := `{}`
				if dryRun {
					body = `{"dryRun":true}`
				}
				req := httptest.NewRequest("POST", "/api/git/"+route, strings.NewReader(body))
				rec := httptest.NewRecorder()
				if route == "commit" {
					s.handleGitCommit(rec, req)
				} else {
					s.handlePush(rec, req)
				}
				if rec.Code != 400 || !strings.Contains(rec.Body.String(), "not-ignored") {
					t.Fatalf("missing staging refusal: %d %s", rec.Code, rec.Body.String())
				}
				if got := testutil.RunGit(t, source, "ls-files"); got != "" {
					t.Fatalf("staged despite refusal: %s", got)
				}
			})
		}
	}
}

func TestServerSourceMutationRefusesLinks(t *testing.T) {
	for _, route := range []string{"pull", "checkout"} {
		for _, condition := range []string{"indexed", "declared", "undeclared"} {
			t.Run(route+"/"+condition, func(t *testing.T) {
				s, source := newTestServer(t)
				base := t.TempDir()
				remote := testutil.SetupBareRemoteRepo(t, base)
				files := map[string]string{"README.md": "# Source\n", ".gitignore": "/_dev\n"}
				if condition != "undeclared" {
					files[".skillfollow"] = "_dev\n"
				}
				testutil.SeedRemoteBranch(t, base, remote, "main", files)
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
				file := filepath.Join(seed, rel)
				if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte("# Incoming"), 0644); err != nil {
					t.Fatal(err)
				}
				testutil.RunGit(t, seed, "add", "-f", "--", rel)
				testutil.RunGit(t, seed, "commit", "-m", "incoming")
				testutil.RunGit(t, seed, "push", "origin", "HEAD:main", "HEAD:incoming")
				body := `{}`
				if route == "checkout" {
					body = `{"branch":"incoming"}`
				}
				req := httptest.NewRequest("POST", "/api/git/"+route, strings.NewReader(body))
				rec := httptest.NewRecorder()
				if route == "pull" {
					s.handlePull(rec, req)
				} else {
					s.handleGitCheckout(rec, req)
				}
				if rec.Code < 400 || !strings.Contains(rec.Body.String(), "refusing incoming commit") {
					t.Fatalf("missing guard refusal: %d %s", rec.Code, rec.Body.String())
				}
				if testutil.RunGit(t, source, "rev-parse", "HEAD") != before {
					t.Fatal("source HEAD changed")
				}
				if got, err := os.Readlink(filepath.Join(source, "_dev")); err != nil || got != target {
					t.Fatalf("link changed: %s %v", got, err)
				}
			})
		}
	}
}
