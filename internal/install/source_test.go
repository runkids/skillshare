package install

import (
	"testing"
)

func TestParseSource_LocalPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType SourceType
		wantName string
	}{
		{
			name:     "absolute path",
			input:    "/path/to/my-skill",
			wantType: SourceTypeLocalPath,
			wantName: "my-skill",
		},
		{
			name:     "tilde path",
			input:    "~/skills/my-skill",
			wantType: SourceTypeLocalPath,
			wantName: "my-skill",
		},
		{
			name:     "relative path with dot",
			input:    "./local-skill",
			wantType: SourceTypeLocalPath,
			wantName: "local-skill",
		},
		{
			name:     "parent directory path",
			input:    "../other-skill",
			wantType: SourceTypeLocalPath,
			wantName: "other-skill",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource() error = %v", err)
			}
			if source.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", source.Type, tt.wantType)
			}
			if source.Name != tt.wantName {
				t.Errorf("Name = %v, want %v", source.Name, tt.wantName)
			}
		})
	}
}

func TestParseSource_GitHubShorthand(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantCloneURL string
		wantSubdir   string
		wantName     string
	}{
		{
			name:         "basic github shorthand",
			input:        "github.com/user/repo",
			wantCloneURL: "https://github.com/user/repo.git",
			wantSubdir:   "",
			wantName:     "repo",
		},
		{
			name:         "github shorthand with .git",
			input:        "github.com/user/repo.git",
			wantCloneURL: "https://github.com/user/repo.git",
			wantSubdir:   "",
			wantName:     "repo",
		},
		{
			name:         "github with subdirectory",
			input:        "github.com/user/repo/path/to/skill",
			wantCloneURL: "https://github.com/user/repo.git",
			wantSubdir:   "path/to/skill",
			wantName:     "skill",
		},
		{
			name:         "github with https prefix",
			input:        "https://github.com/user/repo",
			wantCloneURL: "https://github.com/user/repo.git",
			wantSubdir:   "",
			wantName:     "repo",
		},
		{
			name:         "github https with .git",
			input:        "https://github.com/user/repo.git",
			wantCloneURL: "https://github.com/user/repo.git",
			wantSubdir:   "",
			wantName:     "repo",
		},
		{
			name:         "github web URL with tree/main",
			input:        "https://github.com/user/repo/tree/main/path/to/skill",
			wantCloneURL: "https://github.com/user/repo.git",
			wantSubdir:   "path/to/skill",
			wantName:     "skill",
		},
		{
			name:         "github web URL with tree/master",
			input:        "github.com/user/repo/tree/master/skills/my-skill",
			wantCloneURL: "https://github.com/user/repo.git",
			wantSubdir:   "skills/my-skill",
			wantName:     "my-skill",
		},
		{
			name:         "github web URL with blob (file view)",
			input:        "https://github.com/user/repo/blob/main/path/to/skill",
			wantCloneURL: "https://github.com/user/repo.git",
			wantSubdir:   "path/to/skill",
			wantName:     "skill",
		},
		{
			name:         "github web URL tree/branch only (no subdir)",
			input:        "https://github.com/user/repo/tree/main",
			wantCloneURL: "https://github.com/user/repo.git",
			wantSubdir:   "",
			wantName:     "repo",
		},
		{
			name:         "github dot subdir normalized to root",
			input:        "github.com/user/repo/.",
			wantCloneURL: "https://github.com/user/repo.git",
			wantSubdir:   "",
			wantName:     "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource() error = %v", err)
			}
			if source.Type != SourceTypeGitHub {
				t.Errorf("Type = %v, want %v", source.Type, SourceTypeGitHub)
			}
			if source.CloneURL != tt.wantCloneURL {
				t.Errorf("CloneURL = %v, want %v", source.CloneURL, tt.wantCloneURL)
			}
			if source.Subdir != tt.wantSubdir {
				t.Errorf("Subdir = %v, want %v", source.Subdir, tt.wantSubdir)
			}
			if source.Name != tt.wantName {
				t.Errorf("Name = %v, want %v", source.Name, tt.wantName)
			}
		})
	}
}

