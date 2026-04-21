package config

import (
	"path/filepath"
	"testing"
)

func TestPluginsSourceDirProject(t *testing.T) {
	got := PluginsSourceDirProject("/projects/myapp")
	want := filepath.Join("/projects/myapp", ".skillshare", "plugins")
	if got != want {
		t.Fatalf("PluginsSourceDirProject() = %q, want %q", got, want)
	}
}

func TestHooksSourceDirProject(t *testing.T) {
	got := HooksSourceDirProject("/projects/myapp")
	want := filepath.Join("/projects/myapp", ".skillshare", "hooks")
	if got != want {
		t.Fatalf("HooksSourceDirProject() = %q, want %q", got, want)
	}
}
