package install

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skillshare/internal/audit"
)

const outsideSecret = "PRIVATE KEY"

// linkedRepo commits a repository with the skill skills/demo, a file
// shared/ref.md, and a link at rel (relative to the repository) pointing at
// target. It returns the repository.
func linkedRepo(t *testing.T, rel, target string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := initTestRepo(t)
	for path, body := range map[string]string{
		"skills/demo/SKILL.md": "---\nname: demo\n---\n# Demo\n",
		"shared/ref.md":        "shared reference",
	} {
		full := filepath.Join(repo, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add skill with link")
	return repo
}

// outsideFile writes a file outside any repository, as a private key on the
// user's machine would be.
func outsideFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, []byte(outsideSecret), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// discoverLinked discovers the skills and agents in repo through a clone.
func discoverLinked(t *testing.T, repo string) *DiscoveryResult {
	t.Helper()
	source, err := ParseSource(fileURL(repo))
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := DiscoverFromGit(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { CleanupDiscovery(discovery) })
	return discovery
}

// assertRefused fails unless the install returned an error and wrote nothing
// at dest.
func assertRefused(t *testing.T, err error, dest string) {
	t.Helper()
	if _, statErr := os.Lstat(dest); err == nil || !os.IsNotExist(statErr) {
		t.Fatalf("err = %v, dest stat = %v; want an error and nothing written", err, statErr)
	}
}

func TestInstall_SubdirRefusesLinkOutsideRepository(t *testing.T) {
	repo := linkedRepo(t, "skills/demo/notes.md", outsideFile(t))
	source, err := ParseSource("file://" + repo + "//skills/demo")
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	dest := filepath.Join(sourceDir, "demo")

	_, err = Install(source, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true})

	assertRefused(t, err, dest)
}

func TestInstallFromDiscovery_CopiesLinkInsideRepository(t *testing.T) {
	discovery := discoverLinked(t, linkedRepo(t, "skills/demo/ref.md", filepath.Join("..", "..", "shared", "ref.md")))
	sourceDir := t.TempDir()
	dest := filepath.Join(sourceDir, "demo")

	if _, err := InstallFromDiscovery(discovery, discovery.Skills[0], dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}

	if got, _ := os.ReadFile(filepath.Join(dest, "ref.md")); string(got) != "shared reference" {
		t.Fatalf("ref.md = %q, want the shared file's content", got)
	}
}

func TestInstallFromDiscovery_RefusesLinkOutsideRepository(t *testing.T) {
	discovery := discoverLinked(t, linkedRepo(t, "skills/demo/notes.md", outsideFile(t)))
	dest := filepath.Join(t.TempDir(), "demo")

	_, err := InstallFromDiscovery(discovery, discovery.Skills[0], dest, InstallOptions{SourceDir: filepath.Dir(dest), SkipAudit: true})

	assertRefused(t, err, dest)
}

// A link into the clone's .git folder stays inside the clone but can expose
// its remote URL, which may carry a token. On a case-insensitive file system
// .GIT names the same folder.
func TestInstallFromDiscovery_RefusesLinkIntoGitFolder(t *testing.T) {
	for _, dir := range []string{".git", ".GIT"} {
		t.Run(dir, func(t *testing.T) {
			discovery := discoverLinked(t, linkedRepo(t, "skills/demo/cfg", filepath.Join("..", "..", dir, "config")))
			dest := filepath.Join(t.TempDir(), "demo")

			_, err := InstallFromDiscovery(discovery, discovery.Skills[0], dest, InstallOptions{SourceDir: filepath.Dir(dest), SkipAudit: true})

			assertRefused(t, err, dest)
		})
	}
}

// A refused link sorted after an ordinary file must not leave that file behind.
func TestInstallFromDiscovery_RefusedLinkWritesNothing(t *testing.T) {
	discovery := discoverLinked(t, linkedRepo(t, "skills/demo/zz.md", outsideFile(t)))
	dest := filepath.Join(t.TempDir(), "demo")

	_, err := InstallFromDiscovery(discovery, discovery.Skills[0], dest, InstallOptions{SourceDir: filepath.Dir(dest), SkipAudit: true})

	assertRefused(t, err, dest)
}

func TestInstallFromDiscovery_RefusesRootSkillFileOutsideRepository(t *testing.T) {
	secret := outsideFile(t)
	if err := os.WriteFile(secret, []byte("---\nname: root\n---\n"+outsideSecret), 0o600); err != nil {
		t.Fatal(err)
	}
	discovery := discoverLinked(t, linkedRepo(t, "SKILL.md", secret))
	var root *SkillInfo
	for i := range discovery.Skills {
		if discovery.Skills[i].Path == "." {
			root = &discovery.Skills[i]
		}
	}
	if root == nil {
		t.Fatalf("root skill not discovered: %+v", discovery.Skills)
	}
	dest := filepath.Join(t.TempDir(), "root")

	_, err := InstallFromDiscovery(discovery, *root, dest, InstallOptions{SourceDir: filepath.Dir(dest), SkipAudit: true})

	assertRefused(t, err, dest)
}

func TestInstallAgentFromDiscovery_RefusesLinkOutsideRepository(t *testing.T) {
	discovery := discoverLinked(t, linkedRepo(t, "agents/notes.md", outsideFile(t)))
	var agent *AgentInfo
	for i := range discovery.Agents {
		if discovery.Agents[i].Path == "agents/notes.md" {
			agent = &discovery.Agents[i]
		}
	}
	if agent == nil {
		t.Fatalf("linked agent not discovered: %+v", discovery.Agents)
	}
	agentsDir := t.TempDir()

	_, err := InstallAgentFromDiscovery(discovery, *agent, agentsDir, InstallOptions{SourceDir: agentsDir, SkipAudit: true})

	assertRefused(t, err, filepath.Join(agentsDir, "notes.md"))
}

func TestInstallTrackedRepo_RefusesLinkOutsideRepository(t *testing.T) {
	source, err := ParseSource(fileURL(linkedRepo(t, "skills/demo/notes.md", outsideFile(t))))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()

	_, err = InstallTrackedRepo(source, root, InstallOptions{Name: "demo", Track: true, SourceDir: root, SkipAudit: true})

	assertRefused(t, err, filepath.Join(root, "_demo"))
}

// An update with the audit skipped must still refuse a link the upstream added.
func TestInstallTrackedRepo_UpdateRefusesLinkOutsideRepository(t *testing.T) {
	for _, skipAudit := range []bool{true, false} {
		t.Run(fmt.Sprintf("skipAudit=%v", skipAudit), func(t *testing.T) {
			repo := linkedRepo(t, "skills/demo/ref.md", filepath.Join("..", "..", "shared", "ref.md"))
			source, err := ParseSource(fileURL(repo))
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			opts := InstallOptions{Name: "demo", Track: true, SourceDir: root, SkipAudit: true}
			if _, err := InstallTrackedRepo(source, root, opts); err != nil {
				t.Fatal(err)
			}
			commitLink(t, repo, "skills/demo/notes.md", outsideFile(t))
			if source, err = ParseSource(fileURL(repo)); err != nil {
				t.Fatal(err)
			}

			opts.Update, opts.SkipAudit = true, skipAudit
			_, err = InstallTrackedRepo(source, root, opts)

			// The audit's block offers --force and --skip-audit; neither gets past this.
			if !errors.Is(err, ErrLinkOutside) || errors.Is(err, audit.ErrBlocked) {
				t.Fatalf("err = %v; want a link refusal, not an audit block", err)
			}
			assertRefused(t, err, filepath.Join(root, "_demo", "skills", "demo", "notes.md"))
		})
	}
}

func TestInstall_WholeRepoRefusesLinkOutsideRepository(t *testing.T) {
	repo := linkedRepo(t, "notes.md", outsideFile(t))
	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("---\nname: whole\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "rm", "-rq", "skills")
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "single skill at the root")
	source, err := ParseSource(fileURL(repo))
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	dest := filepath.Join(sourceDir, "whole")

	_, err = Install(source, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true})

	assertRefused(t, err, dest)
}

