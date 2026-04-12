package rules

import (
	"errors"
	"os"
	"strings"
	"testing"

	"skillshare/internal/inspect"
)

func TestCollectRules_OverwriteAndDuplicate(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	_, err := store.Put(Save{
		ID:         "claude/backend.md",
		Content:    []byte("# Existing\n"),
		Targets:    []string{"gemini"},
		SourceType: "tracked",
		Disabled:   true,
	})
	if err != nil {
		t.Fatalf("seed Put() error = %v", err)
	}

	discovered := []inspect.RuleItem{
		{
			Name:        "backend.md",
			SourceTool:  "claude",
			Scope:       inspect.ScopeProject,
			Path:        "/tmp/project/.claude/rules/backend.md",
			Content:     "# Backend\n",
			Collectible: true,
		},
	}

	result, err := Collect(projectRoot, discovered, CollectOptions{Strategy: StrategyDuplicate})
	if err != nil {
		t.Fatalf("Collect(duplicate) error = %v", err)
	}
	if len(result.Created) != 1 || result.Created[0] != "claude/backend-copy.md" {
		t.Fatalf("Collect(duplicate) Created = %v, want [claude/backend-copy.md]", result.Created)
	}

	original, err := store.Get("claude/backend.md")
	if err != nil {
		t.Fatalf("Get(original) error = %v", err)
	}
	if string(original.Content) != "# Existing\n" {
		t.Fatalf("original content = %q, want %q", string(original.Content), "# Existing\n")
	}

	copyRule, err := store.Get("claude/backend-copy.md")
	if err != nil {
		t.Fatalf("Get(copy) error = %v", err)
	}
	if string(copyRule.Content) != "# Backend\n" {
		t.Fatalf("copy content = %q, want %q", string(copyRule.Content), "# Backend\n")
	}
	if copyRule.Targets != nil {
		t.Fatalf("copy targets = %v, want nil", copyRule.Targets)
	}
	if copyRule.SourceType != "local" {
		t.Fatalf("copy sourceType = %q, want %q", copyRule.SourceType, "local")
	}
	if copyRule.Disabled {
		t.Fatalf("copy disabled = %v, want false", copyRule.Disabled)
	}

	discovered[0].Content = "# Overwritten\n"
	result, err = Collect(projectRoot, discovered, CollectOptions{Strategy: StrategyOverwrite})
	if err != nil {
		t.Fatalf("Collect(overwrite) error = %v", err)
	}
	if len(result.Overwritten) != 1 || result.Overwritten[0] != "claude/backend.md" {
		t.Fatalf("Collect(overwrite) Overwritten = %v, want [claude/backend.md]", result.Overwritten)
	}

	overwritten, err := store.Get("claude/backend.md")
	if err != nil {
		t.Fatalf("Get(overwritten) error = %v", err)
	}
	if string(overwritten.Content) != "# Overwritten\n" {
		t.Fatalf("overwritten content = %q, want %q", string(overwritten.Content), "# Overwritten\n")
	}
	if len(overwritten.Targets) != 1 || overwritten.Targets[0] != "gemini" {
		t.Fatalf("overwritten targets = %v, want [gemini]", overwritten.Targets)
	}
	if overwritten.SourceType != "tracked" {
		t.Fatalf("overwritten sourceType = %q, want %q", overwritten.SourceType, "tracked")
	}
	if !overwritten.Disabled {
		t.Fatalf("overwritten disabled = %v, want true", overwritten.Disabled)
	}

	discovered[0].Content = "# ShouldSkip\n"
	result, err = Collect(projectRoot, discovered, CollectOptions{Strategy: StrategySkip})
	if err != nil {
		t.Fatalf("Collect(skip) error = %v", err)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != "claude/backend.md" {
		t.Fatalf("Collect(skip) Skipped = %v, want [claude/backend.md]", result.Skipped)
	}

	skipped, err := store.Get("claude/backend.md")
	if err != nil {
		t.Fatalf("Get(skipped) error = %v", err)
	}
	if string(skipped.Content) != "# Overwritten\n" {
		t.Fatalf("skipped content = %q, want %q", string(skipped.Content), "# Overwritten\n")
	}
}

func TestCollectRules_DoesNotPartiallyWriteOnLaterFailure(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	discovered := []inspect.RuleItem{
		{
			Name:        "backend.md",
			SourceTool:  "claude",
			Scope:       inspect.ScopeProject,
			Path:        "/tmp/project/.claude/rules/backend.md",
			Content:     "# Backend\n",
			Collectible: true,
		},
		{
			Name:          "blocked.md",
			SourceTool:    "claude",
			Scope:         inspect.ScopeProject,
			Path:          "/tmp/project/.claude/rules/blocked.md",
			Content:       "# Blocked\n",
			Collectible:   false,
			CollectReason: "blocked by policy",
		},
	}

	_, err := Collect(projectRoot, discovered, CollectOptions{Strategy: StrategyOverwrite})
	if err == nil {
		t.Fatal("Collect() error = nil, want non-collectible error")
	}

	_, err = store.Get("claude/backend.md")
	if !os.IsNotExist(err) {
		t.Fatalf("Get(claude/backend.md) error = %v, want not-exist", err)
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("List() len = %d, want 0", len(all))
	}
}

func TestCollectRules_RejectsCanonicalManagedIDCollisions(t *testing.T) {
	projectRoot := t.TempDir()

	discovered := []inspect.RuleItem{
		{
			Name:        "CLAUDE.md",
			SourceTool:  "claude",
			Scope:       inspect.ScopeProject,
			Path:        "/tmp/project/CLAUDE.md",
			Content:     "# Root\n",
			Collectible: true,
		},
		{
			Name:        "CLAUDE.md",
			SourceTool:  "claude",
			Scope:       inspect.ScopeProject,
			Path:        "/tmp/project/.claude/CLAUDE.md",
			Content:     "# Nested\n",
			Collectible: true,
		},
	}

	_, err := Collect(projectRoot, discovered, CollectOptions{Strategy: StrategyOverwrite})
	if err == nil {
		t.Fatal("expected collect collision error")
	}
	if !errors.Is(err, ErrInvalidCollect) {
		t.Fatalf("Collect() error = %v, want ErrInvalidCollect", err)
	}
	if !strings.Contains(err.Error(), "canonical managed id") {
		t.Fatalf("Collect() error = %v, want canonical managed id collision", err)
	}

	store := NewStore(projectRoot)
	all, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("List() len = %d, want 0 after collision failure", len(all))
	}
}

