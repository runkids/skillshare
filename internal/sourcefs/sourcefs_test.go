package sourcefs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixture builds src/ with a real skill and _f, a symlink to ext/ outside src.
// It returns the open root and ext.
func fixture(t *testing.T) (*Root, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need Developer Mode; the Windows runbook covers junctions")
	}
	base := t.TempDir()
	src := filepath.Join(base, "src")
	ext := filepath.Join(base, "ext")
	mustWrite(t, filepath.Join(src, "local", "SKILL.md"), "local")
	mustWrite(t, filepath.Join(ext, "SKILL.md"), "external")
	mustWrite(t, filepath.Join(ext, "child", "SKILL.md"), "external child")
	if err := os.Symlink(ext, filepath.Join(src, "_f")); err != nil {
		t.Fatal(err)
	}
	r, err := Open(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r, ext
}

func mustWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// snapshot records every path and file content below dir.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if d.IsDir() {
			out[rel] = "<dir>"
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func assertUnchanged(t *testing.T, before map[string]string, dir string) {
	t.Helper()
	after := snapshot(t, dir)
	if len(after) != len(before) {
		t.Fatalf("external tree changed: before %v, after %v", before, after)
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("external tree changed at %s: %q -> %q", k, v, after[k])
		}
	}
}

func TestWritesThroughEscapingLinkAreRefused(t *testing.T) {
	r, ext := fixture(t)
	before := snapshot(t, ext)
	name := filepath.Join("_f", "SKILL.md")

	ops := map[string]func() error{
		"WriteFile":       func() error { return r.WriteFile(name, []byte("x"), 0o644) },
		"WriteFileAtomic": func() error { return r.WriteFileAtomic(name, []byte("x"), 0o644) },
		"MkdirAll":        func() error { return r.MkdirAll(filepath.Join("_f", "new", "deep"), 0o755) },
		"Remove":          func() error { return r.Remove(name) },
		"RemoveAll":       func() error { return r.RemoveAll(filepath.Join("_f", "child")) },
		"Rename":          func() error { return r.Rename(name, filepath.Join("local", "moved.md")) },
		"OpenFile": func() error {
			f, err := r.OpenFile(name, os.O_WRONLY|os.O_TRUNC, 0o644)
			if err == nil {
				f.Close()
			}
			return err
		},
	}
	for op, fn := range ops {
		t.Run(op, func(t *testing.T) {
			err := fn()
			if !errors.Is(err, ErrLink) {
				t.Fatalf("%s through _f: got %v, want ErrLink", op, err)
			}
			if !strings.Contains(err.Error(), "is a link; edit its target directly") {
				t.Fatalf("message = %q", err.Error())
			}
			assertUnchanged(t, before, ext)
		})
	}
}

func TestStatThroughEscapingLinkIsRefused(t *testing.T) {
	r, _ := fixture(t)
	if _, err := r.Stat(filepath.Join("_f", "SKILL.md")); err == nil {
		t.Fatal("Stat through an escaping link should fail")
	}
}

func TestUnlinkRemovesOnlyTheLink(t *testing.T) {
	r, ext := fixture(t)
	before := snapshot(t, ext)
	if err := r.Unlink("_f"); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(r.Dir(), "_f")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("link still present: %v", err)
	}
	assertUnchanged(t, before, ext)
}

func TestUnlinkRefusesRealDirectory(t *testing.T) {
	r, _ := fixture(t)
	if err := r.Unlink("local"); err == nil {
		t.Fatal("Unlink of a real directory should fail")
	}
	if _, err := os.Stat(filepath.Join(r.Dir(), "local", "SKILL.md")); err != nil {
		t.Fatalf("real directory was touched: %v", err)
	}
}

func TestReplacingLastComponentLinkIsRefused(t *testing.T) {
	r, ext := fixture(t)
	before := snapshot(t, ext)
	// SKILL.md of a real skill that links to a shared file.
	shared := filepath.Join(ext, "SKILL.md")
	if err := os.Symlink(shared, filepath.Join(r.Dir(), "local", "LINKED.md")); err != nil {
		t.Fatal(err)
	}
	for op, fn := range map[string]func() error{
		"RemoveAll _f":      func() error { return r.RemoveAll("_f") },
		"Remove _f":         func() error { return r.Remove("_f") },
		"Rename onto _f":    func() error { return r.Rename(filepath.Join("local", "SKILL.md"), "_f") },
		"WriteFileAtomic":   func() error { return r.WriteFileAtomic(filepath.Join("local", "LINKED.md"), []byte("x"), 0o644) },
		"WriteFile":         func() error { return r.WriteFile(filepath.Join("local", "LINKED.md"), []byte("x"), 0o644) },
		"MoveIn onto _f":    func() error { return r.MoveIn(t.TempDir(), "_f") },
		"MkdirAll at link":  func() error { return r.MkdirAll("_f", 0o755) },
		"CheckNoLink at _f": func() error { return r.CheckNoLink("_f") },
		"CheckParent below": func() error { return r.CheckParent(filepath.Join("_f", "child")) },
	} {
		if err := fn(); !errors.Is(err, ErrLink) {
			t.Errorf("%s: got %v, want ErrLink", op, err)
		}
	}
	if fi, err := os.Lstat(filepath.Join(r.Dir(), "_f")); err != nil || fi.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("_f is no longer a link: %v", err)
	}
	if fi, err := os.Lstat(filepath.Join(r.Dir(), "local", "LINKED.md")); err != nil || fi.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("LINKED.md is no longer a link: %v", err)
	}
	assertUnchanged(t, before, ext)
}

