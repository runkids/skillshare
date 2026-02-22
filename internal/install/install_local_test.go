package install

import (
	"os"
	"path/filepath"
	"testing"
)

func createLocalSkillSource(t *testing.T, dir, name string) string {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n# "+name), 0644)
	return skillDir
}

func TestInstall_LocalPath_Basic(t *testing.T) {
	tmp := t.TempDir()
	srcDir := createLocalSkillSource(t, tmp, "my-skill")
	destDir := filepath.Join(tmp, "dest", "my-skill")

	source := &Source{
		Type: SourceTypeLocalPath,
		Raw:  srcDir,
		Path: srcDir,
		Name: "my-skill",
	}

	result, err := Install(source, destDir, InstallOptions{SkipAudit: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "copied" {
		t.Errorf("expected action 'copied', got %q", result.Action)
	}
	if result.SkillName != "my-skill" {
		t.Errorf("expected skill name 'my-skill', got %q", result.SkillName)
	}

	// Verify SKILL.md was copied
	if _, err := os.Stat(filepath.Join(destDir, "SKILL.md")); err != nil {
		t.Error("expected SKILL.md to exist in destination")
	}

	// Verify metadata was written
	if !HasMeta(destDir) {
		t.Error("expected metadata to be written")
	}
}

func TestInstall_LocalPath_AlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	srcDir := createLocalSkillSource(t, tmp, "my-skill")
	destDir := filepath.Join(tmp, "dest", "my-skill")
	os.MkdirAll(destDir, 0755)

	source := &Source{
		Type: SourceTypeLocalPath,
		Raw:  srcDir,
		Path: srcDir,
		Name: "my-skill",
	}

	_, err := Install(source, destDir, InstallOptions{SkipAudit: true})
	if err == nil {
		t.Error("expected error when destination already exists")
	}
}

func TestInstall_LocalPath_Force(t *testing.T) {
	tmp := t.TempDir()
	srcDir := createLocalSkillSource(t, tmp, "my-skill")
	destDir := filepath.Join(tmp, "dest", "my-skill")
	os.MkdirAll(destDir, 0755)
	os.WriteFile(filepath.Join(destDir, "old-file.txt"), []byte("old"), 0644)

	source := &Source{
		Type: SourceTypeLocalPath,
		Raw:  srcDir,
		Path: srcDir,
		Name: "my-skill",
	}

	result, err := Install(source, destDir, InstallOptions{Force: true, SkipAudit: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "copied" {
		t.Errorf("expected action 'copied', got %q", result.Action)
	}

	// Old file should be gone
	if _, err := os.Stat(filepath.Join(destDir, "old-file.txt")); !os.IsNotExist(err) {
		t.Error("expected old file to be removed after force install")
	}
}

func TestInstall_LocalPath_DryRun(t *testing.T) {
	tmp := t.TempDir()
	srcDir := createLocalSkillSource(t, tmp, "my-skill")
	destDir := filepath.Join(tmp, "dest", "my-skill")

	source := &Source{
		Type: SourceTypeLocalPath,
		Raw:  srcDir,
		Path: srcDir,
		Name: "my-skill",
	}

	result, err := Install(source, destDir, InstallOptions{DryRun: true, SkipAudit: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "would copy" {
		t.Errorf("expected action 'would copy', got %q", result.Action)
	}

	// Destination should NOT exist
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		t.Error("expected destination to not exist in dry-run mode")
	}
}

func TestInstall_LocalPath_NonExistent(t *testing.T) {
	tmp := t.TempDir()
	destDir := filepath.Join(tmp, "dest", "my-skill")

	source := &Source{
		Type: SourceTypeLocalPath,
		Raw:  "/nonexistent/source",
		Path: "/nonexistent/source",
		Name: "my-skill",
	}

	_, err := Install(source, destDir, InstallOptions{SkipAudit: true})
	if err == nil {
		t.Error("expected error for non-existent source")
	}
}

func TestInstall_LocalPath_WritesFileHashes(t *testing.T) {
	tmp := t.TempDir()
	srcDir := createLocalSkillSource(t, tmp, "my-skill")
	// Add an extra file
	os.WriteFile(filepath.Join(srcDir, "helpers.sh"), []byte("echo hi"), 0644)
	destDir := filepath.Join(tmp, "dest", "my-skill")

	source := &Source{
		Type: SourceTypeLocalPath,
		Raw:  srcDir,
		Path: srcDir,
		Name: "my-skill",
	}

	if _, err := Install(source, destDir, InstallOptions{SkipAudit: true}); err != nil {
		t.Fatal(err)
	}

	meta, err := ReadMeta(destDir)
	if err != nil {
		t.Fatal(err)
	}
	if meta == nil {
		t.Fatal("expected meta to exist")
	}
	if len(meta.FileHashes) < 2 {
		t.Errorf("expected at least 2 file hashes (SKILL.md + helpers.sh), got %d", len(meta.FileHashes))
	}
	for _, hash := range meta.FileHashes {
		if len(hash) < 7 || hash[:7] != "sha256:" {
			t.Errorf("expected sha256: prefixed hash, got %q", hash)
		}
	}
}

func TestInstall_LocalPath_NoSKILLMD(t *testing.T) {
	tmp := t.TempDir()
	// Create a source without SKILL.md
	srcDir := filepath.Join(tmp, "no-skill")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("readme"), 0644)
	destDir := filepath.Join(tmp, "dest", "no-skill")

	source := &Source{
		Type: SourceTypeLocalPath,
		Raw:  srcDir,
		Path: srcDir,
		Name: "no-skill",
	}

	result, err := Install(source, destDir, InstallOptions{SkipAudit: true})
	if err != nil {
		t.Fatal(err)
	}
	// Should have a warning about missing SKILL.md
	hasWarning := false
	for _, w := range result.Warnings {
		if w == "no SKILL.md found in skill directory" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("expected warning about missing SKILL.md, got warnings: %v", result.Warnings)
	}
}

func TestInstall_LocalPath_WithAudit(t *testing.T) {
	tmp := t.TempDir()
	srcDir := createLocalSkillSource(t, tmp, "audited-skill")
	destDir := filepath.Join(tmp, "dest", "audited-skill")

	source := &Source{
		Type: SourceTypeLocalPath,
		Raw:  srcDir,
		Path: srcDir,
		Name: "audited-skill",
	}

	// Install WITHOUT SkipAudit — audit runs on clean skill
	result, err := Install(source, destDir, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.AuditSkipped {
		t.Error("expected audit to run")
	}
	if result.AuditThreshold == "" {
		t.Error("expected audit threshold to be set")
	}
}

func TestInstall_LocalPath_AuditSkipped(t *testing.T) {
	tmp := t.TempDir()
	srcDir := createLocalSkillSource(t, tmp, "skip-audit")
	destDir := filepath.Join(tmp, "dest", "skip-audit")

	source := &Source{
		Type: SourceTypeLocalPath,
		Raw:  srcDir,
		Path: srcDir,
		Name: "skip-audit",
	}

	result, err := Install(source, destDir, InstallOptions{SkipAudit: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.AuditSkipped {
		t.Error("expected audit to be skipped")
	}
}

func TestIsGitRepo_True(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	if !IsGitRepo(dir) {
		t.Error("expected IsGitRepo true for dir with .git")
	}
}

func TestIsGitRepo_False(t *testing.T) {
	dir := t.TempDir()
	if IsGitRepo(dir) {
		t.Error("expected IsGitRepo false for dir without .git")
	}
}

func TestCheckSkillFile_Present(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill"), 0644)
	result := &InstallResult{}
	checkSkillFile(dir, result)
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings when SKILL.md present, got %v", result.Warnings)
	}
}

func TestCheckSkillFile_Missing(t *testing.T) {
	dir := t.TempDir()
	result := &InstallResult{}
	checkSkillFile(dir, result)
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning for missing SKILL.md, got %d", len(result.Warnings))
	}
}
