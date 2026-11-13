package install

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFetchGitSubdir_GitOnlyRefsAndWholeRepo(t *testing.T) {
	url, first, feature := setupPinRemote(t)
	for _, sparse := range []bool{false, true} {
		for _, tc := range []struct{ name, branch, pin, want string }{{"tag", "v1", "", first}, {"branch", "feature", "", feature}, {"pin", "", first, first}} {
			t.Run(tc.name+map[bool]string{false: "/clone", true: "/sparse"}[sparse], func(t *testing.T) {
				old := sparseCheckoutSupported
				sparseCheckoutSupported = func() bool { return sparse }
				defer func() { sparseCheckoutSupported = old }()
				src := &Source{CloneURL: url, Branch: tc.branch, Commit: tc.pin}
				fetched, err := fetchGitSubdir(src, filepath.Join(t.TempDir(), "repo"), &InstallResult{}, InstallOptions{}, true)
				if err != nil {
					t.Fatal(err)
				}
				if fetched.Commit != tc.want || len(fetched.Commit) != 40 || fetched.SubdirPath != fetched.RepoPath {
					t.Fatalf("wrong fetch: %+v", fetched)
				}
				dst := filepath.Join(t.TempDir(), "stage")
				if err := stageFetchedTree(fetched.RepoPath, fetched.Subdir, dst); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
					t.Fatalf(".git present: %v", err)
				}
			})
		}
	}
}

func TestFetchGitSubdir_GitOnlyExactAndUnavailable(t *testing.T) {
	url, _, _ := setupPinRemote(t)
	for _, sparse := range []bool{false, true} {
		t.Run(map[bool]string{false: "clone", true: "sparse"}[sparse], func(t *testing.T) {
			old := sparseCheckoutSupported
			sparseCheckoutSupported = func() bool { return sparse }
			defer func() { sparseCheckoutSupported = old }()
			for _, tc := range []struct{ subdir, pin, want string }{{"missing", "", "missing"}, {"", strings.Repeat("a", 40), "locked commit"}} {
				src := &Source{CloneURL: url, Subdir: tc.subdir, Commit: tc.pin}
				_, err := fetchGitSubdir(src, filepath.Join(t.TempDir(), "repo"), &InstallResult{}, InstallOptions{}, true)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("got %v, want %s", err, tc.want)
				}
			}
		})
	}
}

func TestFetchGitSubdir_GitOnlyRejectsSubmodule(t *testing.T) {
	url, _ := newHubWithSubmodule(t)
	src := hubSource(url, "vendor/up/skills/a")
	_, err := fetchGitSubdir(src, filepath.Join(t.TempDir(), "repo"), &InstallResult{}, InstallOptions{}, true)
	if err == nil || !strings.Contains(err.Error(), "submodule") {
		t.Fatalf("got %v", err)
	}
}

