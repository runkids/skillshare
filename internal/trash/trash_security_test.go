package trash

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// --- Helpers ---

// setupTraversalTree creates:
//
//	root/
//	  base/     (trashBase)
//	  outside/
//	    canary.txt   (must never be touched)
func setupTraversalTree(t *testing.T) (root, base, outside, canary string) {
	t.Helper()
	root = t.TempDir()
	base = filepath.Join(root, "base")
	outside = filepath.Join(root, "outside")
	canary = filepath.Join(outside, "canary.txt")
	os.MkdirAll(base, 0755)
	os.MkdirAll(outside, 0755)
	os.WriteFile(canary, []byte("canary"), 0644)
	return
}

func assertCanaryUntouched(t *testing.T, canary string) {
	t.Helper()
	data, err := os.ReadFile(canary)
	if err != nil {
		t.Fatalf("canary file missing or unreadable: %v", err)
	}
	if string(data) != "canary" {
		t.Errorf("canary was modified: got %q", string(data))
	}
}

func assertNothingWrittenOutside(t *testing.T, root, base string) {
	t.Helper()
	baseClean := filepath.Clean(base)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		clean := filepath.Clean(path)
		if clean == baseClean || strings.HasPrefix(clean, baseClean+string(filepath.Separator)) {
			return nil
		}
		// Allow the root itself and the outside/ directory structure
		rel, _ := filepath.Rel(root, clean)
		if rel == "." || rel == "outside" || rel == "outside/canary.txt" {
			return nil
		}
		if !info.IsDir() {
			t.Errorf("unexpected file outside base: %s", path)
		}
		return nil
	})
}

// --- Adversarial Tests: MoveToTrash ---

func TestMoveToTrash_RejectsTraversal_DotDotSlash(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)
	srcDir := filepath.Join(base, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("content"), 0644)

	_, err := MoveToTrash(srcDir, "../outside/pwn", base)
	if err == nil {
		t.Fatal("expected error for traversal name")
	}

	assertCanaryUntouched(t, canary)
	assertNothingWrittenOutside(t, filepath.Dir(base), base)
}

func TestMoveToTrash_RejectsTraversal_DoubleDot(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)
	srcDir := filepath.Join(base, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("content"), 0644)

	_, err := MoveToTrash(srcDir, "../../outside/pwn", base)
	if err == nil {
		t.Fatal("expected error for traversal name")
	}

	assertCanaryUntouched(t, canary)
	assertNothingWrittenOutside(t, filepath.Dir(base), base)
}

func TestMoveToTrash_RejectsTraversal_Backslash(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)
	srcDir := filepath.Join(base, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("content"), 0644)

	_, err := MoveToTrash(srcDir, `..\outside\pwn`, base)
	if err == nil {
		t.Fatal("expected error for backslash traversal name")
	}

	assertCanaryUntouched(t, canary)
}

func TestMoveToTrash_RejectsTraversal_AbsolutePath(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)
	srcDir := filepath.Join(base, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("content"), 0644)

	_, err := MoveToTrash(srcDir, "/absolute/path", base)
	if err == nil {
		t.Fatal("expected error for absolute name")
	}

	assertCanaryUntouched(t, canary)
}

func TestMoveToTrash_RejectsTraversal_Empty(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)

	_, err := MoveToTrash(base, "", base)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestMoveToTrash_RejectsTraversal_DotAlone(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)

	_, err := MoveToTrash(base, ".", base)
	if err == nil {
		t.Fatal("expected error for '.' name")
	}
}

func TestMoveToTrash_RejectsTraversal_DotDotAlone(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)

	_, err := MoveToTrash(base, "..", base)
	if err == nil {
		t.Fatal("expected error for '..' name")
	}
}

func TestMoveToTrash_RejectsTraversal_NulByte(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)

	_, err := MoveToTrash(base, "skill\x00/../../etc", base)
	if err == nil {
		t.Fatal("expected error for NUL byte in name")
	}
}

func TestMoveToTrash_RejectsTraversal_EmbeddedDotDot(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)
	srcDir := filepath.Join(base, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("content"), 0644)

	_, err := MoveToTrash(srcDir, "a/../../outside", base)
	if err == nil {
		t.Fatal("expected error for embedded traversal name")
	}

	assertCanaryUntouched(t, canary)
	assertNothingWrittenOutside(t, filepath.Dir(base), base)
}

func TestEnsureStrictlyUnderBase_RejectsCandidateEqualsBase(t *testing.T) {
	dir := t.TempDir()
	err := ensureStrictlyUnderBase(dir, dir)
	if err == nil {
		t.Fatal("ensureStrictlyUnderBase must reject candidate == base")
	}
}

