package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMCPConfigRoundTrip(t *testing.T) {
	const input = `sources:
  mcp: ./shared/mcp.yaml
mcp:
  targets: [claude, codex]
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Sources.MCP != "./shared/mcp.yaml" || len(cfg.MCP.Targets) != 2 {
		t.Fatalf("MCP source and targets must survive config loading: %+v", cfg)
	}
	data, err := yaml.Marshal(struct {
		Sources GlobalSources `yaml:"sources"`
		MCP     *MCPConfig    `yaml:"mcp"`
	}{cfg.Sources, cfg.MCP})
	if err != nil {
		t.Fatal(err)
	}
	var project ProjectConfig
	if err := yaml.Unmarshal(data, &project); err != nil {
		t.Fatal(err)
	}
	if project.Sources.MCP != cfg.Sources.MCP || len(project.MCP.Targets) != 2 {
		t.Fatal("global and project MCP configuration must share their schema")
	}
}

func TestMCPEmptyInlinePresenceSurvivesSave(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("mcp:\n  servers: {}\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "servers: {}") {
		t.Fatalf("lost explicit empty source: %s", data)
	}
	if err := ValidateMCP(cfg.MCP, "mcp.yaml"); err == nil {
		t.Fatal("accepted two sources")
	}
}

func TestMCPValidationRejectsTwoSources(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("sources:\n  mcp: ./mcp.yaml\nmcp:\n  servers: {}\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMCP(cfg.MCP, cfg.Sources.MCP); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("MCP validation accepted ambiguous source: %v", err)
	}
}

func TestInvalidMCPSectionStillLoadsAndSaves(t *testing.T) {
	const input = "mcp:\n  targets: claude\n  typo: true\n"
	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("invalid MCP section broke config loading: %v", err)
	}
	if err := ValidateMCP(cfg.MCP, ""); err == nil {
		t.Fatal("invalid MCP section passed validation")
	}
	data, err := yaml.Marshal(cfg)
	if err != nil || !strings.Contains(string(data), "typo: true") {
		t.Fatalf("invalid MCP section was not kept as written: %v\n%s", err, data)
	}
}
