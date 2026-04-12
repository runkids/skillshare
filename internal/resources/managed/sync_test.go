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

func TestSync_HonorsAssignedTargetsAndDisabledState(t *testing.T) {
	xdgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	workRoot := filepath.Join(t.TempDir(), "work")
	personalRoot := filepath.Join(t.TempDir(), "personal")
	ensureClaudeTargetFiles(t, workRoot)
	ensureClaudeTargetFiles(t, personalRoot)

	ruleStore := managedrules.NewStore("")
	if _, err := ruleStore.Put(managedrules.Save{
		ID:         "claude/backend.md",
		Content:    []byte("# Backend\n"),
		Targets:    []string{"claude-work"},
		SourceType: "local",
	}); err != nil {
		t.Fatalf("put managed rule: %v", err)
	}

	hookStore := managedhooks.NewStore("")
	hookID, err := managedhooks.CanonicalRelativePath("claude", "PreToolUse", "Bash")
	if err != nil {
		t.Fatalf("canonical hook id: %v", err)
	}
	if _, err := hookStore.Put(managedhooks.Save{
		ID:         hookID,
		Tool:       "claude",
		Event:      "PreToolUse",
		Matcher:    "Bash",
		Targets:    []string{"claude-work"},
		SourceType: "local",
		Disabled:   true,
		Handlers: []managedhooks.Handler{{
			Type:    "command",
			Command: "echo hook",
		}},
	}); err != nil {
		t.Fatalf("put managed hook: %v", err)
	}

	results := Sync(SyncRequest{
		DryRun:    false,
		Resources: ResourceSet{Rules: true, Hooks: true},
		Targets: []TargetSyncSpec{
			{
				Name:   "claude-work",
				Target: config.TargetConfig{Path: filepath.Join(workRoot, ".claude", "skills")},
			},
			{
				Name:   "claude-personal",
				Target: config.TargetConfig{Path: filepath.Join(personalRoot, ".claude", "skills")},
			},
		},
	})

	workRuleResult := findSyncResult(t, results, "claude-work", "rules")
	if workRuleResult.Err != nil {
		t.Fatalf("work rules sync error = %v", workRuleResult.Err)
	}
	if !containsAll(workRuleResult.Updated, filepath.Join(workRoot, ".claude", "rules", "backend.md")) {
		t.Fatalf("work rules updated = %v, want backend rule output", workRuleResult.Updated)
	}

	personalRuleResult := findSyncResult(t, results, "claude-personal", "rules")
	if personalRuleResult.Err != nil {
		t.Fatalf("personal rules sync error = %v", personalRuleResult.Err)
	}
	if len(personalRuleResult.Updated) != 0 {
		t.Fatalf("personal rules updated = %v, want none", personalRuleResult.Updated)
	}

	workHookResult := findSyncResult(t, results, "claude-work", "hooks")
	if workHookResult.Err != nil {
		t.Fatalf("work hooks sync error = %v", workHookResult.Err)
	}
	if !containsAll(workHookResult.Updated, filepath.Join(workRoot, ".claude", "settings.json")) {
		t.Fatalf("work hooks updated = %v, want empty carrier settings output", workHookResult.Updated)
	}

	if _, err := os.Stat(filepath.Join(workRoot, ".claude", "rules", "backend.md")); err != nil {
		t.Fatalf("expected work rule output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(personalRoot, ".claude", "rules", "backend.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no personal rule output, got err=%v", err)
	}
	workHookConfig := readFile(t, filepath.Join(workRoot, ".claude", "settings.json"))
	if strings.Contains(workHookConfig, "echo hook") {
		t.Fatalf("work hook config = %q, want disabled hook omitted", workHookConfig)
	}
}

func TestSync_SkipsGeminiHooksUntilSupported(t *testing.T) {
	projectRoot := t.TempDir()

	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "gemini/backend.md",
		Content: []byte("# Backend\n"),
	}); err != nil {
		t.Fatalf("put managed gemini rule: %v", err)
	}

	results := Sync(SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true, Hooks: true},
		Targets: []TargetSyncSpec{{
			Name:   "gemini",
			Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".gemini", "skills")},
		}},
	})

	if got := len(results); got != 1 {
		t.Fatalf("Sync() results len = %d, want 1 rule result and no hook result", got)
	}

	ruleResult := findSyncResult(t, results, "gemini", "rules")
	if ruleResult.Err != nil {
		t.Fatalf("gemini rules sync error = %v", ruleResult.Err)
	}
	if !containsAll(ruleResult.Updated, filepath.Join(projectRoot, ".gemini", "rules", "backend.md")) {
		t.Fatalf("gemini rules updated = %v, want backend rule output", ruleResult.Updated)
	}
}

