package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/audit"
	"skillshare/internal/check"
	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestBatchUpdateTrackedAgents_ConfiguredHosts(t *testing.T) {
	for _, tt := range []struct {
		host, username, tokenVar string
		opts                     install.ParseOptions
	}{
		{"cnb.corp.example", "cnb", "CNB_TOKEN", install.ParseOptions{CNBHosts: []string{"cnb.corp.example"}}},
		{"gitea.corp.example", "x-access-token", "GITEA_TOKEN", install.ParseOptions{GiteaHosts: []string{"gitea.corp.example"}}},
	} {
		t.Run(tt.host, func(t *testing.T) {
			base := t.TempDir()
			remote := testutil.SetupBareRemoteRepo(t, base)
			testutil.SeedRemoteBranch(t, base, remote, "main", map[string]string{"reviewer.md": "# Original agent\n"})
			root := filepath.Join(base, "agents")
			checkout := filepath.Join(root, "_repo")
			// The private transport replaces global Git config; keep checkout
			// line endings stable when it disables the Windows autocrlf default.
			testutil.RunGit(t, "", "clone", "--config", "core.autocrlf=false", remote, checkout)
			before := testutil.RunGit(t, checkout, "rev-parse", "HEAD")
			const updated = "# Updated reviewer\nReview changes carefully.\n"
			seed := filepath.Join(base, "seed-main")
			if err := os.WriteFile(filepath.Join(seed, "reviewer.md"), []byte(updated), 0o644); err != nil {
				t.Fatal(err)
			}
			testutil.RunGit(t, seed, "add", ".")
			testutil.RunGit(t, seed, "commit", "-m", "update reviewer")
			testutil.RunGit(t, seed, "push", "origin", "HEAD:main")
			t.Setenv(tt.tokenVar, "tracked-agent-update-token")
			testutil.ServePrivateGit(t, remote, tt.username, "tracked-agent-update-token")
			cloneURL := "https://" + tt.host + "/org/repo.git"
			testutil.RunGit(t, checkout, "remote", "set-url", "origin", cloneURL)
			store := install.NewMetadataStore()
			store.Set("_repo", &install.MetadataEntry{Source: cloneURL, Kind: "agent", Tracked: true, Branch: "main"})
			if err := store.Save(root); err != nil {
				t.Fatal(err)
			}
			agents := []check.AgentCheckResult{{Name: "_repo/reviewer.md", RepoPath: checkout, RepoURL: cloneURL}}
			items, count, failed := batchUpdateAgents(root, agents, &updateAgentArgs{}, "", false, tt.opts)
			if count != 1 || failed != 0 {
				t.Fatalf("tracked agent update: updated=%d, failed=%d, items=%+v", count, failed, items)
			}
			if body, err := os.ReadFile(filepath.Join(checkout, "reviewer.md")); err != nil || string(body) != updated {
				t.Fatalf("updated agent content = %q, error = %v", body, err)
			}
			if after := testutil.RunGit(t, checkout, "rev-parse", "HEAD"); after == before {
				t.Fatal("tracked checkout did not advance to the updated revision")
			}
		})
	}
}

