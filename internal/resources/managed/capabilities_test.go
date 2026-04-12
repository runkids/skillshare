package managed

import "testing"

func TestResolveManagedFamily(t *testing.T) {
	cases := []struct {
		kind      ResourceKind
		target    string
		path      string
		want      string
		wantFound bool
	}{
		{kind: ResourceKindRules, target: "claude", path: ".claude/skills", want: "claude", wantFound: true},
		{kind: ResourceKindRules, target: "pi", path: ".pi/skills", want: "pi", wantFound: true},
		{kind: ResourceKindRules, target: "warp", path: ".agents/skills", want: "codex", wantFound: true},
		{kind: ResourceKindRules, target: "xcode-claude", path: "~/Library/Developer/Xcode/CodingAssistant/ClaudeAgentConfig/skills", want: "claude", wantFound: true},
		{kind: ResourceKindHooks, target: "gemini", path: ".gemini/skills", want: "gemini", wantFound: true},
		{kind: ResourceKindHooks, target: "pi", path: ".pi/skills", wantFound: false},
		{kind: ResourceKindHooks, target: "universal", path: ".agents/skills", want: "codex", wantFound: true},
	}

	for _, tc := range cases {
		got, found := ResolveManagedFamily(tc.kind, tc.target, tc.path)
		if got != tc.want || found != tc.wantFound {
			t.Fatalf("%s %s => (%q, %v), want (%q, %v)", tc.kind, tc.target, got, found, tc.want, tc.wantFound)
		}
	}
}

func TestCapabilitySnapshot_ContainsExhaustiveTargetClassification(t *testing.T) {
	snapshot := CapabilitySnapshot()
	if _, ok := snapshot.Targets["claude"]; !ok {
		t.Fatal("expected claude classification")
	}
	if _, ok := snapshot.Targets["pi"]; !ok {
		t.Fatal("expected pi classification")
	}
	if _, ok := snapshot.Targets["windsurf"]; !ok {
		t.Fatal("expected windsurf classification")
	}
	if got := snapshot.Targets["warp"]; got.RulesFamily != "codex" || got.HooksFamily != "codex" {
		t.Fatalf("warp classification = %#v, want codex family for rules/hooks", got)
	}
	if got := snapshot.Targets["xcode-claude"]; got.RulesFamily != "claude" || got.HooksFamily != "claude" {
		t.Fatalf("xcode-claude classification = %#v, want claude family for rules/hooks", got)
	}
	if got := snapshot.Families["codex"]; !containsString(got.CompatibleTargets, "warp") {
		t.Fatalf("codex compatible targets = %v, want warp included", got.CompatibleTargets)
	}
}
