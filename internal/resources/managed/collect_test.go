package managed

import (
	"path/filepath"
	"testing"

	"skillshare/internal/inspect"
	managedhooks "skillshare/internal/resources/hooks"
	managedrules "skillshare/internal/resources/rules"
)

func TestPreviewCollectRules(t *testing.T) {
	projectRoot := t.TempDir()

	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "claude/CLAUDE.md",
		Content: []byte("old root\n"),
	}); err != nil {
		t.Fatalf("seed managed rule: %v", err)
	}
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "claude/backend.md",
		Content: []byte("old backend\n"),
	}); err != nil {
		t.Fatalf("seed managed rule: %v", err)
	}

	items := []inspect.RuleItem{
		{
			SourceTool:  "claude",
			Collectible: true,
			Path:        filepath.Join(projectRoot, "CLAUDE.md"),
		},
		{
			SourceTool:  "claude",
			Collectible: true,
			Path:        filepath.Join(projectRoot, ".claude", "rules", "backend.md"),
		},
	}

	t.Run("skips existing without force", func(t *testing.T) {
		result, err := PreviewCollectRules(projectRoot, items, false)
		if err != nil {
			t.Fatalf("PreviewCollectRules() error = %v", err)
		}

		wantRootID, err := managedRuleCollectID(items[0])
		if err != nil {
			t.Fatalf("managedRuleCollectID(root) error = %v", err)
		}
		wantBackendID, err := managedRuleCollectID(items[1])
		if err != nil {
			t.Fatalf("managedRuleCollectID(backend) error = %v", err)
		}

		if !containsString(result.Skipped, wantRootID) {
			t.Fatalf("PreviewCollectRules() skipped = %v, want %q", result.Skipped, wantRootID)
		}
		if !containsString(result.Skipped, wantBackendID) {
			t.Fatalf("PreviewCollectRules() skipped = %v, want %q", result.Skipped, wantBackendID)
		}
		if len(result.Pulled) != 0 {
			t.Fatalf("PreviewCollectRules() pulled = %v, want none", result.Pulled)
		}
	})

	t.Run("pulls existing with force", func(t *testing.T) {
		result, err := PreviewCollectRules(projectRoot, items, true)
		if err != nil {
			t.Fatalf("PreviewCollectRules() error = %v", err)
		}

		wantRootID, err := managedRuleCollectID(items[0])
		if err != nil {
			t.Fatalf("managedRuleCollectID(root) error = %v", err)
		}
		wantBackendID, err := managedRuleCollectID(items[1])
		if err != nil {
			t.Fatalf("managedRuleCollectID(backend) error = %v", err)
		}

		if !containsString(result.Pulled, wantRootID) || !containsString(result.Pulled, wantBackendID) {
			t.Fatalf("PreviewCollectRules() pulled = %v, want both canonical ids", result.Pulled)
		}
	})

	t.Run("rejects canonical collisions", func(t *testing.T) {
		colliding := []inspect.RuleItem{
			{
				SourceTool:  "claude",
				Collectible: true,
				Path:        filepath.Join(projectRoot, "alpha", ".claude", "rules", "backend.md"),
			},
			{
				SourceTool:  "claude",
				Collectible: true,
				Path:        filepath.Join(projectRoot, "beta", ".claude", "rules", "backend.md"),
			},
		}
		if _, err := PreviewCollectRules(projectRoot, colliding, false); err == nil {
			t.Fatal("PreviewCollectRules() error = nil, want canonical collision failure")
		}
	})

	t.Run("rejects non collectible rules like collect", func(t *testing.T) {
		discovered := []inspect.RuleItem{
			{
				SourceTool:  "claude",
				Collectible: true,
				Path:        filepath.Join(projectRoot, ".claude", "rules", "backend.md"),
				Content:     "# Backend\n",
			},
			{
				SourceTool:    "claude",
				Collectible:   false,
				CollectReason: "blocked by policy",
				Path:          filepath.Join(projectRoot, ".claude", "rules", "blocked.md"),
				Content:       "# Blocked\n",
			},
		}

		previewResult, previewErr := PreviewCollectRules(projectRoot, discovered, true)
		if previewErr == nil {
			t.Fatalf("PreviewCollectRules() error = nil, want non-collectible failure; result=%#v", previewResult)
		}

		collectResult, collectErr := CollectRules(projectRoot, discovered, managedrules.StrategyOverwrite)
		if collectErr == nil {
			t.Fatalf("CollectRules() error = nil, want non-collectible failure; result=%#v", collectResult)
		}
		if previewErr.Error() != collectErr.Error() {
			t.Fatalf("preview error = %q, collect error = %q, want parity", previewErr.Error(), collectErr.Error())
		}
	})

	t.Run("rejects unsupported pi paths early like collect", func(t *testing.T) {
		discovered := []inspect.RuleItem{
			{
				SourceTool:  "pi",
				Collectible: true,
				Path:        filepath.Join(projectRoot, ".pi", "extra.md"),
				Content:     "# Extra\n",
			},
		}

		previewResult, previewErr := PreviewCollectRules(projectRoot, discovered, true)
		if previewErr == nil {
			t.Fatalf("PreviewCollectRules() error = nil, want unsupported pi path failure; result=%#v", previewResult)
		}

		collectResult, collectErr := CollectRules(projectRoot, discovered, managedrules.StrategyOverwrite)
		if collectErr == nil {
			t.Fatalf("CollectRules() error = nil, want unsupported pi path failure; result=%#v", collectResult)
		}
		if previewErr.Error() != collectErr.Error() {
			t.Fatalf("preview error = %q, collect error = %q, want parity", previewErr.Error(), collectErr.Error())
		}
	})
}

