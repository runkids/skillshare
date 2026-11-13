package install

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestFetchGitSubdir_GitOnlyRefsAndWholeRepo(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		url, first, feature := setupPinRemote(t)
		for _, sparse := range []bool{false, true} {
			for _, tc := range []struct{ name, branch, pin, want string }{{"tag", "v1", "", first}, {"branch", "feature", "", feature}, {"pin", "", first, first}} {
				t.Run(tc.name+map[bool]string{false: "/clone", true: "/sparse-enabled-root-clone"}[sparse], func(t *testing.T) {
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
	})
	t.Run("TestFetchGitSubdir_GitOnlyExactAndUnavailable", func(t *testing.T) {
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
	})
	t.Run("TestFetchGitSubdir_ExactDirectoryBothPaths", func(t *testing.T) {
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
				if sparse {
					flag, err := gitOutput(got.RepoPath, "config", "--bool", "core.sparseCheckout")
					if err != nil || flag != "true" {
						t.Fatalf("sparse row fell back to clone: %q %v", flag, err)
					}
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
	})
	t.Run("TestFetchGitSubdir_SparseTagAndBranch", func(t *testing.T) {
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
		for _, tc := range []struct{ name, ref, pin, sha string }{{"tag", "v1", "", tagSHA}, {"branch", "feature", "", branchSHA}, {"full-sha", "", tagSHA, tagSHA}} {
			t.Run(tc.name, func(t *testing.T) {
				old := sparseCheckoutSupported
				sparseCheckoutSupported = func() bool { return true }
				defer func() { sparseCheckoutSupported = old }()
				src := &Source{CloneURL: "file://" + filepath.ToSlash(repo), Subdir: "rules", Branch: tc.ref, Commit: tc.pin}
				got, err := fetchGitSubdir(src, filepath.Join(t.TempDir(), "repo"), &InstallResult{}, InstallOptions{}, true)
				if err != nil {
					t.Fatal(err)
				}
				flag, routeErr := gitOutput(got.RepoPath, "config", "--bool", "core.sparseCheckout")
				if routeErr != nil || flag != "true" {
					t.Fatalf("ref sparse row fell back to clone: %q %v", flag, routeErr)
				}
				if got.Commit != tc.sha {
					t.Fatalf("got %s, want %s", got.Commit, tc.sha)
				}
			})
		}
	})
}

func TestFetchGitSubdir_GitOnlyRejectsSubmodule(t *testing.T) {
	url, _ := newHubWithSubmodule(t)
	src := hubSource(url, "vendor/up/skills/a")
	_, err := fetchGitSubdir(src, filepath.Join(t.TempDir(), "repo"), &InstallResult{}, InstallOptions{}, true)
	if err == nil || !strings.Contains(err.Error(), "submodule") {
		t.Fatalf("got %v", err)
	}
	for _, sparse := range []bool{false, true} {
		for _, selected := range []string{"", "vendor", "vendor/up", "vendor/up/skills/a", "own"} {
			t.Run(map[bool]string{false: "clone/", true: "sparse-enabled/"}[sparse]+map[string]string{"": "whole", "vendor": "descendant", "vendor/up": "gitlink", "vendor/up/skills/a": "within", "own": "outside-control"}[selected], func(t *testing.T) {
				old := sparseCheckoutSupported
				sparseCheckoutSupported = func() bool { return sparse }
				defer func() { sparseCheckoutSupported = old }()
				root := t.TempDir()
				candidate, cleanup, err := prepareExtraCandidate(hubSource(url, selected), root, filepath.Join(root, "rules"), InstallOptions{SkipAudit: true})
				if cleanup != nil {
					defer cleanup()
				}
				if selected != "own" {
					if err == nil || !strings.Contains(err.Error(), "submodule") {
						t.Errorf("selection %q published an incomplete candidate: %+v %v", selected, candidate, err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if data, err := os.ReadFile(filepath.Join(candidate.staged, "SKILL.md")); err != nil || string(data) != "# own" {
					t.Fatalf("unrelated outside gitlink affected selection: %q %v", data, err)
				}
				flag, err := gitOutput(candidate.fetch.RepoPath, "config", "--bool", "core.sparseCheckout")
				if sparse && (err != nil || flag != "true") || !sparse && flag == "true" {
					t.Fatalf("wrong acquisition route: sparse=%t flag=%q err=%v", sparse, flag, err)
				}
			})
		}
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
			if sparse {
				flag, err := gitOutput(got.RepoPath, "config", "--bool", "core.sparseCheckout")
				if err != nil || flag != "true" {
					t.Fatalf("mode sparse row fell back to clone: %q %v", flag, err)
				}
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

// Historical markers may name a gitlink-bearing revision accepted before this
// validation existed. Repair must validate that exact candidate as fresh install does.
func TestInstallRemoteExtra_DescendantGitlinkLifecycle(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, phase := range []string{"install", "update", "reinstall", "repair"} {
			t.Run(map[bool]string{false: "global/", true: "project/"}[project]+phase, func(t *testing.T) {
				req := newRemoteExtraRequest(t)
				t.Setenv("GIT_ALLOW_PROTOCOL", "file")
				if project {
					req.ProjectRoot = t.TempDir()
					req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
					req.ExtrasRoot = filepath.Join(req.LockDir, "extras")
				}
				req.Branch = "main"
				req.PinBranch = func() string { return "main" }
				var initial *RemoteExtraResult
				var oldTree map[string]string
				var oldMetadata *MetadataStore
				snapshots := map[string]string{}
				if phase != "install" {
					var err error
					initial, err = InstallRemoteExtra(req)
					if err != nil {
						t.Fatal(err)
					}
					oldTree, err = fingerprintExtraTree(filepath.Join(req.ExtrasRoot, req.Name))
					if err != nil {
						t.Fatal(err)
					}
					oldMetadata, _, err = ReadMetadata(req.ExtrasRoot)
					if err != nil {
						t.Fatal(err)
					}
					paths := []string{req.ConfigPath}
					if phase != "reinstall" {
						paths = append(paths, filepath.Join(req.ExtrasRoot, MetadataFileName))
					}
					if project {
						paths = append(paths, filepath.Join(req.LockDir, LockFileName))
					}
					for _, path := range paths {
						data, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						snapshots[path] = string(data)
					}
				}
				seed := filepath.Join(t.TempDir(), "seed")
				runGit(t, "", "clone", "--branch", "main", req.RawSource, seed)
				runGit(t, seed, "-c", "protocol.file.allow=always", "submodule", "add", req.RawSource, "nested/sub")
				if err := os.WriteFile(filepath.Join(seed, "SKILL.md"), []byte("rejected new revision"), 0644); err != nil {
					t.Fatal(err)
				}
				runGit(t, seed, "add", ".")
				runGit(t, seed, "commit", "-m", "add descendant gitlink")
				runGit(t, seed, "push", "origin", "main")
				badCommit, err := getGitFullHash(seed)
				if err != nil {
					t.Fatal(err)
				}
				var marker []byte
				if phase == "repair" {
					// Recreate the exact valid marker the pre-fix publisher could write,
					// using a real commit and the actual safe predecessor fingerprint.
					subdir := ""
					j := extraJournal{Version: 1, Name: req.Name, Remote: req.RawSource, Branch: "main", Commit: badCommit, Subdir: &subdir, HadPrevious: true, Old: oldTree}
					if err := writeExtraJournal(req.ExtrasRoot, j); err != nil {
						t.Fatal(err)
					}
					marker, err = os.ReadFile(extraJournalPath(req.ExtrasRoot, req.Name))
					if err != nil {
						t.Fatal(err)
					}
				}
				req.Opts.Update = phase != "reinstall"
				result, err := InstallRemoteExtra(req)
				if phase == "reinstall" {
					if err != nil || result.Commit != initial.Commit {
						t.Fatalf("safe pinned reinstall changed revision: %+v %v", result, err)
					}
					after, _, err := ReadMetadata(req.ExtrasRoot)
					if err != nil || after.Get(req.Name) == nil {
						t.Fatalf("safe pinned reinstall lost metadata: %v", err)
					}
					// A successful reinstall refreshes only the installation timestamp.
					after.Get(req.Name).InstalledAt = oldMetadata.Get(req.Name).InstalledAt
					if !reflect.DeepEqual(after, oldMetadata) {
						t.Errorf("safe pinned reinstall changed provenance, hashes or acceptance: %+v", after)
					}
				} else if err == nil || !strings.Contains(err.Error(), "submodule") {
					t.Errorf("%s accepted incomplete revision: %+v %v", phase, result, err)
				}
				state := InspectRemoteExtra(req.ExtrasRoot, req.Name)
				if phase == "install" {
					if state.Err != nil || !state.Missing || state.State() == "intact" {
						t.Errorf("refused fresh candidate reported intact: %+v", state)
					}
					if _, err := os.Stat(filepath.Join(req.ExtrasRoot, MetadataFileName)); !os.IsNotExist(err) {
						t.Errorf("refused fresh candidate published provenance: %v", err)
					}
					return
				}
				for path, before := range snapshots {
					if after, err := os.ReadFile(path); err != nil || string(after) != before {
						t.Errorf("%s changed prior lifecycle record %s: %v", phase, path, err)
					}
				}
				if after, err := fingerprintExtraTree(filepath.Join(req.ExtrasRoot, req.Name)); err != nil || !sameExtraFingerprint(oldTree, after) {
					t.Errorf("%s changed safe prior content: %v", phase, err)
				}
				if phase == "repair" {
					if after, err := os.ReadFile(extraJournalPath(req.ExtrasRoot, req.Name)); err != nil || string(after) != string(marker) || !state.Interrupted {
						t.Errorf("refused repair lost its marker: %v %+v", err, state)
					}
				} else if state.State() != "intact" || state.Commit != initial.Commit {
					t.Errorf("safe prior revision no longer intact: %+v", state)
				}
			})
		}
	}
}