func TestEnsureStrictlyUnderBase_RejectsEscape(t *testing.T) {
	dir := t.TempDir()
	err := ensureStrictlyUnderBase(dir, filepath.Join(dir, "..", "pwn"))
	if err == nil {
		t.Fatal("ensureStrictlyUnderBase must reject escape")
	}
}

func TestEnsureStrictlyUnderBase_AllowsStrictSubpath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "child")
	if err := ensureStrictlyUnderBase(dir, sub); err != nil {
		t.Fatalf("ensureStrictlyUnderBase should allow subpath: %v", err)
	}
}

func TestMoveToTrash_AllowsNestedName(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)
	srcDir := filepath.Join(base, "src")
	os.MkdirAll(filepath.Join(srcDir, "org", "team"), 0755)
	os.WriteFile(filepath.Join(srcDir, "org", "team", "SKILL.md"), []byte("nested"), 0644)

	trashPath, err := MoveToTrash(filepath.Join(srcDir, "org", "team"), "org/team-skill", base)
	if err != nil {
		t.Fatalf("nested name should be allowed: %v", err)
	}

	// Verify trashPath is under base
	clean := filepath.Clean(trashPath)
	baseClean := filepath.Clean(base)
	if !strings.HasPrefix(clean, baseClean+string(filepath.Separator)) {
		t.Errorf("trashPath %q escapes base %q", trashPath, base)
	}

	// Verify source was moved
	if _, err := os.Stat(filepath.Join(srcDir, "org", "team")); !os.IsNotExist(err) {
		t.Error("source should be removed after MoveToTrash")
	}
}

// --- Adversarial Tests: MoveAgentToTrash ---

func TestMoveAgentToTrash_RejectsTraversal_DotDotSlash(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)

	_, err := MoveAgentToTrash("dummy.md", "", "../outside/pwn", base)
	if err == nil {
		t.Fatal("expected error for traversal name")
	}

	assertCanaryUntouched(t, canary)
}

func TestMoveAgentToTrash_RejectsTraversal_Backslash(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)

	_, err := MoveAgentToTrash("dummy.md", "", `..\outside\pwn`, base)
	if err == nil {
		t.Fatal("expected error for backslash traversal name")
	}

	assertCanaryUntouched(t, canary)
}

func TestMoveAgentToTrash_RejectsTraversal_AbsolutePath(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)

	_, err := MoveAgentToTrash("dummy.md", "", "/absolute/path", base)
	if err == nil {
		t.Fatal("expected error for absolute name")
	}

	assertCanaryUntouched(t, canary)
}

func TestMoveAgentToTrash_AllowsNestedName(t *testing.T) {
	_, base, outside, canary := setupTraversalTree(t)

	// Create a valid agent file to exercise the full path
	agentFile := filepath.Join(outside, "agent.md")
	os.WriteFile(agentFile, []byte("payload"), 0644)

	// "demo/my-agent" should be allowed
	_, err := MoveAgentToTrash(agentFile, "", "demo/my-agent", base)
	// It may error because agentFile isn't inside base for rename,
	// but it must NOT create files outside base.
	if err != nil {
		t.Logf("MoveAgentToTrash returned error (expected for cross-device): %v", err)
	}

	assertCanaryUntouched(t, canary)
	assertNothingWrittenOutside(t, filepath.Dir(base), base)
}

// --- Adversarial Tests: Restore ---

func TestRestore_RejectsTraversal_DotDotSlash(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)
	destDir := filepath.Join(filepath.Dir(base), "dest")
	os.MkdirAll(destDir, 0755)

	entry := &TrashEntry{
		Name:      "../../outside/pwn",
		Path:      filepath.Join(base, "fake_2026-01-01_10-00-00"),
		Timestamp: "2026-01-01_10-00-00",
		Date:      time.Now(),
	}

	err := Restore(entry, destDir)
	if err == nil {
		t.Fatal("expected error for traversal entry name")
	}

	assertCanaryUntouched(t, canary)
}

func TestRestore_RejectsTraversal_Backslash(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)
	destDir := filepath.Join(filepath.Dir(base), "dest")
	os.MkdirAll(destDir, 0755)

	entry := &TrashEntry{
		Name:      `..\outside\pwn`,
		Path:      filepath.Join(base, "fake_2026-01-01_10-00-00"),
		Timestamp: "2026-01-01_10-00-00",
		Date:      time.Now(),
	}

	err := Restore(entry, destDir)
	if err == nil {
		t.Fatal("expected error for backslash traversal entry name")
	}

	assertCanaryUntouched(t, canary)
}

