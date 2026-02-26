package main

import (
	"strings"
	"testing"

	"skillshare/internal/install"
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

func TestRepoSourceForGroupedClone(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "github shorthand subdir",
			input: "openai/skills/skills/react",
		},
		{
			name:  "gitlab web tree url",
			input: "https://gitlab.com/team/repo/-/tree/main/skills/docker",
		},
		{
			name:  "bitbucket web src url",
			input: "https://bitbucket.org/team/repo/src/main/skills/vue",
		},
		{
			name:  "azure devops https subdir",
			input: "https://dev.azure.com/org/project/_git/repo/skills/node",
		},
		{
			name:  "git ssh subdir",
			input: "git@gitlab.com:team/monorepo.git//frontend/ui-skill",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := install.ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource(%q) error: %v", tt.input, err)
			}
			if src.Subdir == "" {
				t.Fatalf("test input must include subdir, got empty for %q", tt.input)
			}

			repo := repoSourceForGroupedClone(src)

			if repo.Subdir != "" {
				t.Errorf("repo.Subdir = %q, want empty", repo.Subdir)
			}
			if repo.Raw == "" {
				t.Fatal("repo.Raw should not be empty")
			}
			if repo.CloneURL != src.CloneURL {
				t.Errorf("repo.CloneURL = %q, want %q", repo.CloneURL, src.CloneURL)
			}
			if repo.Name == "" {
				t.Fatal("repo.Name should not be empty")
			}

			parsedRoot, err := install.ParseSource(repo.Raw)
			if err != nil {
				t.Fatalf("repo.Raw should be parseable as root source, got error: %v (raw=%q)", err, repo.Raw)
			}
			if parsedRoot.Subdir != "" {
				t.Errorf("parsed root source should not have subdir, got %q", parsedRoot.Subdir)
			}
			if parsedRoot.CloneURL != src.CloneURL {
				t.Errorf("parsed root CloneURL = %q, want %q", parsedRoot.CloneURL, src.CloneURL)
			}
		})
	}
}
