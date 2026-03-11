package config

import (
	"path/filepath"
	"testing"
)

func TestExtrasSourceDir(t *testing.T) {
	got := ExtrasSourceDir("/home/user/.config/skillshare/skills", "rules")
	want := filepath.Join("/home/user/.config/skillshare", "extras", "rules")
	if got != want {
		t.Errorf("ExtrasSourceDir() = %q, want %q", got, want)
	}
}

func TestExtrasSourceDirProject(t *testing.T) {
	got := ExtrasSourceDirProject("/projects/myapp", "rules")
	want := filepath.Join("/projects/myapp", ".skillshare", "extras", "rules")
	if got != want {
		t.Errorf("ExtrasSourceDirProject() = %q, want %q", got, want)
	}
}

func TestExtrasParentDir(t *testing.T) {
	got := ExtrasParentDir("/home/user/.config/skillshare/skills")
	want := filepath.Join("/home/user/.config/skillshare", "extras")
	if got != want {
		t.Errorf("ExtrasParentDir() = %q, want %q", got, want)
	}
}