func TestPreviewCollectHooks(t *testing.T) {
	projectRoot := t.TempDir()

	hookStore := managedhooks.NewStore(projectRoot)
	existingID, err := managedHookCollectID("claude", "PreToolUse", ".*")
	if err != nil {
		t.Fatalf("managedHookCollectID() error = %v", err)
	}
	if _, err := hookStore.Put(managedhooks.Save{
		ID:      existingID,
		Tool:    "claude",
		Event:   "PreToolUse",
		Matcher: ".*",
		Handlers: []managedhooks.Handler{{
			Type:    "command",
			Command: "old hook",
		}},
	}); err != nil {
		t.Fatalf("seed managed hook: %v", err)
	}

	items := []inspect.HookItem{
		{
			GroupID:     "group-1",
			SourceTool:  "claude",
			Collectible: true,
			Event:       "PreToolUse",
			Matcher:     ".*",
			ActionType:  "command",
			Path:        filepath.Join(projectRoot, ".claude", "settings.json"),
		},
	}

	result, err := PreviewCollectHooks(projectRoot, items, false)
	if err != nil {
		t.Fatalf("PreviewCollectHooks() error = %v", err)
	}
	if !containsString(result.Skipped, existingID) {
		t.Fatalf("PreviewCollectHooks() skipped = %v, want %q", result.Skipped, existingID)
	}
	if len(result.Pulled) != 0 {
		t.Fatalf("PreviewCollectHooks() pulled = %v, want none", result.Pulled)
	}

	forced, err := PreviewCollectHooks(projectRoot, items, true)
	if err != nil {
		t.Fatalf("PreviewCollectHooks(force) error = %v", err)
	}
	if !containsString(forced.Pulled, existingID) {
		t.Fatalf("PreviewCollectHooks(force) pulled = %v, want %q", forced.Pulled, existingID)
	}

	t.Run("rejects invalid hook grouping like collect", func(t *testing.T) {
		discovered := []inspect.HookItem{
			{
				GroupID:     "group-mismatch",
				SourceTool:  "claude",
				Collectible: true,
				Event:       "PreToolUse",
				Matcher:     "Edit",
				ActionType:  "command",
				Command:     "./first",
				Path:        filepath.Join(projectRoot, ".claude", "settings.json"),
			},
			{
				GroupID:     "group-mismatch",
				SourceTool:  "claude",
				Collectible: true,
				Event:       "PreToolUse",
				Matcher:     "Bash",
				ActionType:  "command",
				Command:     "./second",
				Path:        filepath.Join(projectRoot, ".claude", "settings.json"),
			},
		}

		previewResult, previewErr := PreviewCollectHooks(projectRoot, discovered, true)
		if previewErr == nil {
			t.Fatalf("PreviewCollectHooks() error = nil, want grouping failure; result=%#v", previewResult)
		}

		collectResult, collectErr := CollectHooks(projectRoot, discovered, managedhooks.StrategyOverwrite)
		if collectErr == nil {
			t.Fatalf("CollectHooks() error = nil, want grouping failure; result=%#v", collectResult)
		}
		if previewErr.Error() != collectErr.Error() {
			t.Fatalf("preview error = %q, collect error = %q, want parity", previewErr.Error(), collectErr.Error())
		}
	})

	t.Run("rejects missing matcher for non codex groups like collect", func(t *testing.T) {
		discovered := []inspect.HookItem{
			{
				GroupID:     "group-missing-matcher",
				SourceTool:  "claude",
				Collectible: true,
				Event:       "PreToolUse",
				Matcher:     "",
				ActionType:  "command",
				Command:     "./missing-matcher",
				Path:        filepath.Join(projectRoot, ".claude", "settings.json"),
			},
		}

		previewResult, previewErr := PreviewCollectHooks(projectRoot, discovered, true)
		if previewErr == nil {
			t.Fatalf("PreviewCollectHooks() error = nil, want missing matcher failure; result=%#v", previewResult)
		}

		collectResult, collectErr := CollectHooks(projectRoot, discovered, managedhooks.StrategyOverwrite)
		if collectErr == nil {
			t.Fatalf("CollectHooks() error = nil, want missing matcher failure; result=%#v", collectResult)
		}
		if previewErr.Error() != collectErr.Error() {
			t.Fatalf("preview error = %q, collect error = %q, want parity", previewErr.Error(), collectErr.Error())
		}
	})
}

