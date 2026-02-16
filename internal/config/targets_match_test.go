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
