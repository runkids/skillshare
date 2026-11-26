package install

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCopyDir_Basic(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0644)
	sub := filepath.Join(src, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "nested.txt"), []byte("world"), 0644)

	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	// Verify files were copied
	data, err := os.ReadFile(filepath.Join(dst, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}

	data, err = os.ReadFile(filepath.Join(dst, "sub", "nested.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "world" {
		t.Errorf("expected 'world', got %q", string(data))
	}
}

func TestCopyDir_SkipsGit(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Skill"), 0644)
	gitDir := filepath.Join(src, ".git")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0644)

	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	// SKILL.md should exist
	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Error("expected SKILL.md to be copied")
	}

	// .git should NOT exist
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Error("expected .git to be skipped during copy")
	}
}

func TestCopyDir_SkipsDestinationInsideSource(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(src, ".skillshare", "skills", "project")

	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Project"), 0644)
	os.MkdirAll(filepath.Join(src, "content"), 0755)
	os.WriteFile(filepath.Join(src, "content", "notes.md"), []byte("notes"), 0644)
	// Project mode creates .skillshare/skills before local install copies the
	// project root into .skillshare/skills/<name>.
	os.MkdirAll(filepath.Dir(dst), 0755)

	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Fatalf("expected root SKILL.md to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "content", "notes.md")); err != nil {
		t.Fatalf("expected regular content to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".skillshare", "skills", "project")); !os.IsNotExist(err) {
		t.Fatalf("destination must not be recursively copied into itself, stat err=%v", err)
	}
}

func TestCopyDirExcluding_SkipsChildSkillDirs(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	// Orchestrator-style repo layout (issue #124):
	//   SKILL.md              (root skill)
	//   README.md
	//   src/helper.go         (root-owned asset, must remain)
	//   skills/child-a/SKILL.md  (child skill, must be excluded)
	//   skills/child-b/SKILL.md  (child skill, must be excluded)
	//   skills/shared/README.md  (NOT a skill dir, must remain)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Root"), 0644)
	os.WriteFile(filepath.Join(src, "README.md"), []byte("root readme"), 0644)
	os.MkdirAll(filepath.Join(src, "src"), 0755)
	os.WriteFile(filepath.Join(src, "src", "helper.go"), []byte("package main"), 0644)
	os.MkdirAll(filepath.Join(src, "skills", "child-a"), 0755)
	os.WriteFile(filepath.Join(src, "skills", "child-a", "SKILL.md"), []byte("# A"), 0644)
	os.MkdirAll(filepath.Join(src, "skills", "child-b"), 0755)
	os.WriteFile(filepath.Join(src, "skills", "child-b", "SKILL.md"), []byte("# B"), 0644)
	os.MkdirAll(filepath.Join(src, "skills", "shared"), 0755)
	os.WriteFile(filepath.Join(src, "skills", "shared", "README.md"), []byte("shared"), 0644)

	excludes := map[string]bool{
		"skills/child-a": true,
		"skills/child-b": true,
	}
	if err := copyDirExcluding(src, dst, excludes); err != nil {
		t.Fatal(err)
	}

	// Files that MUST exist in the copy
	mustExist := []string{
		"SKILL.md",
		"README.md",
		filepath.Join("src", "helper.go"),
		filepath.Join("skills", "shared", "README.md"),
	}
	for _, rel := range mustExist {
		if _, err := os.Stat(filepath.Join(dst, rel)); err != nil {
			t.Errorf("expected %q to be copied, got %v", rel, err)
		}
	}

	// Files that MUST NOT exist (excluded child skill dirs)
	mustNotExist := []string{
		filepath.Join("skills", "child-a"),
		filepath.Join("skills", "child-b"),
		filepath.Join("skills", "child-a", "SKILL.md"),
	}
	for _, rel := range mustNotExist {
		if _, err := os.Stat(filepath.Join(dst, rel)); !os.IsNotExist(err) {
			t.Errorf("expected %q to be excluded, but it exists (err=%v)", rel, err)
		}
	}
}

func TestCopyDirExcluding_NilMapEqualsCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0644)
	os.MkdirAll(filepath.Join(src, "sub"), 0755)
	os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("b"), 0644)

	if err := copyDirExcluding(src, dst, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "b.txt")); err != nil {
		t.Errorf("expected sub/b.txt to be copied, got %v", err)
	}
}

