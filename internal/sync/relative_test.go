package sync

import (
	"testing"
)

func TestShouldUseRelative(t *testing.T) {
	tests := []struct {
		name        string
		projectRoot string
		sourcePath  string
		targetPath  string
		want        bool
	}{
		{
			name:        "both under project root",
			projectRoot: "/home/user/project",
			sourcePath:  "/home/user/project/.skillshare/skills/foo",
			targetPath:  "/home/user/project/.claude/skills",
			want:        true,
		},
		{
			name:        "source outside project root",
			projectRoot: "/home/user/project",
			sourcePath:  "/opt/shared/skills/foo",
			targetPath:  "/home/user/project/.claude/skills",
			want:        false,
		},
		{
			name:        "target outside project root",
			projectRoot: "/home/user/project",
			sourcePath:  "/home/user/project/.skillshare/skills/foo",
			targetPath:  "/opt/shared/claude/skills",
			want:        false,
		},
		{
			name:        "empty project root (global mode)",
			projectRoot: "",
			sourcePath:  "/home/user/.config/skillshare/skills/foo",
			targetPath:  "/home/user/.claude/skills",
			want:        false,
		},
		{
			name:        "both outside project root",
			projectRoot: "/home/user/project",
			sourcePath:  "/opt/a",
			targetPath:  "/opt/b",
			want:        false,
		},
		{
			name:        "project root itself as source",
			projectRoot: "/home/user/project",
			sourcePath:  "/home/user/project",
			targetPath:  "/home/user/project/.claude/skills",
			want:        true,
		},
		{
			name:        "filesystem root as project root",
			projectRoot: "/",
			sourcePath:  "/skills/foo",
			targetPath:  "/claude/skills",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseRelative(tt.projectRoot, tt.sourcePath, tt.targetPath)
			if got != tt.want {
				t.Errorf("shouldUseRelative(%q, %q, %q) = %v, want %v",
					tt.projectRoot, tt.sourcePath, tt.targetPath, got, tt.want)
			}
		})
	}
}

func TestShouldUseRelative_CleansPaths(t *testing.T) {
	got := shouldUseRelative(
		"/home/user/project/",
		"/home/user/project/.skillshare/skills/../skills/foo",
		"/home/user/project/.claude/skills/",
	)
	if !got {
		t.Error("expected true for unclean but valid paths under project root")
	}
}

func TestResolveReadlink(t *testing.T) {
	tests := []struct {
		name     string
		dest     string
		linkPath string
		want     string
	}{
		{
			"absolute dest unchanged",
			"/abs/source/skill",
			"/project/.claude/skills/skill",
			"/abs/source/skill",
		},
		{
			"relative dest resolved from link parent",
			"../../.skillshare/skills/skill",
			"/project/.claude/skills/skill",
			"/project/.skillshare/skills/skill",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveReadlink(tt.dest, tt.linkPath)
			if got != tt.want {
				t.Errorf("resolveReadlink(%q, %q) = %q, want %q", tt.dest, tt.linkPath, got, tt.want)
			}
		})
	}
}

func TestLinkNeedsReformat(t *testing.T) {
	tests := []struct {
		name         string
		dest         string
		wantRelative bool
		expected     bool
	}{
		{"absolute dest wants relative", "/abs/path/to/skill", true, true},
		{"absolute dest wants absolute", "/abs/path/to/skill", false, false},
		{"relative dest wants relative", "../../.skillshare/skills/foo", true, false},
		{"relative dest wants absolute", "../../.skillshare/skills/foo", false, true},
		{"empty dest", "", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := linkNeedsReformat(tt.dest, tt.wantRelative); got != tt.expected {
				t.Errorf("linkNeedsReformat(%q, %v) = %v, want %v", tt.dest, tt.wantRelative, got, tt.expected)
			}
		})
	}
}