func TestCollectRules_RollsBackOnMidApplyWriteFailure(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	_, err := store.Put(Save{
		ID:      "claude/existing-one.md",
		Content: []byte("# Original One\n"),
	})
	if err != nil {
		t.Fatalf("seed Put() error = %v", err)
	}
	_, err = store.Put(Save{
		ID:      "claude/existing-two.md",
		Content: []byte("# Original Two\n"),
	})
	if err != nil {
		t.Fatalf("second seed Put() error = %v", err)
	}

	discovered := []inspect.RuleItem{
		{
			Name:        "existing-one.md",
			SourceTool:  "claude",
			Scope:       inspect.ScopeProject,
			Path:        "/tmp/project/.claude/rules/existing-one.md",
			Content:     "# Updated One\n",
			Collectible: true,
		},
		{
			Name:        "existing-two.md",
			SourceTool:  "claude",
			Scope:       inspect.ScopeProject,
			Path:        "/tmp/project/.claude/rules/existing-two.md",
			Content:     "# Updated Two\n",
			Collectible: true,
		},
	}

	origWrite := ruleWriteFile
	defer func() { ruleWriteFile = origWrite }()

	writeCalls := 0
	ruleWriteFile = func(name string, data []byte, perm os.FileMode) error {
		writeCalls++
		if writeCalls == 2 {
			// Simulate a partial current write before failure.
			_ = origWrite(name, []byte("# CORRUPT\n"), perm)
			return errors.New("injected write failure")
		}
		return origWrite(name, data, perm)
	}

	_, err = Collect(projectRoot, discovered, CollectOptions{Strategy: StrategyOverwrite})
	if err == nil {
		t.Fatal("Collect() error = nil, want injected write failure")
	}

	existingOne, err := store.Get("claude/existing-one.md")
	if err != nil {
		t.Fatalf("Get(existing-one) error = %v", err)
	}
	if string(existingOne.Content) != "# Original One\n" {
		t.Fatalf("existing-one content = %q, want %q", string(existingOne.Content), "# Original One\n")
	}

	existingTwo, err := store.Get("claude/existing-two.md")
	if err != nil {
		t.Fatalf("Get(existing-two) error = %v", err)
	}
	if string(existingTwo.Content) != "# Original Two\n" {
		t.Fatalf("existing-two content = %q, want %q", string(existingTwo.Content), "# Original Two\n")
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List() len = %d, want 2", len(all))
	}
	if all[0].ID != "claude/existing-one.md" {
		t.Fatalf("List()[0].ID = %q, want %q", all[0].ID, "claude/existing-one.md")
	}
	if string(all[0].Content) != "# Original One\n" {
		t.Fatalf("List()[0].Content = %q, want %q", string(all[0].Content), "# Original One\n")
	}
	if all[1].ID != "claude/existing-two.md" {
		t.Fatalf("List()[1].ID = %q, want %q", all[1].ID, "claude/existing-two.md")
	}
	if string(all[1].Content) != "# Original Two\n" {
		t.Fatalf("List()[1].Content = %q, want %q", string(all[1].Content), "# Original Two\n")
	}
}

func TestManagedIDForDiscoveredRule_PiFamily(t *testing.T) {
	tests := []struct {
		name string
		item inspect.RuleItem
		want string
	}{
		{
			name: "pi agents root",
			item: inspect.RuleItem{
				SourceTool: "pi",
				Path:       "/tmp/project/AGENTS.md",
			},
			want: "pi/AGENTS.md",
		},
		{
			name: "pi system file",
			item: inspect.RuleItem{
				SourceTool: "pi",
				Path:       "/tmp/project/.pi/SYSTEM.md",
			},
			want: "pi/SYSTEM.md",
		},
		{
			name: "pi append system file",
			item: inspect.RuleItem{
				SourceTool: "pi",
				Path:       "/tmp/project/.pi/APPEND_SYSTEM.md",
			},
			want: "pi/APPEND_SYSTEM.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ManagedIDForDiscoveredRule(tt.item)
			if err != nil {
				t.Fatalf("ManagedIDForDiscoveredRule() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ManagedIDForDiscoveredRule() = %q, want %q", got, tt.want)
			}
		})
	}
}
