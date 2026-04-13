package hooks

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/inspect"
)

func TestCollectHooks_RejectsPrivateLocalGroups(t *testing.T) {
	root := t.TempDir()
	discovered := []inspect.HookItem{
		{
			GroupID:       "claude:project:/tmp/project/.claude/settings.local.json:PreToolUse:Edit",
			SourceTool:    "claude",
			Scope:         inspect.ScopeProject,
			Event:         "PreToolUse",
			Matcher:       "Edit",
			ActionType:    "command",
			Command:       "./bin/local-only",
			Path:          "/tmp/project/.claude/settings.local.json",
			Collectible:   false,
			CollectReason: "private project-local override files stay diagnostics-only",
		},
	}

	_, err := Collect(root, discovered, CollectOptions{Strategy: StrategyOverwrite})
	if err == nil {
		t.Fatal("expected collect error for non-collectible local hook group")
	}
}

func TestCollectHooks_PreservesGroupedHandlers(t *testing.T) {
	root := t.TempDir()
	wantID := mustCanonicalRelativePath(t, "claude", "PreToolUse", "Bash")
	discovered := []inspect.HookItem{
		{
			GroupID:       "claude:project:/tmp/project/.claude/settings.json:PreToolUse:Bash",
			SourceTool:    "claude",
			Scope:         inspect.ScopeProject,
			Event:         "PreToolUse",
			Matcher:       "Bash",
			ActionType:    "command",
			Command:       "./bin/check",
			Timeout:       "30s",
			StatusMessage: "Running check",
			Path:          "/tmp/project/.claude/settings.json",
			Collectible:   true,
		},
		{
			GroupID:       "claude:project:/tmp/project/.claude/settings.json:PreToolUse:Bash",
			SourceTool:    "claude",
			Scope:         inspect.ScopeProject,
			Event:         "PreToolUse",
			Matcher:       "Bash",
			ActionType:    "prompt",
			Prompt:        "Summarize the tool input",
			Timeout:       "15s",
			StatusMessage: "Prompting for summary",
			Path:          "/tmp/project/.claude/settings.json",
			Collectible:   true,
		},
	}

	result, err := Collect(root, discovered, CollectOptions{Strategy: StrategyOverwrite})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(result.Created) != 1 {
		t.Fatalf("Collect() Created = %v, want one record", result.Created)
	}
	if result.Created[0] != wantID {
		t.Fatalf("Collect() Created[0] = %q, want %q", result.Created[0], wantID)
	}

	store := NewStore(root)
	got, err := store.Get(wantID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(got.Handlers) != 2 {
		t.Fatalf("Get() handlers len = %d, want 2", len(got.Handlers))
	}
	if got.Handlers[0].Type != "command" || got.Handlers[0].Command != "./bin/check" {
		t.Fatalf("first handler = %#v, want command handler", got.Handlers[0])
	}
	if got.Handlers[0].Timeout != "30s" || got.Handlers[0].StatusMessage != "Running check" {
		t.Fatalf("first handler metadata = %#v, want timeout/statusMessage preserved", got.Handlers[0])
	}
	if got.Handlers[1].Type != "prompt" || got.Handlers[1].Prompt != "Summarize the tool input" {
		t.Fatalf("second handler = %#v, want prompt handler", got.Handlers[1])
	}
	if got.Handlers[1].Timeout != "15s" || got.Handlers[1].StatusMessage != "Prompting for summary" {
		t.Fatalf("second handler metadata = %#v, want timeout/statusMessage preserved", got.Handlers[1])
	}
	if got.Targets != nil {
		t.Fatalf("Get() targets = %v, want nil", got.Targets)
	}
	if got.SourceType != "local" {
		t.Fatalf("Get() sourceType = %q, want %q", got.SourceType, "local")
	}
	if got.Disabled {
		t.Fatalf("Get() disabled = %v, want false", got.Disabled)
	}
}

func TestCollectHooks_OverwritePreservesExistingMetadata(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	id := mustCanonicalRelativePath(t, "claude", "PreToolUse", "Bash")

	_, err := store.Put(Save{
		ID:         id,
		Tool:       "claude",
		Event:      "PreToolUse",
		Matcher:    "Bash",
		Targets:    []string{"claude-work"},
		SourceType: "tracked",
		Disabled:   true,
		Handlers: []Handler{{
			Type:    "command",
			Command: "./bin/original",
		}},
	})
	if err != nil {
		t.Fatalf("seed Put() error = %v", err)
	}

	discovered := []inspect.HookItem{{
		GroupID:     "claude:project:/tmp/project/.claude/settings.json:PreToolUse:Bash",
		SourceTool:  "claude",
		Scope:       inspect.ScopeProject,
		Event:       "PreToolUse",
		Matcher:     "Bash",
		ActionType:  "command",
		Command:     "./bin/updated",
		Path:        "/tmp/project/.claude/settings.json",
		Collectible: true,
	}}

	result, err := Collect(root, discovered, CollectOptions{Strategy: StrategyOverwrite})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(result.Overwritten) != 1 || result.Overwritten[0] != id {
		t.Fatalf("Collect() Overwritten = %v, want [%s]", result.Overwritten, id)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(got.Handlers) != 1 || got.Handlers[0].Command != "./bin/updated" {
		t.Fatalf("Get() handlers = %#v, want updated handler", got.Handlers)
	}
	if len(got.Targets) != 1 || got.Targets[0] != "claude-work" {
		t.Fatalf("Get() targets = %v, want [claude-work]", got.Targets)
	}
	if got.SourceType != "tracked" {
		t.Fatalf("Get() sourceType = %q, want %q", got.SourceType, "tracked")
	}
	if !got.Disabled {
		t.Fatalf("Get() disabled = %v, want true", got.Disabled)
	}
}

func TestCollectHooks_PreservesDiscoveredHandlerOrder(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")
	wantID := mustCanonicalRelativePath(t, "claude", "PreToolUse", "Bash")

	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir hook config dir error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, ".claude", "settings.json"), []byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"http","url":"https://example.com/hook","statusMessage":"Sending webhook"},{"type":"command","command":"./bin/first","timeout":"30s","statusMessage":"First command"},{"type":"prompt","prompt":"Summarize the tool input","timeout":"15s","statusMessage":"Prompting for summary"},{"type":"command","command":"./bin/second","timeout":"45s","statusMessage":"Second command"}]}]}}`), 0644); err != nil {
		t.Fatalf("write hook config error = %v", err)
	}

	t.Setenv("HOME", home)

	discovered, warnings, err := inspect.ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(discovered) != 4 {
		t.Fatalf("expected 4 discovered hook items, got %d", len(discovered))
	}
	shuffled := []inspect.HookItem{discovered[2], discovered[0], discovered[3], discovered[1]}
	result, err := Collect(project, shuffled, CollectOptions{Strategy: StrategyOverwrite})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(result.Created) != 1 || result.Created[0] != wantID {
		t.Fatalf("Collect() Created = %v, want [%s]", result.Created, wantID)
	}

	store := NewStore(project)
	got, err := store.Get(wantID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(got.Handlers) != 4 {
		t.Fatalf("Get() handlers len = %d, want 4", len(got.Handlers))
	}
	if got.Handlers[0].Type != "http" || got.Handlers[1].Command != "./bin/first" || got.Handlers[2].Type != "prompt" || got.Handlers[3].Command != "./bin/second" {
		t.Fatalf("handlers order was not preserved: %#v", got.Handlers)
	}
}

func TestCollectHooks_PreservesDuplicateEntryOrder(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")
	wantID := mustCanonicalRelativePath(t, "claude", "PreToolUse", "Bash")

	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir hook config dir error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, ".claude", "settings.json"), []byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./bin/entry-one-a"},{"type":"command","command":"./bin/entry-one-b"}]},{"matcher":"Bash","hooks":[{"type":"command","command":"./bin/entry-two-a"},{"type":"command","command":"./bin/entry-two-b"}]}]}}`), 0644); err != nil {
		t.Fatalf("write hook config error = %v", err)
	}

	t.Setenv("HOME", home)

	discovered, warnings, err := inspect.ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(discovered) != 4 {
		t.Fatalf("expected 4 discovered hook items, got %d", len(discovered))
	}

	shuffled := []inspect.HookItem{
		hookItemWithCommand(t, discovered, "./bin/entry-two-b"),
		hookItemWithCommand(t, discovered, "./bin/entry-one-a"),
		hookItemWithCommand(t, discovered, "./bin/entry-two-a"),
		hookItemWithCommand(t, discovered, "./bin/entry-one-b"),
	}

	result, err := Collect(project, shuffled, CollectOptions{Strategy: StrategyOverwrite})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(result.Created) != 1 || result.Created[0] != wantID {
		t.Fatalf("Collect() Created = %v, want [%s]", result.Created, wantID)
	}

	store := NewStore(project)
	got, err := store.Get(wantID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	wantCommands := []string{"./bin/entry-one-a", "./bin/entry-one-b", "./bin/entry-two-a", "./bin/entry-two-b"}
	if len(got.Handlers) != len(wantCommands) {
		t.Fatalf("Get() handlers len = %d, want %d", len(got.Handlers), len(wantCommands))
	}
	for i, wantCommand := range wantCommands {
		if got.Handlers[i].Command != wantCommand {
			t.Fatalf("Get() Handlers[%d].Command = %q, want %q; handlers=%#v", i, got.Handlers[i].Command, wantCommand, got.Handlers)
		}
	}
}

