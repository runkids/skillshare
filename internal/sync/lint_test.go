package sync

import (
	"strings"
	"testing"
)

func TestLintSkill_MissingName(t *testing.T) {
	issues := LintSkill("", "some description", 100)
	assertHasIssue(t, issues, "missing-name", LintError)
}

func TestLintSkill_MissingDescription(t *testing.T) {
	issues := LintSkill("my-skill", "", 100)
	assertHasIssue(t, issues, "missing-description", LintError)
}

func TestLintSkill_EmptyBody(t *testing.T) {
	issues := LintSkill("my-skill", "Use this skill when testing", 0)
	assertHasIssue(t, issues, "empty-body", LintError)
}

func TestLintSkill_DescriptionTooShort(t *testing.T) {
	issues := LintSkill("my-skill", "Short desc", 100)
	assertHasIssue(t, issues, "description-too-short", LintWarning)
	for _, issue := range issues {
		if issue.Rule == "description-too-short" && !strings.Contains(issue.Message, "10 chars") {
			t.Errorf("expected message to contain char count, got: %s", issue.Message)
		}
	}
}

func TestLintSkill_DescriptionTooLong(t *testing.T) {
	longDesc := strings.Repeat("a", 1025)
	issues := LintSkill("my-skill", longDesc, 100)
	assertHasIssue(t, issues, "description-too-long", LintWarning)
}

func TestLintSkill_DescriptionNearLimit(t *testing.T) {
	desc := strings.Repeat("a", 950)
	issues := LintSkill("my-skill", desc, 100)
	assertHasIssue(t, issues, "description-near-limit", LintWarning)
	assertNoIssue(t, issues, "description-too-long")
}

func TestLintSkill_DescriptionAtExact1024(t *testing.T) {
	desc := strings.Repeat("a", 1024)
	issues := LintSkill("my-skill", desc, 100)
	assertHasIssue(t, issues, "description-near-limit", LintWarning)
	assertNoIssue(t, issues, "description-too-long")
}

func TestLintSkill_NoTriggerPhrase(t *testing.T) {
	issues := LintSkill("my-skill", "This analyzes CSV data and generates reports for the user", 100)
	assertHasIssue(t, issues, "no-trigger-phrase", LintWarning)
}

func TestLintSkill_WithTriggerPhrase(t *testing.T) {
	issues := LintSkill("my-skill", "Use this skill when analyzing CSV data and generating reports", 100)
	assertNoIssue(t, issues, "no-trigger-phrase")
}

func TestLintSkill_TriggerPhraseVariants(t *testing.T) {
	variants := []string{
		"Use when the user wants to analyze data",
		"Trigger when files need processing",
		"Use this when building APIs",
		"Use for data transformation tasks",
		"Activate when the user mentions CSV",
	}
	for _, desc := range variants {
		issues := LintSkill("my-skill", desc, 100)
		assertNoIssue(t, issues, "no-trigger-phrase")
	}
}

func TestLintSkill_CleanSkill(t *testing.T) {
	issues := LintSkill("my-skill", "Use this skill when the user needs to analyze CSV data and generate charts. Works with CSV, TSV, and Excel files.", 500)
	if len(issues) != 0 {
		t.Errorf("expected no issues for clean skill, got %d: %v", len(issues), issues)
	}
}

func TestLintSkill_EmptyDescriptionNoDoubleReport(t *testing.T) {
	issues := LintSkill("my-skill", "", 100)
	assertHasIssue(t, issues, "missing-description", LintError)
	assertNoIssue(t, issues, "description-too-short")
	assertNoIssue(t, issues, "no-trigger-phrase")
}

func TestLintSkill_CategoryField(t *testing.T) {
	issues := LintSkill("", "", 0)
	for _, issue := range issues {
		if issue.Category == "" {
			t.Errorf("issue %s has empty category", issue.Rule)
		}
	}
}

func assertHasIssue(t *testing.T, issues []LintIssue, rule string, severity LintSeverity) {
	t.Helper()
	for _, issue := range issues {
		if issue.Rule == rule {
			if issue.Severity != severity {
				t.Errorf("rule %s: expected severity %s, got %s", rule, severity, issue.Severity)
			}
			return
		}
	}
	t.Errorf("expected issue %s not found in %v", rule, issues)
}

func assertNoIssue(t *testing.T, issues []LintIssue, rule string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Rule == rule {
			t.Errorf("unexpected issue %s found: %s", rule, issue.Message)
		}
	}
}