func TestParseSource_GitSSH(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantCloneURL string
		wantSubdir   string
		wantName     string
	}{
		{
			name:         "github ssh",
			input:        "git@github.com:user/repo.git",
			wantCloneURL: "git@github.com:user/repo.git",
			wantName:     "repo",
		},
		{
			name:         "gitlab ssh",
			input:        "git@gitlab.com:user/repo.git",
			wantCloneURL: "git@gitlab.com:user/repo.git",
			wantName:     "repo",
		},
		{
			name:         "ssh without .git",
			input:        "git@github.com:user/my-skill",
			wantCloneURL: "git@github.com:user/my-skill.git",
			wantName:     "my-skill",
		},
		{
			name:         "ssh with subpath using //",
			input:        "git@github.com:owner/repo.git//path/to/skill",
			wantCloneURL: "git@github.com:owner/repo.git",
			wantSubdir:   "path/to/skill",
			wantName:     "skill",
		},
		{
			name:         "ssh with subpath no .git",
			input:        "git@github.com:owner/repo//skills/react",
			wantCloneURL: "git@github.com:owner/repo.git",
			wantSubdir:   "skills/react",
			wantName:     "react",
		},
		{
			name:         "ssh gitlab with subpath",
			input:        "git@gitlab.com:team/monorepo.git//frontend/ui-skill",
			wantCloneURL: "git@gitlab.com:team/monorepo.git",
			wantSubdir:   "frontend/ui-skill",
			wantName:     "ui-skill",
		},
		{
			name:         "ssh with single-level subpath",
			input:        "git@github.com:owner/skills.git//pdf",
			wantCloneURL: "git@github.com:owner/skills.git",
			wantSubdir:   "pdf",
			wantName:     "pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource() error = %v", err)
			}
			if source.Type != SourceTypeGitSSH {
				t.Errorf("Type = %v, want %v", source.Type, SourceTypeGitSSH)
			}
			if source.CloneURL != tt.wantCloneURL {
				t.Errorf("CloneURL = %v, want %v", source.CloneURL, tt.wantCloneURL)
			}
			if source.Subdir != tt.wantSubdir {
				t.Errorf("Subdir = %v, want %v", source.Subdir, tt.wantSubdir)
			}
			if source.Name != tt.wantName {
				t.Errorf("Name = %v, want %v", source.Name, tt.wantName)
			}
		})
	}
}

func TestParseSource_GitHTTPS(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantCloneURL string
		wantSubdir   string
		wantName     string
	}{
		{
			name:         "gitlab https",
			input:        "https://gitlab.com/user/repo",
			wantCloneURL: "https://gitlab.com/user/repo.git",
			wantName:     "repo",
		},
		{
			name:         "bitbucket https",
			input:        "https://bitbucket.org/user/repo.git",
			wantCloneURL: "https://bitbucket.org/user/repo.git",
			wantName:     "repo",
		},
		{
			name:         "gitlab https dot subdir normalized to root",
			input:        "https://gitlab.com/user/repo/.",
			wantCloneURL: "https://gitlab.com/user/repo.git",
			wantName:     "repo",
		},
		{
			name:         "bitbucket web URL with src/main",
			input:        "https://bitbucket.org/team/skills/src/main/learn-and-update",
			wantCloneURL: "https://bitbucket.org/team/skills.git",
			wantSubdir:   "learn-and-update",
			wantName:     "learn-and-update",
		},
		{
			name:         "bitbucket web URL with src/main trailing slash",
			input:        "https://bitbucket.org/team/skills/src/main/learn-and-update/",
			wantCloneURL: "https://bitbucket.org/team/skills.git",
			wantSubdir:   "learn-and-update",
			wantName:     "learn-and-update",
		},
		{
			name:         "bitbucket web URL src/branch only (no subdir)",
			input:        "https://bitbucket.org/team/skills/src/main",
			wantCloneURL: "https://bitbucket.org/team/skills.git",
			wantName:     "skills",
		},
		{
			name:         "bitbucket web URL nested subdir",
			input:        "https://bitbucket.org/team/skills/src/develop/frontend/react",
			wantCloneURL: "https://bitbucket.org/team/skills.git",
			wantSubdir:   "frontend/react",
			wantName:     "react",
		},
		{
			name:         "gitlab web URL with -/tree/main",
			input:        "https://gitlab.com/user/repo/-/tree/main/path/to/skill",
			wantCloneURL: "https://gitlab.com/user/repo.git",
			wantSubdir:   "path/to/skill",
			wantName:     "skill",
		},
		{
			name:         "gitlab web URL with -/blob/main",
			input:        "https://gitlab.com/user/repo/-/blob/main/path/to/skill",
			wantCloneURL: "https://gitlab.com/user/repo.git",
			wantSubdir:   "path/to/skill",
			wantName:     "skill",
		},
		{
			name:         "gitlab web URL -/tree/branch only",
			input:        "https://gitlab.com/user/repo/-/tree/main",
			wantCloneURL: "https://gitlab.com/user/repo.git",
			wantName:     "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource() error = %v", err)
			}
			if source.Type != SourceTypeGitHTTPS {
				t.Errorf("Type = %v, want %v", source.Type, SourceTypeGitHTTPS)
			}
			if source.CloneURL != tt.wantCloneURL {
				t.Errorf("CloneURL = %v, want %v", source.CloneURL, tt.wantCloneURL)
			}
			if source.Subdir != tt.wantSubdir {
				t.Errorf("Subdir = %v, want %v", source.Subdir, tt.wantSubdir)
			}
			if source.Name != tt.wantName {
				t.Errorf("Name = %v, want %v", source.Name, tt.wantName)
			}
		})
	}
}

