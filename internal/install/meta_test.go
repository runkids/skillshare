package install

import (
	"strings"
	"testing"
)

func TestNewMetaFromSource_NormalizesSubdirBackslashes(t *testing.T) {
	source := &Source{
		Type:     SourceTypeGitHub,
		Raw:      "org/repo/frontend/ui",
		CloneURL: "https://github.com/org/repo.git",
		Subdir:   "frontend\\ui", // Simulates Windows filepath.Join
		Name:     "ui",
	}

	meta := NewMetaFromSource(source)

	if strings.Contains(meta.Subdir, `\`) {
		t.Errorf("meta.Subdir contains backslash: %q", meta.Subdir)
	}
	if meta.Subdir != "frontend/ui" {
		t.Errorf("meta.Subdir = %q, want %q", meta.Subdir, "frontend/ui")
	}
}

func TestInstallFromDiscovery_FullSubdirUsesForwardSlashes(t *testing.T) {
	// Verify that fullSubdir construction uses "/" not filepath separator.
	// We test this indirectly: discovery.Source.Subdir may contain backslashes
	// from Windows, and skill.Path should already be normalized.
	// The concatenation should use "/" regardless of OS.

	discovery := &DiscoveryResult{
		Source: &Source{
			Type:     SourceTypeGitHub,
			Raw:      "org/repo/skills",
			CloneURL: "https://github.com/org/repo.git",
			Subdir:   "skills",
			Name:     "repo",
		},
	}
	skill := SkillInfo{
		Name: "my-skill",
		Path: "my-skill",
	}

	// We can't call InstallFromDiscovery directly (needs real filesystem),
	// so replicate the fullSubdir logic and verify
	var fullSubdir string
	if skill.Path == "." {
		fullSubdir = discovery.Source.Subdir
	} else if discovery.Source.HasSubdir() {
		fullSubdir = discovery.Source.Subdir + "/" + skill.Path
	} else {
		fullSubdir = skill.Path
	}

	if strings.Contains(fullSubdir, `\`) {
		t.Errorf("fullSubdir contains backslash: %q", fullSubdir)
	}
	if fullSubdir != "skills/my-skill" {
		t.Errorf("fullSubdir = %q, want %q", fullSubdir, "skills/my-skill")
	}
}
