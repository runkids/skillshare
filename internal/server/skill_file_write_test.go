package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteSkillFileRejectsTraversal(t *testing.T) {
	t.Run("rejects path traversal", func(t *testing.T) {
		skillDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("seed"), 0644); err != nil {
			t.Fatalf("failed to seed SKILL.md: %v", err)
		}

		_, err := writeSkillFile(skillDir, "../SKILL.md", []byte("updated"))
		if err == nil {
			t.Fatal("expected traversal write to fail")
		}
		if !strings.Contains(err.Error(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %v", err)
		}
	})

	t.Run("rejects symlink markdown path", func(t *testing.T) {
		skillDir := t.TempDir()
		targetPath := filepath.Join(t.TempDir(), "outside.md")
		if err := os.WriteFile(targetPath, []byte("seed"), 0644); err != nil {
			t.Fatalf("failed to seed target markdown file: %v", err)
		}

		linkPath := filepath.Join(skillDir, "linked.md")
		if err := os.Symlink(targetPath, linkPath); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		_, err := writeSkillFile(skillDir, "linked.md", []byte("updated"))
		if err == nil {
			t.Fatal("expected symlink markdown write to fail")
		}
		if !strings.Contains(err.Error(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %v", err)
		}
	})

	t.Run("rejects path through symlinked parent directory", func(t *testing.T) {
		skillDir := t.TempDir()
		outsideDir := t.TempDir()
		targetPath := filepath.Join(outsideDir, "out.md")
		if err := os.WriteFile(targetPath, []byte("seed"), 0644); err != nil {
			t.Fatalf("failed to seed target markdown file: %v", err)
		}

		linkDir := filepath.Join(skillDir, "linkdir")
		if err := os.Symlink(outsideDir, linkDir); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		_, err := writeSkillFile(skillDir, filepath.Join("linkdir", "out.md"), []byte("updated"))
		if err == nil {
			t.Fatal("expected symlinked parent path write to fail")
		}
		if !strings.Contains(err.Error(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %v", err)
		}
	})

	t.Run("rejects missing file through symlinked parent directory", func(t *testing.T) {
		skillDir := t.TempDir()
		outsideDir := t.TempDir()

		linkDir := filepath.Join(skillDir, "linkdir")
		if err := os.Symlink(outsideDir, linkDir); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		_, err := writeSkillFile(skillDir, filepath.Join("linkdir", "missing.md"), []byte("updated"))
		if err == nil {
			t.Fatal("expected missing file through symlinked parent path write to fail")
		}
		if !strings.Contains(err.Error(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %v", err)
		}
	})

	t.Run("rejects non-regular markdown path", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("mkfifo test is not supported on windows")
		}

		skillDir := t.TempDir()
		fifoPath := filepath.Join(skillDir, "pipe.md")
		if _, err := exec.LookPath("mkfifo"); err != nil {
			t.Skipf("mkfifo not available in this environment: %v", err)
		}
		if err := exec.Command("mkfifo", fifoPath).Run(); err != nil {
			t.Skipf("mkfifo not supported in this environment: %v", err)
		}

		_, err := writeSkillFile(skillDir, "pipe.md", []byte("updated"))
		if err == nil {
			t.Fatal("expected non-regular markdown write to fail")
		}
		if !strings.Contains(err.Error(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %v", err)
		}
	})
}

func TestWriteSkillFileAllowsDoubleDotsInFilename(t *testing.T) {
	skillDir := t.TempDir()
	notesDir := filepath.Join(skillDir, "notes")
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		t.Fatalf("failed to create notes directory: %v", err)
	}

	relPath := filepath.Join("notes", "v1..draft.md")
	absPath := filepath.Join(skillDir, relPath)
	if err := os.WriteFile(absPath, []byte("seed"), 0644); err != nil {
		t.Fatalf("failed to seed markdown file: %v", err)
	}

	filename, err := writeSkillFile(skillDir, relPath, []byte("updated"))
	if err != nil {
		t.Fatalf("expected dotted filename to be writable, got error: %v", err)
	}
	if filename != "v1..draft.md" {
		t.Fatalf("filename = %q, want %q", filename, "v1..draft.md")
	}

	got, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}
	if string(got) != "updated" {
		t.Fatalf("updated file content = %q, want %q", string(got), "updated")
	}
}