func TestParseSource_FileURL(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantCloneURL string
		wantName     string
	}{
		{
			name:         "file url",
			input:        "file:///path/to/repo",
			wantCloneURL: "file:///path/to/repo",
			wantName:     "repo",
		},
		{
			name:         "file url with dot suffix normalized to root",
			input:        "file:///path/to/repo/.",
			wantCloneURL: "file:///path/to/repo",
			wantName:     "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource() error = %v", err)
			}
			if source.Type != SourceTypeGitHTTPS {
				t.Errorf("Type = %v, want %v", source.Type, SourceTypeGitHTTPS)
			}
			if source.CloneURL != tt.wantCloneURL {
				t.Errorf("CloneURL = %v, want %v", source.CloneURL, tt.wantCloneURL)
			}
			if source.Name != tt.wantName {
				t.Errorf("Name = %v, want %v", source.Name, tt.wantName)
			}
		})
	}
}

func TestParseSource_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "whitespace only",
			input: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSource(tt.input)
			if err == nil {
				t.Error("ParseSource() should return error")
			}
		})
	}
}

func TestSource_HasSubdir(t *testing.T) {
	source := &Source{Subdir: "path/to/skill"}
	if !source.HasSubdir() {
		t.Error("HasSubdir() should return true")
	}

	source = &Source{Subdir: ""}
	if source.HasSubdir() {
		t.Error("HasSubdir() should return false")
	}
}

func TestSource_IsGit(t *testing.T) {
	tests := []struct {
		sourceType SourceType
		wantIsGit  bool
	}{
		{SourceTypeGitHub, true},
		{SourceTypeGitHTTPS, true},
		{SourceTypeGitSSH, true},
		{SourceTypeLocalPath, false},
		{SourceTypeUnknown, false},
	}

	for _, tt := range tests {
		source := &Source{Type: tt.sourceType}
		if source.IsGit() != tt.wantIsGit {
			t.Errorf("IsGit() for %v = %v, want %v", tt.sourceType, source.IsGit(), tt.wantIsGit)
		}
	}
}

func TestSource_MetaType(t *testing.T) {
	tests := []struct {
		source   *Source
		wantType string
	}{
		{
			source:   &Source{Type: SourceTypeGitHub},
			wantType: "github",
		},
		{
			source:   &Source{Type: SourceTypeGitHub, Subdir: "path"},
			wantType: "github-subdir",
		},
		{
			source:   &Source{Type: SourceTypeLocalPath},
			wantType: "local",
		},
	}

	for _, tt := range tests {
		if tt.source.MetaType() != tt.wantType {
			t.Errorf("MetaType() = %v, want %v", tt.source.MetaType(), tt.wantType)
		}
	}
}