// A whole-repository install updates with git pull in place; a link the
// upstream added must be rolled back.
func TestInstall_WholeRepoUpdateRefusesLinkOutsideRepository(t *testing.T) {
	repo := linkedRepo(t, "ref.md", filepath.Join("shared", "ref.md"))
	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("---\nname: whole\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "rm", "-rq", "skills")
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "single skill at the root")
	source, err := ParseSource(fileURL(repo))
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	dest := filepath.Join(sourceDir, "whole")
	if _, err := Install(source, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}
	commitLink(t, repo, "notes.md", outsideFile(t))
	// An update parses the recorded source again; the install pinned this one's commit.
	if source, err = ParseSource(fileURL(repo)); err != nil {
		t.Fatal(err)
	}

	_, err = Install(source, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true, Update: true})

	assertRefused(t, err, filepath.Join(dest, "notes.md"))
}

// commitLink commits a link at rel in repo pointing at target.
func commitLink(t *testing.T, repo, rel, target string) {
	t.Helper()
	if err := os.Symlink(target, filepath.Join(repo, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add "+rel)
}

func TestInstallFromDiscovery_CopiesLinkedFolderInsideRepository(t *testing.T) {
	discovery := discoverLinked(t, linkedRepo(t, "skills/demo/refs", filepath.Join("..", "..", "shared")))
	dest := filepath.Join(t.TempDir(), "demo")

	if _, err := InstallFromDiscovery(discovery, discovery.Skills[0], dest, InstallOptions{SourceDir: filepath.Dir(dest), SkipAudit: true}); err != nil {
		t.Fatal(err)
	}

	if got, _ := os.ReadFile(filepath.Join(dest, "refs", "ref.md")); string(got) != "shared reference" {
		t.Fatalf("refs/ref.md = %q, want the linked folder's file", got)
	}
}

// A linked folder inside the repository may not carry a link out of it.
func TestInstallFromDiscovery_RefusesOutsideLinkInLinkedFolder(t *testing.T) {
	repo := linkedRepo(t, "skills/demo/refs", filepath.Join("..", "..", "shared"))
	commitLink(t, repo, "shared/key", outsideFile(t))
	discovery := discoverLinked(t, repo)
	dest := filepath.Join(t.TempDir(), "demo")

	_, err := InstallFromDiscovery(discovery, discovery.Skills[0], dest, InstallOptions{SourceDir: filepath.Dir(dest), SkipAudit: true})

	assertRefused(t, err, dest)
}

// A subdirectory install fetches only the folder; a link to a file elsewhere
// in the repository must still bring that file.
func TestInstall_SubdirCopiesLinkInsideRepository(t *testing.T) {
	repo := linkedRepo(t, "skills/demo/ref.md", filepath.Join("..", "..", "shared", "ref.md"))
	source, err := ParseSource("file://" + repo + "//skills/demo")
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	dest := filepath.Join(sourceDir, "demo")

	if _, err := Install(source, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}

	if got, _ := os.ReadFile(filepath.Join(dest, "ref.md")); string(got) != "shared reference" {
		t.Fatalf("ref.md = %q, want the shared file's content", got)
	}
}

func TestDiscoverFromGitSubdir_CopiesLinkInsideRepository(t *testing.T) {
	repo := linkedRepo(t, "skills/demo/ref.md", filepath.Join("..", "..", "shared", "ref.md"))
	source, err := ParseSource("file://" + repo + "//skills/demo")
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := DiscoverFromGitSubdir(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { CleanupDiscovery(discovery) })
	dest := filepath.Join(t.TempDir(), "demo")

	if _, err := InstallFromDiscovery(discovery, discovery.Skills[0], dest, InstallOptions{SourceDir: filepath.Dir(dest), SkipAudit: true}); err != nil {
		t.Fatal(err)
	}

	if got, _ := os.ReadFile(filepath.Join(dest, "ref.md")); string(got) != "shared reference" {
		t.Fatalf("ref.md = %q, want the shared file's content", got)
	}
}

func TestCheckoutLinksWithin(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		commit, ok   bool
	}{
		{"dangling link inside", filepath.Join("packages", "missing.md"), true, true},
		{"dangling link outside", filepath.Join("..", "missing.md"), true, false},
		{"committed link outside", "", true, false},
		{"untracked link outside", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := linkedRepo(t, "skills/demo/ref.md", filepath.Join("..", "..", "shared", "ref.md"))
			target := tc.target
			if target == "" {
				target = outsideFile(t)
			}
			if err := os.Symlink(target, filepath.Join(repo, "readme.md")); err != nil {
				t.Fatal(err)
			}
			if tc.commit {
				gitAdd(t, repo, ".")
				gitCommit(t, repo, "add readme.md")
			}

			err := checkoutLinksWithin(repo)

			if (err == nil) != tc.ok {
				t.Fatalf("err = %v, want ok = %v", err, tc.ok)
			}
		})
	}
}

