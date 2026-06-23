package main

import (
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestCheckSharedTargetPaths_NoCollision(t *testing.T) {
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"claude": {Skills: &config.ResourceTargetConfig{Path: "/tmp/.claude/skills"}},
			"cursor": {Skills: &config.ResourceTargetConfig{Path: "/tmp/.cursor/skills"}},
		},
	}
	r := &doctorResult{}
	checkSharedTargetPaths(cfg, r, false)

	if r.warnings != 0 {
		t.Errorf("expected 0 warnings, got %d", r.warnings)
	}
	if len(r.checks) != 1 || r.checks[0].Status != checkPass {
		t.Errorf("expected single passing check, got %+v", r.checks)
	}
}

func TestCheckSharedTargetPaths_Collision(t *testing.T) {
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"universal": {Skills: &config.ResourceTargetConfig{Path: "/tmp/.agents/skills"}},
			"warp":      {Skills: &config.ResourceTargetConfig{Path: "/tmp/.agents/skills"}},
			"witsy":     {Skills: &config.ResourceTargetConfig{Path: "/tmp/.agents/skills"}},
			"claude":    {Skills: &config.ResourceTargetConfig{Path: "/tmp/.claude/skills"}},
		},
	}
	r := &doctorResult{}
	checkSharedTargetPaths(cfg, r, false)

	if r.warnings != 1 {
		t.Errorf("expected 1 warning (1 collision row), got %d", r.warnings)
	}
	if len(r.checks) != 1 || r.checks[0].Status != checkWarning {
		t.Fatalf("expected single warning check, got %+v", r.checks)
	}
	detail := r.checks[0].Details[0]
	for _, want := range []string{"universal", "warp", "witsy", "/tmp/.agents/skills"} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail %q missing %q", detail, want)
		}
	}
	if len(r.checks[0].Suggestions) != 1 {
		t.Fatalf("expected one suggestion, got %v", r.checks[0].Suggestions)
	}
	suggestion := r.checks[0].Suggestions[0]
	for _, want := range []string{"Choose one authoritative target", "skillshare target remove <name> --global --dry-run", "universal", "warp", "witsy", "/tmp/.agents/skills"} {
		if !strings.Contains(suggestion, want) {
			t.Errorf("suggestion %q missing %q", suggestion, want)
		}
	}
	if strings.Contains(detail, "claude") {
		t.Errorf("claude should not appear in collision detail: %q", detail)
	}
}

func TestCheckSharedTargetPaths_TildeExpansion(t *testing.T) {
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"a": {Skills: &config.ResourceTargetConfig{Path: "~/.agents/skills"}},
			"b": {Skills: &config.ResourceTargetConfig{Path: "~/.agents/skills/"}},
		},
	}
	r := &doctorResult{}
	checkSharedTargetPaths(cfg, r, false)

	if r.warnings != 1 {
		t.Errorf("expected collision after tilde+trailing-slash normalization, got %d warnings", r.warnings)
	}
}

func TestCheckSharedTargetPaths_ProjectSuggestionUsesProjectFlag(t *testing.T) {
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"codex":  {Skills: &config.ResourceTargetConfig{Path: ".agents/skills"}},
			"cursor": {Skills: &config.ResourceTargetConfig{Path: ".agents/skills"}},
		},
	}
	r := &doctorResult{}
	checkSharedTargetPaths(cfg, r, true)

	if len(r.checks) != 1 || len(r.checks[0].Suggestions) != 1 {
		t.Fatalf("expected one suggestion, got %+v", r.checks)
	}
	suggestion := r.checks[0].Suggestions[0]
	if !strings.Contains(suggestion, "skillshare target remove <name> --project --dry-run") {
		t.Fatalf("suggestion %q missing project target remove command", suggestion)
	}
	if strings.Contains(suggestion, "skillshare target remove <name> --global --dry-run") {
		t.Fatalf("suggestion %q should not use global target remove command", suggestion)
	}
}

