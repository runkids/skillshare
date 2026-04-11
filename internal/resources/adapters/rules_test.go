package adapters

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileClaudeRules_InstructionAndAdditionalRules(t *testing.T) {
	projectRoot := "/tmp/project"
	records := []RuleRecord{
		{ID: "claude/CLAUDE.md", Tool: "claude", RelativePath: "claude/CLAUDE.md", Name: "CLAUDE.md", Content: "# Claude Root\n"},
		{ID: "claude/backend.md", Tool: "claude", RelativePath: "claude/backend.md", Name: "backend.md", Content: "# Backend\n"},
	}

	files, warnings, err := CompileClaudeRules(records, projectRoot)
	if err != nil {
		t.Fatalf("CompileClaudeRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileClaudeRules() warnings = %v, want none", warnings)
	}

	_ = findCompiledFile(t, files, filepath.Join(projectRoot, "CLAUDE.md"))
	rule := findCompiledFile(t, files, filepath.Join(projectRoot, ".claude", "rules", "backend.md"))
	if !strings.Contains(rule.Content, "# Backend") {
		t.Fatalf("compiled backend rule content = %q, want to include backend markdown", rule.Content)
	}
}

func TestCompileClaudeRules_GlobalConfigRootUsesRulesSubdir(t *testing.T) {
	globalRoot := "/tmp/home/.claude"
	records := []RuleRecord{
		{ID: "claude/CLAUDE.md", Tool: "claude", RelativePath: "claude/CLAUDE.md", Name: "CLAUDE.md", Content: "# Claude Root\n"},
		{ID: "claude/backend.md", Tool: "claude", RelativePath: "claude/backend.md", Name: "backend.md", Content: "# Backend\n"},
	}

	files, warnings, err := CompileClaudeRules(records, globalRoot)
	if err != nil {
		t.Fatalf("CompileClaudeRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileClaudeRules() warnings = %v, want none", warnings)
	}

	_ = findCompiledFile(t, files, filepath.Join(globalRoot, "CLAUDE.md"))
	_ = findCompiledFile(t, files, filepath.Join(globalRoot, "rules", "backend.md"))
	mustNotContainCompiledFile(t, files, filepath.Join(globalRoot, ".claude", "rules", "backend.md"))
}

func TestCompileCodexRules_AggregatesWithMarkers(t *testing.T) {
	projectRoot := "/tmp/project"
	records := []RuleRecord{
		{ID: "codex/AGENTS.md", Tool: "codex", RelativePath: "codex/AGENTS.md", Name: "AGENTS.md", Content: "# Root\n"},
		{ID: "codex/backend.md", Tool: "codex", RelativePath: "codex/backend.md", Name: "backend.md", Content: "# Backend\n"},
	}

	files, warnings, err := CompileCodexRules(records, projectRoot)
	if err != nil {
		t.Fatalf("CompileCodexRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileCodexRules() warnings = %v, want none", warnings)
	}

	compiled := findCompiledFile(t, files, filepath.Join(projectRoot, "AGENTS.md"))
	if !strings.Contains(compiled.Content, "<!-- skillshare:codex/backend.md -->") {
		t.Fatalf("compiled AGENTS missing source marker for backend; content = %q", compiled.Content)
	}
}

func TestCompileGeminiRules_InstructionAndAdditionalRules(t *testing.T) {
	projectRoot := "/tmp/project"
	records := []RuleRecord{
		{ID: "gemini/GEMINI.md", Tool: "gemini", RelativePath: "gemini/GEMINI.md", Name: "GEMINI.md", Content: "# Gemini Root\n"},
		{ID: "gemini/backend.md", Tool: "gemini", RelativePath: "gemini/backend.md", Name: "backend.md", Content: "# Backend\n"},
	}

	files, warnings, err := CompileGeminiRules(records, projectRoot)
	if err != nil {
		t.Fatalf("CompileGeminiRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileGeminiRules() warnings = %v, want none", warnings)
	}

	_ = findCompiledFile(t, files, filepath.Join(projectRoot, "GEMINI.md"))
	_ = findCompiledFile(t, files, filepath.Join(projectRoot, ".gemini", "rules", "backend.md"))
}

func TestCompileGeminiRules_GlobalConfigRootUsesRulesSubdir(t *testing.T) {
	globalRoot := "/tmp/home/.gemini"
	records := []RuleRecord{
		{ID: "gemini/GEMINI.md", Tool: "gemini", RelativePath: "gemini/GEMINI.md", Name: "GEMINI.md", Content: "# Gemini Root\n"},
		{ID: "gemini/backend.md", Tool: "gemini", RelativePath: "gemini/backend.md", Name: "backend.md", Content: "# Backend\n"},
	}

	files, warnings, err := CompileGeminiRules(records, globalRoot)
	if err != nil {
		t.Fatalf("CompileGeminiRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileGeminiRules() warnings = %v, want none", warnings)
	}

	_ = findCompiledFile(t, files, filepath.Join(globalRoot, "GEMINI.md"))
	_ = findCompiledFile(t, files, filepath.Join(globalRoot, "rules", "backend.md"))
	mustNotContainCompiledFile(t, files, filepath.Join(globalRoot, ".gemini", "rules", "backend.md"))
}

func findCompiledFile(t *testing.T, files []CompiledFile, wantPath string) CompiledFile {
	t.Helper()
	for _, file := range files {
		if file.Path == wantPath {
			return file
		}
	}
	t.Fatalf("compiled output missing %q", wantPath)
	return CompiledFile{}
}

func mustNotContainCompiledFile(t *testing.T, files []CompiledFile, wantPath string) {
	t.Helper()
	for _, file := range files {
		if file.Path == wantPath {
			t.Fatalf("compiled output unexpectedly contained %q", wantPath)
		}
	}
}
