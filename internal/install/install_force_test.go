package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/audit"
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

func TestInstallFromDiscovery_ForceKeepsSkillWhenReinstallFails(t *testing.T) {
	url, _, _ := setupPinRemote(t)
	source, err := ParseSource(url)
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := DiscoverFromGit(source)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanupDiscovery(discovery)
	skill := discovery.Skills[0]
	sourceDir := t.TempDir()
	dest := filepath.Join(sourceDir, "demo")
	if _, err := InstallFromDiscovery(discovery, skill, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	// The copy fails when the skill is gone from the clone.
	if err := os.RemoveAll(filepath.Join(discoverySourceRoot(discovery), skill.Path)); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallFromDiscovery(discovery, skill, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true, Force: true}); err == nil {
		t.Fatal("expected the reinstall to fail")
	}

	if got, _ := os.ReadFile(filepath.Join(dest, "SKILL.md")); len(before) == 0 || string(got) != string(before) {
		t.Fatalf("SKILL.md = %q, want the installed %q kept", got, before)
	}
}

// A whole-repository install clones in place and audits the clone; when the
// audit cannot run, the restored skill must keep its own metadata.
func TestInstall_ForceWholeRepoScanErrorKeepsMetadata(t *testing.T) {
	repo := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("---\nname: whole\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v1")
	source, err := ParseSource(fileURL(repo))
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	dest := filepath.Join(sourceDir, "whole")
	if _, err := Install(source, dest, InstallOptions{SourceDir: sourceDir}); err != nil {
		t.Fatal(err)
	}
	installed := LoadMetadataOrNew(sourceDir).Get("whole").Version
	if err := os.WriteFile(filepath.Join(repo, "notes.md"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v2")
	cfgDir := t.TempDir()
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(cfgDir, "config.yaml"))
	if err := os.WriteFile(filepath.Join(cfgDir, "audit-rules.yaml"), []byte("rules:\n  - id: bad-regex\n    severity: HIGH\n    pattern: custom\n    message: bad\n    regex: '[invalid'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	audit.ResetGlobalCache()
	t.Cleanup(audit.ResetGlobalCache)
	if source, err = ParseSource(fileURL(repo)); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(source, dest, InstallOptions{SourceDir: sourceDir, Force: true, AuditOverride: true}); err == nil {
		t.Fatal("expected the scan error to fail the reinstall")
	}

	if got := LoadMetadataOrNew(sourceDir).Get("whole").Version; got != installed {
		t.Fatalf("metadata version = %q, want the restored install's %q", got, installed)
	}
}

// `install ./ -p --force` copies a folder that holds the skills source; the
// previous version set aside there must not be copied into the new one.
func TestInstall_ForceDoesNotCopyItsBackup(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(src, "installed")
	dest := filepath.Join(sourceDir, "demo")
	source := &Source{Type: SourceTypeLocalPath, Path: src, Name: "demo", Raw: src}
	if _, err := Install(source, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(source, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true, Force: true}); err != nil {
		t.Fatal(err)
	}

	if copied, _ := filepath.Glob(filepath.Join(dest, "installed", ".skillshare-demo.old.*")); len(copied) != 0 {
		t.Fatalf("backup copied into the new version: %v", copied)
	}
}
