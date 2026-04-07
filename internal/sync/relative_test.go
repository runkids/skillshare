package sync

import (
	"os"
	"path/filepath"
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

func TestShouldUseRelative_SymlinkedProjectRoot(t *testing.T) {
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	realProject := filepath.Join(tmp, "real-project")
	os.MkdirAll(filepath.Join(realProject, ".skillshare", "skills"), 0755)
	os.MkdirAll(filepath.Join(realProject, ".claude", "skills"), 0755)

	symlinkProject := filepath.Join(tmp, "workspace")
	if err := os.Symlink(realProject, symlinkProject); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	// Symlinked root: both resolve under the real project
	got := shouldUseRelative(
		symlinkProject,
		filepath.Join(symlinkProject, ".skillshare", "skills"),
		filepath.Join(symlinkProject, ".claude", "skills"),
	)
	if !got {
		t.Error("expected true: both paths under symlinked project root")
	}
}

func TestShouldUseRelative_DivergentSymlink(t *testing.T) {
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	realProject := filepath.Join(tmp, "project")
	otherDir := filepath.Join(tmp, "other")
	os.MkdirAll(filepath.Join(realProject, ".skillshare", "skills"), 0755)
	os.MkdirAll(filepath.Join(otherDir, "skills"), 0755)

	// Symlink .claude/skills to outside the project
	os.MkdirAll(filepath.Join(realProject, ".claude"), 0755)
	if err := os.Symlink(filepath.Join(otherDir, "skills"), filepath.Join(realProject, ".claude", "skills")); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	// Target resolves to outside project root → should use absolute
	got := shouldUseRelative(
		realProject,
		filepath.Join(realProject, ".skillshare", "skills"),
		filepath.Join(realProject, ".claude", "skills"),
	)
	if got {
		t.Error("expected false: target symlink resolves outside project root")
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

func TestReformatLink(t *testing.T) {
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	source := filepath.Join(tmp, "source")
	os.MkdirAll(source, 0755)

	link := filepath.Join(tmp, "link")
	os.Symlink(source, link)

	// Verify setup: absolute link
	dest, _ := os.Readlink(link)
	if !filepath.IsAbs(dest) {
		t.Fatal("setup: link should be absolute")
	}

	// Reformat to relative (atomic)
	if err := reformatLink(link, source, true); err != nil {
		t.Fatalf("reformatLink: %v", err)
	}

	// Verify: now relative
	dest, _ = os.Readlink(link)
	if filepath.IsAbs(dest) {
		t.Errorf("after reformat, link should be relative, got %q", dest)
	}

	// Verify: still resolves correctly
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatalf("symlink should resolve: %v", err)
	}
	if resolved != source {
		t.Errorf("resolved = %q, want %q", resolved, source)
	}

	// Verify: no temp file left behind
	if _, err := os.Lstat(link + ".ss-reformat"); err == nil {
		t.Error("temp file should be cleaned up")
	}
}