func TestSource_TrackName(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "github shorthand",
			raw:  "openai/skills",
			want: "openai-skills",
		},
		{
			name: "github HTTPS URL",
			raw:  "https://github.com/anthropics/skills",
			want: "anthropics-skills",
		},
		{
			name: "github SSH URL",
			raw:  "git@github.com:openai/skills.git",
			want: "openai-skills",
		},
		{
			name: "gitlab HTTPS URL",
			raw:  "https://gitlab.com/team/my-repo.git",
			want: "team-my-repo",
		},
		{
			name: "github shorthand with subdir",
			raw:  "openai/skills/skills/pdf",
			want: "openai-skills",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ParseSource(tt.raw)
			if err != nil {
				t.Fatalf("ParseSource(%q) error: %v", tt.raw, err)
			}
			got := source.TrackName()
			if got != tt.want {
				t.Errorf("TrackName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripGitBranchPrefix(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		subdir string
		want   string
	}{
		{"empty", "bitbucket.org", "", ""},
		{"bitbucket src/main/path", "bitbucket.org", "src/main/learn-and-update", "learn-and-update"},
		{"bitbucket src/main/nested", "bitbucket.org", "src/develop/a/b/c", "a/b/c"},
		{"bitbucket src/branch only", "bitbucket.org", "src/main", ""},
		{"bitbucket trailing slash", "bitbucket.org", "src/main/skill/", "skill"},
		{"gitlab -/tree/main/path", "gitlab.com", "-/tree/main/path/to/skill", "path/to/skill"},
		{"gitlab -/blob/main/path", "gitlab.com", "-/blob/main/path/to/skill", "path/to/skill"},
		{"gitlab -/tree/branch only", "gitlab.com", "-/tree/main", ""},
		{"non-platform passthrough", "example.com", "some/path", "some/path"},
		{"bitbucket host variant", "bitbucket.mycompany.com", "src/main/skill", "skill"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripGitBranchPrefix(tt.host, tt.subdir)
			if got != tt.want {
				t.Errorf("stripGitBranchPrefix(%q, %q) = %q, want %q", tt.host, tt.subdir, got, tt.want)
			}
		})
	}
}

func TestParseSource_DomainShorthand(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantType     SourceType
		wantCloneURL string
		wantSubdir   string
		wantName     string
	}{
		{
			name:         "gitlab shorthand",
			input:        "gitlab.com/user/repo",
			wantType:     SourceTypeGitHTTPS,
			wantCloneURL: "https://gitlab.com/user/repo.git",
			wantName:     "repo",
		},
		{
			name:         "bitbucket shorthand",
			input:        "bitbucket.org/user/repo",
			wantType:     SourceTypeGitHTTPS,
			wantCloneURL: "https://bitbucket.org/user/repo.git",
			wantName:     "repo",
		},
		{
			name:         "gitlab with subdir",
			input:        "gitlab.com/user/repo/path/to/skill",
			wantType:     SourceTypeGitHTTPS,
			wantCloneURL: "https://gitlab.com/user/repo.git",
			wantSubdir:   "path/to/skill",
			wantName:     "skill",
		},
		{
			name:         "custom domain",
			input:        "git.company.com/team/skills",
			wantType:     SourceTypeGitHTTPS,
			wantCloneURL: "https://git.company.com/team/skills.git",
			wantName:     "skills",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource(%q) error = %v", tt.input, err)
			}
			if source.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", source.Type, tt.wantType)
			}
			if source.CloneURL != tt.wantCloneURL {
				t.Errorf("CloneURL = %q, want %q", source.CloneURL, tt.wantCloneURL)
			}
			if source.Subdir != tt.wantSubdir {
				t.Errorf("Subdir = %q, want %q", source.Subdir, tt.wantSubdir)
			}
			if source.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", source.Name, tt.wantName)
			}
		})
	}
}