func TestCollectHooks_RejectsCanonicalIDCollisionsAcrossSources(t *testing.T) {
	root := t.TempDir()
	discovered := []inspect.HookItem{
		{
			GroupID:     "claude:project:/tmp/project/.claude/settings.json:PreToolUse:Bash",
			SourceTool:  "claude",
			Scope:       inspect.ScopeProject,
			Event:       "PreToolUse",
			Matcher:     "Bash",
			ActionType:  "command",
			Command:     "./bin/project",
			Path:        "/tmp/project/.claude/settings.json",
			Collectible: true,
		},
		{
			GroupID:     "claude:user:/tmp/home/.claude/settings.json:PreToolUse:Bash",
			SourceTool:  "claude",
			Scope:       inspect.ScopeUser,
			Event:       "PreToolUse",
			Matcher:     "Bash",
			ActionType:  "command",
			Command:     "./bin/user",
			Path:        "/tmp/home/.claude/settings.json",
			Collectible: true,
		},
	}

	_, err := Collect(root, discovered, CollectOptions{Strategy: StrategyOverwrite})
	if err == nil {
		t.Fatal("expected collect collision error")
	}
	if !strings.Contains(err.Error(), "canonical managed id") {
		t.Fatalf("Collect() error = %v, want canonical id collision", err)
	}

	store := NewStore(root)
	all, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("List() len = %d, want 0 after collision failure", len(all))
	}
}

