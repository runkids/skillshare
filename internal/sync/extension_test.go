package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExtensionSpec_Directory(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, "md2gemini")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := "run: [\"python\", \"convert.py\"]\noutput_ext: toml\ndescription: x\n"
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadExtensionSpec(extDir, "md2gemini")
	if err != nil {
		t.Fatalf("LoadExtensionSpec: %v", err)
	}
	if got := spec.Run; len(got) != 2 || got[0] != "python" || got[1] != "convert.py" {
		t.Errorf("Run = %v", got)
	}
	if spec.OutputExt != "toml" {
		t.Errorf("OutputExt = %q, want toml", spec.OutputExt)
	}
	if spec.Dir != extDir {
		t.Errorf("Dir = %q, want %q", spec.Dir, extDir)
	}
}

func TestLoadExtensionSpec_SingleFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "conv.sh")
	if err := os.WriteFile(file, []byte("#!/bin/sh\ncat\n"), 0755); err != nil {
		t.Fatal(err)
	}
	spec, err := LoadExtensionSpec(file, "conv.sh")
	if err != nil {
		t.Fatalf("LoadExtensionSpec: %v", err)
	}
	if len(spec.Run) != 1 || spec.Run[0] != file {
		t.Errorf("Run = %v, want [%q]", spec.Run, file)
	}
	if spec.OutputExt != "" {
		t.Errorf("OutputExt = %q, want empty", spec.OutputExt)
	}
}

func TestLoadExtensionSpec_NotFound(t *testing.T) {
	if _, err := LoadExtensionSpec(filepath.Join(t.TempDir(), "nope"), "nope"); err == nil {
		t.Error("expected error for missing extension")
	}
}

func TestApplyOutputExt(t *testing.T) {
	if got := applyOutputExt("review/x.md", "toml"); got != "review/x.toml" {
		t.Errorf("got %q, want review/x.toml", got)
	}
	if got := applyOutputExt("x.md", ""); got != "x.md" {
		t.Errorf("got %q, want x.md (empty ext keeps original)", got)
	}
}
