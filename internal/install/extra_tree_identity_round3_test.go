package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtraCandidate_GitTreeIdentityRootAndContainedLinks(t *testing.T) {
	for _, sparse := range []bool{false, true} {
		for _, subdir := range []string{"", "rules"} {
			t.Run(fmt.Sprintf("sparse=%t/subdir=%s", sparse, subdir), func(t *testing.T) {
				req := newRemoteExtraRequest(t)
				oldSparse := sparseCheckoutSupported
				sparseCheckoutSupported = func() bool { return sparse }
				t.Cleanup(func() { sparseCheckoutSupported = oldSparse })
				seed := filepath.Join(t.TempDir(), "seed")
				runGit(t, "", "clone", "--branch", "main", req.RawSource, seed)
				if err := os.MkdirAll(filepath.Join(seed, "rules"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(seed, "rules", "target.md"), []byte("contained target"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("target.md", filepath.Join(seed, "rules", "link.md")); err != nil {
					t.Fatal(err)
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
				if candidate.treeHash != want {
					t.Errorf("extra lost complete selected Git tree: got %q want %q", candidate.treeHash, want)
				}
				link := filepath.Join(candidate.staged, "link.md")
				if subdir == "" {
					link = filepath.Join(candidate.staged, "rules", "link.md")
				}
				info, err := os.Lstat(link)
				if err != nil || !info.Mode().IsRegular() {
					t.Fatalf("contained link not materialized: %v %v", info, err)
				}
				if data, err := os.ReadFile(link); err != nil || string(data) != "contained target" {
					t.Errorf("contained target changed: %q %v", data, err)
				}
				if got := getSubdirTreeHash(seed, subdir); got != "" {
					t.Errorf("skill tree helper changed its root/link policy: %q", got)
				}
				if _, err := os.Lstat(req.ExtrasRoot); !os.IsNotExist(err) {
					t.Errorf("private candidate published extras root: %v", err)
				}
			})
		}
	}
}

// Use the actual existing pin producer; variants below alter only supported
// legacy/ambiguous pin evidence, never its source or commit admission.
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