func TestParseSource_GitHubEnterprise(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantType     SourceType
		wantCloneURL string
		wantSubdir   string
		wantName     string
	}{
		// GitHub Enterprise Server: github.COMPANY.com
		{
			name:         "GHE Server shorthand",
			input:        "github.mycompany.com/org/repo",
			wantType:     SourceTypeGitHTTPS,
			wantCloneURL: "https://github.mycompany.com/org/repo.git",
			wantName:     "repo",
		},
		{
			name:         "GHE Server full URL",
			input:        "https://github.mycompany.com/org/repo",
			wantType:     SourceTypeGitHTTPS,
			wantCloneURL: "https://github.mycompany.com/org/repo.git",
			wantName:     "repo",
		},
		{
			name:         "GHE Server with .git suffix",
			input:        "https://github.mycompany.com/org/repo.git",
			wantType:     SourceTypeGitHTTPS,
			wantCloneURL: "https://github.mycompany.com/org/repo.git",
			wantName:     "repo",
		},
		{
			name:         "GHE Server with subdir",
			input:        "https://github.mycompany.com/org/repo/skills/my-skill",
			wantType:     SourceTypeGitHTTPS,
			wantCloneURL: "https://github.mycompany.com/org/repo.git",
			wantSubdir:   "skills/my-skill",
			wantName:     "my-skill",
		},
		{
			name:         "GHE Server different company",
			input:        "github.acme.com/team/project",
			wantType:     SourceTypeGitHTTPS,
			wantCloneURL: "https://github.acme.com/team/project.git",
			wantName:     "project",
		},
		// GitHub Enterprise Cloud: COMPANY.github.com
		{
			name:         "GHE Cloud shorthand",
			input:        "mycompany.github.com/org/repo",
			wantType:     SourceTypeGitHTTPS,
			wantCloneURL: "https://mycompany.github.com/org/repo.git",
			wantName:     "repo",
		},
		{
			name:         "GHE Cloud full URL",
			input:        "https://mycompany.github.com/org/repo",
			wantType:     SourceTypeGitHTTPS,
			wantCloneURL: "https://mycompany.github.com/org/repo.git",
			wantName:     "repo",
		},
		{
			name:         "GHE Cloud with subdir",
			input:        "https://enterprise.github.com/team/skills/frontend/react",
			wantType:     SourceTypeGitHTTPS,
			wantCloneURL: "https://enterprise.github.com/team/skills.git",
			wantSubdir:   "frontend/react",
			wantName:     "react",
		},
		// SSH format
		{
			name:         "GHE Server SSH",
			input:        "git@github.mycompany.com:org/repo.git",
			wantType:     SourceTypeGitSSH,
			wantCloneURL: "git@github.mycompany.com:org/repo.git",
			wantName:     "repo",
		},
		{
			name:         "GHE Cloud SSH",
			input:        "git@mycompany.github.com:team/skills.git",
			wantType:     SourceTypeGitSSH,
			wantCloneURL: "git@mycompany.github.com:team/skills.git",
			wantName:     "skills",
		},
		{
			name:         "GHE SSH with subdir",
			input:        "git@github.mycompany.com:org/repo.git//path/to/skill",
			wantType:     SourceTypeGitSSH,
			wantCloneURL: "git@github.mycompany.com:org/repo.git",
			wantSubdir:   "path/to/skill",
			wantName:     "skill",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource(%q) error = %v", tt.input, err)
			}
			if source.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", source.Type, tt.wantType)
			}
			if source.CloneURL != tt.wantCloneURL {
				t.Errorf("CloneURL = %q, want %q", source.CloneURL, tt.wantCloneURL)
			}
			if source.Subdir != tt.wantSubdir {
				t.Errorf("Subdir = %q, want %q", source.Subdir, tt.wantSubdir)
			}
			if source.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", source.Name, tt.wantName)
			}
		})
	}
}

