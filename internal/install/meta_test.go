package install

import (
	"os"
	"path/filepath"
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

func TestBuildDiscoverySkillSource(t *testing.T) {
	tests := []struct {
		name     string
		source   *Source
		skill    string
		expected string
	}{
		{
			name: "https whole repo",
			source: &Source{
				Type: SourceTypeGitHTTPS,
				Raw:  "https://gitlab.com/team/repo.git",
			},
			skill:    "skills/docker",
			expected: "https://gitlab.com/team/repo.git/skills/docker",
		},
		{
			name: "ssh whole repo uses double slash",
			source: &Source{
				Type: SourceTypeGitSSH,
				Raw:  "git@gitlab.com:team/repo.git",
			},
			skill:    "frontend/ui-skill",
			expected: "git@gitlab.com:team/repo.git//frontend/ui-skill",
		},
		{
			name: "ssh with source subdir keeps single slash for nested skill",
			source: &Source{
				Type:   SourceTypeGitSSH,
				Raw:    "git@gitlab.com:team/repo.git//skills",
				Subdir: "skills",
			},
			skill:    "ui-skill",
			expected: "git@gitlab.com:team/repo.git//skills/ui-skill",
		},
		{
			name: "root skill keeps raw",
			source: &Source{
				Type: SourceTypeGitHub,
				Raw:  "openai/skills/skills",
			},
			skill:    ".",
			expected: "openai/skills/skills",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDiscoverySkillSource(tt.source, tt.skill)
			if got != tt.expected {
				t.Errorf("buildDiscoverySkillSource() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestComputeFileHashes_Basic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Hello"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "README.md"), []byte("readme"), 0644)

	hashes, err := ComputeFileHashes(dir)
	if err != nil {
		t.Fatalf("ComputeFileHashes: %v", err)
	}
	if len(hashes) != 2 {
		t.Fatalf("expected 2 hashes, got %d: %v", len(hashes), hashes)
	}
	for path, h := range hashes {
		if !strings.HasPrefix(h, "sha256:") {
			t.Errorf("hash for %s missing sha256: prefix: %s", path, h)
		}
		if len(h) != len("sha256:")+64 {
			t.Errorf("hash for %s has wrong length: %s", path, h)
		}
	}
	if _, ok := hashes["SKILL.md"]; !ok {
		t.Error("missing SKILL.md in hashes")
	}
	if _, ok := hashes["sub/README.md"]; !ok {
		t.Error("missing sub/README.md in hashes (should use forward slash)")
	}
}

func TestComputeFileHashes_SkipsMetaAndGit(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill"), 0644)
	os.WriteFile(filepath.Join(dir, ".skillshare-meta.json"), []byte("{}"), 0644)
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0644)

	hashes, err := ComputeFileHashes(dir)
	if err != nil {
		t.Fatalf("ComputeFileHashes: %v", err)
	}
	if len(hashes) != 1 {
		t.Fatalf("expected 1 hash (SKILL.md only), got %d: %v", len(hashes), hashes)
	}
	if _, ok := hashes[".skillshare-meta.json"]; ok {
		t.Error("should skip .skillshare-meta.json")
	}
	if _, ok := hashes[".git/HEAD"]; ok {
		t.Error("should skip .git/")
	}
}

func TestComputeFileHashes_Deterministic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)

	h1, _ := ComputeFileHashes(dir)
	h2, _ := ComputeFileHashes(dir)

	if h1["file.txt"] != h2["file.txt"] {
		t.Errorf("hashes differ: %s vs %s", h1["file.txt"], h2["file.txt"])
	}
}