func TestRestore_RejectsTraversal_AbsolutePath(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)
	destDir := filepath.Join(filepath.Dir(base), "dest")
	os.MkdirAll(destDir, 0755)

	entry := &TrashEntry{
		Name:      "/absolute/path",
		Path:      filepath.Join(base, "fake_2026-01-01_10-00-00"),
		Timestamp: "2026-01-01_10-00-00",
		Date:      time.Now(),
	}

	err := Restore(entry, destDir)
	if err == nil {
		t.Fatal("expected error for absolute entry name")
	}

	assertCanaryUntouched(t, canary)
}

func TestRestore_AllowsNestedName(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)
	destDir := filepath.Join(filepath.Dir(base), "dest")
	os.MkdirAll(destDir, 0755)

	// Create a real trash entry
	trashDir := filepath.Join(base, "org_skill_2026-01-01_10-00-00")
	os.MkdirAll(trashDir, 0755)
	os.WriteFile(filepath.Join(trashDir, "SKILL.md"), []byte("nested"), 0644)

	entry := &TrashEntry{
		Name:      "org/skill",
		Path:      trashDir,
		Timestamp: "2026-01-01_10-00-00",
		Date:      time.Now(),
	}

	if err := Restore(entry, destDir); err != nil {
		t.Fatalf("nested restore should succeed: %v", err)
	}

	// Verify restored file is under destDir
	restored := filepath.Join(destDir, "org", "skill", "SKILL.md")
	if _, err := os.Stat(restored); err != nil {
		t.Errorf("restored file not found: %v", err)
	}
}

// --- Adversarial Tests: RestoreAgent ---

func TestRestoreAgent_RejectsTraversal_DotDotSlash(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)
	destDir := filepath.Join(filepath.Dir(base), "dest")
	os.MkdirAll(destDir, 0755)

	trashDir := filepath.Join(base, "agent_2026-01-01_10-00-00")
	os.MkdirAll(trashDir, 0755)
	os.WriteFile(filepath.Join(trashDir, "helper.md"), []byte("content"), 0644)

	entry := &TrashEntry{
		Name:      "../../outside/pwn",
		Path:      trashDir,
		Timestamp: "2026-01-01_10-00-00",
		Date:      time.Now(),
	}

	err := RestoreAgent(entry, destDir)
	if err == nil {
		t.Fatal("expected error for traversal entry name")
	}

	assertCanaryUntouched(t, canary)
}

func TestRestoreAgent_RejectsTraversal_Backslash(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)
	destDir := filepath.Join(filepath.Dir(base), "dest")
	os.MkdirAll(destDir, 0755)

	trashDir := filepath.Join(base, "agent_2026-01-01_10-00-00")
	os.MkdirAll(trashDir, 0755)
	os.WriteFile(filepath.Join(trashDir, "helper.md"), []byte("content"), 0644)

	entry := &TrashEntry{
		Name:      `..\outside\pwn`,
		Path:      trashDir,
		Timestamp: "2026-01-01_10-00-00",
		Date:      time.Now(),
	}

	err := RestoreAgent(entry, destDir)
	if err == nil {
		t.Fatal("expected error for backslash traversal entry name")
	}

	assertCanaryUntouched(t, canary)
}

func TestRestoreAgent_RejectsTraversal_AbsolutePath(t *testing.T) {
	_, base, _, canary := setupTraversalTree(t)
	destDir := filepath.Join(filepath.Dir(base), "dest")
	os.MkdirAll(destDir, 0755)

	entry := &TrashEntry{
		Name:      "/absolute/path",
		Path:      filepath.Join(base, "agent_2026-01-01_10-00-00"),
		Timestamp: "2026-01-01_10-00-00",
		Date:      time.Now(),
	}

	err := RestoreAgent(entry, destDir)
	if err == nil {
		t.Fatal("expected error for absolute entry name")
	}

	assertCanaryUntouched(t, canary)
}

func TestRestoreAgent_AllowsNestedName(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)
	destDir := filepath.Join(filepath.Dir(base), "dest")
	os.MkdirAll(destDir, 0755)

	trashDir := filepath.Join(base, "demo_agent_2026-01-01_10-00-00")
	os.MkdirAll(trashDir, 0755)
	os.WriteFile(filepath.Join(trashDir, "helper.md"), []byte("nested"), 0644)

	entry := &TrashEntry{
		Name:      "demo/my-agent",
		Path:      trashDir,
		Timestamp: "2026-01-01_10-00-00",
		Date:      time.Now(),
	}

	if err := RestoreAgent(entry, destDir); err != nil {
		t.Fatalf("nested agent restore should succeed: %v", err)
	}

	// RestoreAgent uses filepath.Dir(entry.Name) = "demo" as targetDir,
	// then copies files from entry.Path into targetDir.
	// So helper.md lands at destDir/demo/helper.md.
	restored := filepath.Join(destDir, "demo", "helper.md")
	if _, err := os.Stat(restored); err != nil {
		t.Errorf("restored agent file not found: %v", err)
	}
}

