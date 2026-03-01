package main

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

// --- resolveUninstallByGlob tests ---

func TestResolveUninstallByGlob_MatchesDirs(t *testing.T) {
	src := t.TempDir()
	for _, name := range []string{"core-auth", "core-db", "utils"} {
		os.MkdirAll(filepath.Join(src, name), 0755)
	}

	cfg := &config.Config{Source: src}
	targets, err := resolveUninstallByGlob("core-*", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(targets))
	}
	names := map[string]bool{}
	for _, tgt := range targets {
		names[tgt.name] = true
	}
	if !names["core-auth"] || !names["core-db"] {
		t.Errorf("expected core-auth and core-db, got %v", names)
	}
}

func TestResolveUninstallByGlob_DetectsTrackedRepos(t *testing.T) {
	src := t.TempDir()
	// Tracked repo (has .git)
	repoDir := filepath.Join(src, "_team-skills")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	// Regular skill
	os.MkdirAll(filepath.Join(src, "_team-docs"), 0755)

	cfg := &config.Config{Source: src}
	targets, err := resolveUninstallByGlob("_team-*", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(targets))
	}
	for _, tgt := range targets {
		if tgt.name == "_team-skills" && !tgt.isTrackedRepo {
			t.Error("_team-skills should be detected as tracked repo")
		}
		if tgt.name == "_team-docs" && tgt.isTrackedRepo {
			t.Error("_team-docs should not be detected as tracked repo")
		}
	}
}

func TestResolveUninstallByGlob_CaseInsensitive(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "Core-Auth"), 0755)
	os.MkdirAll(filepath.Join(src, "core-db"), 0755)

	cfg := &config.Config{Source: src}
	targets, err := resolveUninstallByGlob("core-*", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 case-insensitive matches, got %d", len(targets))
	}
}

func TestResolveUninstallByGlob_NoMatch(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "utils"), 0755)

	cfg := &config.Config{Source: src}
	targets, err := resolveUninstallByGlob("core-*", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 matches, got %d", len(targets))
	}
}

func TestResolveUninstallByGlob_SkipsFiles(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "core-skill"), 0755)
	os.WriteFile(filepath.Join(src, "core-file.txt"), []byte("not a dir"), 0644)

	cfg := &config.Config{Source: src}
	targets, err := resolveUninstallByGlob("core-*", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 match (dirs only), got %d", len(targets))
	}
	if targets[0].name != "core-skill" {
		t.Errorf("expected core-skill, got %s", targets[0].name)
	}
}

// Verify unused imports are referenced
var _ = install.IsGitRepo
