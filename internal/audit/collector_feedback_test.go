package audit

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestSpecialMode(t *testing.T) {
	for _, tt := range []struct {
		mode fs.FileMode
		want bool
	}{
		{0, false}, {fs.ModeIrregular, false}, {fs.ModeSymlink, false},
		{fs.ModeNamedPipe, true}, {fs.ModeSocket, true}, {fs.ModeDevice, true}, {fs.ModeCharDevice, true},
	} {
		if got := isSpecialMode(tt.mode); got != tt.want {
			t.Errorf("mode %v: got %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestCollectorNonTextAndDanglingLink(t *testing.T) {
	for _, tt := range []struct {
		name             string
		linked, dangling bool
		want             int
	}{
		{"regular", false, false, 1}, {"linked", true, false, 0}, {"dangling", true, true, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "skill")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill\n"), 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "logo.png")
			target := path
			if tt.linked {
				target = filepath.Join(root, "target.png")
			}
			if !tt.dangling {
				if err := os.WriteFile(target, []byte("ordinary asset"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if tt.linked {
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			result, err := scanSkillImpl(dir, []rule{}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if result.SkippedFiles != tt.want {
				t.Fatalf("got %d skipped files, want %d", result.SkippedFiles, tt.want)
			}
		})
	}
}
