package main

import (
	"testing"

	hookpkg "skillshare/internal/hooks"
	pluginpkg "skillshare/internal/plugins"
)

func TestBuildDoctorOutputIncludesPluginAndHookSections(t *testing.T) {
	result := &doctorResult{
		checks: []doctorCheck{
			{Name: "plugins", Status: checkWarning, Message: "plugins need sync"},
			{Name: "hooks", Status: checkPass, Message: "hooks ok"},
		},
	}
	plugins := []pluginpkg.Bundle{{Name: "demo", HasClaude: true}}
	hooks := []hookpkg.Bundle{{Name: "audit", Targets: map[string]int{"claude": 1}}}

	output := buildDoctorOutput(result, plugins, hooks)
	if len(output.Plugins) != 1 || output.Plugins[0].Name != "demo" {
		t.Fatalf("expected plugins section, got %+v", output.Plugins)
	}
	if len(output.Hooks) != 1 || output.Hooks[0].Name != "audit" {
		t.Fatalf("expected hooks section, got %+v", output.Hooks)
	}
	if output.Summary.Warnings != 1 || output.Summary.Pass != 1 {
		t.Fatalf("unexpected summary: %+v", output.Summary)
	}
}
