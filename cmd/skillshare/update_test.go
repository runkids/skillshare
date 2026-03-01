package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/audit"
	"skillshare/internal/install"
)

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
	install.WriteMeta(dir, &install.SkillMeta{Source: "github.com/test/" + name, Type: "github"})
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
