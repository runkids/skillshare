package config

import (
	"testing"
)

func TestMatchesTargetName_SameName(t *testing.T) {
	if !MatchesTargetName("claude", "claude") {
		t.Error("identical names should match")
	}
}

func TestMatchesTargetName_CrossMode(t *testing.T) {
	// "claude" is global_name/project_name, "claude-code" is an alias for the same spec
	if !MatchesTargetName("claude", "claude-code") {
		t.Error("claude should match claude-code (cross-mode)")
	}
	if !MatchesTargetName("claude-code", "claude") {
		t.Error("claude-code should match claude (cross-mode)")
	}
}

func TestMatchesTargetName_DifferentTargets(t *testing.T) {
	if MatchesTargetName("claude", "cursor") {
		t.Error("claude should not match cursor")
	}
}

func TestMatchesTargetName_UnknownName(t *testing.T) {
	if MatchesTargetName("unknown-tool", "claude") {
		t.Error("unknown target should not match claude")
	}
}

func TestMatchesTargetName_SharedProjectPath(t *testing.T) {
	// codex and universal share project_path ".agents/skills"
	if !MatchesTargetName("codex", "universal") {
		t.Error("codex should match universal (shared project path)")
	}
	if !MatchesTargetName("universal", "codex") {
		t.Error("universal should match codex (shared project path)")
	}
}

func TestMatchesTargetName_SharedPath_NoFalsePositive(t *testing.T) {
	// claude and cursor have different paths — must not match via path fallback
	if MatchesTargetName("claude", "cursor") {
		t.Error("claude should not match cursor (different paths)")
	}
}

func TestKnownTargetNames(t *testing.T) {
	names := KnownTargetNames()
	if len(names) == 0 {
		t.Fatal("expected known target names to be non-empty")
	}

	// Verify some well-known names are present
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	for _, want := range []string{"claude", "claude-code", "cursor"} {
		if !found[want] {
			t.Errorf("expected %q in known target names", want)
		}
	}
}
