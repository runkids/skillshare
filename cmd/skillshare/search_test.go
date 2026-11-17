package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	searchpkg "skillshare/internal/search"
	"skillshare/internal/testutil"
)

func TestInstallFromSearchResult_LogsInstalledSkillDetails(t *testing.T) {
	tmp := t.TempDir()
	testutil.SetIsolatedXDG(t, tmp)
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

// Picking an installed skill again keeps it, and the log names it.
func TestInstallFromSearchResult_LogsSkippedSkill(t *testing.T) {
	tmp := t.TempDir()
	testutil.SetIsolatedXDG(t, tmp)
	cfgPath := filepath.Join(tmp, "home", ".config", "skillshare", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	sourceDir := filepath.Join(tmp, "source")
	if err := os.MkdirAll(filepath.Join(sourceDir, "kept"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Source: sourceDir, Targets: map[string]config.TargetConfig{}}

	if err := installFromSearchResult(searchpkg.SearchResult{Name: "kept", Source: filepath.Join(tmp, "unused")}, cfg); err != nil {
		t.Fatalf("installFromSearchResult returned error: %v", err)
	}
	entries, err := oplog.Read(cfgPath, oplog.OpsFile, 1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("read log: %v, %d entries", err, len(entries))
	}
	if detail := formatInstallLogDetail(entries[0].Args); !strings.Contains(detail, "skipped=kept") {
		t.Fatalf("expected skipped skill in detail, got: %s", detail)
	}
}

func TestLogSkillInstallOp_LogsSkippedSkill(t *testing.T) {
	tmp := t.TempDir()
	testutil.SetIsolatedXDG(t, tmp)
	cfgPath := filepath.Join(tmp, "config.yaml")
	logSkillInstallOp(cfgPath, searchpkg.SearchResult{Name: "kept", Source: "owner/repo"}, searchInstallResult{name: "kept", status: "skipped"}, "global")
	entries, err := oplog.Read(cfgPath, oplog.OpsFile, 1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("read log: %v, %d entries", err, len(entries))
	}
	if detail := formatInstallLogDetail(entries[0].Args); !strings.Contains(detail, "skipped=kept") {
		t.Fatalf("expected skipped skill in detail, got: %s", detail)
	}
}
