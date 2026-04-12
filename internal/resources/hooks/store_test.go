package hooks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHookStore_PutGetListDelete(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	wantHandlers := []Handler{
		{
			Type:          "command",
			Command:       "./scripts/guard.sh",
			Timeout:       "30s",
			StatusMessage: "Running guard checks",
		},
		{
			Type:          "http",
			URL:           "https://example.com/hook",
			Timeout:       "5s",
			StatusMessage: "Sending webhook",
		},
		{
			Type:          "prompt",
			Prompt:        "Summarize the tool input",
			StatusMessage: "Prompting",
		},
	}

	saved, err := store.Put(Save{
		ID:       "claude/pre-tool-use/bash.yaml",
		Tool:     "claude",
		Event:    "pre-tool-use",
		Matcher:  `^bash\b`,
		Handlers: wantHandlers,
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if saved.ID != "claude/pre-tool-use/bash.yaml" {
		t.Fatalf("Put() ID = %q, want %q", saved.ID, "claude/pre-tool-use/bash.yaml")
	}

	got, err := store.Get("claude/pre-tool-use/bash.yaml")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Tool != "claude" {
		t.Fatalf("Get() Tool = %q, want %q", got.Tool, "claude")
	}
	if got.Event != "pre-tool-use" {
		t.Fatalf("Get() Event = %q, want %q", got.Event, "pre-tool-use")
	}
	if got.Matcher != `^bash\b` {
		t.Fatalf("Get() Matcher = %q, want %q", got.Matcher, `^bash\b`)
	}
	if len(got.Handlers) != len(wantHandlers) {
		t.Fatalf("Get() Handlers len = %d, want %d", len(got.Handlers), len(wantHandlers))
	}
	for i := range wantHandlers {
		if got.Handlers[i] != wantHandlers[i] {
			t.Fatalf("Get() Handlers[%d] = %#v, want %#v", i, got.Handlers[i], wantHandlers[i])
		}
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("List() len = %d, want 1", len(all))
	}
	if all[0].ID != "claude/pre-tool-use/bash.yaml" {
		t.Fatalf("List()[0].ID = %q, want %q", all[0].ID, "claude/pre-tool-use/bash.yaml")
	}

	if err := store.Delete("claude/pre-tool-use/bash.yaml"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = store.Get("claude/pre-tool-use/bash.yaml")
	if !os.IsNotExist(err) {
		t.Fatalf("Get() after Delete error = %v, want not-exist", err)
	}
}

func TestHookStore_PutAndGet_RoundTripsMetadata(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	saved, err := store.Put(Save{
		ID:         "claude/pre-tool-use/bash.yaml",
		Tool:       "claude",
		Event:      "pre-tool-use",
		Matcher:    "^bash\\b",
		Targets:    []string{"claude-work"},
		SourceType: "local",
		Disabled:   true,
		Handlers: []Handler{
			{Type: "command", Command: "./bin/check"},
		},
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, err := store.Get(saved.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !reflect.DeepEqual(got.Targets, []string{"claude-work"}) {
		t.Fatalf("Get() Targets = %#v, want %#v", got.Targets, []string{"claude-work"})
	}
	if got.SourceType != "local" {
		t.Fatalf("Get() SourceType = %q, want %q", got.SourceType, "local")
	}
	if !got.Disabled {
		t.Fatal("Get() Disabled = false, want true")
	}
}

func TestHookStore_GeminiRoundTripsSequentialAndHandlerMetadata(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)
	sequential := true

	saved, err := store.Put(Save{
		ID:         "gemini/before-tool/read.yaml",
		Tool:       "gemini",
		Event:      "BeforeTool",
		Matcher:    "Read",
		Sequential: &sequential,
		Handlers: []Handler{{
			Type:        "command",
			Name:        "lint-read",
			Description: "Run read lint",
			Command:     "./bin/gemini-lint",
			Timeout:     "30000",
		}},
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, err := store.Get(saved.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Sequential == nil || !*got.Sequential {
		t.Fatalf("Get() Sequential = %#v, want true", got.Sequential)
	}
	if len(got.Handlers) != 1 {
		t.Fatalf("Get() Handlers len = %d, want 1", len(got.Handlers))
	}
	if got.Handlers[0].Name != "lint-read" || got.Handlers[0].Description != "Run read lint" {
		t.Fatalf("Get() gemini handler metadata = %#v, want name/description preserved", got.Handlers[0])
	}
}

func TestHookStore_RejectsEmptyHandlers(t *testing.T) {
	store := NewStore(t.TempDir())

	cases := []Save{
		{
			ID:       "claude/pre-tool-use/bash.yaml",
			Tool:     "claude",
			Event:    "pre-tool-use",
			Matcher:  "^bash",
			Handlers: nil,
		},
		{
			ID:       "claude/pre-tool-use/bash.yaml",
			Tool:     "claude",
			Event:    "pre-tool-use",
			Matcher:  "^bash",
			Handlers: []Handler{},
		},
		{
			ID:      "claude/pre-tool-use/bash.yaml",
			Tool:    "claude",
			Event:   "pre-tool-use",
			Matcher: "^bash",
			Handlers: []Handler{
				{Command: "./scripts/guard.sh"},
			},
		},
		{
			ID:      "claude/pre-tool-use/bash.yaml",
			Tool:    "claude",
			Event:   "pre-tool-use",
			Matcher: "^bash",
			Handlers: []Handler{
				{Type: "command"},
			},
		},
		{
			ID:      "claude/pre-tool-use/bash.yaml",
			Tool:    "claude",
			Event:   "pre-tool-use",
			Matcher: "^bash",
			Handlers: []Handler{
				{Type: "http"},
			},
		},
		{
			ID:      "claude/pre-tool-use/bash.yaml",
			Tool:    "claude",
			Event:   "pre-tool-use",
			Matcher: "^bash",
			Handlers: []Handler{
				{Type: "prompt"},
			},
		},
		{
			ID:      "claude/pre-tool-use/bash.yaml",
			Tool:    "claude",
			Event:   "pre-tool-use",
			Matcher: "^bash",
			Handlers: []Handler{
				{Type: "agent"},
			},
		},
		{
			ID:      "claude/pre-tool-use/bash.yaml",
			Tool:    "claude",
			Event:   "pre-tool-use",
			Matcher: "^bash",
			Handlers: []Handler{
				{Type: "unknown", Command: "./scripts/guard.sh"},
			},
		},
	}

	for _, tc := range cases {
		if _, err := store.Put(tc); err == nil {
			t.Fatalf("Put() error = nil, want error for handlers=%#v", tc.Handlers)
		}
	}
}

func TestHookStore_RejectsInvalidIDs(t *testing.T) {
	store := NewStore(t.TempDir())

	invalidIDs := []string{
		"",
		"   ",
		".",
		"..",
		"../outside.yaml",
		"..\\outside.yaml",
		"..\\..\\outside.yaml",
		"/tmp/outside.yaml",
		`C:\outside.yaml`,
		"C:/outside.yaml",
		`\\server\share\file.yaml`,
		"claude/.hook-tmp-test.yaml",
		"claude/pre-tool-use/.hook-tmp-test.yaml",
	}

	validSave := Save{
		Tool:    "claude",
		Event:   "pre-tool-use",
		Matcher: "^bash",
		Handlers: []Handler{
			{Type: "command", Command: "./scripts/guard.sh"},
		},
	}

	for _, id := range invalidIDs {
		id := id
		t.Run(id, func(t *testing.T) {
			in := validSave
			in.ID = id
			if _, err := store.Put(in); err == nil {
				t.Fatalf("Put(%q) error = nil, want error", id)
			}
			if _, err := store.Get(id); err == nil {
				t.Fatalf("Get(%q) error = nil, want error", id)
			}
			if err := store.Delete(id); err == nil {
				t.Fatalf("Delete(%q) error = nil, want error", id)
			}
		})
	}
}

func TestHookStore_ListIgnoresTempFiles(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	_, err := store.Put(Save{
		ID:       "claude/pre-tool-use/bash.yaml",
		Tool:     "claude",
		Event:    "pre-tool-use",
		Matcher:  "^bash\\b",
		Handlers: []Handler{{Type: "command", Command: "./scripts/guard.sh"}},
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	managedRoot := filepath.Join(projectRoot, ".skillshare", "hooks", "claude", "pre-tool-use")
	if err := os.MkdirAll(managedRoot, 0755); err != nil {
		t.Fatalf("mkdir managed root error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(managedRoot, ".hook-tmp-crash"), []byte("not yaml"), 0644); err != nil {
		t.Fatalf("write stray temp file error = %v", err)
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("List() len = %d, want 1", len(all))
	}
	if all[0].ID != "claude/pre-tool-use/bash.yaml" {
		t.Fatalf("List()[0].ID = %q, want %q", all[0].ID, "claude/pre-tool-use/bash.yaml")
	}
}

func TestHookStore_RejectsMismatchedToolAndIDPrefix(t *testing.T) {
	store := NewStore(t.TempDir())

	cases := []Save{
		{
			ID:       "claude/pre-tool-use/bash.yaml",
			Tool:     "codex",
			Event:    "pre-tool-use",
			Matcher:  "^bash",
			Handlers: []Handler{{Type: "command", Command: "./scripts/guard.sh"}},
		},
	}

	for _, tc := range cases {
		if _, err := store.Put(tc); err == nil {
			t.Fatalf("Put(%#v) error = nil, want validation error", tc)
		}
	}
}

func TestHookStore_RejectsInvalidGeminiRecords(t *testing.T) {
	store := NewStore(t.TempDir())
	timeoutMillis := 30000

	cases := []Save{
		{
			ID:       "gemini/before-tool/read.yaml",
			Tool:     "gemini",
			Event:    "FileChanged",
			Matcher:  "Read",
			Handlers: []Handler{{Type: "command", Command: "./bin/check"}},
		},
		{
			ID:       "gemini/before-tool/read.yaml",
			Tool:     "gemini",
			Event:    "BeforeTool",
			Matcher:  "Read",
			Handlers: []Handler{{Type: "http", URL: "https://example.com/hook"}},
		},
		{
			ID:       "gemini/before-tool/read.yaml",
			Tool:     "gemini",
			Event:    "BeforeTool",
			Matcher:  "Read",
			Handlers: []Handler{{Type: "command", Command: "./bin/check", Timeout: "30s"}},
		},
		{
			ID:       "gemini/before-tool/read.yaml",
			Tool:     "gemini",
			Event:    "BeforeTool",
			Matcher:  "Read",
			Handlers: []Handler{{Type: "command", Command: "./bin/check", Timeout: "30s", TimeoutSeconds: &timeoutMillis}},
		},
	}

	for _, tc := range cases {
		if _, err := store.Put(tc); err == nil {
			t.Fatalf("Put(%#v) error = nil, want validation error", tc)
		}
	}
}

func TestHookStore_RejectsCodexMatchersForMatcherlessEvents(t *testing.T) {
	store := NewStore(t.TempDir())

	cases := []Save{
		{
			ID:       "codex/user-prompt-submit/bash.yaml",
			Tool:     "codex",
			Event:    "UserPromptSubmit",
			Matcher:  "Bash",
			Handlers: []Handler{{Type: "command", Command: "./scripts/guard.sh"}},
		},
		{
			ID:       "codex/stop/bash.yaml",
			Tool:     "codex",
			Event:    "Stop",
			Matcher:  "Write",
			Handlers: []Handler{{Type: "command", Command: "./scripts/guard.sh"}},
		},
	}

	for _, tc := range cases {
		if _, err := store.Put(tc); err == nil {
			t.Fatalf("Put(%#v) error = nil, want matcher validation error", tc)
		}
	}
}

func TestHookStore_RejectsInvalidCodexRecords(t *testing.T) {
	store := NewStore(t.TempDir())

	cases := []Save{
		{
			ID:       "codex/pre-tool-use/bash.yaml",
			Tool:     "codex",
			Event:    "FileChanged",
			Matcher:  "^bash",
			Handlers: []Handler{{Type: "command", Command: "./bin/check"}},
		},
		{
			ID:       "codex/pre-tool-use/bash.yaml",
			Tool:     "codex",
			Event:    "PreToolUse",
			Matcher:  "^bash",
			Handlers: []Handler{{Type: "http", URL: "https://example.com/hook"}},
		},
		{
			ID:       "codex/pre-tool-use/bash.yaml",
			Tool:     "codex",
			Event:    "PreToolUse",
			Matcher:  "^bash",
			Handlers: []Handler{{Type: "prompt", Prompt: "Summarize"}},
		},
		{
			ID:       "codex/pre-tool-use/bash.yaml",
			Tool:     "codex",
			Event:    "PreToolUse",
			Matcher:  "^bash",
			Handlers: []Handler{{Type: "agent", Prompt: "Summarize"}},
		},
	}

	for _, tc := range cases {
		if _, err := store.Put(tc); err == nil {
			t.Fatalf("Put(%#v) error = nil, want validation error", tc)
		}
	}
}
