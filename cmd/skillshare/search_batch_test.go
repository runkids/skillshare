package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/search"
	"skillshare/internal/testutil"
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

			repo := repoSourceForGroupedClone(src, install.ParseOptions{})

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

func TestGroupByRepo_PinnedWebURLs(t *testing.T) {
	// Stands in for the remote: "feature/x" is a branch, "nope" is unknown,
	// and any other first segment is a branch or tag.
	resolve := func(s *install.Source) error {
		switch {
		case strings.Contains(s.Raw, "/tree/feature/x/"):
			s.ApplyRecordedBranch("feature/x")
		case strings.Contains(s.Raw, "/tree/nope/"):
			return errors.New("ref not found")
		default:
			s.ApplyRecordedBranch(s.Branch)
		}
		return nil
	}
	selected := []search.SearchResult{
		{Name: "a", Source: "github.com/org/skills/tree/v1/skills/a"},
		{Name: "b", Source: "github.com/org/skills/tree/v1/skills/b"},
		{Name: "c", Source: "github.com/org/skills/tree/v2/skills/c"},
		{Name: "d", Source: "github.com/org/skills/tree/feature/x/skills/d"},
		{Name: "e", Source: "github.com/org/skills/tree/feature/x/skills/e"},
		{Name: "f", Source: "github.com/org/skills/tree/nope/skills/f"},
		{Name: "g", Source: "github.com/org/skills/skills/g"},
		{Name: "h", Source: "github.com/org/skills/skills/h"},
	}

	groups, singles := groupByRepo(selected, install.ParseOptions{}, resolve)

	var got []string
	for _, g := range groups {
		var names []string
		for _, r := range g.results {
			names = append(names, r.Name)
		}
		got = append(got, g.source.Branch+":"+strings.Join(names, ","))
	}
	if want := []string{"v1:a,b", ":g,h"}; !slices.Equal(got, want) {
		t.Errorf("groups = %v, want %v", got, want)
	}
	var single []string
	for _, sr := range singles {
		single = append(single, sr.Name)
	}
	// c is alone on v2; d and e have a ref containing "/"; f did not resolve.
	if want := []string{"d", "e", "f", "c"}; !slices.Equal(single, want) {
		t.Errorf("singles = %v, want %v", single, want)
	}
}

func TestGroupByRepo_ConfiguredHosts(t *testing.T) {
	base := t.TempDir()
	repo := testutil.SetupBareRemoteRepo(t, base)
	const skillA = "---\nname: a\ndescription: First skill\n---\n# First skill\n"
	const skillB = "---\nname: b\ndescription: Second skill\n---\n# Second skill\n"
	testutil.SeedRemoteBranch(t, base, repo, "main", map[string]string{
		"skills/a/SKILL.md": skillA,
		"skills/b/SKILL.md": skillB,
	})
	for _, tt := range []struct {
		host, username, tokenVar string
		opts                     install.ParseOptions
	}{
		{"cnb.corp.example", "cnb", "CNB_TOKEN", install.ParseOptions{CNBHosts: []string{"cnb.corp.example"}}},
		{"forge.corp.example", "x-access-token", "GITEA_TOKEN", install.ParseOptions{GiteaHosts: []string{"forge.corp.example"}}},
	} {
		t.Run(tt.host, func(t *testing.T) {
			t.Setenv(tt.tokenVar, "grouped-search-token")
			testutil.ServePrivateGit(t, repo, tt.username, "grouped-search-token")
			selected := []search.SearchResult{
				{Name: "a", Source: "https://" + tt.host + "/org/repo/skills/a"},
				{Name: "b", Source: "https://" + tt.host + "/org/repo/skills/b"},
			}
			groups, singles := groupByRepo(selected, tt.opts, func(*install.Source) error { return nil })
			if len(groups) != 1 || len(singles) != 0 {
				t.Fatalf("groups = %d, singles = %d; want one shared clone", len(groups), len(singles))
			}
			root := repoSourceForGroupedClone(groups[0].source, tt.opts)
			discovery, err := install.DiscoverFromGitWithProgress(&root, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer install.CleanupDiscovery(discovery)
			dest := t.TempDir()
			for _, result := range selected {
				got := installFromDiscoveryResult(discovery, result, dest, tt.opts)
				if got.status != "installed" {
					t.Fatalf("grouped install %s: %+v", result.Name, got)
				}
				body, err := os.ReadFile(filepath.Join(dest, result.Name, "SKILL.md"))
				if err != nil || !strings.Contains(string(body), "name: "+result.Name) {
					t.Fatalf("installed %s content = %q, error = %v", result.Name, body, err)
				}
			}
		})
	}
}
