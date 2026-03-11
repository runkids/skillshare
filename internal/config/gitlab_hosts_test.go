package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_GitLabHosts_Valid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
source: /tmp/skills
targets:
  claude:
    path: /tmp/claude/skills
gitlab_hosts:
  - git.corp.com
  - Code.Internal.IO
`), 0644)

	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.GitLabHosts) != 2 {
		t.Fatalf("expected 2 gitlab_hosts, got %d", len(cfg.GitLabHosts))
	}
	// Should be lowercased
	if cfg.GitLabHosts[0] != "git.corp.com" {
		t.Errorf("expected git.corp.com, got %s", cfg.GitLabHosts[0])
	}
	if cfg.GitLabHosts[1] != "code.internal.io" {
		t.Errorf("expected code.internal.io, got %s", cfg.GitLabHosts[1])
	}
}

func TestLoad_GitLabHosts_OmittedWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
source: /tmp/skills
targets:
  claude:
    path: /tmp/claude/skills
`), 0644)

	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GitLabHosts != nil {
		t.Errorf("expected nil gitlab_hosts when omitted, got %v", cfg.GitLabHosts)
	}
}

func TestLoad_GitLabHosts_InvalidEntries(t *testing.T) {
	tests := []struct {
		name  string
		entry string
	}{
		{"scheme", "https://git.corp.com"},
		{"slash", "git.corp.com/path"},
		{"port", "git.corp.com:443"},
		{"empty", "\"  \""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "config.yaml")
			os.WriteFile(cfgPath, []byte(`
source: /tmp/skills
targets:
  claude:
    path: /tmp/claude/skills
gitlab_hosts:
  - `+tt.entry+`
`), 0644)

			t.Setenv("SKILLSHARE_CONFIG", cfgPath)
			_, err := Load()
			if err == nil {
				t.Errorf("expected error for gitlab_hosts entry %q, got nil", tt.entry)
			}
		})
	}
}

func TestLoadProject_GitLabHosts(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, ".skillshare")
	os.MkdirAll(projDir, 0755)

	cfgPath := filepath.Join(projDir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
targets:
  - claude
gitlab_hosts:
  - git.company.com
`), 0644)

	cfg, err := LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.GitLabHosts) != 1 {
		t.Fatalf("expected 1 gitlab_hosts, got %d", len(cfg.GitLabHosts))
	}
	if cfg.GitLabHosts[0] != "git.company.com" {
		t.Errorf("expected git.company.com, got %s", cfg.GitLabHosts[0])
	}
}
