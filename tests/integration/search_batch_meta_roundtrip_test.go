//go:build !online

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/search"
	"skillshare/internal/testutil"
)

func TestSearchBatchGroupedInstall_MetadataSourceParseRoundTrip(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	repoPath := filepath.Join(sb.Root, "monorepo")
	if err := os.MkdirAll(filepath.Join(repoPath, "skills", "alpha-skill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoPath, "skills", "beta-skill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "skills", "alpha-skill", "SKILL.md"), []byte("# alpha"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "skills", "beta-skill", "SKILL.md"), []byte("# beta"), 0644); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoPath)

	// Route GitLab clone URL to the local test repo so the test remains offline.
	configureGitURLRewriteOrSkip(t, sb.Home, repoPath, "https://gitlab.com/team/monorepo.git")

	selected := []search.SearchResult{
		{
			Name:   "alpha-skill",
			Source: "https://gitlab.com/team/monorepo/-/tree/main/skills/alpha-skill",
		},
		{
			Name:   "beta-skill",
			Source: "https://gitlab.com/team/monorepo/-/tree/main/skills/beta-skill",
		},
	}

	// Simulate grouped search install: clone repo once, install both skills from discovery.
	firstSource, err := install.ParseSource(selected[0].Source)
	if err != nil {
		t.Fatalf("parse first source: %v", err)
	}
	repoSource := repoSourceForGroupedCloneForTest(firstSource)

	discovery, err := install.DiscoverFromGitWithProgress(&repoSource, nil)
	if err != nil {
		t.Fatalf("discover from grouped clone source: %v", err)
	}
	defer install.CleanupDiscovery(discovery)

	for _, sr := range selected {
		src, parseErr := install.ParseSource(sr.Source)
		if parseErr != nil {
			t.Fatalf("parse source %q: %v", sr.Source, parseErr)
		}

		skill, ok := findSkillBySubdir(discovery, src.Subdir)
		if !ok {
			t.Fatalf("skill subdir %q not found in discovery", src.Subdir)
		}

		destPath := filepath.Join(sb.SourcePath, sr.Name)
		if _, installErr := install.InstallFromDiscovery(discovery, skill, destPath, install.InstallOptions{}); installErr != nil {
			t.Fatalf("install %s from discovery failed: %v", sr.Name, installErr)
		}
	}

	for _, name := range []string{"alpha-skill", "beta-skill"} {
		skillPath := filepath.Join(sb.SourcePath, name)
		meta, err := install.ReadMeta(skillPath)
		if err != nil {
			t.Fatalf("read meta for %s: %v", name, err)
		}
		if meta == nil {
			t.Fatalf("meta missing for %s", name)
		}

		parsed, err := install.ParseSource(meta.Source)
		if err != nil {
			t.Fatalf("meta source for %s is not parseable: %q (%v)", name, meta.Source, err)
		}
		if parsed.CloneURL != "https://gitlab.com/team/monorepo.git" {
			t.Fatalf("unexpected clone URL for %s: got %q", name, parsed.CloneURL)
		}

		wantSubdir := "skills/" + name
		if parsed.Subdir != wantSubdir {
			t.Fatalf("unexpected subdir for %s: got %q, want %q (source=%q)", name, parsed.Subdir, wantSubdir, meta.Source)
		}
	}
}

func repoSourceForGroupedCloneForTest(src *install.Source) install.Source {
	repoSource := *src
	repoSource.Subdir = ""
	repoSource.Raw = repoSource.CloneURL

	if root, err := install.ParseSource(repoSource.CloneURL); err == nil {
		repoSource.Type = root.Type
		repoSource.Raw = root.Raw
		repoSource.Name = root.Name
	}

	return repoSource
}

func findSkillBySubdir(discovery *install.DiscoveryResult, subdir string) (install.SkillInfo, bool) {
	for _, sk := range discovery.Skills {
		if sk.Path == subdir {
			return sk, true
		}
	}
	return install.SkillInfo{}, false
}

func configureGitURLRewriteOrSkip(t *testing.T, home, repoPath, remoteURL string) {
	t.Helper()

	key := fmt.Sprintf("url.file://%s/.insteadOf", repoPath)
	cmd := exec.Command("git", "config", "--global", key, remoteURL)
	cmd.Env = append(os.Environ(), "HOME="+home)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git config unavailable: %v (%s)", err, string(out))
	}
}
