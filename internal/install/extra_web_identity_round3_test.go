package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallRemoteExtra_DeletedSlashURLRefKeepsOverriddenDirectory(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, sparse := range []bool{false, true} {
			for _, phase := range []string{"reinstall", "missing", "marker", "live", "metadata"} {
				t.Run(fmt.Sprintf("project=%t/sparse=%t/%s", project, sparse, phase), func(t *testing.T) {
					req := newRemoteExtraRequest(t)
					oldSparse := sparseCheckoutSupported
					sparseCheckoutSupported = func() bool { return sparse }
					t.Cleanup(func() { sparseCheckoutSupported = oldSparse })
					if project {
						req.ProjectRoot = t.TempDir()
						req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
						req.ExtrasRoot = filepath.Join(req.LockDir, "extras")
					}
					remote := req.RawSource
					runGit(t, strings.TrimPrefix(remote, "file://"), "branch", "-D", "feature")
					seed := filepath.Join(t.TempDir(), "seed")
					runGit(t, "", "clone", "--branch", "main", remote, seed)
					runGit(t, seed, "config", "user.name", "Fixture")
					runGit(t, seed, "config", "user.email", "fixture@example.invalid")
					for path, body := range map[string]string{"rules/rule.md": "pinned correct", "x/rules/rule.md": "WRONG competing directory"} {
						file := filepath.Join(seed, path)
						if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(file, []byte(body), 0644); err != nil {
							t.Fatal(err)
						}
					}
					runGit(t, seed, "add", ".")
					runGit(t, seed, "commit", "-m", "resolved directory identity fixture")
					runGit(t, seed, "push", "origin", "HEAD:main", "HEAD:feature/x")
					want, err := getGitFullHash(seed)
					if err != nil {
						t.Fatal(err)
					}
					gitConfig := filepath.Join(t.TempDir(), "gitconfig")
					runGit(t, "", "config", "--file", gitConfig, "url."+remote+".insteadOf", "https://github.com/example-org/ai-hub.git")
					t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
					t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
					t.Setenv("GIT_ALLOW_PROTOCOL", "file")
					req.RawSource = "https://github.com/example-org/ai-hub/tree/feature/x/rules"
					req.Source, err = ParseSource(req.RawSource)
					if err != nil {
						t.Fatal(err)
					}
					req.Branch = "main" // unrelated explicit override; URL still selects rules/.
					branch := "main"
					req.PinBranch = func() string { return branch }
					req.WriteConfig = func() error { branch = req.Source.Branch; return nil }
					req.RepairConfig = func(remote, repaired string) error {
						if remote != req.RawSource {
							return fmt.Errorf("repair changed origin identity")
						}
						branch = repaired
						return nil
					}
					repair := phase != "reinstall" && phase != "missing"
					oldBoundary := extraBoundary
					t.Cleanup(func() { extraBoundary = oldBoundary })
					if repair {
						extraBoundary = func(step string) error {
							if step == phase {
								return errSimulatedDeath
							}
							return nil
						}
					}
					_, err = InstallRemoteExtra(req)
					extraBoundary = oldBoundary
					if repair && !errors.Is(err, errSimulatedDeath) || !repair && err != nil {
						t.Fatalf("initial publication: %v", err)
					}
					// The pin stays reachable, but neither the URL ref nor current main
					// can supply the original directory selection/content by accident.
					if err := os.WriteFile(filepath.Join(seed, "rules", "rule.md"), []byte("new default content"), 0644); err != nil {
						t.Fatal(err)
					}
					runGit(t, seed, "commit", "-am", "advance default after pin")
					runGit(t, seed, "push", "origin", "main", ":feature/x")
					if phase == "missing" {
						if err := os.RemoveAll(filepath.Join(req.ExtrasRoot, req.Name)); err != nil {
							t.Fatal(err)
						}
					}
					req.Source, err = ParseSource(req.RawSource)
					if err != nil {
						t.Fatal(err)
					}
					req.Branch = ""
					if repair {
						req.Branch = "fixture-missing-after-repair"
					}
					result, err := InstallRemoteExtra(req)
					if repair {
						if err == nil || !strings.Contains(err.Error(), req.Branch) {
							t.Fatalf("expected only subsequent fetch refusal: %v", err)
						}
					} else if err != nil || result.Commit != want || result.Branch != "main" {
						t.Fatalf("pinned reinstall: %+v %v", result, err)
					}
					if data, err := os.ReadFile(filepath.Join(req.ExtrasRoot, req.Name, "rule.md")); err != nil || string(data) != "pinned correct" {
						t.Errorf("resolved directory changed: %q %v", data, err)
					}
					store, _, err := ReadMetadata(req.ExtrasRoot)
					if err != nil {
						t.Fatal(err)
					}
					entry := store.GetExact(req.Name)
					if entry == nil || entry.Source != req.RawSource || entry.Subdir != "rules" || entry.Commit != want || entry.Branch != "main" || branch != "main" {
						t.Errorf("resolved provenance: %+v configBranch=%s want=%s", entry, branch, want)
					}
					if project {
						lock, err := LoadLock(req.LockDir)
						if err != nil || lock.ExtraCommitFor(req.Name, req.RawSource) != want {
							t.Errorf("pin identity changed: %+v %v", lock, err)
						}
					}
				})
			}
		}
	}
}
