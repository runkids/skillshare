package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExtraCandidate_GitTreeIdentityRootAndContainedLinks(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		for _, sparse := range []bool{false, true} {
			for _, subdir := range []string{"", "rules"} {
				for _, linked := range []bool{false, true} {
					t.Run(fmt.Sprintf("sparse-enabled=%t/subdir=%s/linked=%t", sparse, subdir, linked), func(t *testing.T) {
						if linked && runtime.GOOS == "windows" {
							t.Skip("link fixture requires symlink privilege")
						}
						req := newRemoteExtraRequest(t)
						oldSparse := sparseCheckoutSupported
						sparseCheckoutSupported = func() bool { return sparse }
						t.Cleanup(func() { sparseCheckoutSupported = oldSparse })
						seed := filepath.Join(t.TempDir(), "seed")
						runGit(t, "", "clone", "--branch", "main", req.RawSource, seed)
						runGit(t, seed, "config", "user.name", "Fixture")
						runGit(t, seed, "config", "user.email", "fixture@example.invalid")
						if err := os.MkdirAll(filepath.Join(seed, "rules"), 0755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(seed, "rules", "target.md"), []byte("contained target"), 0644); err != nil {
							t.Fatal(err)
						}
						if linked {
							if err := os.Symlink("target.md", filepath.Join(seed, "rules", "link.md")); err != nil {
								t.Fatal(err)
							}
						}
						runGit(t, seed, "add", ".")
						runGit(t, seed, "commit", "-m", "complete extra tree identity fixture")
						runGit(t, seed, "push", "origin", "HEAD:main")
						commit, err := getGitFullHash(seed)
						if err != nil {
							t.Fatal(err)
						}
						expr := "HEAD^{tree}"
						if subdir != "" {
							expr = "HEAD:" + subdir
						}
						want, err := gitOutput(seed, "rev-parse", expr)
						if err != nil || len(want) != 40 {
							t.Fatalf("Git object fixture: %q %v", want, err)
						}
						source := *req.Source
						source.Subdir, source.Commit = subdir, commit
						candidate, cleanup, err := prepareExtraCandidate(&source, req.ExtrasRoot, filepath.Join(req.ExtrasRoot, req.Name), req.Opts)
						if err != nil {
							t.Fatal(err)
						}
						defer cleanup()
						if sparse && subdir != "" {
							flag, err := gitOutput(candidate.fetch.RepoPath, "config", "--bool", "core.sparseCheckout")
							if err != nil || flag != "true" {
								t.Fatalf("candidate sparse row fell back to clone: %q %v", flag, err)
							}
						}
						if candidate.treeHash != want {
							t.Errorf("extra lost complete selected Git tree: got %q want %q", candidate.treeHash, want)
						}
						name := "target.md"
						if linked {
							name = "link.md"
						}
						link := filepath.Join(candidate.staged, name)
						if subdir == "" {
							link = filepath.Join(candidate.staged, "rules", name)
						}
						info, err := os.Lstat(link)
						if err != nil || !info.Mode().IsRegular() {
							t.Fatalf("contained link not materialized: %v %v", info, err)
						}
						if data, err := os.ReadFile(link); err != nil || string(data) != "contained target" {
							t.Errorf("contained target changed: %q %v", data, err)
						}
						wantSkillHash := want
						if linked || subdir == "" {
							wantSkillHash = ""
						}
						if got := getSubdirTreeHash(seed, subdir); got != wantSkillHash {
							t.Errorf("skill tree helper changed its root/link policy: %q", got)
						}
						if _, err := os.Lstat(req.ExtrasRoot); !os.IsNotExist(err) {
							t.Errorf("private candidate published extras root: %v", err)
						}
					})
				}
			}
		}
	})
	t.Run("defensive selection evidence, not acquisition history", func(t *testing.T) {
		for _, identical := range []bool{false, true} {
			t.Run(fmt.Sprintf("identical=%t", identical), func(t *testing.T) {
				req, repo := pinnedWebTreeFixture(t, false, identical)
				lock, err := LoadLock(req.LockDir)
				if err != nil {
					t.Fatal(err)
				}
				pin := lock.Extras[req.Name]
				source, err := ParseSource(req.RawSource)
				if err != nil {
					t.Fatal(err)
				}
				outside, err := gitOutput(repo, "rev-parse", pin.Commit+":outside")
				if err != nil {
					t.Fatal(err)
				}
				paths := []string{req.ConfigPath, filepath.Join(req.LockDir, LockFileName), filepath.Join(req.ExtrasRoot, MetadataFileName), filepath.Join(req.LockDir, ".gitignore")}
				before := make([]extraFileSnapshot, len(paths))
				for i, path := range paths {
					before[i], err = snapshotExtraFile(path)
					if err != nil {
						t.Fatal(err)
					}
				}
				for _, tc := range []struct{ name, tree string }{{"zero", outside}, {"absent-legacy", ""}, {"invalid", "bad"}, {"selected", pin.TreeHash}} {
					t.Run(tc.name, func(t *testing.T) {
						got, err := pinnedExtraSubdir(gitFetch{RepoPath: repo, Commit: pin.Commit}, source.webRef, tc.tree)
						if tc.name == "selected" && !identical {
							if err != nil || got != "rules" {
								t.Fatalf("unique evidence: %q %v", got, err)
							}
						} else if err == nil || !strings.Contains(err.Error(), "selection") || !strings.Contains(err.Error(), "--update") {
							t.Fatalf("unsafe selection accepted: %q %v", got, err)
						}
					})
				}
				for i, path := range paths {
					after, err := snapshotExtraFile(path)
					if err != nil || before[i].exists != after.exists || string(before[i].data) != string(after.data) || before[i].mode != after.mode {
						t.Fatalf("defensive selection changed participant %s: %v", path, err)
					}
				}
			})
		}
	})
	t.Run("TestPrepareExtraCandidate_PinnedAndPrivate", func(t *testing.T) {
		remote, first, _ := setupPinRemote(t)
		source := &Source{Type: SourceTypeGitHTTPS, Raw: remote, CloneURL: remote, Commit: first}
		root := filepath.Join(t.TempDir(), "extras")
		candidate, cleanup, err := prepareExtraCandidate(source, root, filepath.Join(root, "rules"), InstallOptions{SkipAudit: true})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if candidate.fetch.Commit != first || !candidate.audit.Skipped {
			t.Fatalf("candidate: %+v", candidate)
		}
		contents, err := os.ReadFile(filepath.Join(candidate.staged, "SKILL.md"))
		if err != nil || string(contents) != "# v1" {
			t.Fatalf("staged: %q %v", contents, err)
		}
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatalf("publication before gate: %v", err)
		}
	})
}

