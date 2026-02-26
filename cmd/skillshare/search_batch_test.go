package main

import (
	"strings"
	"testing"
)

func TestClassifyFailureDetail(t *testing.T) {
	tests := []struct {
		name          string
		detail        string
		wantIcon      string
		wantColorName string // "red", "yellow"
		wantSummary   string
		wantMaxSub    int  // max expected sub-lines
		wantTruncated bool // expect "+N more" in last sub-line
	}{
		{
			name:          "simple not found",
			detail:        "subdirectory 'azure-postgres' does not exist in repository",
			wantIcon:      "✗",
			wantColorName: "red",
			wantSummary:   "subdirectory 'azure-postgres' does not exist in repository",
			wantMaxSub:    0,
		},
		{
			name: "ambiguous with 5 matches truncated to 3+1",
			detail: "subdirectory 'docker-expert' is ambiguous — multiple matches found:\n" +
				"  skills/docker-expert\n" +
				"  web-app/public/skills/docker-expert\n" +
				"  .cursor/skills/docker-expert\n" +
				"  .claude/skills/docker-expert\n" +
				"  .codex/skills/docker-expert",
			wantIcon:      "!",
			wantColorName: "yellow",
			wantSummary:   "subdirectory 'docker-expert' is ambiguous — multiple matches found:",
			wantMaxSub:    4, // 3 paths + "(+2 more)"
			wantTruncated: true,
		},
		{
			name: "ambiguous with 2 matches no truncation",
			detail: "subdirectory 'seo-geo' is ambiguous — multiple matches found:\n" +
				"  .agents/skills/seo-geo\n" +
				"  skills/seo-geo",
			wantIcon:      "!",
			wantColorName: "yellow",
			wantSummary:   "subdirectory 'seo-geo' is ambiguous — multiple matches found:",
			wantMaxSub:    2,
			wantTruncated: false,
		},
		{
			name: "security audit block truncated",
			detail: "security audit failed — findings at/above CRITICAL detected:\n" +
				"  CRITICAL: Command sends environment variables externally (file.sh:547)\n" +
				"    \"curl -L https://example.com\"\n" +
				"  CRITICAL: Command sends environment variables externally (file.sh:549)\n" +
				"    \"curl -L https://example.com/arm64\"\n" +
				"  CRITICAL: Writes to system path (file.sh:600)\n" +
				"    \"mv /tmp/bin /usr/local/bin\"\n" +
				"  CRITICAL: Modifies PATH (file.sh:610)\n" +
				"    \"export PATH=...\"\n" +
				"\n" +
				"Use --force to override or --skip-audit to bypass scanning: blocked by security audit",
			wantIcon:      "✗",
			wantColorName: "red",
			wantSummary:   "security audit failed — findings at/above CRITICAL detected:",
			wantMaxSub:    4, // 3 findings + "(+N more...)"
			wantTruncated: true,
		},
		{
			name:          "clone failed",
			detail:        "clone failed: exit status 128",
			wantIcon:      "✗",
			wantColorName: "red",
			wantSummary:   "clone failed: exit status 128",
			wantMaxSub:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icon, _, summary, subLines := classifyFailureDetail(tt.detail)

			if icon != tt.wantIcon {
				t.Errorf("icon = %q, want %q", icon, tt.wantIcon)
			}
			if summary != tt.wantSummary {
				t.Errorf("summary = %q, want %q", summary, tt.wantSummary)
			}
			if len(subLines) > tt.wantMaxSub {
				t.Errorf("subLines count = %d, want at most %d\n  lines: %v", len(subLines), tt.wantMaxSub, subLines)
			}
			if tt.wantTruncated {
				if len(subLines) == 0 {
					t.Fatal("expected truncation indicator but got no sub-lines")
				}
				last := subLines[len(subLines)-1]
				if !strings.Contains(last, "+") || !strings.Contains(last, "more") {
					t.Errorf("last sub-line should contain truncation indicator, got %q", last)
				}
			}
		})
	}
}
