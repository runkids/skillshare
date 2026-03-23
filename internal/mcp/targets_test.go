package mcp_test

import (
	"strings"
	"testing"

	"skillshare/internal/mcp"
)

func TestMCPTargets_Load(t *testing.T) {
	targets, err := mcp.MCPTargets()
	if err != nil {
		t.Fatalf("MCPTargets() error: %v", err)
	}
	if len(targets) == 0 {
		t.Fatal("MCPTargets() returned empty slice")
	}
}

func TestMCPTargets_ClaudeFound(t *testing.T) {
	target, ok := mcp.LookupMCPTarget("claude")
	if !ok {
		t.Fatal("LookupMCPTarget(\"claude\") not found")
	}
	if target.Name != "claude" {
		t.Errorf("got Name=%q, want %q", target.Name, "claude")
	}
	if target.ProjectConfig != ".mcp.json" {
		t.Errorf("got ProjectConfig=%q, want %q", target.ProjectConfig, ".mcp.json")
	}
	if target.Key != "mcpServers" {
		t.Errorf("got Key=%q, want %q", target.Key, "mcpServers")
	}
}

func TestMCPTargets_CursorFound(t *testing.T) {
	target, ok := mcp.LookupMCPTarget("cursor")
	if !ok {
		t.Fatal("LookupMCPTarget(\"cursor\") not found")
	}
	if target.Name != "cursor" {
		t.Errorf("got Name=%q, want %q", target.Name, "cursor")
	}
	if target.GlobalConfig == "" {
		t.Error("cursor GlobalConfig should not be empty")
	}
	if target.ProjectConfig == "" {
		t.Error("cursor ProjectConfig should not be empty")
	}
}

func TestMCPTargets_LookupMissing(t *testing.T) {
	_, ok := mcp.LookupMCPTarget("nonexistent-tool")
	if ok {
		t.Error("LookupMCPTarget(\"nonexistent-tool\") should return false")
	}
}

func TestEffectiveURLKey_Default(t *testing.T) {
	// cursor has no url_key set — should default to "url"
	target, ok := mcp.LookupMCPTarget("cursor")
	if !ok {
		t.Fatal("cursor not found")
	}
	if got := target.EffectiveURLKey(); got != "url" {
		t.Errorf("EffectiveURLKey() = %q, want %q", got, "url")
	}
}

func TestEffectiveURLKey_Override(t *testing.T) {
	// windsurf has url_key: serverUrl
	target, ok := mcp.LookupMCPTarget("windsurf")
	if !ok {
		t.Fatal("windsurf not found")
	}
	if got := target.EffectiveURLKey(); got != "serverUrl" {
		t.Errorf("EffectiveURLKey() = %q, want %q", got, "serverUrl")
	}
}

func TestGlobalConfigPath_ExpandsTilde(t *testing.T) {
	target, ok := mcp.LookupMCPTarget("cursor")
	if !ok {
		t.Fatal("cursor not found")
	}
	path := target.GlobalConfigPath()
	if strings.HasPrefix(path, "~") {
		t.Errorf("GlobalConfigPath() still has tilde: %q", path)
	}
	if path == "" {
		t.Error("GlobalConfigPath() returned empty string for cursor")
	}
}

func TestProjectConfigPath(t *testing.T) {
	target, ok := mcp.LookupMCPTarget("cursor")
	if !ok {
		t.Fatal("cursor not found")
	}
	projectRoot := "/some/project"
	path := target.ProjectConfigPath(projectRoot)
	expected := projectRoot + "/.cursor/mcp.json"
	if path != expected {
		t.Errorf("ProjectConfigPath() = %q, want %q", path, expected)
	}
}

func TestMCPTargetsForMode_GlobalExcludesProjectOnly(t *testing.T) {
	// roo, cline, copilot, claude have empty global_config — should be excluded
	globalTargets := mcp.MCPTargetsForMode(false)
	projectOnlyNames := []string{"roo", "cline", "copilot", "claude"}
	globalNames := make(map[string]bool)
	for _, t := range globalTargets {
		globalNames[t.Name] = true
	}
	for _, name := range projectOnlyNames {
		if globalNames[name] {
			t.Errorf("global mode should not include %q (project-only target)", name)
		}
	}
}

func TestMCPTargetsForMode_ProjectExcludesGlobalOnly(t *testing.T) {
	// windsurf has empty project_config — should be excluded from project mode
	projectTargets := mcp.MCPTargetsForMode(true)
	projectNames := make(map[string]bool)
	for _, t := range projectTargets {
		projectNames[t.Name] = true
	}
	if projectNames["windsurf"] {
		t.Error("project mode should not include windsurf (global-only target)")
	}
}

func TestMCPTargetNames(t *testing.T) {
	names := mcp.MCPTargetNames()
	if len(names) == 0 {
		t.Fatal("MCPTargetNames() returned empty slice")
	}
	// Check a few known names
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"claude", "cursor", "windsurf", "gemini"} {
		if !nameSet[expected] {
			t.Errorf("MCPTargetNames() missing %q", expected)
		}
	}
}
