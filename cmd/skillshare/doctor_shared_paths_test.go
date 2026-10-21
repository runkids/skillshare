package main

import (
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/sync"
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
	// The check prints filepath.Clean output, whose separators are OS-native.
	wantPath := filepath.FromSlash("/tmp/.agents/skills")
	detail := r.checks[0].Details[0]
	for _, want := range []string{"universal", "warp", "witsy", wantPath} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail %q missing %q", detail, want)
		}
	}
	if len(r.checks[0].Suggestions) != 1 {
		t.Fatalf("expected one suggestion, got %v", r.checks[0].Suggestions)
	}
	suggestion := r.checks[0].Suggestions[0]
	for _, want := range []string{"Choose one authoritative target", "skillshare target remove <name> --global --dry-run", "universal", "warp", "witsy", wantPath} {
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
	checkCrossTargetDiscovery(cfg, r, false, nil)

	if r.warnings != 1 {
		t.Fatalf("expected 1 warning, got %d (checks=%+v)", r.warnings, r.checks)
	}
	detail := r.checks[0].Details[0]
	for _, want := range []string{"codex", "universal", filepath.FromSlash(".agents/skills")} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail %q missing %q", detail, want)
		}
	}
	if len(r.checks[0].Suggestions) != 1 {
		t.Fatalf("expected one suggestion, got %v", r.checks[0].Suggestions)
	}
	suggestion := r.checks[0].Suggestions[0]
	// Removing the scanner only affects that runtime; removing the writer would
	// also hide skills from every other tool that reads the shared path.
	for _, want := range []string{"Choose one authoritative route", "skillshare target remove <name> --global --dry-run", "removing the codex target", "universal"} {
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
	checkCrossTargetDiscovery(cfg, r, false, nil)

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
	checkCrossTargetDiscovery(cfg, r, false, nil)

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

func opencodeAndClaude(t *testing.T) *config.Config {
	t.Setenv("HOME", t.TempDir())
	return &config.Config{
		Source: t.TempDir(),
		Mode:   "merge",
		Targets: map[string]config.TargetConfig{
			"opencode": {Skills: &config.ResourceTargetConfig{Path: "~/.config/opencode/skills"}},
			"claude":   {Skills: &config.ResourceTargetConfig{Path: "~/.claude/skills"}},
		},
	}
}

func TestCheckCrossTargetDiscovery_OpenCodeSameSkillsPasses(t *testing.T) {
	// OpenCode reads its own folder last and keeps one skill per name, so a
	// skill synced to both folders loads once.
	r := &doctorResult{}
	checkCrossTargetDiscovery(opencodeAndClaude(t), r, false, []sync.DiscoveredSkill{{FlatName: "shared"}})

	if r.warnings != 0 || r.checks[0].Status != checkPass {
		t.Errorf("expected a passing check, got %+v", r.checks)
	}
}

func TestCheckCrossTargetDiscovery_OpenCodeNamesLeakedSkills(t *testing.T) {
	r := &doctorResult{}
	checkCrossTargetDiscovery(opencodeAndClaude(t), r, false, []sync.DiscoveredSkill{
		{FlatName: "shared"},
		{FlatName: "claude-only", Targets: []string{"claude"}},
	})

	if r.warnings != 1 {
		t.Fatalf("expected 1 warning, got %d (checks=%+v)", r.warnings, r.checks)
	}
	if detail := r.checks[0].Details[0]; !strings.Contains(detail, "loads skills missing from its own folder: claude-only") {
		t.Errorf("detail %q does not name the leaked skill", detail)
	}
	if s := r.checks[0].Suggestions[0]; !strings.Contains(s, "OPENCODE_DISABLE_CLAUDE_CODE_SKILLS=1") {
		t.Errorf("suggestion %q does not say how to stop OpenCode reading .claude/skills", s)
	}
}

func TestCheckCrossTargetDiscovery_OpenCodeAccountNamesLeakedSkills(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfg := &config.Config{
		Source: tmp,
		Mode:   "merge",
		Targets: map[string]config.TargetConfig{
			"opencode-work": {
				Agent:     "opencode",
				ConfigDir: filepath.Join(tmp, "opencode-work"),
				Skills:    &config.ResourceTargetConfig{Path: filepath.Join(tmp, "opencode-work", "skills")},
			},
			"claude": {Skills: &config.ResourceTargetConfig{Path: "~/.claude/skills"}},
		},
	}
	r := &doctorResult{}
	checkCrossTargetDiscovery(cfg, r, false, []sync.DiscoveredSkill{
		{FlatName: "shared"},
		{FlatName: "claude-only", Targets: []string{"claude"}},
	})

	if r.warnings != 1 {
		t.Fatalf("expected 1 warning, got %d (checks=%+v)", r.warnings, r.checks)
	}
	if detail := r.checks[0].Details[0]; !strings.Contains(detail, "loads skills missing from its own folder: claude-only") {
		t.Errorf("detail %q does not name the leaked skill", detail)
	}
	if s := r.checks[0].Suggestions[0]; !strings.Contains(s, "OPENCODE_DISABLE_CLAUDE_CODE_SKILLS=1") {
		t.Errorf("suggestion %q does not say how to stop OpenCode reading .claude/skills", s)
	}
}

func TestCheckCrossTargetDiscovery_CodexAccountSeesUniversal(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfg := &config.Config{
		Source: tmp,
		Mode:   "merge",
		Targets: map[string]config.TargetConfig{
			"codex-work": {
				Agent:     "codex",
				ConfigDir: filepath.Join(tmp, "codex-work"),
				Skills:    &config.ResourceTargetConfig{Path: filepath.Join(tmp, "codex-work", "skills")},
			},
			"universal": {Skills: &config.ResourceTargetConfig{Path: "~/.agents/skills"}},
		},
	}
	r := &doctorResult{}
	checkCrossTargetDiscovery(cfg, r, false, nil)

	if r.warnings != 1 {
		t.Fatalf("expected 1 warning, got %d (checks=%+v)", r.warnings, r.checks)
	}
	if len(r.checks[0].Details) == 0 || !strings.Contains(r.checks[0].Details[0], "universal") {
		t.Errorf("expected warning mentioning universal, got %+v", r.checks[0].Details)
	}
}

func TestCheckCrossTargetDiscovery_OpenCodeEnvOffPassesWithNote(t *testing.T) {
	cfg := opencodeAndClaude(t)
	t.Setenv("OPENCODE_DISABLE_CLAUDE_CODE_SKILLS", "1")
	r := &doctorResult{}
	checkCrossTargetDiscovery(cfg, r, false, []sync.DiscoveredSkill{
		{FlatName: "claude-only", Targets: []string{"claude"}},
	})

	if r.warnings != 0 || r.checks[0].Status != checkPass {
		t.Fatalf("expected a passing check, got %+v", r.checks)
	}
	if d := strings.Join(r.checks[0].Details, "\n"); !strings.Contains(d, "OPENCODE_DISABLE_CLAUDE_CODE_SKILLS is set") {
		t.Errorf("details %q do not say why ~/.claude/skills is skipped", d)
	}
}

func TestCheckCrossTargetDiscovery_OpenCodeAgentsLeakOnlySuggestsExternalOff(t *testing.T) {
	// OPENCODE_DISABLE_CLAUDE_CODE_SKILLS leaves .agents/skills loaded.
	cfg := opencodeAndClaude(t)
	cfg.Targets["universal"] = config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: "~/.agents/skills"}}
	r := &doctorResult{}
	checkCrossTargetDiscovery(cfg, r, false, []sync.DiscoveredSkill{
		{FlatName: "shared"},
		{FlatName: "universal-only", Targets: []string{"universal"}},
	})

	s := r.checks[0].Suggestions[0]
	if !strings.Contains(s, "OPENCODE_DISABLE_EXTERNAL_SKILLS=1") || strings.Contains(s, "OPENCODE_DISABLE_CLAUDE_CODE_SKILLS") {
		t.Errorf("suggestion %q should offer only OPENCODE_DISABLE_EXTERNAL_SKILLS=1", s)
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
	checkCrossTargetDiscovery(cfg, r, true, nil)

	if r.warnings == 0 {
		t.Errorf("expected project-mode warning (cursor scans claude's project path), got 0")
	}
}