func TestCheckCrossTargetDiscovery_CodexSeesUniversal(t *testing.T) {
	// codex's also_scans.global includes ~/.agents/skills (from targets.yaml).
	// universal writes to ~/.agents/skills. Enabling both should warn.
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"codex":     {Skills: &config.ResourceTargetConfig{Path: "~/.codex/skills"}},
			"universal": {Skills: &config.ResourceTargetConfig{Path: "~/.agents/skills"}},
		},
	}
	r := &doctorResult{}
	checkCrossTargetDiscovery(cfg, r, false)

	if r.warnings != 1 {
		t.Fatalf("expected 1 warning, got %d (checks=%+v)", r.warnings, r.checks)
	}
	detail := r.checks[0].Details[0]
	for _, want := range []string{"codex", "universal", ".agents/skills"} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail %q missing %q", detail, want)
		}
	}
	if len(r.checks[0].Suggestions) != 1 {
		t.Fatalf("expected one suggestion, got %v", r.checks[0].Suggestions)
	}
	suggestion := r.checks[0].Suggestions[0]
	for _, want := range []string{"Choose one authoritative route", "skillshare target remove <name> --global --dry-run", "codex", "universal"} {
		if !strings.Contains(suggestion, want) {
			t.Errorf("suggestion %q missing %q", suggestion, want)
		}
	}
}

func TestCheckCrossTargetDiscovery_NoOverlapWhenScannerAlone(t *testing.T) {
	// codex enabled but universal disabled — no other writer to ~/.agents/skills.
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"codex":  {Skills: &config.ResourceTargetConfig{Path: "~/.codex/skills"}},
			"claude": {Skills: &config.ResourceTargetConfig{Path: "~/.claude/skills"}},
		},
	}
	r := &doctorResult{}
	checkCrossTargetDiscovery(cfg, r, false)

	if r.warnings != 0 {
		t.Errorf("expected 0 warnings, got %d", r.warnings)
	}
	if len(r.checks) != 1 || r.checks[0].Status != checkPass {
		t.Errorf("expected single passing check, got %+v", r.checks)
	}
}

func TestCheckCrossTargetDiscovery_FirebenderMultiOverlap(t *testing.T) {
	// firebender also_scans includes claude, codex, cursor, goose, agents.
	// When all enabled, firebender warns about each overlap.
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"firebender": {Skills: &config.ResourceTargetConfig{Path: "~/.firebender/skills"}},
			"claude":     {Skills: &config.ResourceTargetConfig{Path: "~/.claude/skills"}},
			"codex":      {Skills: &config.ResourceTargetConfig{Path: "~/.codex/skills"}},
		},
	}
	r := &doctorResult{}
	checkCrossTargetDiscovery(cfg, r, false)

	// firebender overlaps with claude AND codex, but grouped into a single
	// per-scanner warning. The details array still contains one entry per
	// (scanner, path) tuple for JSON consumers.
	if r.warnings != 1 {
		t.Errorf("expected 1 warning (grouped per scanner), got %d (checks=%+v)", r.warnings, r.checks)
	}
	if len(r.checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(r.checks))
	}
	if len(r.checks[0].Details) != 2 {
		t.Errorf("expected 2 detail entries (claude path + codex path), got %d: %v", len(r.checks[0].Details), r.checks[0].Details)
	}
}

func TestCheckCrossTargetDiscovery_ProjectMode(t *testing.T) {
	// In project mode, cursor.also_scans.project includes .claude/skills.
	// If claude (project) writes to .claude/skills, cursor overlaps.
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"cursor": {Skills: &config.ResourceTargetConfig{Path: ".agents/skills"}},
			"claude": {Skills: &config.ResourceTargetConfig{Path: ".claude/skills"}},
		},
	}
	r := &doctorResult{}
	checkCrossTargetDiscovery(cfg, r, true)

	if r.warnings == 0 {
		t.Errorf("expected project-mode warning (cursor scans claude's project path), got 0")
	}
}

func TestCheckSharedTargetPaths_EmptyPathSkipped(t *testing.T) {
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"a": {Skills: &config.ResourceTargetConfig{Path: ""}},
			"b": {Skills: &config.ResourceTargetConfig{Path: ""}},
		},
	}
	r := &doctorResult{}
	checkSharedTargetPaths(cfg, r, false)

	if r.warnings != 0 {
		t.Errorf("empty paths must not collide; got %d warnings", r.warnings)
	}
}
