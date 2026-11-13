package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallRemoteExtra_DeletedWebRefUsesRecordedPin(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, sparse := range []bool{false, true} {
			for _, ref := range []string{"feature", "feature/x"} {
				for _, phase := range []string{"reinstall", "missing", "repair"} {
					t.Run(fmt.Sprintf("project=%t/sparse=%t/ref=%s/%s", project, sparse, ref, phase), func(t *testing.T) {
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
						bare := strings.TrimPrefix(remote, "file://")
						if ref != "feature" {
							runGit(t, bare, "branch", "-m", "feature", ref)
						}
						seed := filepath.Join(t.TempDir(), "seed")
						runGit(t, "", "clone", "--branch", ref, remote, seed)
						runGit(t, seed, "config", "user.name", "Fixture")
						runGit(t, seed, "config", "user.email", "fixture@example.invalid")
						file := filepath.Join(seed, "rules", "rule.md")
						if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(file, []byte("pinned content"), 0644); err != nil {
							t.Fatal(err)
						}
						runGit(t, seed, "add", ".")
						runGit(t, seed, "commit", "-m", "ref path fixture")
						runGit(t, seed, "push", "origin", "HEAD:"+ref)
						want, err := getGitFullHash(seed)
						if err != nil {
							t.Fatal(err)
						}
						gitConfig := filepath.Join(t.TempDir(), "gitconfig")
						runGit(t, "", "config", "--file", gitConfig, "url."+remote+".insteadOf", "https://github.com/example-org/ai-hub.git")
						t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
						t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
						t.Setenv("GIT_ALLOW_PROTOCOL", "file")
						req.RawSource = "https://github.com/example-org/ai-hub/tree/" + ref + "/rules"
						req.Source, err = ParseSource(req.RawSource)
						if err != nil {
							t.Fatal(err)
						}
						branch := req.Source.Branch
						req.PinBranch = func() string { return branch }
						req.WriteConfig = func() error { branch = req.Source.Branch; return nil }
						req.RepairConfig = func(remote, repaired string) error {
							if remote != req.RawSource {
								return fmt.Errorf("journal identity changed")
							}
							branch = repaired
							return nil
						}
						oldBoundary := extraBoundary
						t.Cleanup(func() { extraBoundary = oldBoundary })
						if phase == "repair" {
							extraBoundary = func(step string) error {
								if step == "live" {
									return errSimulatedDeath
								}
								return nil
							}
						}
						_, err = InstallRemoteExtra(req)
						extraBoundary = oldBoundary
						if phase == "repair" {
							if !errors.Is(err, errSimulatedDeath) {
								t.Fatalf("death boundary not reached: %v", err)
							}
						} else if err != nil {
							t.Fatal(err)
						}
						// Keep the pin reachable through main, then remove its live ref.
						runGit(t, seed, "checkout", "-B", "main", "origin/main")
						runGit(t, seed, "merge", "--ff-only", ref)
						if err := os.WriteFile(file, []byte("new default content"), 0644); err != nil {
							t.Fatal(err)
						}
						runGit(t, seed, "add", ".")
						runGit(t, seed, "commit", "-m", "advance default")
						runGit(t, seed, "push", "origin", "main", ":"+ref)
						if phase == "missing" {
							if err := os.RemoveAll(filepath.Join(req.ExtrasRoot, req.Name)); err != nil {
								t.Fatal(err)
							}
							// Teammate restore has the project pin but no local metadata.
							if project {
								if err := os.Remove(filepath.Join(req.ExtrasRoot, MetadataFileName)); err != nil {
									t.Fatal(err)
								}
							}
						}
						// Every invocation reparses the original URL, as the CLI does.
						req.Source, err = ParseSource(req.RawSource)
						if err != nil {
							t.Fatal(err)
						}
						result, err := InstallRemoteExtra(req)
						if err != nil {
							t.Fatalf("reachable pin requires deleted live ref: %v", err)
						}
						if result.Commit != want || result.Branch != ref || branch != ref || result.Interrupted != (phase == "repair") {
							t.Errorf("recorded selection changed: %+v configBranch=%q want=%s", result, branch, want)
						}
						if data, err := os.ReadFile(filepath.Join(req.ExtrasRoot, req.Name, "rule.md")); err != nil || string(data) != "pinned content" {
							t.Errorf("wrong pinned directory: %q %v", data, err)
						}
					})
				}
			}
		}
	}
}
