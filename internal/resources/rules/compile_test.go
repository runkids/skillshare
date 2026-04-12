package rules

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileRulesForTargets(t *testing.T) {
	projectRoot := "/tmp/project"
	ruleSet := []Record{
		{ID: "claude/CLAUDE.md", Content: []byte("# Claude Root\n")},
		{ID: "claude/backend.md", Content: []byte("# Claude Backend\n")},
		{ID: "codex/AGENTS.md", Content: []byte("# Codex Root\n")},
		{ID: "codex/backend.md", Content: []byte("# Codex Backend\n")},
		{ID: "gemini/GEMINI.md", Content: []byte("# Gemini Root\n")},
		{ID: "gemini/backend.md", Content: []byte("# Gemini Backend\n")},
		{ID: "pi/AGENTS.md", Content: []byte("# Pi Root\n")},
		{ID: "pi/SYSTEM.md", Content: []byte("# Pi System\n")},
		{ID: "pi/APPEND_SYSTEM.md", Content: []byte("# Pi Append\n")},
	}

	codexFiles, warnings, err := CompileTarget(ruleSet, "codex", "codex", projectRoot)
	if err != nil {
		t.Fatalf("CompileTarget(codex) error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileTarget(codex) warnings = %v, want none", warnings)
	}
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	agentsContent := mustFindCompiledContent(t, codexFiles, agentsPath)
	if !strings.Contains(agentsContent, "<!-- skillshare:codex/backend.md -->") {
		t.Fatalf("AGENTS output missing backend marker; content = %q", agentsContent)
	}
	if !strings.Contains(agentsContent, "# Codex Backend") {
		t.Fatalf("AGENTS output missing codex backend content; content = %q", agentsContent)
	}

	claudeFiles, warnings, err := CompileTarget(ruleSet, "claude", "claude", projectRoot)
	if err != nil {
		t.Fatalf("CompileTarget(claude) error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileTarget(claude) warnings = %v, want none", warnings)
	}
	_ = mustFindCompiledContent(t, claudeFiles, filepath.Join(projectRoot, "CLAUDE.md"))
	_ = mustFindCompiledContent(t, claudeFiles, filepath.Join(projectRoot, ".claude", "rules", "backend.md"))

	geminiFiles, warnings, err := CompileTarget(ruleSet, "gemini", "gemini", projectRoot)
	if err != nil {
		t.Fatalf("CompileTarget(gemini) error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileTarget(gemini) warnings = %v, want none", warnings)
	}
	_ = mustFindCompiledContent(t, geminiFiles, filepath.Join(projectRoot, "GEMINI.md"))
	_ = mustFindCompiledContent(t, geminiFiles, filepath.Join(projectRoot, ".gemini", "rules", "backend.md"))

	piFiles, warnings, err := CompileTarget(ruleSet, "pi", "pi", projectRoot)
	if err != nil {
		t.Fatalf("CompileTarget(pi) error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileTarget(pi) warnings = %v, want none", warnings)
	}
	_ = mustFindCompiledContent(t, piFiles, filepath.Join(projectRoot, "AGENTS.md"))
	_ = mustFindCompiledContent(t, piFiles, filepath.Join(projectRoot, ".pi", "SYSTEM.md"))
	_ = mustFindCompiledContent(t, piFiles, filepath.Join(projectRoot, ".pi", "APPEND_SYSTEM.md"))
}

func TestCompileRulesForTargets_NestedInstructionNamesStayNested(t *testing.T) {
	projectRoot := "/tmp/project"
	ruleSet := []Record{
		{ID: "claude/nested/CLAUDE.md", Content: []byte("# Nested Claude\n")},
		{ID: "gemini/nested/GEMINI.md", Content: []byte("# Nested Gemini\n")},
	}

	claudeFiles, warnings, err := CompileTarget(ruleSet, "claude", "claude", projectRoot)
	if err != nil {
		t.Fatalf("CompileTarget(claude) error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileTarget(claude) warnings = %v, want none", warnings)
	}
	_ = mustFindCompiledContent(t, claudeFiles, filepath.Join(projectRoot, ".claude", "rules", "nested", "CLAUDE.md"))
	mustNotContainCompiledPath(t, claudeFiles, filepath.Join(projectRoot, "CLAUDE.md"))

	geminiFiles, warnings, err := CompileTarget(ruleSet, "gemini", "gemini", projectRoot)
	if err != nil {
		t.Fatalf("CompileTarget(gemini) error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileTarget(gemini) warnings = %v, want none", warnings)
	}
	_ = mustFindCompiledContent(t, geminiFiles, filepath.Join(projectRoot, ".gemini", "rules", "nested", "GEMINI.md"))
	mustNotContainCompiledPath(t, geminiFiles, filepath.Join(projectRoot, "GEMINI.md"))
}

func TestCompileRulesForTargets_SkipsWhenTargetNotAssigned(t *testing.T) {
	files, warnings, err := CompileTarget([]Record{{
		ID:         "claude/manual.md",
		Tool:       "claude",
		Name:       "manual.md",
		Content:    []byte("# Manual\n"),
		Targets:    []string{"claude-work"},
		SourceType: "local",
	}}, "claude", "claude-personal", t.TempDir())
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

func mustFindCompiledContent(t *testing.T, files []CompiledFile, path string) string {
	t.Helper()
	for _, file := range files {
		if file.Path == path {
			return file.Content
		}
	}
	t.Fatalf("compiled output missing path %q", path)
	return ""
}

func mustNotContainCompiledPath(t *testing.T, files []CompiledFile, path string) {
	t.Helper()
	for _, file := range files {
		if file.Path == path {
			t.Fatalf("compiled output unexpectedly contained path %q", path)
		}
	}
}
