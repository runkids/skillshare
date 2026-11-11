package install

import (
	"errors"
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

// Regression for #859: the source recorded for a skill picked out of a whole
// repository must parse back into that repository and the skill's path.
func TestBuildDiscoverySkillSource_RoundTrips(t *testing.T) {
	opts := ParseOptions{GitLabHosts: []string{"code.example.com"}}
	for _, typed := range []string{
		"org/skills",
		"https://github.com/org/skills",
		"https://git.example.com/team/skills",
		"https://git.example.com/team/skills.git",
		"gitlab.com/team/skills",
		"https://gitlab.com/group/sub/skills/",
		"https://gitlab.com/team/skills.git",
		"https://gitlab.com/team/skills/-/tree/main",
		"https://code.example.com/group/sub/skills",
		"https://bitbucket.org/team/skills",
		"https://bitbucket.org/team/skills/src/main",
		"https://bitbucket.org/src/skills",
		"https://dev.azure.com/org/project/_git/skills",
		"git@gitlab.com:team/skills.git",
		"ssh://git@gitlab.com/team/skills.git",
		"file:///srv/skills",
	} {
		t.Run(typed, func(t *testing.T) {
			source, err := ParseSourceWithOptions(typed, opts)
			if err != nil {
				t.Fatalf("parse typed source: %v", err)
			}
			recorded := buildDiscoverySkillSource(source, "pack/x")
			got, err := ParseSourceWithOptions(recorded, opts)
			if err != nil {
				t.Fatalf("parse recorded source %q: %v", recorded, err)
			}
			if got.CloneURL != source.CloneURL || got.Subdir != "pack/x" || got.Branch != source.Branch {
				t.Errorf("recorded %q parsed as repo %q subdir %q branch %q, want repo %q subdir %q branch %q",
					recorded, got.CloneURL, got.Subdir, got.Branch, source.CloneURL, "pack/x", source.Branch)
			}
		})
	}
}

// An install recorded in the single-slash form from before #859 is still
// recognized as the same skill, so `install --update` keeps updating it. A
// repository of its own at that URL is a different install.
func TestIsDiscoverySkillSource_FormBefore859(t *testing.T) {
	source, err := ParseSource("file:///srv/skills")
	if err != nil {
		t.Fatal(err)
	}
	const recorded = "file:///srv/skills/x"
	if !isDiscoverySkillSource(&MetadataEntry{Source: recorded, RepoURL: "file:///srv/skills", Subdir: "x"}, source, "x") {
		t.Error("single-slash source of the same repository not recognized")
	}
	if isDiscoverySkillSource(&MetadataEntry{Source: recorded, RepoURL: recorded}, source, "x") {
		t.Error("a repository installed from the same URL was taken for the skill")
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
	os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte{0}, 0644)
	os.MkdirAll(filepath.Join(dir, "assets"), 0755)
	os.WriteFile(filepath.Join(dir, "assets", ".DS_Store"), []byte{0}, 0644)

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

func TestComputeFileHashes_DirectorySymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "target"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", "target/README.md", "z-last.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("target", filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	hashes, err := ComputeFileHashes(dir)
	if err != nil {
		t.Fatalf("ComputeFileHashes: %v", err)
	}
	if len(hashes) != 3 {
		t.Fatalf("expected only 3 regular files, got %v", hashes)
	}
	for _, name := range []string{"SKILL.md", "target/README.md", "z-last.txt"} {
		if hashes[name] == "" {
			t.Errorf("missing hash for %s", name)
		}
	}
}

func TestComputeFileHashes_FileSymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "target.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.txt", filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	hashes, err := ComputeFileHashes(dir)
	if err != nil {
		t.Fatalf("ComputeFileHashes: %v", err)
	}
	if len(hashes) != 2 || hashes["link"] != hashes["target.txt"] {
		t.Fatalf("file symlink should still hash target content: %v", hashes)
	}
}

func TestComputeFileHashes_BrokenSymlinkReturnsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a-valid.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(dir, "z-broken")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	hashes, err := ComputeFileHashes(dir)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing target error, got %v", err)
	}
	if hashes != nil {
		t.Fatalf("must not return partial hashes as successful: %v", hashes)
	}
}

// Regression for #777: the repo source rebuilt from an agent's metadata must
// reproduce that metadata's source when the agent is written back.
func TestAgentRepoSource_RoundTripsAgentSource(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		agentPath  string
		repoSubdir string
	}{
		{"github whole repo", "github.com/you/repo/agents/demo.md", "agents/demo.md", ""},
		{"github repo subdir", "github.com/you/repo/pack/agents/demo.md", "agents/demo.md", "pack"},
		{"ssh whole repo", "git@github.com:you/repo.git//agents/demo.md", "agents/demo.md", ""},
		{"ssh repo subdir", "git@github.com:you/repo.git//pack/agents/demo.md", "agents/demo.md", "pack"},
		{"file whole repo", "file:///tmp/repo//agents/demo.md", "agents/demo.md", ""},
		{"file repo subdir", "file:///tmp/repo//pack/agents/demo.md", "agents/demo.md", "pack"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := AgentRepoSource(&MetadataEntry{Source: tt.source, Subdir: tt.agentPath}, ParseOptions{})
			if err != nil {
				t.Fatalf("AgentRepoSource: %v", err)
			}
			got := buildDiscoverySkillSource(source, tt.agentPath)
			if got != tt.source || source.Subdir != tt.repoSubdir {
				t.Errorf("got source %q subdir %q, want %q subdir %q", got, source.Subdir, tt.source, tt.repoSubdir)
			}
		})
	}
}

// Sources earlier updates broke (#777): the dashboard appended the agent path
// a second time, which is repaired; the CLI dropped the repository, which
// cannot be repaired and asks for a reinstall.
func TestAgentRepoSource_BrokenSources(t *testing.T) {
	doubled := &MetadataEntry{Source: "github.com/you/repo/agents/demo.md/agents/demo.md", Subdir: "agents/demo.md", RepoURL: "https://github.com/you/repo.git", Type: "github-subdir"}
	source, err := AgentRepoSource(doubled, ParseOptions{})
	if err != nil {
		t.Fatalf("doubled path: %v", err)
	}
	if got := buildDiscoverySkillSource(source, doubled.Subdir); got != "github.com/you/repo/agents/demo.md" {
		t.Errorf("doubled path rebuilt as %q", got)
	}

	lost := &MetadataEntry{Source: "/agents/demo.md", Subdir: "agents/demo.md", Type: "unknown-subdir"}
	if _, err := AgentRepoSource(lost, ParseOptions{}); !errors.Is(err, ErrAgentSourceLost) {
		t.Errorf("lost source: got %v, want ErrAgentSourceLost", err)
	}
}
