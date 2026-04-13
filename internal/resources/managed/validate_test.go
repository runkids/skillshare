package managed

import "testing"

func TestValidateManagedRuleSave_AllowsPiRules(t *testing.T) {
	err := ValidateManagedRuleSave(RuleInput{
		Tool:         "pi",
		RelativePath: "pi/SYSTEM.md",
		Content:      []byte("# Pi\n"),
	})
	if err != nil {
		t.Fatalf("expected pi rules to validate, got %v", err)
	}
}

func TestValidateManagedHookSave_RejectsPiHooks(t *testing.T) {
	err := ValidateManagedHookSave(HookInput{
		Tool:    "pi",
		Event:   "PreToolUse",
		Matcher: "Read",
		Handlers: []HookHandlerInput{{
			Type:    "command",
			Command: "./bin/check",
		}},
	})
	if err == nil {
		t.Fatal("expected pi hooks validation error")
	}
	if got := err.Error(); got != `tool "pi" does not support managed hooks` {
		t.Fatalf("validation error = %q, want managed family support error", got)
	}
}
