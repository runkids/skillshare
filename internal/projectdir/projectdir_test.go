package projectdir

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte("targets: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestFind(t *testing.T) {
	tests := []struct {
		name    string
		create  []string
		want    string
		wantOK  bool
		comment string
	}{
		{name: "hidden only", create: []string{Default}, want: Default, wantOK: true},
		{name: "visible only", create: []string{Visible}, want: Visible, wantOK: true},
		{
			name:    "both present prefers hidden",
			create:  []string{Default, Visible},
			want:    Default,
			wantOK:  true,
			comment: "back-compat: an existing project must never move",
		},
		{name: "neither present", create: nil, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range tt.create {
				writeConfig(t, filepath.Join(root, name))
			}

			got, ok := Find(root)
			if ok != tt.wantOK {
				t.Fatalf("Find() ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if want := filepath.Join(root, tt.want); got != want {
				t.Errorf("Find() = %q, want %q", got, want)
			}
		})
	}
}

func TestFindIgnoresConfigDirectory(t *testing.T) {
	root := t.TempDir()
	// A directory named config.yaml is not a config file.
	if err := os.MkdirAll(filepath.Join(root, Default, ConfigFileName), 0755); err != nil {
		t.Fatal(err)
	}

	if _, ok := Find(root); ok {
		t.Error("Find() found a project where config.yaml is a directory")
	}
}

func TestResolveDefaultsToHidden(t *testing.T) {
	root := t.TempDir()

	if got, want := Resolve(root), filepath.Join(root, Default); got != want {
		t.Errorf("Resolve(empty root) = %q, want %q", got, want)
	}
}

func TestResolveUsesVisibleProject(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, filepath.Join(root, Visible))

	if got, want := Resolve(root), filepath.Join(root, Visible); got != want {
		t.Errorf("Resolve() = %q, want %q", got, want)
	}
	if got, want := ConfigPath(root), filepath.Join(root, Visible, ConfigFileName); got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestFindPartial(t *testing.T) {
	t.Run("hidden directory without config", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, Default), 0755); err != nil {
			t.Fatal(err)
		}

		got, ok := FindPartial(root)
		if !ok {
			t.Fatal("FindPartial() did not report a partial project")
		}
		if want := filepath.Join(root, Default); got != want {
			t.Errorf("FindPartial() = %q, want %q", got, want)
		}
	})

	t.Run("visible directory needs a resource directory", func(t *testing.T) {
		root := t.TempDir()
		// An unrelated directory that merely shares the name is not a project.
		if err := os.MkdirAll(filepath.Join(root, Visible, "docs"), 0755); err != nil {
			t.Fatal(err)
		}

		if _, ok := FindPartial(root); ok {
			t.Error("FindPartial() claimed an unrelated skillshare/ directory")
		}

		if err := os.MkdirAll(filepath.Join(root, Visible, "skills"), 0755); err != nil {
			t.Fatal(err)
		}
		got, ok := FindPartial(root)
		if !ok {
			t.Fatal("FindPartial() did not report a partial visible project")
		}
		if want := filepath.Join(root, Visible); got != want {
			t.Errorf("FindPartial() = %q, want %q", got, want)
		}
	})

	// A shared repo created before any skill was committed clones with only the
	// tracked .gitignore: git does not carry the empty skills/ and agents/.
	t.Run("visible directory with only a gitignored config", func(t *testing.T) {
		root := t.TempDir()
		writeGitignore(t, filepath.Join(root, Visible), "logs\ntrash\nbackups\nconfig.yaml\n")

		got, ok := FindPartial(root)
		if !ok {
			t.Fatal("FindPartial() did not report a freshly cloned shared repository")
		}
		if want := filepath.Join(root, Visible); got != want {
			t.Errorf("FindPartial() = %q, want %q", got, want)
		}
	})

	t.Run("visible directory with an unrelated gitignore", func(t *testing.T) {
		root := t.TempDir()
		writeGitignore(t, filepath.Join(root, Visible), "# config.yaml\nbuild/\nnotes.md\n")

		if _, ok := FindPartial(root); ok {
			t.Error("FindPartial() claimed a skillshare/ directory whose .gitignore does not cover config.yaml")
		}
	})
}

func writeGitignore(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestIsName(t *testing.T) {
	for _, name := range []string{Default, Visible} {
		if !IsName(name) {
			t.Errorf("IsName(%q) = false, want true", name)
		}
	}
	for _, name := range []string{".claude", "skills", "", "Skillshare"} {
		if IsName(name) {
			t.Errorf("IsName(%q) = true, want false", name)
		}
	}
}

func TestNamesOrderAndIsolation(t *testing.T) {
	names := Names()
	if len(names) != 2 || names[0] != Default || names[1] != Visible {
		t.Fatalf("Names() = %v, want [%q %q]", names, Default, Visible)
	}

	names[0] = "mutated"
	if Names()[0] != Default {
		t.Error("Names() returned a slice aliasing package state")
	}
}

func TestName(t *testing.T) {
	if got := Name(false); got != Default {
		t.Errorf("Name(false) = %q, want %q", got, Default)
	}
	if got := Name(true); got != Visible {
		t.Errorf("Name(true) = %q, want %q", got, Visible)
	}
}
