package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractRepoOwner(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/acme-corp/skills.git", "acme-corp"},
		{"https://gitlab.com/Team-X/monorepo.git", "team-x"},
		{"git@github.com:MyOrg/repo.git", "myorg"},
		{"git@gitlab.com:CompanyY/skills.git", "companyy"},
		{"file:///path/to/repo", ""},
		{"", ""},
		{"https://dev.azure.com/org/proj/_git/repo", "org"},
	}
	for _, tt := range tests {
		got := extractRepoOwner(tt.url)
		if got != tt.want {
			t.Errorf("extractRepoOwner(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestParseFrontmatterNameDesc(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantName string
		wantDesc string
	}{
		{
			name:     "standard frontmatter",
			content:  "---\nname: my-skill\ndescription: A useful tool\n---\n# Content",
			wantName: "my-skill",
			wantDesc: "A useful tool",
		},
		{
			name:     "quoted values",
			content:  "---\nname: \"my-skill\"\ndescription: 'Official tool from Acme Corp'\n---\n",
			wantName: "my-skill",
			wantDesc: "Official tool from Acme Corp",
		},
		{
			name:     "no frontmatter",
			content:  "# Just content",
			wantName: "",
			wantDesc: "",
		},
		{
			name:     "empty description",
			content:  "---\nname: test\n---\n",
			wantName: "test",
			wantDesc: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, desc := parseFrontmatterNameDesc([]byte(tt.content))
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if desc != tt.wantDesc {
				t.Errorf("description = %q, want %q", desc, tt.wantDesc)
			}
		})
	}
}

func TestCheckPublisherMismatch(t *testing.T) {
	tests := []struct {
		name    string
		skillN  string
		desc    string
		repoURL string
		wantNil bool
		wantSev string
	}{
		{
			name:    "mismatch: claims Acme but from random-user",
			skillN:  "formatter",
			desc:    "Official formatter from Acme Corp",
			repoURL: "https://github.com/random-user/skills.git",
			wantNil: false,
			wantSev: SeverityHigh,
		},
		{
			name:    "match: claims acme and from acme-corp",
			skillN:  "formatter",
			desc:    "formatter from acme-corp",
			repoURL: "https://github.com/acme-corp/skills.git",
			wantNil: true,
		},
		{
			name:    "match: @mention matches owner",
			skillN:  "tool",
			desc:    "A tool by @myorg",
			repoURL: "https://github.com/myorg/skills.git",
			wantNil: true,
		},
		{
			name:    "mismatch: @mention doesn't match",
			skillN:  "tool",
			desc:    "A tool by @bigcorp",
			repoURL: "https://github.com/evil-fork/skills.git",
			wantNil: false,
			wantSev: SeverityHigh,
		},
		{
			name:    "no claim in description",
			skillN:  "my-tool",
			desc:    "A great tool for coding",
			repoURL: "https://github.com/someone/skills.git",
			wantNil: true,
		},
		{
			name:    "empty repo URL",
			skillN:  "tool",
			desc:    "from Acme Corp",
			repoURL: "",
			wantNil: true,
		},
		{
			name:    "SSH URL mismatch",
			skillN:  "tool",
			desc:    "made by Google",
			repoURL: "git@github.com:evil-fork/repo.git",
			wantNil: false,
			wantSev: SeverityHigh,
		},
		{
			name:    "owner is substring of claim",
			skillN:  "tool",
			desc:    "maintained by Anthropics team",
			repoURL: "https://github.com/anthropics/skills.git",
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := checkPublisherMismatch(tt.skillN, tt.desc, tt.repoURL)
			if tt.wantNil && f != nil {
				t.Errorf("expected nil, got finding: %s", f.Message)
			}
			if !tt.wantNil {
				if f == nil {
					t.Fatal("expected finding, got nil")
				}
				if f.Severity != tt.wantSev {
					t.Errorf("severity = %q, want %q", f.Severity, tt.wantSev)
				}
				if f.RuleID != rulePublisherMismatch {
					t.Errorf("ruleID = %q, want %q", f.RuleID, rulePublisherMismatch)
				}
			}
		})
	}
}