// --- Adversarial Tests: Cleanup ---

func TestCleanup_PathTraversal(t *testing.T) {
	root, base, _, canary := setupTraversalTree(t)

	old := time.Now().Add(-8 * 24 * time.Hour).Format("2006-01-02_15-04-05")
	os.MkdirAll(filepath.Join(base, "skill_"+old), 0755)
	os.WriteFile(filepath.Join(base, "skill_"+old, "SKILL.md"), []byte("old"), 0644)

	removed, err := Cleanup(base, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("Cleanup error: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	assertCanaryUntouched(t, canary)
	assertNothingWrittenOutside(t, root, base)
}

// --- Production Invariant Tests ---

func TestList_EntryPathAlwaysUnderTrashBase(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)

	os.MkdirAll(filepath.Join(base, "skill-a_2026-01-01_10-00-00"), 0755)
	os.MkdirAll(filepath.Join(base, "org", "repo_2026-01-02_10-00-00"), 0755)

	items := List(base)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	for _, item := range items {
		clean := filepath.Clean(item.Path)
		baseClean := filepath.Clean(base)
		if !strings.HasPrefix(clean, baseClean+string(filepath.Separator)) && clean != baseClean {
			t.Errorf("item.Path %q escapes trash base %q", item.Path, base)
		}
	}
}

func TestList_NameNeverContainsDotDot(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)

	os.MkdirAll(filepath.Join(base, "skill_2026-01-01_10-00-00"), 0755)
	os.MkdirAll(filepath.Join(base, "org", "repo_2026-01-02_10-00-00"), 0755)

	items := List(base)
	for _, item := range items {
		if strings.Contains(item.Name, "..") {
			t.Errorf("item.Name contains '..': %q", item.Name)
		}
	}
}

func TestList_NameJoinDestDirNeverEscapes(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)
	destDir := t.TempDir()

	os.MkdirAll(filepath.Join(base, "skill_2026-01-01_10-00-00"), 0755)
	os.MkdirAll(filepath.Join(base, "org", "repo_2026-01-02_10-00-00"), 0755)

	items := List(base)
	for _, item := range items {
		destPath := filepath.Join(destDir, item.Name)
		clean := filepath.Clean(destPath)
		destClean := filepath.Clean(destDir)
		if !strings.HasPrefix(clean, destClean+string(filepath.Separator)) && clean != destClean {
			t.Errorf("Join(destDir, %q) = %q escapes destDir %q", item.Name, destPath, destDir)
		}
	}
}

func TestRestoreAgent_EntryNameFromListNeverEscapes(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)
	destDir := t.TempDir()

	os.MkdirAll(filepath.Join(base, "skill_2026-01-01_10-00-00"), 0755)
	os.MkdirAll(filepath.Join(base, "org", "repo_2026-01-02_10-00-00"), 0755)

	items := List(base)
	for _, item := range items {
		subDir := filepath.Dir(item.Name)
		targetDir := filepath.Join(destDir, subDir)
		clean := filepath.Clean(targetDir)
		destClean := filepath.Clean(destDir)
		if !strings.HasPrefix(clean, destClean+string(filepath.Separator)) && clean != destClean {
			t.Errorf("RestoreAgent targetDir %q escapes destDir %q for entry.Name %q",
				targetDir, destDir, item.Name)
		}
	}
}

func TestFindByName_ResultPathUnderBase(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)

	os.MkdirAll(filepath.Join(base, "my-skill_2026-01-01_10-00-00"), 0755)

	entry := FindByName(base, "my-skill")
	if entry == nil {
		t.Fatal("expected to find my-skill")
	}

	clean := filepath.Clean(entry.Path)
	baseClean := filepath.Clean(base)
	if !strings.HasPrefix(clean, baseClean+string(filepath.Separator)) && clean != baseClean {
		t.Errorf("FindByName path %q escapes base %q", entry.Path, base)
	}
}

