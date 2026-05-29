package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadExtensionSpec_Directory(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, "gemini-commands")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := "run: [\"python\", \"convert.py\"]\noutput_ext: toml\ndescription: x\n"
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadExtensionSpec(extDir, "gemini-commands")
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

func TestListExtensions(t *testing.T) {
	dir := t.TempDir()

	// (a) directory-form extension (has extension.yaml)
	geminiDir := filepath.Join(dir, "gemini-commands")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(geminiDir, "extension.yaml"), []byte("run: [\"cat\"]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// (b) single-file executable extension
	if err := os.WriteFile(filepath.Join(dir, "conv.sh"), []byte("#!/bin/sh\ncat\n"), 0755); err != nil {
		t.Fatal(err)
	}
	// (c) directory without manifest — excluded
	if err := os.MkdirAll(filepath.Join(dir, "notanext"), 0755); err != nil {
		t.Fatal(err)
	}
	// (d) non-executable regular file — excluded
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ListExtensions(dir)
	if err != nil {
		t.Fatalf("ListExtensions: %v", err)
	}
	want := []string{"conv.sh", "gemini-commands"} // sorted
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			break
		}
	}
}

func TestListExtensions_MissingDirReturnsEmpty(t *testing.T) {
	got, err := ListExtensions(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
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

func TestSyncExtraTransform_GeneratesRenamedFiles(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(srcDir, "review"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "review", "pr.md"), []byte("body"), 0644); err != nil {
		t.Fatal(err)
	}
	tgtDir := filepath.Join(dir, "tgt")
	spec := &ExtensionSpec{Run: []string{"cat"}, Dir: dir, Name: "id", OutputExt: "toml"}

	res, err := SyncExtra(srcDir, tgtDir, "copy", false, false, false, "", spec)
	if err != nil {
		t.Fatalf("SyncExtra: %v", err)
	}
	if res.Synced != 1 {
		t.Errorf("Synced = %d, want 1", res.Synced)
	}
	out := filepath.Join(tgtDir, "review", "pr.toml")
	if _, statErr := os.Stat(out); statErr != nil {
		t.Errorf("expected generated file %s: %v", out, statErr)
	}
}

func TestSyncExtraTransform_PrunesGeneratedOrphans(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.md"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	tgtDir := filepath.Join(dir, "tgt")
	if err := os.MkdirAll(tgtDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tgtDir, "old.toml"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	spec := &ExtensionSpec{Run: []string{"cat"}, Dir: dir, Name: "id", OutputExt: "toml"}

	res, err := SyncExtra(srcDir, tgtDir, "copy", false, false, false, "", spec)
	if err != nil {
		t.Fatalf("SyncExtra: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(tgtDir, "a.toml")); statErr != nil {
		t.Errorf("expected a.toml to exist (not pruned): %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(tgtDir, "old.toml")); !os.IsNotExist(statErr) {
		t.Errorf("expected old.toml to be pruned")
	}
	if res.Pruned != 1 {
		t.Errorf("Pruned = %d, want 1", res.Pruned)
	}
}

func TestSyncExtraTransform_DryRunNoSpawn(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.md"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	tgtDir := filepath.Join(dir, "tgt")
	spec := &ExtensionSpec{Run: []string{"false"}, Dir: dir, Name: "x", OutputExt: "toml"}

	res, err := SyncExtra(srcDir, tgtDir, "copy", true, false, false, "", spec)
	if err != nil {
		t.Fatalf("SyncExtra dry-run: %v", err)
	}
	if res.Synced != 1 {
		t.Errorf("Synced = %d, want 1 (counted, not run)", res.Synced)
	}
	if _, statErr := os.Stat(filepath.Join(tgtDir, "a.toml")); !os.IsNotExist(statErr) {
		t.Errorf("dry-run must not write files")
	}
}