func TestCheckCrossTargetDiscovery_IgnoresScannerOwnPath(t *testing.T) {
	// warp and universal share ~/.agents/skills, which is also warp's own
	// runtime scan path. shared_target_paths already reports that collision —
	// cross_target_discovery must not double-report it.
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"universal": {Skills: &config.ResourceTargetConfig{Path: "~/.agents/skills"}},
			"warp":      {Skills: &config.ResourceTargetConfig{Path: "~/.agents/skills"}},
		},
	}
	r := &doctorResult{}
	checkCrossTargetDiscovery(cfg, r, false, nil)

	if r.warnings != 0 {
		t.Errorf("expected 0 warnings, got %d (checks=%+v)", r.warnings, r.checks)
	}
}

func TestCheckCodexOnUniversalPath_ReportedOnceAsSharedPath(t *testing.T) {
	// A fresh init points codex at universal's ~/.agents/skills. That is a
	// shared-primary collision, so shared_target_paths owns the warning and
	// cross_target_discovery must not repeat it.
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"codex":     {Skills: &config.ResourceTargetConfig{Path: "~/.agents/skills"}},
			"universal": {Skills: &config.ResourceTargetConfig{Path: "~/.agents/skills"}},
		},
	}

	shared := &doctorResult{}
	checkSharedTargetPaths(cfg, shared, false)
	if shared.warnings != 1 {
		t.Fatalf("expected 1 shared_target_paths warning, got %d (checks=%+v)", shared.warnings, shared.checks)
	}

	cross := &doctorResult{}
	checkCrossTargetDiscovery(cfg, cross, false, nil)
	if cross.warnings != 0 {
		t.Errorf("expected 0 cross_target_discovery warnings, got %d (checks=%+v)", cross.warnings, cross.checks)
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

func TestDoctorPathChecks_IgnoreSkillsOffTargets(t *testing.T) {
	off := false
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"codex":     {Skills: &config.ResourceTargetConfig{Path: "~/.agents/skills", Enabled: &off}},
			"gemini":    {Skills: &config.ResourceTargetConfig{Path: "~/.gemini/skills", Enabled: &off}},
			"universal": {Skills: &config.ResourceTargetConfig{Path: "~/.agents/skills"}},
		},
	}
	r := &doctorResult{}
	checkSharedTargetPaths(cfg, r, false)
	checkCrossTargetDiscovery(cfg, r, false, nil)

	if r.warnings != 0 {
		t.Errorf("targets with skills off should not overlap, got %d warning(s): %+v", r.warnings, r.checks)
	}
}

func TestCheckSharedTargetPaths_DifferentFiltersSuggestsSkillsOff(t *testing.T) {
	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"universal": {Skills: &config.ResourceTargetConfig{Path: "/tmp/.agents/skills", Exclude: []string{"feature-radar*"}}},
			"codex":     {Skills: &config.ResourceTargetConfig{Path: "/tmp/.agents/skills"}},
		},
	}
	r := &doctorResult{}
	checkSharedTargetPaths(cfg, r, true)

	if len(r.checks) != 1 || r.checks[0].Status != checkWarning {
		t.Fatalf("expected single warning check, got %+v", r.checks)
	}
	if detail := r.checks[0].Details[0]; !strings.Contains(detail, "undo each other") {
		t.Errorf("detail %q should say the targets undo each other", detail)
	}
	suggestion := r.checks[0].Suggestions[0]
	if !strings.Contains(suggestion, "skillshare target codex --skills=false -p") {
		t.Errorf("suggestion %q should turn skills off for codex", suggestion)
	}
	if strings.Contains(suggestion, "target remove") {
		t.Errorf("suggestion %q should not suggest removing a target", suggestion)
	}
}
