package version

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"skillshare/internal/config"
)

func TestReadLocalSkillVersion(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "reads metadata.version",
			content: `---
name: skillshare
description: test
metadata:
  version: v0.17.11
---
# Skillshare Skill`,
			expected: "0.17.11",
		},
		{
			name: "returns empty when no version field",
			content: `---
name: skillshare
---
# Skillshare Skill`,
			expected: "",
		},
		{
			name:     "returns empty for empty file",
			content:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create source/skillshare/SKILL.md structure
			sourceDir := t.TempDir()
			skillDir := filepath.Join(sourceDir, "skillshare")
			if err := os.MkdirAll(skillDir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got := ReadLocalSkillVersion(sourceDir)
			if got != tt.expected {
				t.Errorf("ReadLocalSkillVersion() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestReadLocalSkillVersion_NoSkillDir(t *testing.T) {
	sourceDir := t.TempDir()
	got := ReadLocalSkillVersion(sourceDir)
	if got != "" {
		t.Errorf("expected empty string for missing skillshare dir, got %q", got)
	}
}

func TestReadLocalSkillVersion_MovedIntoFolder(t *testing.T) {
	sourceDir := t.TempDir()
	writeSkill(t, sourceDir, "tools/skillshare", "v0.31.0")

	if got := ReadLocalSkillVersion(sourceDir); got != "0.31.0" {
		t.Errorf("ReadLocalSkillVersion() = %q, want the version of the skill moved into tools/", got)
	}
}

// With follow_source_links on, a skill moved into a linked folder is found as the dashboard lists it.
func TestFindLocalSkill_FollowedSourceLink(t *testing.T) {
	sourceDir := t.TempDir()
	linked := t.TempDir()
	writeSkill(t, linked, "skillshare", "v1.0.0")
	if err := os.Symlink(linked, filepath.Join(sourceDir, "tools")); err != nil {
		t.Fatal(err)
	}

	skill, ok := FindLocalSkill(sourceDir, config.SkillsWalk(true, sourceDir, nil))
	if !ok || skill.RelPath != "tools/skillshare" {
		t.Fatalf("FindLocalSkill() = %+v, %v; want tools/skillshare", skill, ok)
	}
}

// writeSkill writes a SKILL.md at rel under sourceDir, with version in its metadata when set.
func writeSkill(t *testing.T, sourceDir, rel, version string) {
	t.Helper()
	dir := filepath.Join(sourceDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + filepath.Base(dir) + "\n"
	if version != "" {
		content += "metadata:\n  version: " + version + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content+"---\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestFindLocalSkill(t *testing.T) {
	tests := []struct {
		name         string
		skills       []string // SKILL.md folders, each with version v1.0.0
		trackedRepo  string   // folder given a .git, making it a tracked repo
		ignore       string   // .skillignore content
		wantRel      string   // "" when no skill should be found
		wantDisabled bool
	}{
		{name: "source root", skills: []string{"skillshare"}, wantRel: "skillshare"},
		{name: "moved into a folder", skills: []string{"tools/skillshare"}, wantRel: "tools/skillshare"},
		{name: "two folders deep", skills: []string{"a/b/skillshare"}, wantRel: "a/b/skillshare"},
		{name: "only in a tracked repo", skills: []string{"_team/skillshare"}, trackedRepo: "_team"},
		{name: "root wins over a folder", skills: []string{"skillshare", "tools/skillshare"}, wantRel: "skillshare"},
		{name: "first folder wins", skills: []string{"b/skillshare", "a/skillshare"}, wantRel: "a/skillshare"},
		{name: "disabled at the root", skills: []string{"skillshare"}, ignore: "skillshare\n", wantRel: "skillshare", wantDisabled: true},
		{name: "disabled through its folder", skills: []string{"tools/skillshare"}, ignore: "tools/\n", wantRel: "tools/skillshare", wantDisabled: true},
		{name: "other skill disabled", skills: []string{"skillshare"}, ignore: "other-skill\n", wantRel: "skillshare"},
		{name: "not installed", skills: []string{"other-skill"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceDir := t.TempDir()
			for _, rel := range tt.skills {
				writeSkill(t, sourceDir, rel, "v1.0.0")
			}
			if tt.trackedRepo != "" {
				if err := os.MkdirAll(filepath.Join(sourceDir, tt.trackedRepo, ".git"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if tt.ignore != "" {
				if err := os.WriteFile(filepath.Join(sourceDir, ".skillignore"), []byte(tt.ignore), 0644); err != nil {
					t.Fatal(err)
				}
			}

			skill, ok := FindLocalSkill(sourceDir)
			if ok != (tt.wantRel != "") || skill.RelPath != tt.wantRel || skill.Disabled != tt.wantDisabled {
				t.Fatalf("FindLocalSkill() = %+v, %v; want rel %q, disabled %v", skill, ok, tt.wantRel, tt.wantDisabled)
			}
			if ok && skill.Version != "1.0.0" {
				t.Errorf("Version = %q, want 1.0.0", skill.Version)
			}
		})
	}
}

func TestSkillOutdated(t *testing.T) {
	tests := []struct {
		local, remote string
		want          bool
	}{
		{"0.23.4", "0.23.5", true},
		{"0.23.5", "0.23.5", false},
		{"0.24.0", "0.23.5", false}, // a local skill ahead of main is not outdated
		{"0.23.5", "", false},       // offline
		{"", "0.23.5", false},
		{"custom", "0.23.5", false},
	}
	for _, tt := range tests {
		if got := SkillOutdated(tt.local, tt.remote); got != tt.want {
			t.Errorf("SkillOutdated(%q, %q) = %v, want %v", tt.local, tt.remote, got, tt.want)
		}
	}
}

func TestCachedRemoteSkillVersion_FreshCacheSkipsFetch(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	// A version GitHub will never serve, so a fetch would be visible.
	SaveRemoteSkillVersion("9.9.9")

	if got := CachedRemoteSkillVersion(); got != "9.9.9" {
		t.Fatalf("CachedRemoteSkillVersion() = %q, want the cached 9.9.9", got)
	}
}

func TestCachedRemoteSkillVersion_RefetchesWhenCLIReleaseIsNewer(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	SaveRemoteSkillVersion("1.0.0")
	_ = saveCache(&Cache{LastChecked: time.Now(), LatestVersion: "9.9.9"})

	if got := CachedRemoteSkillVersion(); got == "1.0.0" {
		t.Fatal("CachedRemoteSkillVersion() kept a skill cache older than the cached CLI release")
	}
}
