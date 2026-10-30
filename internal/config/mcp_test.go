package config

import (
	"strconv"
	"strings"
	"testing"
	"time"

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

func TestMCPProjectsSurviveSaveAndAreValidated(t *testing.T) {
	var cfg Config
	p1 := absPath("/work/p1")
	if err := yaml.Unmarshal([]byte("mcp:\n  projects:\n    "+strconv.Quote(p1)+":\n      servers:\n        docs:\n          command: docs\n          targets: [cursor]\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMCP(cfg.MCP, ""); err != nil {
		t.Fatalf("valid projects rejected: %v", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil || !strings.Contains(string(data), "p1") {
		t.Fatalf("projects lost on save: %v\n%s", err, data)
	}
	if err := yaml.Unmarshal([]byte("mcp:\n  projects:\n    work/p1:\n      servers: {}\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMCP(cfg.MCP, ""); err == nil {
		t.Fatal("relative project root passed validation")
	}
}

func TestMCPDirectToolsDefaultSurvivesSaveAndIsValidated(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("mcp:\n  directTools: [search]\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMCP(cfg.MCP, ""); err != nil {
		t.Fatalf("valid default rejected: %v", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil || !strings.Contains(string(data), "directTools:") {
		t.Fatalf("default lost on save: %v\n%s", err, data)
	}
	if err := yaml.Unmarshal([]byte("mcp:\n  directTools: all\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMCP(cfg.MCP, ""); err == nil {
		t.Fatal("bad default passed validation")
	}
}

func TestMCPServeEditSurvivesSave(t *testing.T) {
	const input = "mcp:\n  serve:\n    edit:\n      skills: [pdf, drafts/*]\n      preview_ttl: 45m\n"
	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatal(err)
	}
	skills, ttl, err := cfg.MCP.ServeEdit()
	if err != nil || len(skills) != 2 || ttl != 45*time.Minute {
		t.Fatalf("ServeEdit = %v, %v, %v", skills, ttl, err)
	}
	data, err := yaml.Marshal(cfg.MCP)
	if err != nil || !strings.Contains(string(data), "preview_ttl: 45m") {
		t.Errorf("saved mcp section lost serve.edit:\n%s", data)
	}
}

func TestMCPServeRejectsAMisspeltKey(t *testing.T) {
	// A typo must not leave editing silently off.
	for _, input := range []string{
		"mcp:\n  serve:\n    edits:\n      skills: [pdf]\n",
		"mcp:\n  serve:\n    edit:\n      skill: [pdf]\n",
	} {
		var cfg Config
		if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
			t.Fatal(err)
		}
		if _, _, err := cfg.MCP.ServeEdit(); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Errorf("%q: ServeEdit error = %v, want the unknown field named", input, err)
		}
	}
}

func TestMCPValidationRejectsAnInvalidPreviewLifetime(t *testing.T) {
	// The dashboard saves through ValidateMCP; mcp serve would then fail to start.
	var cfg Config
	if err := yaml.Unmarshal([]byte("mcp:\n  serve:\n    edit:\n      skills: [pdf]\n      preview_ttl: soon\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMCP(cfg.MCP, ""); err == nil || !strings.Contains(err.Error(), "preview_ttl") {
		t.Errorf("ValidateMCP = %v, want the preview_ttl error", err)
	}
}

func TestMCPServeEditRejectsABlankPatternAndABadLifetimeAlone(t *testing.T) {
	// A blank pattern would otherwise match every skill.
	for _, input := range []string{
		"mcp:\n  serve:\n    edit:\n      skills: [' ']\n",
		"mcp:\n  serve:\n    edit:\n      preview_ttl: soon\n",
	} {
		var cfg Config
		if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
			t.Fatal(err)
		}
		if err := ValidateMCP(cfg.MCP, ""); err == nil {
			t.Errorf("%q: ValidateMCP accepted it", input)
		}
	}
}