func TestCollectHooks_CodexEmptyMatcherAndNumericTimeoutRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	if err := os.MkdirAll(filepath.Join(project, ".codex"), 0755); err != nil {
		t.Fatalf("mkdir codex dir error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, ".codex", "hooks.json"), []byte(`{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":"./submit.sh","timeout":30}]}],"Stop":[{"matcher":"","hooks":[{"type":"command","command":"./stop.sh","timeoutSec":45}]}]}}`), 0644); err != nil {
		t.Fatalf("write codex hook config error = %v", err)
	}

	t.Setenv("HOME", home)

	discovered, warnings, err := inspect.ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(discovered) != 2 {
		t.Fatalf("expected 2 discovered hook items, got %d", len(discovered))
	}

	var submitItem, stopItem inspect.HookItem
	for _, item := range discovered {
		if item.Event == "UserPromptSubmit" {
			submitItem = item
		}
		if item.Event == "Stop" {
			stopItem = item
		}
		if item.Matcher != "" {
			t.Fatalf("expected empty matcher for codex event %s, got %q", item.Event, item.Matcher)
		}
	}
	if submitItem.TimeoutSeconds == nil || *submitItem.TimeoutSeconds != 30 {
		t.Fatalf("submit timeoutSeconds = %#v, want 30", submitItem.TimeoutSeconds)
	}
	if stopItem.TimeoutSeconds == nil || *stopItem.TimeoutSeconds != 45 {
		t.Fatalf("stop timeoutSeconds = %#v, want 45", stopItem.TimeoutSeconds)
	}

	result, err := Collect(project, discovered, CollectOptions{Strategy: StrategyOverwrite})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(result.Created) != 2 {
		t.Fatalf("Collect() Created len = %d, want 2", len(result.Created))
	}

	store := NewStore(project)
	submitID := mustCanonicalRelativePath(t, "codex", "UserPromptSubmit", "")
	stopID := mustCanonicalRelativePath(t, "codex", "Stop", "")

	submitRecord, err := store.Get(submitID)
	if err != nil {
		t.Fatalf("Get(submit) error = %v", err)
	}
	if submitRecord.Matcher != "" {
		t.Fatalf("submit matcher = %q, want empty", submitRecord.Matcher)
	}
	if len(submitRecord.Handlers) != 1 || submitRecord.Handlers[0].TimeoutSeconds == nil || *submitRecord.Handlers[0].TimeoutSeconds != 30 {
		t.Fatalf("submit handlers = %#v, want numeric timeout preserved", submitRecord.Handlers)
	}

	stopRecord, err := store.Get(stopID)
	if err != nil {
		t.Fatalf("Get(stop) error = %v", err)
	}
	if stopRecord.Matcher != "" {
		t.Fatalf("stop matcher = %q, want empty", stopRecord.Matcher)
	}
	if len(stopRecord.Handlers) != 1 || stopRecord.Handlers[0].TimeoutSeconds == nil || *stopRecord.Handlers[0].TimeoutSeconds != 45 {
		t.Fatalf("stop handlers = %#v, want numeric timeout preserved", stopRecord.Handlers)
	}
}

func TestCollectHooks_GeminiPreservesSequentialAndHandlerMetadata(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	if err := os.MkdirAll(filepath.Join(project, ".gemini"), 0o755); err != nil {
		t.Fatalf("mkdir gemini dir error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(project, ".gemini", "settings.json"),
		[]byte(`{"hooks":{"BeforeTool":[{"matcher":"Read","sequential":true,"hooks":[{"type":"command","name":"lint-read","description":"Run read lint","command":"./bin/gemini-lint","timeout":30000}]}]}}`),
		0o644,
	); err != nil {
		t.Fatalf("write gemini hook config error = %v", err)
	}

	t.Setenv("HOME", home)

	discovered, warnings, err := inspect.ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(discovered) != 1 {
		t.Fatalf("expected 1 discovered gemini hook item, got %d", len(discovered))
	}
	if !discovered[0].Collectible {
		t.Fatalf("discovered item collectible = false, want true")
	}

	result, err := Collect(project, discovered, CollectOptions{Strategy: StrategyOverwrite})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(result.Created) != 1 {
		t.Fatalf("Collect() Created = %v, want 1 created hook", result.Created)
	}

	store := NewStore(project)
	id := mustCanonicalRelativePath(t, "gemini", "BeforeTool", "Read")
	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Sequential == nil || !*got.Sequential {
		t.Fatalf("Get() Sequential = %#v, want true", got.Sequential)
	}
	if len(got.Handlers) != 1 {
		t.Fatalf("Get() handlers len = %d, want 1", len(got.Handlers))
	}
	if got.Handlers[0].Name != "lint-read" || got.Handlers[0].Description != "Run read lint" {
		t.Fatalf("Get() gemini handler metadata = %#v, want name/description preserved", got.Handlers[0])
	}
	if got.Handlers[0].Timeout != "30000" {
		t.Fatalf("Get() gemini timeout = %q, want 30000", got.Handlers[0].Timeout)
	}
}

func TestCollectHooks_GeminiAllowsEmptyMatcher(t *testing.T) {
	root := t.TempDir()
	wantID := mustCanonicalRelativePath(t, "gemini", "Notification", "")
	discovered := []inspect.HookItem{
		{
			GroupID:     "gemini:project:/tmp/project/.gemini/settings.json:Notification:",
			SourceTool:  "gemini",
			Scope:       inspect.ScopeProject,
			Event:       "Notification",
			Matcher:     "",
			ActionType:  "command",
			Command:     "./bin/notify",
			Path:        "/tmp/project/.gemini/settings.json",
			Collectible: true,
		},
	}

	result, err := Collect(root, discovered, CollectOptions{Strategy: StrategyOverwrite})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(result.Created) != 1 || result.Created[0] != wantID {
		t.Fatalf("Collect() Created = %v, want %q", result.Created, wantID)
	}

	store := NewStore(root)
	got, err := store.Get(wantID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Matcher != "" {
		t.Fatalf("Get() Matcher = %q, want empty matcher", got.Matcher)
	}
}

func TestCollectHooks_RollsBackOnLaterWriteFailure(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	alphaID := mustCanonicalRelativePath(t, "claude", "PreToolUse", "Alpha")
	betaID := mustCanonicalRelativePath(t, "claude", "PreToolUse", "Beta")

	_, err := store.Put(Save{
		ID:       alphaID,
		Tool:     "claude",
		Event:    "PreToolUse",
		Matcher:  "Alpha",
		Handlers: []Handler{{Type: "command", Command: "./bin/alpha"}},
	})
	if err != nil {
		t.Fatalf("seed Put(alpha) error = %v", err)
	}
	_, err = store.Put(Save{
		ID:       betaID,
		Tool:     "claude",
		Event:    "PreToolUse",
		Matcher:  "Beta",
		Handlers: []Handler{{Type: "command", Command: "./bin/beta"}},
	})
	if err != nil {
		t.Fatalf("seed Put(beta) error = %v", err)
	}

	discovered := []inspect.HookItem{
		{
			GroupID:     "claude:project:/tmp/project/.claude/settings.json:PreToolUse:Alpha",
			SourceTool:  "claude",
			Scope:       inspect.ScopeProject,
			Event:       "PreToolUse",
			Matcher:     "Alpha",
			ActionType:  "command",
			Command:     "./bin/alpha-updated",
			Path:        "/tmp/project/.claude/settings.json",
			Collectible: true,
		},
		{
			GroupID:     "claude:project:/tmp/project/.claude/settings.json:PreToolUse:Beta",
			SourceTool:  "claude",
			Scope:       inspect.ScopeProject,
			Event:       "PreToolUse",
			Matcher:     "Beta",
			ActionType:  "command",
			Command:     "./bin/beta-updated",
			Path:        "/tmp/project/.claude/settings.json",
			Collectible: true,
		},
	}

	origWrite := hookWriteFile
	defer func() { hookWriteFile = origWrite }()

	writeCalls := 0
	hookWriteFile = func(name string, data []byte, perm os.FileMode) error {
		writeCalls++
		if writeCalls == 2 {
			_ = origWrite(name, []byte("corrupt"), perm)
			return errors.New("injected write failure")
		}
		return origWrite(name, data, perm)
	}

	_, err = Collect(root, discovered, CollectOptions{Strategy: StrategyOverwrite})
	if err == nil {
		t.Fatal("Collect() error = nil, want injected write failure")
	}

	alpha, err := store.Get(alphaID)
	if err != nil {
		t.Fatalf("Get(alpha) error = %v", err)
	}
	if len(alpha.Handlers) != 1 || alpha.Handlers[0].Command != "./bin/alpha" {
		t.Fatalf("alpha handlers = %#v, want original content restored", alpha.Handlers)
	}

	beta, err := store.Get(betaID)
	if err != nil {
		t.Fatalf("Get(beta) error = %v", err)
	}
	if len(beta.Handlers) != 1 || beta.Handlers[0].Command != "./bin/beta" {
		t.Fatalf("beta handlers = %#v, want original content restored", beta.Handlers)
	}
}

func TestCanonicalRelativePath_PunctuationOnlyMatcher(t *testing.T) {
	id, err := canonicalRelativePath("claude", "PreToolUse", ".*")
	if err != nil {
		t.Fatalf("canonicalRelativePath() error = %v", err)
	}
	if id == "" {
		t.Fatal("canonicalRelativePath() returned empty id")
	}
	if path.Base(id) == ".yaml" {
		t.Fatalf("canonicalRelativePath() returned empty matcher stem: %q", id)
	}
}

func TestCanonicalRelativePath_DistinctMatchersDoNotCollide(t *testing.T) {
	left, err := canonicalRelativePath("claude", "PreToolUse", "Bash!")
	if err != nil {
		t.Fatalf("canonicalRelativePath(left) error = %v", err)
	}
	right, err := canonicalRelativePath("claude", "PreToolUse", "Bash?")
	if err != nil {
		t.Fatalf("canonicalRelativePath(right) error = %v", err)
	}
	if left == right {
		t.Fatalf("canonicalRelativePath() collision: %q", left)
	}
}

func mustCanonicalRelativePath(t *testing.T, tool, event, matcher string) string {
	t.Helper()
	id, err := canonicalRelativePath(tool, event, matcher)
	if err != nil {
		t.Fatalf("canonicalRelativePath(%q, %q, %q) error = %v", tool, event, matcher, err)
	}
	return id
}

func hookItemWithCommand(t *testing.T, items []inspect.HookItem, command string) inspect.HookItem {
	t.Helper()
	for _, item := range items {
		if item.Command == command {
			return item
		}
	}
	t.Fatalf("could not find discovered hook item with command %q", command)
	return inspect.HookItem{}
}