func TestSync_PrunesDeletedPiRuleOutputs(t *testing.T) {
	xdgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	homeRoot := filepath.Join(t.TempDir(), "home")
	ruleStore := managedrules.NewStore("")
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "pi/SYSTEM.md",
		Content: []byte("# Pi System\n"),
	}); err != nil {
		t.Fatalf("put managed pi rule: %v", err)
	}

	targetPath := filepath.Join(homeRoot, ".pi", "agent", "skills")
	req := SyncRequest{
		DryRun:    false,
		Resources: ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{{
			Name:   "pi",
			Target: config.TargetConfig{Path: targetPath},
		}},
	}

	first := Sync(req)
	firstResult := findSyncResult(t, first, "pi", "rules")
	if firstResult.Err != nil {
		t.Fatalf("first pi rules sync error = %v", firstResult.Err)
	}

	compiledPath := filepath.Join(homeRoot, ".pi", "agent", "SYSTEM.md")
	if _, err := os.Stat(compiledPath); err != nil {
		t.Fatalf("expected compiled pi rule at %s: %v", compiledPath, err)
	}

	if err := ruleStore.Delete("pi/SYSTEM.md"); err != nil {
		t.Fatalf("delete managed pi rule: %v", err)
	}

	second := Sync(req)
	secondResult := findSyncResult(t, second, "pi", "rules")
	if secondResult.Err != nil {
		t.Fatalf("second pi rules sync error = %v", secondResult.Err)
	}
	if !containsAll(secondResult.Pruned, compiledPath) {
		t.Fatalf("second pi rules pruned = %v, want %q", secondResult.Pruned, compiledPath)
	}
	if _, err := os.Stat(compiledPath); !os.IsNotExist(err) {
		t.Fatalf("expected compiled pi rule to be pruned, got err=%v", err)
	}
}

func TestSync_PrunesManagedProjectRootAgentsAfterPiSourceDeletion(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	projectRoot := t.TempDir()
	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "pi/AGENTS.md",
		Content: []byte("# Pi Agents\n"),
	}); err != nil {
		t.Fatalf("put managed pi agents: %v", err)
	}

	req := SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{{
			Name:   "pi",
			Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".pi", "skills")},
		}},
	}

	first := Sync(req)
	firstResult := findSyncResult(t, first, "pi", "rules")
	if firstResult.Err != nil {
		t.Fatalf("first pi rules sync error = %v", firstResult.Err)
	}
	managedAgentsPath := filepath.Join(projectRoot, "AGENTS.md")
	if _, err := os.Stat(managedAgentsPath); err != nil {
		t.Fatalf("expected managed AGENTS.md after first sync: %v", err)
	}

	if err := ruleStore.Delete("pi/AGENTS.md"); err != nil {
		t.Fatalf("delete managed pi agents: %v", err)
	}

	second := Sync(req)
	secondResult := findSyncResult(t, second, "pi", "rules")
	if secondResult.Err != nil {
		t.Fatalf("second pi rules sync error = %v", secondResult.Err)
	}
	if !containsAll(secondResult.Pruned, managedAgentsPath) {
		t.Fatalf("second pi rules pruned = %v, want %q", secondResult.Pruned, managedAgentsPath)
	}
	if _, err := os.Stat(managedAgentsPath); !os.IsNotExist(err) {
		t.Fatalf("expected managed AGENTS.md to be pruned, got err=%v", err)
	}
}