func TestParseSource_GitHubEnterprise_TrackName(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "GHE Server HTTPS",
			raw:  "https://github.mycompany.com/team/skills",
			want: "team-skills",
		},
		{
			name: "GHE Cloud HTTPS",
			raw:  "https://enterprise.github.com/org/repo",
			want: "org-repo",
		},
		{
			name: "GHE Server SSH",
			raw:  "git@github.mycompany.com:org/skills.git",
			want: "org-skills",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ParseSource(tt.raw)
			if err != nil {
				t.Fatalf("ParseSource(%q) error: %v", tt.raw, err)
			}
			got := source.TrackName()
			if got != tt.want {
				t.Errorf("TrackName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSource_GeminiCLIMonorepo(t *testing.T) {
	// Real-world test case from the plan
	input := "github.com/google-gemini/gemini-cli/packages/core/src/skills/builtin/skill-creator"

	source, err := ParseSource(input)
	if err != nil {
		t.Fatalf("ParseSource() error = %v", err)
	}

	if source.Type != SourceTypeGitHub {
		t.Errorf("Type = %v, want %v", source.Type, SourceTypeGitHub)
	}
	if source.CloneURL != "https://github.com/google-gemini/gemini-cli.git" {
		t.Errorf("CloneURL = %v, want https://github.com/google-gemini/gemini-cli.git", source.CloneURL)
	}
	if source.Subdir != "packages/core/src/skills/builtin/skill-creator" {
		t.Errorf("Subdir = %v, want packages/core/src/skills/builtin/skill-creator", source.Subdir)
	}
	if source.Name != "skill-creator" {
		t.Errorf("Name = %v, want skill-creator", source.Name)
	}
}

func TestExpandGitHubShorthand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "owner/repo shorthand",
			input: "anthropics/skills",
			want:  "github.com/anthropics/skills",
		},
		{
			name:  "owner/repo/path shorthand",
			input: "anthropics/skills/skills/pdf",
			want:  "github.com/anthropics/skills/skills/pdf",
		},
		{
			name:  "already has github.com prefix",
			input: "github.com/user/repo",
			want:  "github.com/user/repo",
		},
		{
			name:  "https URL unchanged",
			input: "https://github.com/user/repo",
			want:  "https://github.com/user/repo",
		},
		{
			name:  "http URL unchanged",
			input: "http://example.com/user/repo",
			want:  "http://example.com/user/repo",
		},
		{
			name:  "git SSH unchanged",
			input: "git@github.com:user/repo.git",
			want:  "git@github.com:user/repo.git",
		},
		{
			name:  "file URL unchanged",
			input: "file:///path/to/repo",
			want:  "file:///path/to/repo",
		},
		{
			name:  "absolute path unchanged",
			input: "/path/to/skill",
			want:  "/path/to/skill",
		},
		{
			name:  "tilde path unchanged",
			input: "~/skills/my-skill",
			want:  "~/skills/my-skill",
		},
		{
			name:  "relative path unchanged",
			input: "./local-skill",
			want:  "./local-skill",
		},
		{
			name:  "parent path unchanged",
			input: "../other-skill",
			want:  "../other-skill",
		},
		{
			name:  "single word unchanged (no slash)",
			input: "somename",
			want:  "somename",
		},
		{
			name:  "gitlab domain gets https prefix",
			input: "gitlab.com/user/repo",
			want:  "https://gitlab.com/user/repo",
		},
		{
			name:  "bitbucket domain gets https prefix",
			input: "bitbucket.org/user/repo",
			want:  "https://bitbucket.org/user/repo",
		},
		{
			name:  "custom domain gets https prefix",
			input: "git.company.com/team/skills",
			want:  "https://git.company.com/team/skills",
		},
		{
			name:  "github shorthand still works",
			input: "anthropics/skills",
			want:  "github.com/anthropics/skills",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandGitHubShorthand(tt.input)
			if got != tt.want {
				t.Errorf("expandGitHubShorthand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseSource_GitHubShorthandExpansion(t *testing.T) {
	// Test that shorthand is properly expanded and parsed
	tests := []struct {
		name         string
		input        string
		wantCloneURL string
		wantSubdir   string
		wantName     string
	}{
		{
			name:         "owner/repo shorthand",
			input:        "anthropics/skills",
			wantCloneURL: "https://github.com/anthropics/skills.git",
			wantSubdir:   "",
			wantName:     "skills",
		},
		{
			name:         "owner/repo/subdir shorthand",
			input:        "anthropics/skills/skills/pdf",
			wantCloneURL: "https://github.com/anthropics/skills.git",
			wantSubdir:   "skills/pdf",
			wantName:     "pdf",
		},
		{
			name:         "ComposioHQ example",
			input:        "ComposioHQ/awesome-claude-skills",
			wantCloneURL: "https://github.com/ComposioHQ/awesome-claude-skills.git",
			wantSubdir:   "",
			wantName:     "awesome-claude-skills",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource(%q) error = %v", tt.input, err)
			}
			if source.Type != SourceTypeGitHub {
				t.Errorf("Type = %v, want %v", source.Type, SourceTypeGitHub)
			}
			if source.CloneURL != tt.wantCloneURL {
				t.Errorf("CloneURL = %q, want %q", source.CloneURL, tt.wantCloneURL)
			}
			if source.Subdir != tt.wantSubdir {
				t.Errorf("Subdir = %q, want %q", source.Subdir, tt.wantSubdir)
			}
			if source.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", source.Name, tt.wantName)
			}
		})
	}
}
