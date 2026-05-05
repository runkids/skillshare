package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_AzureHosts_Valid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
source: /tmp/skills
targets:
  claude:
    path: /tmp/claude/skills
azure_hosts:
  - dev.azure.com
  - Code.Internal.IO
`), 0644)

	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AzureHosts) != 2 {
		t.Fatalf("expected 2 azure_hosts, got %d", len(cfg.AzureHosts))
	}
	// Should be lowercased
	if cfg.AzureHosts[0] != "dev.azure.com" {
		t.Errorf("expected dev.azure.com, got %s", cfg.AzureHosts[0])
	}
	if cfg.AzureHosts[1] != "code.internal.io" {
		t.Errorf("expected code.internal.io, got %s", cfg.AzureHosts[1])
	}
}

func TestLoad_AzureHosts_OmittedWhenEmpty(t *testing.T) {
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
	if cfg.AzureHosts != nil {
		t.Errorf("expected nil azure_hosts when omitted, got %v", cfg.AzureHosts)
	}
}

func TestLoad_AzureHosts_InvalidEntries(t *testing.T) {
	tests := []struct {
		name  string
		entry string
	}{
		{"scheme", "https://dev.azure.com"},
		{"slash", "dev.azure.com/path"},
		{"port", "dev.azure.com:443"},
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
azure_hosts:
  - `+tt.entry+`
`), 0644)

			t.Setenv("SKILLSHARE_CONFIG", cfgPath)
			_, err := Load()
			if err == nil {
				t.Errorf("expected error for azure_hosts entry %q, got nil", tt.entry)
			}
		})
	}
}

func TestLoad_AzureHosts_EnvVar(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
source: /tmp/skills
targets:
  claude:
    path: /tmp/claude/skills
azure_hosts:
  - dev.azure.com
`), 0644)

	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	t.Setenv("SKILLSHARE_AZURE_HOSTS", "code.ci.io, Dev.Azure.Com")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	// AzureHosts (persisted) should only have config-file value
	if len(cfg.AzureHosts) != 1 {
		t.Fatalf("AzureHosts (persisted) expected 1, got %d: %v", len(cfg.AzureHosts), cfg.AzureHosts)
	}
	if cfg.AzureHosts[0] != "dev.azure.com" {
		t.Errorf("AzureHosts[0] = %s, want dev.azure.com", cfg.AzureHosts[0])
	}
	// EffectiveAzureHosts should merge config + env (deduped)
	effective := cfg.EffectiveAzureHosts()
	if len(effective) != 2 {
		t.Fatalf("EffectiveAzureHosts expected 2, got %d: %v", len(effective), effective)
	}
	if effective[0] != "dev.azure.com" {
		t.Errorf("EffectiveAzureHosts[0] = %s, want dev.azure.com", effective[0])
	}
	if effective[1] != "code.ci.io" {
		t.Errorf("EffectiveAzureHosts[1] = %s, want code.ci.io", effective[1])
	}
}

func TestLoad_AzureHosts_EnvVarOnly(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
source: /tmp/skills
targets:
  claude:
    path: /tmp/claude/skills
`), 0644)

	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	t.Setenv("SKILLSHARE_AZURE_HOSTS", "dev.azure.com")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	// AzureHosts (persisted) should be nil — nothing in config file
	if cfg.AzureHosts != nil {
		t.Errorf("AzureHosts (persisted) should be nil, got %v", cfg.AzureHosts)
	}
	// EffectiveAzureHosts should have the env var value
	effective := cfg.EffectiveAzureHosts()
	if len(effective) != 1 {
		t.Fatalf("EffectiveAzureHosts expected 1, got %d: %v", len(effective), effective)
	}
	if effective[0] != "dev.azure.com" {
		t.Errorf("EffectiveAzureHosts[0] = %s, want dev.azure.com", effective[0])
	}
}

func TestLoad_AzureHosts_EnvVarSkipsInvalid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
source: /tmp/skills
targets:
  claude:
    path: /tmp/claude/skills
`), 0644)

	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	t.Setenv("SKILLSHARE_AZURE_HOSTS", "good.host, https://bad.url, also-good.io, ,")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	effective := cfg.EffectiveAzureHosts()
	if len(effective) != 2 {
		t.Fatalf("EffectiveAzureHosts expected 2, got %d: %v", len(effective), effective)
	}
	if effective[0] != "good.host" {
		t.Errorf("EffectiveAzureHosts[0] = %s, want good.host", effective[0])
	}
	if effective[1] != "also-good.io" {
		t.Errorf("EffectiveAzureHosts[1] = %s, want also-good.io", effective[1])
	}
}

func TestLoad_AzureHosts_EnvVarNotPersisted(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
source: /tmp/skills
targets:
  claude:
    path: /tmp/claude/skills
azure_hosts:
  - dev.azure.com
`), 0644)

	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	t.Setenv("SKILLSHARE_AZURE_HOSTS", "env-only.host")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	// Save and re-read — env-only host must not be persisted
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(data), "env-only.host") {
		t.Errorf("Save() persisted env-only host to config file:\n%s", data)
	}
	if !strings.Contains(string(data), "dev.azure.com") {
		t.Errorf("Save() lost config-file host dev.azure.com:\n%s", data)
	}
}

func TestLoadProject_AzureHosts(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, ".skillshare")
	os.MkdirAll(projDir, 0755)

	cfgPath := filepath.Join(projDir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
targets:
  - claude
azure_hosts:
  - dev.azure.com
`), 0644)

	cfg, err := LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AzureHosts) != 1 {
		t.Fatalf("expected 1 azure_hosts, got %d", len(cfg.AzureHosts))
	}
	if cfg.AzureHosts[0] != "dev.azure.com" {
		t.Errorf("expected dev.azure.com, got %s", cfg.AzureHosts[0])
	}
}

func TestLoadProject_AzureHosts_EffectiveAzureHosts(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, ".skillshare")
	os.MkdirAll(projDir, 0755)

	os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`
targets:
  - claude
azure_hosts:
  - dev.azure.com
`), 0644)

	t.Setenv("SKILLSHARE_AZURE_HOSTS", "ci-only.host, Dev.Azure.Com")

	cfg, err := LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	// AzureHosts (persisted) should only have config-file value
	if len(cfg.AzureHosts) != 1 {
		t.Fatalf("AzureHosts (persisted) expected 1, got %d: %v", len(cfg.AzureHosts), cfg.AzureHosts)
	}
	// EffectiveAzureHosts should merge config + env (deduped)
	effective := cfg.EffectiveAzureHosts()
	if len(effective) != 2 {
		t.Fatalf("EffectiveAzureHosts expected 2, got %d: %v", len(effective), effective)
	}
	if effective[0] != "dev.azure.com" {
		t.Errorf("EffectiveAzureHosts[0] = %s, want dev.azure.com", effective[0])
	}
	if effective[1] != "ci-only.host" {
		t.Errorf("EffectiveAzureHosts[1] = %s, want ci-only.host", effective[1])
	}
}
