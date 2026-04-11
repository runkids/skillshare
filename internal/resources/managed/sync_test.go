package managed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	managedhooks "skillshare/internal/resources/hooks"
	managedrules "skillshare/internal/resources/rules"
)

func TestSync_SyncsRulesAndHooksForClaudeTarget(t *testing.T) {
	projectRoot := t.TempDir()
	ensureClaudeTargetFiles(t, projectRoot)

	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "claude/backend.md",
		Content: []byte("# Backend\n"),
	}); err != nil {
		t.Fatalf("put managed rule: %v", err)
	}

	hookStore := managedhooks.NewStore(projectRoot)
	hookID, err := managedhooks.CanonicalRelativePath("claude", "PreToolUse", ".*")
	if err != nil {
		t.Fatalf("canonical hook id: %v", err)
	}
	if _, err := hookStore.Put(managedhooks.Save{
		ID:      hookID,
		Tool:    "claude",
		Event:   "PreToolUse",
		Matcher: ".*",
		Handlers: []managedhooks.Handler{{
			Type:    "command",
			Command: "echo hook",
		}},
	}); err != nil {
		t.Fatalf("put managed hook: %v", err)
	}

	results := Sync(SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true, Hooks: true},
		Targets: []TargetSyncSpec{{
			Name:   "claude",
			Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".claude", "skills")},
		}},
	})

	if got := len(results); got != 2 {
		t.Fatalf("Sync() results len = %d, want 2", got)
	}

	ruleResult := findSyncResult(t, results, "claude", "rules")
	if ruleResult.Err != nil {
		t.Fatalf("rules sync error = %v", ruleResult.Err)
	}
	if !containsAll(ruleResult.Updated, filepath.Join(projectRoot, ".claude", "rules", "backend.md")) {
		t.Fatalf("rules updated = %v, want backend rule output", ruleResult.Updated)
	}

	hookResult := findSyncResult(t, results, "claude", "hooks")
	if hookResult.Err != nil {
		t.Fatalf("hooks sync error = %v", hookResult.Err)
	}
	if !containsAll(hookResult.Updated, filepath.Join(projectRoot, ".claude", "settings.json")) {
		t.Fatalf("hooks updated = %v, want settings.json", hookResult.Updated)
	}

	compiledRule := readFile(t, filepath.Join(projectRoot, ".claude", "rules", "backend.md"))
	if !strings.Contains(compiledRule, "# Backend") {
		t.Fatalf("compiled rule content = %q, want backend content", compiledRule)
	}
	compiledHook := readFile(t, filepath.Join(projectRoot, ".claude", "settings.json"))
	if !strings.Contains(compiledHook, `"hooks"`) {
		t.Fatalf("compiled hook content = %q, want hooks section", compiledHook)
	}
}

func TestSync_ContinuesToHooksAfterRuleFailure(t *testing.T) {
	projectRoot := t.TempDir()
	ensureClaudeTargetFiles(t, projectRoot)

	blocker := filepath.Join(projectRoot, ".claude", "rules")
	if err := os.WriteFile(blocker, []byte("block rules directory creation"), 0o644); err != nil {
		t.Fatalf("create blocker file: %v", err)
	}

	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "claude/backend.md",
		Content: []byte("# Backend\n"),
	}); err != nil {
		t.Fatalf("put managed rule: %v", err)
	}

	hookStore := managedhooks.NewStore(projectRoot)
	hookID, err := managedhooks.CanonicalRelativePath("claude", "PreToolUse", ".*")
	if err != nil {
		t.Fatalf("canonical hook id: %v", err)
	}
	if _, err := hookStore.Put(managedhooks.Save{
		ID:      hookID,
		Tool:    "claude",
		Event:   "PreToolUse",
		Matcher: ".*",
		Handlers: []managedhooks.Handler{{
			Type:    "command",
			Command: "echo hook",
		}},
	}); err != nil {
		t.Fatalf("put managed hook: %v", err)
	}

	results := Sync(SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true, Hooks: true},
		Targets: []TargetSyncSpec{{
			Name:   "claude",
			Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".claude", "skills")},
		}},
	})

	ruleResult := findSyncResult(t, results, "claude", "rules")
	if ruleResult.Err == nil {
		t.Fatal("rules sync error = nil, want failure")
	}

	hookResult := findSyncResult(t, results, "claude", "hooks")
	if hookResult.Err != nil {
		t.Fatalf("hooks sync error = %v, want nil", hookResult.Err)
	}
	if !containsAll(hookResult.Updated, filepath.Join(projectRoot, ".claude", "settings.json")) {
		t.Fatalf("hooks updated = %v, want settings.json", hookResult.Updated)
	}
}

func ensureClaudeTargetFiles(t *testing.T, projectRoot string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".claude", "settings.json"), []byte(`{"profiles":{"default":{"model":"gpt-5"}}}`), 0o644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}
}

func findSyncResult(t *testing.T, results []SyncResult, target, resource string) SyncResult {
	t.Helper()
	for _, result := range results {
		if result.Target == target && result.Resource == resource {
			return result
		}
	}
	t.Fatalf("missing sync result for target=%q resource=%q; results=%#v", target, resource, results)
	return SyncResult{}
}

func containsAll(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
