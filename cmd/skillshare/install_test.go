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
