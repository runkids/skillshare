package main

import (
	"strings"
	"testing"

	"skillshare/internal/install"
)

func TestFilterSkillsByName_ExactMatch(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "figma", Path: "figma"},
		{Name: "pdf", Path: "pdf"},
		{Name: "github-actions", Path: "github-actions"},
	}

	matched, notFound := filterSkillsByName(skills, []string{"pdf"})
	if len(notFound) > 0 {
		t.Errorf("unexpected notFound: %v", notFound)
	}
	if len(matched) != 1 || matched[0].Name != "pdf" {
		t.Errorf("expected [pdf], got %v", matched)
	}
}

func TestFilterSkillsByName_FuzzyMatchSingle(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "figma", Path: "figma"},
		{Name: "pdf", Path: "pdf"},
		{Name: "github-actions", Path: "github-actions"},
	}

	matched, notFound := filterSkillsByName(skills, []string{"fig"})
	if len(notFound) > 0 {
		t.Errorf("unexpected notFound: %v", notFound)
	}
	if len(matched) != 1 || matched[0].Name != "figma" {
		t.Errorf("expected fuzzy match [figma], got %v", matched)
	}
}

func TestFilterSkillsByName_FuzzyMatchAmbiguous(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "github-actions", Path: "github-actions"},
		{Name: "github-pr", Path: "github-pr"},
		{Name: "pdf", Path: "pdf"},
	}

	matched, notFound := filterSkillsByName(skills, []string{"github"})
	if len(matched) != 0 {
		t.Errorf("expected no match for ambiguous query, got %v", matched)
	}
	if len(notFound) != 1 {
		t.Fatalf("expected 1 notFound, got %d", len(notFound))
	}
	if notFound[0] == "github" {
		t.Error("notFound should contain 'did you mean' suggestions, got plain name")
	}
}

func TestFilterSkillsByName_NotFound(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "figma", Path: "figma"},
		{Name: "pdf", Path: "pdf"},
	}

	matched, notFound := filterSkillsByName(skills, []string{"zzzzz"})
	if len(matched) != 0 {
		t.Errorf("expected no matches, got %v", matched)
	}
	if len(notFound) != 1 || notFound[0] != "zzzzz" {
		t.Errorf("expected notFound=[zzzzz], got %v", notFound)
	}
}

func TestFilterSkillsByName_ExactTakesPriority(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "fig", Path: "fig"},
		{Name: "figma", Path: "figma"},
	}

	matched, notFound := filterSkillsByName(skills, []string{"fig"})
	if len(notFound) > 0 {
		t.Errorf("unexpected notFound: %v", notFound)
	}
	if len(matched) != 1 || matched[0].Name != "fig" {
		t.Errorf("exact match should take priority, got %v", matched)
	}
}

// --- Glob matching tests ---

func TestFilterSkillsByName_GlobWildcard(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "core-auth", Path: "core-auth"},
		{Name: "core-db", Path: "core-db"},
		{Name: "utils", Path: "utils"},
	}

	matched, notFound := filterSkillsByName(skills, []string{"core-*"})
	if len(notFound) > 0 {
		t.Errorf("unexpected notFound: %v", notFound)
	}
	if len(matched) != 2 {
		t.Fatalf("expected 2 glob matches, got %d: %v", len(matched), matched)
	}
	names := map[string]bool{}
	for _, m := range matched {
		names[m.Name] = true
	}
	if !names["core-auth"] || !names["core-db"] {
		t.Errorf("expected core-auth and core-db, got %v", matched)
	}
}

func TestFilterSkillsByName_GlobQuestionMark(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "test-a", Path: "test-a"},
		{Name: "test-b", Path: "test-b"},
		{Name: "test-ab", Path: "test-ab"},
	}

	matched, notFound := filterSkillsByName(skills, []string{"test-?"})
	if len(notFound) > 0 {
		t.Errorf("unexpected notFound: %v", notFound)
	}
	if len(matched) != 2 {
		t.Fatalf("expected 2 matches (test-a, test-b), got %d", len(matched))
	}
}

func TestFilterSkillsByName_GlobCaseInsensitive(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "Core-Auth", Path: "Core-Auth"},
		{Name: "core-db", Path: "core-db"},
	}

	matched, notFound := filterSkillsByName(skills, []string{"core-*"})
	if len(notFound) > 0 {
		t.Errorf("unexpected notFound: %v", notFound)
	}
	if len(matched) != 2 {
		t.Fatalf("expected 2 case-insensitive glob matches, got %d", len(matched))
	}
}

