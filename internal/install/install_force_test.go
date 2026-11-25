package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forceInstall installs a local skill at sourceDir/demo, then reinstalls it
// with --force from src.
func forceInstall(t *testing.T, src string) (string, error) {
	t.Helper()
	first := t.TempDir()
	if err := os.WriteFile(filepath.Join(first, "SKILL.md"), []byte("---\nname: demo\n---\nold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	dest := filepath.Join(sourceDir, "demo")
	if _, err := Install(&Source{Type: SourceTypeLocalPath, Path: first, Name: "demo", Raw: first}, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}
	_, err := Install(&Source{Type: SourceTypeLocalPath, Path: src, Name: "demo", Raw: src}, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true, Force: true})
	return dest, err
}

func TestInstall_ForceKeepsSkillWhenReinstallFails(t *testing.T) {
	dest, err := forceInstall(t, filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected the reinstall from a missing folder to fail")
	}

	if got, _ := os.ReadFile(filepath.Join(dest, "SKILL.md")); !strings.Contains(string(got), "old") {
		t.Fatalf("SKILL.md = %q, want the installed skill kept", got)
	}
}

func TestInstall_ForceReplacesSkillWithoutLeftovers(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: demo\n---\nnew\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dest, err := forceInstall(t, src)
	if err != nil {
		t.Fatal(err)
	}

	if got, _ := os.ReadFile(filepath.Join(dest, "SKILL.md")); !strings.Contains(string(got), "new") {
		t.Fatalf("SKILL.md = %q, want the new version", got)
	}
	if old, _ := filepath.Glob(filepath.Join(filepath.Dir(dest), ".skillshare-demo.old.*")); len(old) != 0 {
		t.Fatalf("previous version left at %v", old)
	}
}