func TestReadDir_NameNeverContainsSeparator(t *testing.T) {
	_, base, _, _ := setupTraversalTree(t)

	dir := filepath.Join(base, "entry_2026-01-01_10-00-00")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "normal.md"), []byte("ok"), 0644)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("ok"), 0644)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "/") || strings.Contains(e.Name(), "\\") {
			t.Errorf("ReadDir returned name with separator: %q", e.Name())
		}
		if strings.Contains(e.Name(), "..") {
			t.Errorf("ReadDir returned name with '..': %q", e.Name())
		}
	}
}

// --- Regression tests for PR #230 compatibility fixes ---

func TestRestoreAllowsCurrentDirectoryDestination(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	trashBase := filepath.Join(tmpDir, ".trash", "skills")
	trashDir := filepath.Join(trashBase, "demo_2006-01-02_15-04-05")
	os.MkdirAll(trashDir, 0755)
	os.WriteFile(filepath.Join(trashDir, "SKILL.md"), []byte("demo content"), 0644)

	entry := &TrashEntry{
		Name:      "demo",
		Path:      trashDir,
		Timestamp: "2006-01-02_15-04-05",
		Date:      time.Now(),
	}

	if err := Restore(entry, "."); err != nil {
		t.Fatalf("Restore to current directory failed: %v", err)
	}

	restored := filepath.Join(tmpDir, "demo", "SKILL.md")
	if _, err := os.Stat(restored); err != nil {
		t.Errorf("restored file not found: %v", err)
	}

	if _, err := os.Stat(trashDir); !os.IsNotExist(err) {
		t.Error("trash entry should be removed after restore")
	}
}

func TestEnsureUnderBaseAllowsChildOfDotButRejectsEscapes(t *testing.T) {
	if err := ensureUnderBase(".", "demo"); err != nil {
		t.Fatalf("ensureUnderBase(\".\", \"demo\") should succeed: %v", err)
	}
	if err := ensureUnderBase(".", "sub/dir"); err != nil {
		t.Fatalf("ensureUnderBase(\".\", \"sub/dir\") should succeed: %v", err)
	}
	if err := ensureUnderBase(".", "../outside"); err == nil {
		t.Error("ensureUnderBase(\".\", \"../outside\") should fail")
	}
	if err := ensureUnderBase(".", "../../etc/passwd"); err == nil {
		t.Error("ensureUnderBase(\".\", \"../../etc/passwd\") should fail")
	}
}

func TestRestoreNestedEntryReturnedByList(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(destDir, 0755)

	// Create nested trash entry: trash/org/demo_2006-01-02_15-04-05/SKILL.md
	trashDir := filepath.Join(trashBase, "org", "demo_2006-01-02_15-04-05")
	os.MkdirAll(trashDir, 0755)
	os.WriteFile(filepath.Join(trashDir, "SKILL.md"), []byte("nested content"), 0644)

	items := List(trashBase)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Name != "org/demo" {
		t.Errorf("expected Name 'org/demo', got %q", items[0].Name)
	}

	if err := Restore(&items[0], destDir); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	restored := filepath.Join(destDir, "org", "demo", "SKILL.md")
	if _, err := os.Stat(restored); err != nil {
		t.Errorf("restored file not found: %v", err)
	}
}

func TestTrashLogicalNameNormalizesOSNativeSeparators(t *testing.T) {
	parentRel := filepath.Join("org", "team")

	got := trashLogicalName(parentRel, "demo")
	want := "org/team/demo"

	if got != want {
		t.Fatalf("trashLogicalName(%q, %q) = %q, want %q", parentRel, "demo", got, want)
	}
}

func TestTrashLogicalNameDoesNotTranslateLiteralUnixBackslashes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("literal backslash is an OS separator on Windows")
	}

	got := trashLogicalName(`..\outside`, "demo")
	if got == "../outside/demo" {
		t.Fatalf("literal Unix backslash should not be translated into path separator: got %q", got)
	}
	if !strings.Contains(got, `\`) {
		t.Fatalf("literal Unix backslash should be preserved, got %q", got)
	}
}

func TestEnsureUnderBaseRejectsSiblingPrefix(t *testing.T) {
	root := t.TempDir()

	base := filepath.Join(root, "skills")
	sibling := filepath.Join(root, "skills-evil", "demo")

	if err := os.MkdirAll(filepath.Dir(sibling), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ensureUnderBase(base, sibling); err == nil {
		t.Fatal("sibling path with shared prefix should be rejected")
	}
}

func TestValidateTrashNameRejectsBackslash(t *testing.T) {
	if err := validateTrashName(`org\demo`); err == nil {
		t.Fatal("trash name with backslash should be rejected")
	}
}