func TestSync_ProjectModePiSyncDoesNotPruneManualProjectRootAgents(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	projectRoot := t.TempDir()
	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "pi/AGENTS.md",
		Content: []byte("# Managed Agents\n"),
	}); err != nil {
		t.Fatalf("put managed pi agents: %v", err)
	}

	first := Sync(SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{{
			Name:   "pi",
			Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".pi", "skills")},
		}},
	})

	firstResult := findSyncResult(t, first, "pi", "rules")
	if firstResult.Err != nil {
		t.Fatalf("first pi rules sync error = %v", firstResult.Err)
	}
	manualAgentsPath := filepath.Join(projectRoot, "AGENTS.md")
	if err := os.WriteFile(manualAgentsPath, []byte("# Manual Agents\n"), 0o644); err != nil {
		t.Fatalf("rewrite manual AGENTS.md: %v", err)
	}

	if err := ruleStore.Delete("pi/AGENTS.md"); err != nil {
		t.Fatalf("delete managed pi agents: %v", err)
	}

	second := Sync(SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{{
			Name:   "pi",
			Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".pi", "skills")},
		}},
	})

	result := findSyncResult(t, second, "pi", "rules")
	if result.Err != nil {
		t.Fatalf("pi rules sync error = %v", result.Err)
	}
	if containsAll(result.Pruned, manualAgentsPath) {
		t.Fatalf("pi rules pruned = %v, manual AGENTS.md must not be pruned", result.Pruned)
	}
	if got := readFile(t, manualAgentsPath); got != "# Manual Agents\n" {
		t.Fatalf("project AGENTS.md = %q, want manual content preserved", got)
	}
}

func TestSync_ProjectModePiSyncDisownsManualProjectRootAgentsAfterPreserve(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	projectRoot := t.TempDir()
	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "pi/AGENTS.md",
		Content: []byte("# Managed Agents\n"),
	}); err != nil {
		t.Fatalf("put managed pi agents: %v", err)
	}

	req := SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{{
			Name:   "pi",
			Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".pi", "skills")},
		}},
	}

	first := Sync(req)
	if firstResult := findSyncResult(t, first, "pi", "rules"); firstResult.Err != nil {
		t.Fatalf("first pi rules sync error = %v", firstResult.Err)
	}

	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# Manual Agents\n"), 0o644); err != nil {
		t.Fatalf("write manual AGENTS.md: %v", err)
	}
	if err := ruleStore.Delete("pi/AGENTS.md"); err != nil {
		t.Fatalf("delete managed pi agents: %v", err)
	}

	second := Sync(req)
	secondResult := findSyncResult(t, second, "pi", "rules")
	if secondResult.Err != nil {
		t.Fatalf("second pi rules sync error = %v", secondResult.Err)
	}
	if containsAll(secondResult.Pruned, agentsPath) {
		t.Fatalf("second pi rules pruned = %v, manual AGENTS.md must not be pruned", secondResult.Pruned)
	}

	if err := os.WriteFile(agentsPath, []byte("# Managed Agents\n"), 0o644); err != nil {
		t.Fatalf("rewrite AGENTS.md with prior managed content: %v", err)
	}

	third := Sync(req)
	thirdResult := findSyncResult(t, third, "pi", "rules")
	if thirdResult.Err != nil {
		t.Fatalf("third pi rules sync error = %v", thirdResult.Err)
	}
	if containsAll(thirdResult.Pruned, agentsPath) {
		t.Fatalf("third pi rules pruned = %v, stale managed ownership must be cleared after preserve", thirdResult.Pruned)
	}
	if got := readFile(t, agentsPath); got != "# Managed Agents\n" {
		t.Fatalf("project AGENTS.md = %q, want preserved manual content", got)
	}
}

