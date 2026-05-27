package sync

import (
	"os"
	"path/filepath"
	"strings"
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

func TestRunExtensionFile_TransformsAndWrites(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.md")
	if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	// Extension uppercases stdin via tr.
	spec := &ExtensionSpec{Run: []string{"tr", "a-z", "A-Z"}, Dir: dir, Name: "upper"}
	tgt := filepath.Join(dir, "out", "in.toml")

	err := runExtensionFile(spec, src, tgt, map[string]string{"SS_MODE": "sync"})
	if err != nil {
		t.Fatalf("runExtensionFile: %v", err)
	}
	out, _ := os.ReadFile(tgt)
	if string(out) != "HELLO" {
		t.Errorf("output = %q, want HELLO", string(out))
	}
}

func TestRunExtensionFile_NonZeroExitReturnsError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.md")
	if err := os.WriteFile(src, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	spec := &ExtensionSpec{Run: []string{"sh", "-c", "echo boom >&2; exit 3"}, Dir: dir, Name: "fail"}
	err := runExtensionFile(spec, src, filepath.Join(dir, "out.toml"), nil)
	if err == nil {
		t.Fatal("expected error on non-zero exit")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should include stderr, got: %v", err)
	}
}
