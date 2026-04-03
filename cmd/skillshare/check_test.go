package main

import "testing"

func TestURLBranchKey_RoundTrip(t *testing.T) {
	tests := []struct {
		url    string
		branch string
	}{
		{"https://github.com/owner/repo", "develop"},
		{"https://github.com/owner/repo", ""},
		{"git@github.com:org/repo.git", ""},
		{"git@github.com:org/repo.git", "feature"},
		{"https://gitlab.com/group/subgroup/project.git", "main"},
		{"file:///tmp/repo.git", "dev"},
	}

	for _, tt := range tests {
		key := urlBranchKey(tt.url, tt.branch)
		gotURL, gotBranch := splitURLBranch(key)
		if gotURL != tt.url {
			t.Errorf("urlBranchKey(%q, %q) → splitURLBranch: URL = %q, want %q", tt.url, tt.branch, gotURL, tt.url)
		}
		if gotBranch != tt.branch {
			t.Errorf("urlBranchKey(%q, %q) → splitURLBranch: Branch = %q, want %q", tt.url, tt.branch, gotBranch, tt.branch)
		}
	}
}

func TestSplitURLBranch_SSHURLWithoutBranch(t *testing.T) {
	// Regression: "@" in SSH URLs must not be confused with the separator.
	url, branch := splitURLBranch("git@github.com:org/repo.git")
	if url != "git@github.com:org/repo.git" {
		t.Errorf("URL = %q, want full SSH URL", url)
	}
	if branch != "" {
		t.Errorf("Branch = %q, want empty", branch)
	}
}