func TestSync_ProjectModePiSyncDisownsProjectRootAgentsAfterPrune(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	projectRoot := t.TempDir()
	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "pi/AGENTS.md",
		Content: []byte("# Pi Agents\n"),
	}); err != nil {
		t.Fatalf("put managed pi agents: %v", err)
	}

	req := SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{{
			Name:   "pi",
			Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".pi", "skills")},
		}},
	}

	first := Sync(req)
	if firstResult := findSyncResult(t, first, "pi", "rules"); firstResult.Err != nil {
		t.Fatalf("first pi rules sync error = %v", firstResult.Err)
	}

	if err := ruleStore.Delete("pi/AGENTS.md"); err != nil {
		t.Fatalf("delete managed pi agents: %v", err)
	}

	second := Sync(req)
	secondResult := findSyncResult(t, second, "pi", "rules")
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	if secondResult.Err != nil {
		t.Fatalf("second pi rules sync error = %v", secondResult.Err)
	}
	if !containsAll(secondResult.Pruned, agentsPath) {
		t.Fatalf("second pi rules pruned = %v, want %q", secondResult.Pruned, agentsPath)
	}

	if err := os.WriteFile(agentsPath, []byte("# Pi Agents\n"), 0o644); err != nil {
		t.Fatalf("recreate AGENTS.md with prior managed content: %v", err)
	}

	third := Sync(req)
	thirdResult := findSyncResult(t, third, "pi", "rules")
	if thirdResult.Err != nil {
		t.Fatalf("third pi rules sync error = %v", thirdResult.Err)
	}
	if containsAll(thirdResult.Pruned, agentsPath) {
		t.Fatalf("third pi rules pruned = %v, stale managed ownership must be cleared after prune", thirdResult.Pruned)
	}
	if got := readFile(t, agentsPath); got != "# Pi Agents\n" {
		t.Fatalf("project AGENTS.md = %q, want recreated manual content preserved", got)
	}
}

func TestSync_PreservesProjectRootAgentsWhenAnotherCurrentTargetStillProducesIt(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	projectRoot := t.TempDir()
	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "codex/AGENTS.md",
		Content: []byte("# Codex Agents\n"),
	}); err != nil {
		t.Fatalf("put managed codex agents: %v", err)
	}
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "pi/AGENTS.md",
		Content: []byte("# Pi Agents\n"),
	}); err != nil {
		t.Fatalf("put managed pi agents: %v", err)
	}

	first := Sync(SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{
			{
				Name:   "pi",
				Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".pi", "skills")},
			},
		},
	})

	if piFirst := findSyncResult(t, first, "pi", "rules"); piFirst.Err != nil {
		t.Fatalf("first pi rules sync error = %v", piFirst.Err)
	}
	if got := readFile(t, filepath.Join(projectRoot, "AGENTS.md")); !strings.Contains(got, "# Pi Agents") {
		t.Fatalf("managed AGENTS.md = %q, want pi content", got)
	}

	if err := ruleStore.Delete("pi/AGENTS.md"); err != nil {
		t.Fatalf("delete managed pi agents: %v", err)
	}

	second := Sync(SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{
			{
				Name:   "pi",
				Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".pi", "skills")},
			},
			{
				Name:   "codex",
				Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".codex", "skills")},
			},
		},
	})

	piResult := findSyncResult(t, second, "pi", "rules")
	if piResult.Err != nil {
		t.Fatalf("second pi rules sync error = %v", piResult.Err)
	}
	if containsAll(piResult.Pruned, filepath.Join(projectRoot, "AGENTS.md")) {
		t.Fatalf("pi rules pruned = %v, shared AGENTS.md must not be pruned while codex still produces it", piResult.Pruned)
	}
	if codexResult := findSyncResult(t, second, "codex", "rules"); codexResult.Err != nil {
		t.Fatalf("second codex rules sync error = %v", codexResult.Err)
	}
	if got := readFile(t, filepath.Join(projectRoot, "AGENTS.md")); !strings.Contains(got, "# Codex Agents") {
		t.Fatalf("shared AGENTS.md = %q, want codex content preserved", got)
	}
}