func TestCopyDir_PreservesPermissions(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	// Create file with executable permission
	srcFile := filepath.Join(src, "script.sh")
	os.WriteFile(srcFile, []byte("#!/bin/bash\necho hi"), 0755)

	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dst, "script.sh"))
	if err != nil {
		t.Fatal(err)
	}
	// Check that executable bit is preserved (at least user execute)
	if runtime.GOOS != "windows" && info.Mode()&0100 == 0 { // no permission bits on Windows
		t.Errorf("expected executable permission to be preserved, got %o", info.Mode())
	}
}

func TestCopyDirWithin_SymlinkedDirInsideBoundary(t *testing.T) {
	repo := t.TempDir()
	skillDir := filepath.Join(repo, "skills", "demo")
	sharedRefs := filepath.Join(repo, "shared", "refs")
	sharedExtra := filepath.Join(repo, "shared", "extra")
	emptyShared := filepath.Join(repo, "shared", "empty")
	dst := filepath.Join(t.TempDir(), "dest")

	for _, dir := range []string{
		skillDir,
		filepath.Join(sharedRefs, "nested"),
		sharedExtra,
		emptyShared,
		filepath.Join(sharedRefs, ".git"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Demo"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedRefs, "guide.md"), []byte("shared guide"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedRefs, "nested", "deep.md"), []byte("deep ref"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedRefs, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedExtra, "note.md"), []byte("extra note"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(filepath.Join("..", "..", "shared", "refs"), filepath.Join(skillDir, "refs")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join("..", "extra"), filepath.Join(sharedRefs, "more")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "shared", "empty"), filepath.Join(skillDir, "empty-refs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "shared", "refs"), filepath.Join(skillDir, "refs-copy")); err != nil {
		t.Fatal(err)
	}

	if err := copyDirWithin(skillDir, dst, repo, nil); err != nil {
		t.Fatalf("copyDirWithin failed: %v", err)
	}

	for _, dir := range []string{"refs", filepath.Join("refs", "more"), "empty-refs", "refs-copy"} {
		info, err := os.Lstat(filepath.Join(dst, dir))
		if err != nil {
			t.Fatalf("expected %s to exist: %v", dir, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			t.Errorf("expected %s to be a real directory, got mode %v", dir, info.Mode())
		}
	}

	checks := map[string]string{
		"SKILL.md":                                 "# Demo",
		filepath.Join("refs", "guide.md"):          "shared guide",
		filepath.Join("refs", "nested", "deep.md"): "deep ref",
		filepath.Join("refs", "more", "note.md"):   "extra note",
		filepath.Join("refs-copy", "guide.md"):     "shared guide",
	}
	for rel, want := range checks {
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Fatalf("expected %s to be copied: %v", rel, err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", rel, string(got), want)
		}
	}
	if _, err := os.Lstat(filepath.Join(dst, "refs", ".git")); !os.IsNotExist(err) {
		t.Errorf("expected nested .git inside linked dir to be skipped, got err=%v", err)
	}
}

func TestCopyDirWithin_RejectsEscapingSymlinks(t *testing.T) {
	outside := t.TempDir()
	outsideSub := filepath.Join(outside, "secret-dir")
	if err := os.MkdirAll(outsideSub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outsideSub, "leak.txt"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		link   string
		target string
	}{
		{name: "dir symlink", link: "refs", target: outsideSub},
		{name: "file symlink", link: "leak.txt", target: filepath.Join(outside, "secret.txt")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			skillDir := filepath.Join(repo, "skills", "demo")
			if err := os.MkdirAll(skillDir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Demo"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(tc.target, filepath.Join(skillDir, tc.link)); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}

			dst := filepath.Join(t.TempDir(), "dest")
			err := copyDirWithin(skillDir, dst, repo, nil)
			if err == nil {
				t.Fatalf("expected error for escaping %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), "escapes repository boundary") {
				t.Errorf("expected boundary error, got: %v", err)
			}
		})
	}
}

func TestCopyDirWithin_RejectsGitDirSymlink(t *testing.T) {
	repo := t.TempDir()
	skillDir := filepath.Join(repo, "skills", "demo")
	gitDir := filepath.Join(repo, ".git")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("[core]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", ".git", "config"), filepath.Join(skillDir, "cfg")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	err := copyDirWithin(skillDir, filepath.Join(t.TempDir(), "dest"), repo, nil)
	if err == nil || !strings.Contains(err.Error(), "targets .git directory") {
		t.Fatalf("expected .git target rejection, got: %v", err)
	}
}

func TestCopyDirWithin_RejectsSymlinkCycles(t *testing.T) {
	t.Run("self cycle", func(t *testing.T) {
		repo := t.TempDir()
		skillDir := filepath.Join(repo, "skills", "demo")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(".", filepath.Join(skillDir, "self")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		err := copyDirWithin(skillDir, filepath.Join(t.TempDir(), "dest"), repo, nil)
		if err == nil || !strings.Contains(err.Error(), "symlink cycle") {
			t.Fatalf("expected symlink cycle error, got: %v", err)
		}
	})

	t.Run("ancestor cycle", func(t *testing.T) {
		repo := t.TempDir()
		skillDir := filepath.Join(repo, "skills", "demo")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join("..", ".."), filepath.Join(skillDir, "up")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		err := copyDirWithin(skillDir, filepath.Join(t.TempDir(), "dest"), repo, nil)
		if err == nil || !strings.Contains(err.Error(), "symlink cycle") {
			t.Fatalf("expected symlink cycle error, got: %v", err)
		}
	})

	t.Run("mutual cycle", func(t *testing.T) {
		repo := t.TempDir()
		skillDir := filepath.Join(repo, "skills", "demo")
		dirA := filepath.Join(repo, "shared", "a")
		dirB := filepath.Join(repo, "shared", "b")
		for _, d := range []string{skillDir, dirA, dirB} {
			if err := os.MkdirAll(d, 0755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Symlink(filepath.Join("..", "..", "shared", "a"), filepath.Join(skillDir, "a")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if err := os.Symlink(filepath.Join("..", "b"), filepath.Join(dirA, "to-b")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join("..", "a"), filepath.Join(dirB, "to-a")); err != nil {
			t.Fatal(err)
		}
		err := copyDirWithin(skillDir, filepath.Join(t.TempDir(), "dest"), repo, nil)
		if err == nil || !strings.Contains(err.Error(), "symlink cycle") {
			t.Fatalf("expected symlink cycle error, got: %v", err)
		}
	})
}

func TestCopyDirWithin_BrokenSymlinkReturnsError(t *testing.T) {
	repo := t.TempDir()
	skillDir := filepath.Join(repo, "skills", "demo")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "missing"), filepath.Join(skillDir, "broken")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	err := copyDirWithin(skillDir, filepath.Join(t.TempDir(), "dest"), repo, nil)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist for broken symlink, got: %v", err)
	}
}

func TestInstallFromDiscovery_SymlinkedDirInRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := initTestRepo(t)
	skillDir := filepath.Join(repo, "skills", "demo")
	sharedRefs := filepath.Join(repo, "shared", "refs")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sharedRefs, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\n---\n# Demo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedRefs, "guide.md"), []byte("hello from shared"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "shared", "refs"), filepath.Join(skillDir, "refs")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add skill with linked folder")

	source, err := ParseSource(fileURL(repo))
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := DiscoverFromGit(source)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanupDiscovery(discovery)

	if len(discovery.Skills) != 1 {
		t.Fatalf("expected 1 discovered skill, got %d", len(discovery.Skills))
	}

	sourceDir := t.TempDir()
	destPath := filepath.Join(sourceDir, "demo")
	if _, err := InstallFromDiscovery(discovery, discovery.Skills[0], destPath, InstallOptions{
		SourceDir: sourceDir,
	}); err != nil {
		t.Fatalf("InstallFromDiscovery failed: %v", err)
	}

	refsInfo, err := os.Lstat(filepath.Join(destPath, "refs"))
	if err != nil {
		t.Fatalf("expected installed refs dir: %v", err)
	}
	if refsInfo.Mode()&os.ModeSymlink != 0 || !refsInfo.IsDir() {
		t.Fatalf("expected installed refs to be a real directory, got mode %v", refsInfo.Mode())
	}
	data, err := os.ReadFile(filepath.Join(destPath, "refs", "guide.md"))
	if err != nil {
		t.Fatalf("expected refs/guide.md to be installed: %v", err)
	}
	if string(data) != "hello from shared" {
		t.Errorf("refs/guide.md = %q, want %q", string(data), "hello from shared")
	}
	store, err := LoadMetadata(sourceDir)
	if err != nil {
		t.Fatalf("LoadMetadata failed: %v", err)
	}
	entry := store.Get("demo")
	if entry == nil {
		t.Fatalf("expected metadata entry for demo")
	}
	if _, ok := entry.FileHashes["refs/guide.md"]; !ok {
		t.Errorf("expected refs/guide.md in FileHashes, got %v", entry.FileHashes)
	}
}

func TestInstallFromDiscovery_RejectsEscapingDirSymlinkInRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("top-secret"), 0644); err != nil {
		t.Fatal(err)
	}

	repo := initTestRepo(t)
	skillDir := filepath.Join(repo, "skills", "evil")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: evil\n---\n# Evil\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(skillDir, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add skill with escaping symlink")

	source, err := ParseSource(fileURL(repo))
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := DiscoverFromGit(source)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanupDiscovery(discovery)

	sourceDir := t.TempDir()
	destPath := filepath.Join(sourceDir, "evil")
	_, err = InstallFromDiscovery(discovery, discovery.Skills[0], destPath, InstallOptions{
		SourceDir: sourceDir,
	})
	if err == nil || !strings.Contains(err.Error(), "escapes repository boundary") {
		t.Fatalf("expected repository boundary error, got: %v", err)
	}
}

func TestCopyDirWithin_RejectsNestedEscapingSymlink(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	skillDir := filepath.Join(repo, "skills", "demo")
	sharedRefs := filepath.Join(repo, "shared", "refs")
	for _, d := range []string{skillDir, sharedRefs} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join("..", "..", "shared", "refs"), filepath.Join(skillDir, "refs")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(sharedRefs, "escape")); err != nil {
		t.Fatal(err)
	}

	err := copyDirWithin(skillDir, filepath.Join(t.TempDir(), "dest"), repo, nil)
	if err == nil || !strings.Contains(err.Error(), "escapes repository boundary") {
		t.Fatalf("expected boundary error for nested escape, got: %v", err)
	}
}