// Use the real pin producer; invalid/absent evidence is defensive selection input,
// not a historical acquisition claim.
func pinnedWebTreeFixture(t *testing.T, sparse, identical bool) (RemoteExtraRequest, string) {
	t.Helper()
	req := newRemoteExtraRequest(t)
	oldSparse := sparseCheckoutSupported
	sparseCheckoutSupported = func() bool { return sparse }
	t.Cleanup(func() { sparseCheckoutSupported = oldSparse })
	req.ProjectRoot = t.TempDir()
	req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
	req.ExtrasRoot = filepath.Join(req.LockDir, "extras")
	remote := req.RawSource
	runGit(t, strings.TrimPrefix(remote, "file://"), "branch", "-D", "feature")
	seed := filepath.Join(t.TempDir(), "seed")
	runGit(t, "", "clone", "--branch", "main", remote, seed)
	runGit(t, seed, "config", "user.name", "Fixture")
	runGit(t, seed, "config", "user.email", "fixture@example.invalid")
	wrong := "WRONG competing directory"
	if identical {
		wrong = "pinned correct"
	}
	for path, body := range map[string]string{"rules/rule.md": "pinned correct", "x/rules/rule.md": wrong, "outside/other.md": "not a URL partition"} {
		file := filepath.Join(seed, path)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, seed, "add", ".")
	runGit(t, seed, "commit", "-m", "pinned URL tree partition fixture")
	runGit(t, seed, "push", "origin", "HEAD:main", "HEAD:feature/x")
	gitConfig := filepath.Join(t.TempDir(), "gitconfig")
	runGit(t, "", "config", "--file", gitConfig, "url."+remote+".insteadOf", "https://github.com/example-org/ai-hub.git")
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	req.RawSource = "https://github.com/example-org/ai-hub/tree/feature/x/rules"
	var err error
	req.Source, err = ParseSource(req.RawSource)
	if err != nil {
		t.Fatal(err)
	}
	req.Branch = "main"
	req.PinBranch = func() string { return "main" }
	if _, err := InstallRemoteExtra(req); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "push", "origin", ":feature/x")
	return req, seed
}
