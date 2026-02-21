package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	searchpkg "skillshare/internal/search"
)

func TestInstallFromSearchResult_LogsInstalledSkillDetails(t *testing.T) {
	tmp := t.TempDir()
	setIsolatedXDG(t, tmp)
	cfgPath := filepath.Join(tmp, "home", ".config", "skillshare", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	sourceDir := filepath.Join(tmp, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	localSkillSource := filepath.Join(tmp, "search-source")
	if err := os.MkdirAll(localSkillSource, 0755); err != nil {
		t.Fatalf("failed to create local skill source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localSkillSource, "SKILL.md"), []byte("# Search Installed Skill"), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	cfg := &config.Config{
		Source:  sourceDir,
		Targets: map[string]config.TargetConfig{},
	}

	result := searchpkg.SearchResult{
		Name:   "search-installed-skill",
		Source: localSkillSource,
	}
	if err := installFromSearchResult(result, cfg); err != nil {
		t.Fatalf("installFromSearchResult returned error: %v", err)
	}

	entries, err := oplog.Read(cfgPath, oplog.OpsFile, 1)
	if err != nil {
		t.Fatalf("failed to read operations log: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Command != "install" {
		t.Fatalf("expected install command in log, got %q", e.Command)
	}

	detail := formatInstallLogDetail(e.Args)
	if !strings.Contains(detail, "mode=global") {
		t.Fatalf("expected mode in detail, got: %s", detail)
	}
	if !strings.Contains(detail, "skills=1") {
		t.Fatalf("expected skill count in detail, got: %s", detail)
	}
	if !strings.Contains(detail, "installed=search-installed-skill") {
		t.Fatalf("expected installed skill in detail, got: %s", detail)
	}
}

func setIsolatedXDG(t *testing.T, root string) {
	t.Helper()

	xdgRoot := filepath.Join(root, "xdg")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(xdgRoot, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(xdgRoot, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(xdgRoot, "state"))
}