func TestInstallFromGitSubdir_SymlinkedDirInsideSubdir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := initTestRepo(t)
	skillDir := filepath.Join(repo, "skills", "demo")
	internalRefs := filepath.Join(skillDir, "_shared", "refs")
	if err := os.MkdirAll(internalRefs, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\n---\n# Demo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(internalRefs, "guide.md"), []byte("subdir shared"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("_shared", "refs"), filepath.Join(skillDir, "refs")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add subdir skill with linked folder")

	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      fileURL(repo) + "//skills/demo",
		CloneURL: fileURL(repo),
		Subdir:   "skills/demo",
		Name:     "demo",
	}
	sourceDir := t.TempDir()
	destPath := filepath.Join(sourceDir, "demo")
	if _, err := Install(source, destPath, InstallOptions{
		SourceDir: sourceDir,
	}); err != nil {
		t.Fatalf("Install git subdir failed: %v", err)
	}

	refsInfo, err := os.Lstat(filepath.Join(destPath, "refs"))
	if err != nil {
		t.Fatalf("expected installed refs dir: %v", err)
	}
	if refsInfo.Mode()&os.ModeSymlink != 0 || !refsInfo.IsDir() {
		t.Fatalf("expected installed refs to be a real directory, got mode %v", refsInfo.Mode())
	}
	data, err := os.ReadFile(filepath.Join(destPath, "refs", "guide.md"))
	if err != nil {
		t.Fatalf("expected refs/guide.md to be installed: %v", err)
	}
	if string(data) != "subdir shared" {
		t.Errorf("refs/guide.md = %q, want %q", string(data), "subdir shared")
	}
}

func TestInstallFromLocal_SymlinkedDir(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "demo")
	sharedRefs := filepath.Join(root, "shared", "refs")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sharedRefs, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\n---\n# Demo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedRefs, "guide.md"), []byte("local shared"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "shared", "refs"), filepath.Join(skillDir, "refs")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	source, err := ParseSource(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	destPath := filepath.Join(sourceDir, "demo")
	if _, err := Install(source, destPath, InstallOptions{
		SourceDir: sourceDir,
		SkipAudit: true,
	}); err != nil {
		t.Fatalf("local Install failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(destPath, "refs", "guide.md"))
	if err != nil {
		t.Fatalf("expected refs/guide.md in local install: %v", err)
	}
	if string(data) != "local shared" {
		t.Errorf("refs/guide.md = %q, want %q", string(data), "local shared")
	}
}