func TestRelativeLinkInsideRootIsRefused(t *testing.T) {
	r, _ := fixture(t)
	// os.Root alone allows this link; the module does not.
	if err := os.Symlink("local", filepath.Join(r.Dir(), "alias")); err != nil {
		t.Fatal(err)
	}
	err := r.MkdirAll(filepath.Join("alias", "new"), 0o755)
	if !errors.Is(err, ErrLink) {
		t.Fatalf("MkdirAll through alias: got %v, want ErrLink", err)
	}
	if _, err := os.Stat(filepath.Join(r.Dir(), "local", "new")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("directory created through the link: %v", err)
	}
}

func TestWriteFileAtomicKeepsTempInsideRoot(t *testing.T) {
	r, ext := fixture(t)
	before := snapshot(t, ext)
	_ = r.WriteFileAtomic(filepath.Join("_f", "SKILL.md"), []byte("x"), 0o644)
	assertUnchanged(t, before, ext)

	name := filepath.Join("local", "SKILL.md")
	if err := r.WriteFileAtomic(name, []byte("updated"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(r.Dir(), name))
	if err != nil || string(data) != "updated" {
		t.Fatalf("content = %q, %v", data, err)
	}
	entries, err := os.ReadDir(filepath.Join(r.Dir(), "local"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temp file left behind: %v", entries)
	}
}

func TestOrdinaryWritesWork(t *testing.T) {
	r, _ := fixture(t)
	if err := r.MkdirAll(filepath.Join("group", "skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteFile(filepath.Join("group", "skill", "SKILL.md"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := r.Rename(filepath.Join("group", "skill"), filepath.Join("group", "renamed")); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(t.TempDir(), "staged")
	mustWrite(t, filepath.Join(staged, "SKILL.md"), "staged")
	if err := r.MoveIn(staged, filepath.Join("group", "moved")); err != nil {
		t.Fatal(err)
	}
	if err := r.RemoveAll("group"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.Dir(), "group")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("group still present: %v", err)
	}
}

func TestRemoveAllOfDirectoryContainingLinkKeepsTarget(t *testing.T) {
	r, ext := fixture(t)
	before := snapshot(t, ext)
	if err := os.Symlink(ext, filepath.Join(r.Dir(), "local", "inner")); err != nil {
		t.Fatal(err)
	}
	if err := r.RemoveAll("local"); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	assertUnchanged(t, before, ext)
}

func TestSourceRootThatIsALink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need Developer Mode")
	}
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	r, err := Open(alias)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.WriteFile("SKILL.md", []byte("x"), 0o644); err != nil {
		t.Fatalf("write in a linked root: %v", err)
	}
	for _, p := range []string{filepath.Join(alias, "a", "b"), filepath.Join(utilsResolve(t, real), "a", "b")} {
		rel, err := r.Rel(p)
		if err != nil || rel != filepath.Join("a", "b") {
			t.Fatalf("Rel(%s) = %q, %v", p, rel, err)
		}
	}
	if _, err := r.Rel(base); err == nil {
		t.Fatal("Rel outside the root should fail")
	}
}

func utilsResolve(t *testing.T, p string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestCopyInCopiesTreeThroughRoot(t *testing.T) {
	r, ext := fixture(t)
	before := snapshot(t, ext)
	staged := filepath.Join(t.TempDir(), "skill")
	mustWrite(t, filepath.Join(staged, "SKILL.md"), "staged")
	mustWrite(t, filepath.Join(staged, "scripts", "run.sh"), "#!/bin/sh")
	if err := os.Chmod(filepath.Join(staged, "scripts", "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := r.CopyIn(staged, "copied"); err != nil {
		t.Fatalf("CopyIn: %v", err)
	}
	got, want := snapshot(t, filepath.Join(r.Dir(), "copied")), snapshot(t, staged)
	if len(got) != len(want) {
		t.Fatalf("copied tree = %v, want %v", got, want)
	}
	for name, data := range want {
		if got[name] != data {
			t.Fatalf("%s = %q, want %q", name, got[name], data)
		}
	}
	info, err := os.Stat(filepath.Join(r.Dir(), "copied", "scripts", "run.sh"))
	if err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("executable bit not preserved: %v %v", info, err)
	}

	for _, name := range []string{"_f", filepath.Join("_f", "new")} {
		if err := r.CopyIn(staged, name); !errors.Is(err, ErrLink) {
			t.Fatalf("CopyIn to %s: got %v, want ErrLink", name, err)
		}
	}
	if err := r.CopyIn(staged, "copied"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("CopyIn onto an existing name: got %v, want already exists", err)
	}
	assertUnchanged(t, before, ext)
}
