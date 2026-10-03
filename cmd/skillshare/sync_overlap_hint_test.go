package main

import (
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestPrintSyncOverlapHint_NamesConflictAndFix(t *testing.T) {
	keepPathsUnfolded(t)
	targets := map[string]config.TargetConfig{
		"universal": {Skills: &config.ResourceTargetConfig{Path: "/tmp/agents/skills", Exclude: []string{"feature-radar*"}}},
		"codex":     {Skills: &config.ResourceTargetConfig{Path: "/tmp/agents/skills"}},
	}
	out := captureStdout(t, func() { printSyncOverlapHint(targets, true, false) })

	for _, want := range []string{
		"codex and universal sync skills to /tmp/agents/skills with different filters, so each sync undoes the other",
		"keep one: skillshare target codex --skills=false -p",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "share skill folders") {
		t.Errorf("conflict fully explains the overlap, generic line should be gone:\n%s", out)
	}
}

func TestPrintSyncOverlapHint_SameSettingsKeepsGenericLine(t *testing.T) {
	targets := map[string]config.TargetConfig{
		"universal": {Skills: &config.ResourceTargetConfig{Path: "/tmp/agents/skills"}},
		"codex":     {Skills: &config.ResourceTargetConfig{Path: "/tmp/agents/skills"}},
	}
	out := captureStdout(t, func() { printSyncOverlapHint(targets, false, false) })

	if !strings.Contains(out, "2 targets share skill folders") {
		t.Errorf("expected generic overlap line:\n%s", out)
	}
	if strings.Contains(out, "undoes") {
		t.Errorf("same settings is not a conflict:\n%s", out)
	}
}