func TestFetchGitSubdir_ExactDirectoryBothPaths(t *testing.T) {
	url, _ := newHubWithSubmodule(t)
	for _, sparse := range []bool{false, true} {
		t.Run(map[bool]string{false: "clone", true: "sparse"}[sparse], func(t *testing.T) {
			old := sparseCheckoutSupported
			sparseCheckoutSupported = func() bool { return sparse }
			defer func() { sparseCheckoutSupported = old }()
			src := hubSource(url, "own")
			got, err := fetchGitSubdir(src, filepath.Join(t.TempDir(), "repo"), &InstallResult{}, InstallOptions{}, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Commit) != 40 || got.Subdir != "own" || got.SubdirPath != filepath.Join(got.RepoPath, "own") {
				t.Fatalf("wrong fetch: %+v", got)
			}
			dst := filepath.Join(t.TempDir(), "stage")
			if err := stageFetchedTree(got.RepoPath, got.Subdir, dst); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type extraCountingTransport struct{ count int }

func (c *extraCountingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.count++
	return &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}, nil
}

func TestFetchGitSubdir_GitOnlyNeverCallsAPI(t *testing.T) {
	url, _ := newHubWithSubmodule(t)
	gitConfig := filepath.Join(t.TempDir(), "gitconfig")
	mustRunGit(t, "", "config", "--file", gitConfig, "url."+url+".insteadOf", "https://github.com/example-org/fixture.git")
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("SKILLSHARE_GIT_TOKEN", "")
	transport := &extraCountingTransport{}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = transport
	defer func() { http.DefaultTransport = originalTransport }()
	for _, sparse := range []bool{false, true} {
		old := sparseCheckoutSupported
		sparseCheckoutSupported = func() bool { return sparse }
		src := &Source{CloneURL: "https://github.com/example-org/fixture.git", Subdir: "own"}
		_, err := fetchGitSubdir(src, filepath.Join(t.TempDir(), "repo"), &InstallResult{}, InstallOptions{}, true)
		sparseCheckoutSupported = old
		if err != nil {
			t.Fatal(err)
		}
		if transport.count != 0 {
			t.Fatalf("Git-only fetch sent %d API requests", transport.count)
		}
	}
	// The skill caller still tries the API if sparse checkout is unavailable.
	old := sparseCheckoutSupported
	sparseCheckoutSupported = func() bool { return false }
	defer func() { sparseCheckoutSupported = old }()
	src := &Source{CloneURL: "https://github.com/example-org/fixture.git", Subdir: "own"}
	_, err := fetchGitSubdir(src, filepath.Join(t.TempDir(), "repo"), &InstallResult{}, InstallOptions{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if transport.count == 0 {
		t.Fatal("skill fetch did not attempt the API")
	}
}

func TestStageFetchedTree_ModesBothFetchPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	mustRunGit(t, "", "init", "-q", repo)
	mustRunGit(t, repo, "config", "user.email", "fixture@example.com")
	mustRunGit(t, repo, "config", "user.name", "Fixture")
	dir := filepath.Join(repo, "rules")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"run": "executable", "text": "#!/bin/sh\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(dir, "run"), 0755); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, repo, "add", "rules")
	mustRunGit(t, repo, "commit", "-q", "-m", "fixture")
	for _, sparse := range []bool{false, true} {
		t.Run(map[bool]string{false: "clone", true: "sparse"}[sparse], func(t *testing.T) {
			old := sparseCheckoutSupported
			sparseCheckoutSupported = func() bool { return sparse }
			defer func() { sparseCheckoutSupported = old }()
			src := &Source{CloneURL: "file://" + filepath.ToSlash(repo), Subdir: "rules"}
			got, err := fetchGitSubdir(src, filepath.Join(t.TempDir(), "repo"), &InstallResult{}, InstallOptions{}, true)
			if err != nil {
				t.Fatal(err)
			}
			dst := filepath.Join(t.TempDir(), "stage")
			if err := stageFetchedTree(got.RepoPath, got.Subdir, dst); err != nil {
				t.Fatal(err)
			}
			for name, executable := range map[string]bool{"run": true, "text": false} {
				info, err := os.Stat(filepath.Join(dst, name))
				if err != nil {
					t.Fatal(err)
				}
				if (info.Mode()&0111 != 0) != executable {
					t.Errorf("%s mode = %v", name, info.Mode())
				}
			}
		})
	}
}

func TestFetchGitSubdir_SparseTagAndBranch(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	mustRunGit(t, "", "init", "-q", repo)
	mustRunGit(t, repo, "config", "user.email", "fixture@example.com")
	mustRunGit(t, repo, "config", "user.name", "Fixture")
	if err := os.Mkdir(filepath.Join(repo, "rules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "rules", "rule"), []byte("tag"), 0644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, repo, "add", "rules")
	mustRunGit(t, repo, "commit", "-q", "-m", "tag")
	tagSHA, err := getGitFullHash(repo)
	if err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, repo, "tag", "v1")
	mustRunGit(t, repo, "checkout", "-q", "-b", "feature")
	if err := os.WriteFile(filepath.Join(repo, "rules", "rule"), []byte("feature"), 0644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, repo, "add", "rules")
	mustRunGit(t, repo, "commit", "-q", "-m", "feature")
	branchSHA, err := getGitFullHash(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ ref, sha string }{{"v1", tagSHA}, {"feature", branchSHA}} {
		t.Run(tc.ref, func(t *testing.T) {
			old := sparseCheckoutSupported
			sparseCheckoutSupported = func() bool { return true }
			defer func() { sparseCheckoutSupported = old }()
			src := &Source{CloneURL: "file://" + filepath.ToSlash(repo), Subdir: "rules", Branch: tc.ref}
			got, err := fetchGitSubdir(src, filepath.Join(t.TempDir(), "repo"), &InstallResult{}, InstallOptions{}, true)
			if err != nil {
				t.Fatal(err)
			}
			if got.Commit != tc.sha {
				t.Fatalf("got %s, want %s", got.Commit, tc.sha)
			}
		})
	}
}
