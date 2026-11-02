package utils

import "testing"

func TestIsOSMetadataFile(t *testing.T) {
	for name, want := range map[string]bool{
		".DS_Store":   true,
		"Thumbs.db":   true,
		"Desktop.ini": true,
		"SKILL.md":    false,
		".DS_Store~":  false,
	} {
		if got := IsOSMetadataFile(name); got != want {
			t.Errorf("IsOSMetadataFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsHidden(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", false},
		{"hidden file", ".hidden", true},
		{"hidden directory", ".git", true},
		{"normal file", "file.txt", false},
		{"normal directory", "src", false},
		{"dot only", ".", true},
		{"double dot", "..", true},
		{"file starting with number", "1file", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHidden(tt.input)
			if got != tt.expected {
				t.Errorf("IsHidden(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHasTildePrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", false},
		{"tilde path", "~/Documents", true},
		{"tilde only", "~", true},
		{"absolute path", "/home/user", false},
		{"relative path", "./config", false},
		{"tilde in middle", "/home/~user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasTildePrefix(tt.input)
			if got != tt.expected {
				t.Errorf("HasTildePrefix(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestPathToFlatName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"dot only", ".", ""},
		{"single level", "my-skill", "my-skill"},
		{"two levels", "personal/email", "personal__email"},
		{"three levels", "_team/frontend/ui", "_team__frontend__ui"},
		{"deep nesting", "a/b/c/d/e", "a__b__c__d__e"},
		{"with leading slash", "/my-skill", "my-skill"},
		{"with trailing slash", "my-skill/", "my-skill"},
		{"with both slashes", "/my-skill/", "my-skill"},
		{"windows backslash", "a\\b\\c", "a__b__c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PathToFlatName(tt.input)
			if got != tt.expected {
				t.Errorf("PathToFlatName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFlatNameToPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"single level", "my-skill", "my-skill"},
		{"two levels", "personal__email", "personal/email"},
		{"three levels", "_team__frontend__ui", "_team/frontend/ui"},
		{"deep nesting", "a__b__c__d__e", "a/b/c/d/e"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FlatNameToPath(tt.input)
			if got != tt.expected {
				t.Errorf("FlatNameToPath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestPathToFlatNameRoundTrip(t *testing.T) {
	paths := []string{
		"my-skill",
		"personal/email",
		"_team/frontend/ui",
		"a/b/c/d",
	}

	for _, path := range paths {
		flat := PathToFlatName(path)
		back := FlatNameToPath(flat)
		if back != path {
			t.Errorf("Round trip failed: %q -> %q -> %q", path, flat, back)
		}
	}
}

func TestIsTrackedRepoDir(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", false},
		{"tracked repo", "_team-repo", true},
		{"underscore only", "_", true},
		{"normal dir", "my-skill", false},
		{"nested flat name", "_team__ui", true},
		{"underscore in middle", "my_skill", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTrackedRepoDir(tt.input)
			if got != tt.expected {
				t.Errorf("IsTrackedRepoDir(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHasNestedSeparator(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", false},
		{"no separator", "my-skill", false},
		{"single underscore", "my_skill", false},
		{"double underscore", "team__ui", true},
		{"at start", "__prefix", true},
		{"at end", "suffix__", true},
		{"multiple", "a__b__c", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasNestedSeparator(tt.input)
			if got != tt.expected {
				t.Errorf("HasNestedSeparator(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