func TestIsSecurityError(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("connection timeout"), false},
		{fmt.Errorf("skill not found"), false},
		{fmt.Errorf("has uncommitted changes"), false},

		// Sentinel-based detection
		{fmt.Errorf("security audit failed: scan error: %w", audit.ErrBlocked), true},
		{fmt.Errorf("post-update audit failed: scan error — rolled back: %w", audit.ErrBlocked), true},
		{fmt.Errorf("rollback failed: permission denied: %w", audit.ErrBlocked), true},
		{audit.ErrBlocked, true},

		// Wrapped sentinel
		{fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", audit.ErrBlocked)), true},

		// Non-sentinel errors with similar text should NOT match
		{fmt.Errorf("security audit something"), false},
		{fmt.Errorf("was rolled back successfully"), false},
	}

	for _, tt := range tests {
		name := "nil"
		if tt.err != nil {
			name = tt.err.Error()
			if len(name) > 60 {
				name = name[:60]
			}
		}
		t.Run(name, func(t *testing.T) {
			got := isSecurityError(tt.err)
			if got != tt.want {
				t.Errorf("isSecurityError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestParseUpdateArgs_ThresholdAlias(t *testing.T) {
	opts, showHelp, err := parseUpdateArgs([]string{"--all", "-T", "h"})
	if err != nil {
		t.Fatalf("parseUpdateArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if opts.threshold != audit.SeverityHigh {
		t.Fatalf("expected threshold=%s, got %s", audit.SeverityHigh, opts.threshold)
	}
}

func TestParseUpdateArgs_InvalidThreshold(t *testing.T) {
	_, showHelp, err := parseUpdateArgs([]string{"--all", "--threshold", "urgent"})
	if err == nil {
		t.Fatal("expected error for invalid threshold")
	}
	if showHelp {
		t.Fatal("expected showHelp=false on invalid threshold")
	}
}

// --- resolveByGlob tests ---

// setupTrackedRepo creates a fake tracked repo (_-prefixed dir with .git).
func setupTrackedRepo(t *testing.T, sourceDir, name string) {
	t.Helper()
	repoDir := filepath.Join(sourceDir, name)
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	os.WriteFile(filepath.Join(repoDir, "SKILL.md"), []byte("# "+name), 0644)
}

// setupUpdatableSkill creates a skill with .skillshare-meta.json containing a Source.
func setupUpdatableSkill(t *testing.T, sourceDir, name string) {
	t.Helper()
	dir := filepath.Join(sourceDir, name)
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0644)
	store, _ := install.LoadMetadata(sourceDir)
	store.Set(name, &install.MetadataEntry{Source: "github.com/test/" + name, Type: "github"})
	store.Save(sourceDir)
}

func TestResolveByGlob_MatchesTrackedRepos(t *testing.T) {
	src := t.TempDir()
	setupTrackedRepo(t, src, "_team-auth")
	setupTrackedRepo(t, src, "_team-db")
	setupTrackedRepo(t, src, "_other-repo")

	matches, err := resolveByGlob(src, "_team-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	for _, m := range matches {
		if !m.isRepo {
			t.Errorf("%s should be marked as repo", m.name)
		}
	}
}

func TestResolveByGlob_MatchesUpdatableSkills(t *testing.T) {
	src := t.TempDir()
	setupUpdatableSkill(t, src, "core-auth")
	setupUpdatableSkill(t, src, "core-db")
	setupUpdatableSkill(t, src, "utils")

	matches, err := resolveByGlob(src, "core-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	for _, m := range matches {
		if m.isRepo {
			t.Errorf("%s should not be marked as repo", m.name)
		}
	}
}

func TestResolveByGlob_CaseInsensitive(t *testing.T) {
	src := t.TempDir()
	setupUpdatableSkill(t, src, "Core-Auth")
	setupUpdatableSkill(t, src, "core-db")

	matches, err := resolveByGlob(src, "CORE-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 case-insensitive matches, got %d", len(matches))
	}
}

func TestResolveByGlob_NoMatch(t *testing.T) {
	src := t.TempDir()
	setupUpdatableSkill(t, src, "utils")

	matches, err := resolveByGlob(src, "core-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestResolveByGlob_SortedByName(t *testing.T) {
	src := t.TempDir()
	setupUpdatableSkill(t, src, "z-skill")
	setupUpdatableSkill(t, src, "a-skill")
	setupUpdatableSkill(t, src, "m-skill")

	matches, err := resolveByGlob(src, "*-skill")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}
	if matches[0].name > matches[1].name || matches[1].name > matches[2].name {
		t.Errorf("results not sorted: %s, %s, %s", matches[0].name, matches[1].name, matches[2].name)
	}
}

// An unprefixed git checkout is not a tracked repo: a group walk updates it as
// the regular skill its metadata describes, beside the _-prefixed repos (#476).
func TestResolveGroupUpdatable_UnprefixedCheckoutIsRegularSkill(t *testing.T) {
	src := t.TempDir()
	setupTrackedRepo(t, src, "org/_team")
	setupUpdatableSkill(t, src, "org/solo")
	// A submodule or worktree has a .git file, not a directory.
	if err := os.WriteFile(filepath.Join(src, "org", "solo", ".git"), []byte("gitdir: ../../elsewhere\n"), 0644); err != nil {
		t.Fatal(err)
	}

	matches, err := resolveGroupUpdatable("org", src)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]updateTarget{}
	for _, m := range matches {
		got[filepath.ToSlash(m.name)] = m
	}
	if len(got) != 2 || !got["org/_team"].isRepo || got["org/solo"].isRepo || got["org/solo"].meta == nil {
		t.Fatalf("matches = %+v, want org/_team as repo and org/solo as skill with metadata", matches)
	}
}

func TestIsGroupDir_UnprefixedCheckoutIsGroup(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "bundle", ".git"), 0755)
	setupTrackedRepo(t, src, "_team")
	store, _ := install.LoadMetadata(src)

	if !isGroupDir("bundle", src, store) {
		t.Error("an unprefixed checkout is a group, not a tracked repo")
	}
	if isGroupDir("_team", src, store) {
		t.Error("a _-prefixed checkout is a tracked repo, not a group")
	}
}