// A repository installed before links were checked may hold one; an update
// with nothing new must not fail on it.
func TestInstallTrackedRepo_UpToDateUpdateSkipsLinkCheck(t *testing.T) {
	repo := linkedRepo(t, "skills/demo/ref.md", filepath.Join("..", "..", "shared", "ref.md"))
	source, err := ParseSource(fileURL(repo))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	opts := InstallOptions{Name: "demo", Track: true, SourceDir: root, SkipAudit: true}
	if _, err := InstallTrackedRepo(source, root, opts); err != nil {
		t.Fatal(err)
	}
	commitLink(t, repo, "skills/demo/notes.md", outsideFile(t))
	runGit(t, filepath.Join(root, "_demo"), "pull", "--quiet")

	opts.Update = true
	if _, err := InstallTrackedRepo(source, root, opts); err != nil {
		t.Fatalf("up-to-date update failed: %v", err)
	}
}

// install from a project config moves a checkout to the locked commit; a
// link that commit adds must be refused like an update's.
func TestRelockGitRepo_RefusesLinkOutsideRepository(t *testing.T) {
	repo := linkedRepo(t, "skills/demo/ref.md", filepath.Join("..", "..", "shared", "ref.md"))
	source, err := ParseSource(fileURL(repo))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if _, err := InstallTrackedRepo(source, root, InstallOptions{Name: "demo", Track: true, SourceDir: root, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}
	commitLink(t, repo, "skills/demo/notes.md", outsideFile(t))
	locked, err := getGitFullHash(repo)
	if err != nil {
		t.Fatal(err)
	}

	err = relockGitRepo(filepath.Join(root, "_demo"), locked, InstallOptions{SourceDir: root, SkipAudit: true})

	assertRefused(t, err, filepath.Join(root, "_demo", "skills", "demo", "notes.md"))
}