func TestCheckAuthorityLanguage(t *testing.T) {
	tests := []struct {
		name    string
		desc    string
		repoURL string
		wantNil bool
	}{
		{
			name:    "official from unknown source",
			desc:    "Official database connector",
			repoURL: "https://github.com/new-account-2025/skills.git",
			wantNil: false,
		},
		{
			name:    "official from well-known org",
			desc:    "Official Claude Code skill",
			repoURL: "https://github.com/anthropics/skills.git",
			wantNil: true,
		},
		{
			name:    "no authority words",
			desc:    "A simple utility for formatting",
			repoURL: "https://github.com/someone/skills.git",
			wantNil: true,
		},
		{
			name:    "authority but local source",
			desc:    "Official trusted tool",
			repoURL: "",
			wantNil: true,
		},
		{
			name:    "verified from unknown",
			desc:    "Verified and trusted security scanner",
			repoURL: "https://github.com/shady-org/skills.git",
			wantNil: false,
		},
		{
			name:    "empty description",
			desc:    "",
			repoURL: "https://github.com/someone/skills.git",
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := checkAuthorityLanguage(tt.desc, tt.repoURL)
			if tt.wantNil && f != nil {
				t.Errorf("expected nil, got finding: %s", f.Message)
			}
			if !tt.wantNil && f == nil {
				t.Error("expected finding, got nil")
			}
			if !tt.wantNil && f != nil {
				if f.Severity != SeverityMedium {
					t.Errorf("severity = %q, want MEDIUM", f.Severity)
				}
			}
		})
	}
}

func TestIsWellKnownOrg(t *testing.T) {
	tests := []struct {
		owner string
		want  bool
	}{
		{"anthropics", true},
		{"Anthropics", true},
		{"google", true},
		{"random-user", false},
		{"", false},
		{"google-gemini", true},
	}
	for _, tt := range tests {
		got := isWellKnownOrg(tt.owner)
		if got != tt.want {
			t.Errorf("isWellKnownOrg(%q) = %v, want %v", tt.owner, got, tt.want)
		}
	}
}

func TestMetadataAnalyzer_Integration(t *testing.T) {
	// Create a temp skill directory with SKILL.md and .skillshare-meta.json
	dir := t.TempDir()

	// Write SKILL.md claiming "from Acme Corp"
	skillContent := "---\nname: evil-skill\ndescription: Official formatter from Acme Corp\n---\n# Evil\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Write meta pointing to a different org
	meta := metaJSON{RepoURL: "https://github.com/evil-fork/skills.git"}
	metaData, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(dir, ".skillshare-meta.json"), metaData, 0644); err != nil {
		t.Fatal(err)
	}

	a := &metadataAnalyzer{}
	ctx := &AnalyzeContext{
		SkillPath:   dir,
		DisabledIDs: map[string]bool{},
	}
	findings, err := a.Analyze(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Should find both publisher mismatch (HIGH) and authority language (MEDIUM)
	if len(findings) < 2 {
		t.Errorf("expected >= 2 findings, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  %s: %s", f.Severity, f.Message)
		}
		return
	}

	var hasMismatch, hasAuthority bool
	for _, f := range findings {
		if f.RuleID == rulePublisherMismatch {
			hasMismatch = true
		}
		if f.RuleID == ruleAuthorityLanguage {
			hasAuthority = true
		}
	}
	if !hasMismatch {
		t.Error("missing publisher-mismatch finding")
	}
	if !hasAuthority {
		t.Error("missing authority-language finding")
	}
}

func TestMetadataAnalyzer_NoMeta(t *testing.T) {
	// Skill without .skillshare-meta.json — should produce no findings
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: test\n---\n# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	a := &metadataAnalyzer{}
	ctx := &AnalyzeContext{
		SkillPath:   dir,
		DisabledIDs: map[string]bool{},
	}
	findings, err := a.Analyze(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for skill without meta, got %d", len(findings))
	}
}

func TestMetadataAnalyzer_DisabledRules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"),
		[]byte("---\nname: test\ndescription: Official tool from Acme Corp\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	meta := metaJSON{RepoURL: "https://github.com/evil-fork/skills.git"}
	metaData, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(dir, ".skillshare-meta.json"), metaData, 0644); err != nil {
		t.Fatal(err)
	}

	a := &metadataAnalyzer{}
	ctx := &AnalyzeContext{
		SkillPath: dir,
		DisabledIDs: map[string]bool{
			rulePublisherMismatch: true,
			ruleAuthorityLanguage: true,
		},
	}
	findings, err := a.Analyze(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when rules disabled, got %d", len(findings))
	}
}