func TestCollectRules(t *testing.T) {
	projectRoot := t.TempDir()

	store := managedrules.NewStore(projectRoot)
	id := "claude/backend.md"
	if _, err := store.Put(managedrules.Save{
		ID:      id,
		Content: []byte("old backend\n"),
	}); err != nil {
		t.Fatalf("seed managed rule: %v", err)
	}

	items := []inspect.RuleItem{
		{
			SourceTool:  "claude",
			Collectible: true,
			Path:        filepath.Join(projectRoot, ".claude", "rules", "backend.md"),
			Content:     "new backend\n",
		},
	}

	result, err := CollectRules(projectRoot, items, managedrules.StrategyOverwrite)
	if err != nil {
		t.Fatalf("CollectRules() error = %v", err)
	}

	wantID, err := managedRuleCollectID(items[0])
	if err != nil {
		t.Fatalf("managedRuleCollectID() error = %v", err)
	}
	if !containsString(result.Overwritten, wantID) {
		t.Fatalf("CollectRules() overwritten = %v, want %q", result.Overwritten, wantID)
	}

	record, err := store.Get(wantID)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if string(record.Content) != items[0].Content {
		t.Fatalf("store.Get() content = %q, want %q", string(record.Content), items[0].Content)
	}
}

func TestCollectHooks(t *testing.T) {
	projectRoot := t.TempDir()

	store := managedhooks.NewStore(projectRoot)
	id, err := managedHookCollectID("claude", "PreToolUse", ".*")
	if err != nil {
		t.Fatalf("managedHookCollectID() error = %v", err)
	}
	if _, err := store.Put(managedhooks.Save{
		ID:      id,
		Tool:    "claude",
		Event:   "PreToolUse",
		Matcher: ".*",
		Handlers: []managedhooks.Handler{{
			Type:    "command",
			Command: "old hook",
		}},
	}); err != nil {
		t.Fatalf("seed managed hook: %v", err)
	}

	items := []inspect.HookItem{
		{
			GroupID:     "group-1",
			SourceTool:  "claude",
			Event:       "PreToolUse",
			Matcher:     ".*",
			ActionType:  "command",
			Path:        filepath.Join(projectRoot, ".claude", "settings.json"),
			Command:     "new hook",
			Collectible: true,
		},
	}

	result, err := CollectHooks(projectRoot, items, managedhooks.StrategyOverwrite)
	if err != nil {
		t.Fatalf("CollectHooks() error = %v", err)
	}
	if !containsString(result.Overwritten, id) {
		t.Fatalf("CollectHooks() overwritten = %v, want %q", result.Overwritten, id)
	}

	record, err := store.Get(id)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if len(record.Handlers) != 1 {
		t.Fatalf("store.Get() handlers = %d, want 1", len(record.Handlers))
	}
	if record.Handlers[0].Command != "new hook" {
		t.Fatalf("store.Get() command = %q, want %q", record.Handlers[0].Command, "new hook")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