func TestSync_RejectsProjectRootAgentsConflictBetweenCodexAndPi(t *testing.T) {
	projectRoot := t.TempDir()
	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "codex/AGENTS.md",
		Content: []byte("# Codex Agents\n"),
	}); err != nil {
		t.Fatalf("put managed codex rule: %v", err)
	}
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "pi/AGENTS.md",
		Content: []byte("# Pi Agents\n"),
	}); err != nil {
		t.Fatalf("put managed pi rule: %v", err)
	}

	results := Sync(SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{
			{
				Name:   "codex",
				Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".codex", "skills")},
			},
			{
				Name:   "pi",
				Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".pi", "skills")},
			},
		},
	})

	codexResult := findSyncResult(t, results, "codex", "rules")
	if codexResult.Err == nil {
		t.Fatal("codex rules sync error = nil, want shared AGENTS conflict")
	}
	if !strings.Contains(codexResult.Err.Error(), "conflict") || !strings.Contains(codexResult.Err.Error(), "AGENTS.md") {
		t.Fatalf("codex rules sync error = %v, want AGENTS conflict", codexResult.Err)
	}

	piResult := findSyncResult(t, results, "pi", "rules")
	if piResult.Err == nil {
		t.Fatal("pi rules sync error = nil, want shared AGENTS conflict")
	}
	if !strings.Contains(piResult.Err.Error(), "conflict") || !strings.Contains(piResult.Err.Error(), "AGENTS.md") {
		t.Fatalf("pi rules sync error = %v, want AGENTS conflict", piResult.Err)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no shared AGENTS.md to be written, got err=%v", err)
	}
}

func TestSync_RejectsProjectRootAgentsConflictForSingleTargetSync(t *testing.T) {
	projectRoot := t.TempDir()
	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "codex/AGENTS.md",
		Content: []byte("# Codex Agents\n"),
	}); err != nil {
		t.Fatalf("put managed codex rule: %v", err)
	}
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "pi/AGENTS.md",
		Content: []byte("# Pi Agents\n"),
	}); err != nil {
		t.Fatalf("put managed pi rule: %v", err)
	}

	results := Sync(SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{{
			Name:   "pi",
			Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".pi", "skills")},
		}},
		AllTargets: []TargetSyncSpec{
			{
				Name:   "codex",
				Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".codex", "skills")},
			},
			{
				Name:   "pi",
				Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".pi", "skills")},
			},
		},
	})

	piResult := findSyncResult(t, results, "pi", "rules")
	if piResult.Err == nil {
		t.Fatal("pi rules sync error = nil, want shared AGENTS conflict")
	}
	if !strings.Contains(piResult.Err.Error(), "conflict") || !strings.Contains(piResult.Err.Error(), "AGENTS.md") {
		t.Fatalf("pi rules sync error = %v, want AGENTS conflict", piResult.Err)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no shared AGENTS.md to be written, got err=%v", err)
	}
}

