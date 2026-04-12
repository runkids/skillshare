package hooks

import (
	"strings"
	"testing"
)

func TestCompileHooks_CodexAddsFeatureFlag(t *testing.T) {
	configToml := "[profiles.default]\nmodel = \"gpt-5\"\n"
	records := []Record{
		{
			ID:           "codex/pre-tool-use/bash.yaml",
			RelativePath: "codex/pre-tool-use/bash.yaml",
			Tool:         "codex",
			Event:        "PreToolUse",
			Matcher:      "Bash",
			Handlers:     []Handler{{Type: "command", Command: "./bin/check"}},
		},
	}

	files, warnings, err := CompileTarget(records, "codex", "codex", "/tmp/project", configToml)
	if err != nil {
		t.Fatalf("CompileTarget() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if !containsCompiledContent(files, "/tmp/project/.codex/config.toml", "codex_hooks = true") {
		t.Fatalf("expected codex_hooks feature flag")
	}
	if !containsCompiledPath(files, "/tmp/project/.codex/hooks.json") {
		t.Fatalf("expected hooks.json output")
	}
}

func TestCompileHooks_RejectsInvalidRelativePath(t *testing.T) {
	_, _, err := CompileTarget([]Record{
		{
			ID:           "../escape.yaml",
			RelativePath: "../escape.yaml",
			Tool:         "codex",
			Event:        "PreToolUse",
			Matcher:      "Bash",
			Handlers:     []Handler{{Type: "command", Command: "./bin/check"}},
		},
	}, "codex", "codex", "/tmp/project", "")
	if err == nil {
		t.Fatal("expected invalid managed path error")
	}
}

func TestCompileHooks_SkipsDisabledHook(t *testing.T) {
	files, warnings, err := CompileTarget([]Record{{
		ID:       "claude/pre-tool-use/bash.yaml",
		Tool:     "claude",
		Event:    "PreToolUse",
		Matcher:  "Bash",
		Disabled: true,
		Handlers: []Handler{{Type: "command", Command: "./bin/check"}},
	}}, "claude", "claude-work", t.TempDir(), "")
	if err != nil {
		t.Fatalf("CompileTarget() error = %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("CompileTarget() files = %v, want none", files)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileTarget() warnings = %v, want none", warnings)
	}
}

func TestCompileHooks_GeminiWritesSettingsJSON(t *testing.T) {
	projectRoot := t.TempDir()
	sequential := true
	records := []Record{
		{
			ID:           "gemini/before-tool/read.yaml",
			RelativePath: "gemini/before-tool/read.yaml",
			Tool:         "gemini",
			Event:        "BeforeTool",
			Matcher:      "Read",
			Sequential:   &sequential,
			Handlers: []Handler{{
				Type:        "command",
				Name:        "lint-read",
				Description: "Run read lint",
				Command:     "./bin/gemini-lint",
				Timeout:     "30000",
			}},
		},
	}

	files, warnings, err := CompileTarget(records, "gemini", "gemini", projectRoot, `{"theme":"light"}`)
	if err != nil {
		t.Fatalf("CompileTarget() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileTarget() warnings = %v, want none", warnings)
	}
	if !containsCompiledPath(files, projectRoot+"/.gemini/settings.json") {
		t.Fatalf("expected gemini settings output, got %v", files)
	}
	if !containsCompiledContent(files, projectRoot+"/.gemini/settings.json", `"name":"lint-read"`) {
		t.Fatalf("expected gemini hook metadata in compiled settings")
	}
	if !containsCompiledContent(files, projectRoot+"/.gemini/settings.json", `"sequential":true`) {
		t.Fatalf("expected gemini sequential flag in compiled settings")
	}
	if !containsCompiledContent(files, projectRoot+"/.gemini/settings.json", `"theme":"light"`) {
		t.Fatalf("expected raw gemini config content to be preserved")
	}
}

func containsCompiledContent(files []CompiledFile, wantPath, wantSubstring string) bool {
	for _, file := range files {
		if file.Path == wantPath {
			return strings.Contains(file.Content, wantSubstring)
		}
	}
	return false
}

func containsCompiledPath(files []CompiledFile, wantPath string) bool {
	for _, file := range files {
		if file.Path == wantPath {
			return true
		}
	}
	return false
}