func TestFilterSkillsByName_GlobNoMatch(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "figma", Path: "figma"},
		{Name: "pdf", Path: "pdf"},
	}

	matched, notFound := filterSkillsByName(skills, []string{"core-*"})
	if len(matched) != 0 {
		t.Errorf("expected no matches, got %v", matched)
	}
	if len(notFound) != 1 || notFound[0] != "core-*" {
		t.Errorf("expected notFound=[core-*], got %v", notFound)
	}
}

func TestFilterSkillsByName_GlobStarAll(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "a", Path: "a"},
		{Name: "b", Path: "b"},
	}

	matched, notFound := filterSkillsByName(skills, []string{"*"})
	if len(notFound) > 0 {
		t.Errorf("unexpected notFound: %v", notFound)
	}
	if len(matched) != 2 {
		t.Errorf("glob '*' should match all skills, got %d", len(matched))
	}
}

func TestFilterSkillsByName_ExactTakesPriorityOverGlob(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "core-*", Path: "core-*"},
		{Name: "core-auth", Path: "core-auth"},
	}

	// If a skill is literally named "core-*", exact match wins
	matched, notFound := filterSkillsByName(skills, []string{"core-*"})
	if len(notFound) > 0 {
		t.Errorf("unexpected notFound: %v", notFound)
	}
	if len(matched) != 1 || matched[0].Name != "core-*" {
		t.Errorf("exact match should take priority over glob, got %v", matched)
	}
}

// --- applyExclude glob tests ---

func TestApplyExclude_GlobPattern(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "keep-a", Path: "keep-a"},
		{Name: "test-one", Path: "test-one"},
		{Name: "test-two", Path: "test-two"},
	}

	filtered := applyExclude(skills, []string{"test-*"})
	if len(filtered) != 1 {
		t.Fatalf("expected 1 remaining skill, got %d: %v", len(filtered), filtered)
	}
	if filtered[0].Name != "keep-a" {
		t.Errorf("expected keep-a, got %s", filtered[0].Name)
	}
}

func TestApplyExclude_MixedExactAndGlob(t *testing.T) {
	skills := []install.SkillInfo{
		{Name: "keep", Path: "keep"},
		{Name: "drop-a", Path: "drop-a"},
		{Name: "drop-b", Path: "drop-b"},
		{Name: "extra", Path: "extra"},
	}

	filtered := applyExclude(skills, []string{"extra", "drop-*"})
	if len(filtered) != 1 {
		t.Fatalf("expected 1 remaining skill, got %d: %v", len(filtered), filtered)
	}
	if filtered[0].Name != "keep" {
		t.Errorf("expected keep, got %s", filtered[0].Name)
	}
}

// --- isGlobPattern / matchGlob tests ---

func TestIsGlobPattern(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"core-*", true},
		{"test-?", true},
		{"[abc]", true},
		{"plain-name", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isGlobPattern(tt.input); got != tt.want {
			t.Errorf("isGlobPattern(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern, name string
		want          bool
	}{
		{"core-*", "core-auth", true},
		{"core-*", "CORE-AUTH", true},
		{"CORE-*", "core-auth", true},
		{"test-?", "test-a", true},
		{"test-?", "test-ab", false},
		{"*", "anything", true},
		{"no-match", "something", false},
	}
	for _, tt := range tests {
		if got := matchGlob(tt.pattern, tt.name); got != tt.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}

func TestNormalizeInstallAuditThreshold(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "critical", want: "CRITICAL"},
		{in: "high", want: "HIGH"},
		{in: "medium", want: "MEDIUM"},
		{in: "low", want: "LOW"},
		{in: "info", want: "INFO"},
		{in: "c", want: "CRITICAL"},
		{in: "h", want: "HIGH"},
		{in: "m", want: "MEDIUM"},
		{in: "l", want: "LOW"},
		{in: "i", want: "INFO"},
		{in: "crit", want: "CRITICAL"},
		{in: "med", want: "MEDIUM"},
		{in: "x", wantErr: true},
	}

	for _, tt := range tests {
		got, err := normalizeInstallAuditThreshold(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("normalizeInstallAuditThreshold(%q) expected error", tt.in)
			}
			if !strings.Contains(err.Error(), "invalid audit threshold") {
				t.Fatalf("normalizeInstallAuditThreshold(%q) error mismatch: %v", tt.in, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("normalizeInstallAuditThreshold(%q) unexpected error: %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("normalizeInstallAuditThreshold(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