func TestSync_RejectsProjectRootAgentsConflictForSingleTargetSyncWithNonCanonicalCodexFamilyTarget(t *testing.T) {
	projectRoot := t.TempDir()
	customCodexTarget := TargetSyncSpec{
		Name:   "my-codex",
		Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".agents", "skills")},
	}
	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "codex/AGENTS.md",
		Content: []byte("# Codex Agents\n"),
		Targets: []string{"my-codex"},
	}); err != nil {
		t.Fatalf("put managed codex rule: %v", err)
	}
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "pi/AGENTS.md",
		Content: []byte("# Pi Agents\n"),
	}); err != nil {
		t.Fatalf("put managed pi rule: %v", err)
	}

	results := Sync(SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{{
			Name:   "pi",
			Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".pi", "skills")},
		}},
		AllTargets: []TargetSyncSpec{
			{
				Name:   "pi",
				Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".pi", "skills")},
			},
			customCodexTarget,
		},
	})

	piResult := findSyncResult(t, results, "pi", "rules")
	if piResult.Err == nil {
		t.Fatal("pi rules sync error = nil, want shared AGENTS conflict")
	}
	if !strings.Contains(piResult.Err.Error(), "conflict") || !strings.Contains(piResult.Err.Error(), "AGENTS.md") || !strings.Contains(piResult.Err.Error(), "my-codex") {
		t.Fatalf("pi rules sync error = %v, want AGENTS conflict mentioning my-codex target", piResult.Err)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no shared AGENTS.md to be written, got err=%v", err)
	}
}

func TestSync_RejectsSharedOutputConflictBetweenSameFamilyTargets(t *testing.T) {
	projectRoot := t.TempDir()
	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:         "codex/intro.md",
		Content:    []byte("# Codex Intro\n"),
		Targets:    []string{"codex"},
		SourceType: "local",
	}); err != nil {
		t.Fatalf("put codex rule: %v", err)
	}
	if _, err := ruleStore.Put(managedrules.Save{
		ID:         "codex/alt.md",
		Content:    []byte("# Universal Intro\n"),
		Targets:    []string{"universal"},
		SourceType: "local",
	}); err != nil {
		t.Fatalf("put universal rule: %v", err)
	}

	results := Sync(SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{
			{
				Name:   "codex",
				Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".codex", "skills")},
			},
			{
				Name:   "universal",
				Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".agents", "skills")},
			},
		},
	})

	codexResult := findSyncResult(t, results, "codex", "rules")
	if codexResult.Err == nil {
		t.Fatal("codex rules sync error = nil, want shared AGENTS conflict")
	}
	if !strings.Contains(codexResult.Err.Error(), "conflict") || !strings.Contains(codexResult.Err.Error(), "AGENTS.md") {
		t.Fatalf("codex rules sync error = %v, want AGENTS conflict", codexResult.Err)
	}

	universalResult := findSyncResult(t, results, "universal", "rules")
	if universalResult.Err == nil {
		t.Fatal("universal rules sync error = nil, want shared AGENTS conflict")
	}
	if !strings.Contains(universalResult.Err.Error(), "conflict") || !strings.Contains(universalResult.Err.Error(), "AGENTS.md") {
		t.Fatalf("universal rules sync error = %v, want AGENTS conflict", universalResult.Err)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no shared AGENTS.md to be written, got err=%v", err)
	}
}

func TestSync_AllowsSharedOutputBetweenSameFamilyTargetsWhenContentsMatch(t *testing.T) {
	projectRoot := t.TempDir()
	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:         "codex/shared.md",
		Content:    []byte("# Shared Intro\n"),
		SourceType: "local",
	}); err != nil {
		t.Fatalf("put codex rule: %v", err)
	}

	results := Sync(SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      false,
		Resources:   ResourceSet{Rules: true},
		Targets: []TargetSyncSpec{
			{
				Name:   "codex",
				Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".codex", "skills")},
			},
			{
				Name:   "universal",
				Target: config.TargetConfig{Path: filepath.Join(projectRoot, ".agents", "skills")},
			},
		},
	})

	codexResult := findSyncResult(t, results, "codex", "rules")
	if codexResult.Err != nil {
		t.Fatalf("codex rules sync error = %v, want nil", codexResult.Err)
	}

	universalResult := findSyncResult(t, results, "universal", "rules")
	if universalResult.Err != nil {
		t.Fatalf("universal rules sync error = %v, want nil", universalResult.Err)
	}

	if got := readFile(t, filepath.Join(projectRoot, "AGENTS.md")); !strings.Contains(got, "# Shared Intro") {
		t.Fatalf("shared AGENTS.md = %q, want shared content", got)
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
