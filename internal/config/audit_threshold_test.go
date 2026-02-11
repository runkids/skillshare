package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_DefaultAuditThreshold(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	raw := "source: /tmp/skills\ntargets: {}\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Audit.BlockThreshold != "CRITICAL" {
		t.Fatalf("expected default CRITICAL threshold, got %s", cfg.Audit.BlockThreshold)
	}
}

func TestLoad_InvalidAuditThreshold(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	raw := "source: /tmp/skills\ntargets: {}\naudit:\n  block_threshold: urgent\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected Load to fail for invalid threshold")
	}
	if !strings.Contains(err.Error(), "audit.block_threshold") {
		t.Fatalf("expected threshold error, got: %v", err)
	}
}

func TestLoadProject_DefaultAuditThreshold(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, ".skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}

	raw := "targets:\n  - claude-code\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject returned error: %v", err)
	}
	if cfg.Audit.BlockThreshold != "CRITICAL" {
		t.Fatalf("expected default CRITICAL threshold, got %s", cfg.Audit.BlockThreshold)
	}
}

func TestLoadProject_InvalidAuditThreshold(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, ".skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}

	raw := "targets:\n  - claude-code\naudit:\n  block_threshold: urgent\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	_, err := LoadProject(root)
	if err == nil {
		t.Fatal("expected LoadProject to fail for invalid threshold")
	}
	if !strings.Contains(err.Error(), "audit.block_threshold") {
		t.Fatalf("expected threshold error, got: %v", err)
	}
}
