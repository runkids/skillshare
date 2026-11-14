//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"skillshare/internal/testutil"
)

func TestRestore_NoBackup_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("restore", "claude")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "no backup found")
}

func TestRestore_TargetNotInConfig_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("restore", "nonexistent")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestRestore_NoArgs_ListsBackups(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Non-TTY: falls back to backup list (no error)
	result := sb.RunCLI("restore")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No backups found")
}

func TestRestore_NoTUI_Flag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("restore", "--no-tui")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No backups found")
}

func TestRestore_FromLatest_RestoresBackup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	// Create backup directory manually
	backupDir := filepath.Join(sb.Home, ".local", "share", "skillshare", "backups")
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, timestamp, "claude")
	os.MkdirAll(backupPath, 0755)

	// Create a skill in backup
	skillPath := filepath.Join(backupPath, "backed-up-skill")
	os.MkdirAll(skillPath, 0755)
	os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Backed Up"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("restore", "claude")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Restored")

	// Verify skill was restored
	restoredSkillPath := filepath.Join(targetPath, "backed-up-skill", "SKILL.md")
	if !sb.FileExists(restoredSkillPath) {
		t.Error("skill should be restored from backup")
	}
}

func TestRestore_FromTimestamp_RestoresSpecific(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	// Create backup directory manually with specific timestamp
	backupDir := filepath.Join(sb.Home, ".local", "share", "skillshare", "backups")
	timestamp := "20240101-120000"
	backupPath := filepath.Join(backupDir, timestamp, "claude")
	os.MkdirAll(backupPath, 0755)

	// Create a skill in backup
	skillPath := filepath.Join(backupPath, "specific-skill")
	os.MkdirAll(skillPath, 0755)
	os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Specific"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("restore", "claude", "--from", timestamp)

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Restored")
	result.AssertOutputContains(t, timestamp)
}

func TestRestore_OverwritesSymlink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create target as symlink
	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.Symlink(sb.SourcePath, targetPath)

	// Create backup
	backupDir := filepath.Join(sb.Home, ".local", "share", "skillshare", "backups")
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, timestamp, "claude")
	os.MkdirAll(backupPath, 0755)

	skillPath := filepath.Join(backupPath, "restored-skill")
	os.MkdirAll(skillPath, 0755)
	os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Restored"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("restore", "claude")

	result.AssertSuccess(t)

	// Verify target is no longer a symlink
	if sb.IsSymlink(targetPath) {
		t.Error("target should no longer be a symlink after restore")
	}
}

func TestRestore_DryRun_DoesNotRestore(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	backupDir := filepath.Join(sb.Home, ".local", "share", "skillshare", "backups")
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, timestamp, "claude")
	os.MkdirAll(backupPath, 0755)

	skillPath := filepath.Join(backupPath, "dry-run-skill")
	os.MkdirAll(skillPath, 0755)
	os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Dry Run"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("restore", "claude", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Dry run")

	restoredSkillPath := filepath.Join(targetPath, "dry-run-skill", "SKILL.md")
	if sb.FileExists(restoredSkillPath) {
		t.Error("dry-run should not restore files")
	}
}

func TestRestore_ProjectSkills(t *testing.T) {
	for _, moved := range []bool{false, true} {
		t.Run(map[bool]string{false: "current", true: "moved"}[moved], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
				sb.SetEnv(key, "")
			}
			root := filepath.Join(sb.Root, "project")
			sb.WriteProjectConfig(root, "targets:\n  - goose\n")
			entry, rel := "goose", ".agents/skills"
			if moved {
				entry, rel = "goose-moved-2e676f6f73652f736b696c6c73", ".goose/skills"
			}
			stamp := "2024-01-15_14-30-45"
			saved := filepath.Join(root, ".skillshare", "backups", stamp, entry)
			if err := os.MkdirAll(saved, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(saved, "saved.md"), []byte("authored"), 0644); err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(root, filepath.FromSlash(rel))
			if moved {
				current := filepath.Join(root, ".agents", "skills")
				os.MkdirAll(current, 0755)
				os.WriteFile(filepath.Join(current, "keep.md"), []byte("current"), 0644)
			}
			for _, from := range [][]string{nil, {"--from", stamp}} {
				args := append([]string{"restore", entry, "-p", "--dry-run"}, from...)
				sb.RunCLIInDir(root, args...).AssertSuccess(t)
				if _, err := os.Stat(dest); !os.IsNotExist(err) {
					t.Fatalf("preview created destination: %v", err)
				}
			}
			sb.RunCLIInDir(root, "restore", entry, "-p", "--from", stamp).AssertSuccess(t)
			if data, err := os.ReadFile(filepath.Join(dest, "saved.md")); err != nil || string(data) != "authored" {
				t.Fatalf("restored bytes = %q, %v", data, err)
			}
			if err := os.WriteFile(filepath.Join(dest, "saved.md"), []byte("changed"), 0644); err != nil {
				t.Fatal(err)
			}
			sb.RunCLIInDir(root, "restore", entry, "-p").AssertFailure(t)
			if data, err := os.ReadFile(filepath.Join(dest, "saved.md")); err != nil || string(data) != "changed" {
				t.Fatalf("refused restore changed bytes: %q, %v", data, err)
			}
			sb.RunCLIInDir(root, "restore", entry, "-p", "--force").AssertSuccess(t)
			if data, err := os.ReadFile(filepath.Join(dest, "saved.md")); err != nil || string(data) != "authored" {
				t.Fatalf("forced latest restore = %q, %v", data, err)
			}
			if moved {
				if data, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "keep.md")); err != nil || string(data) != "current" {
					t.Fatalf("current directory changed: %q, %v", data, err)
				}
			}
			sb.RunCLIInDir(root, "restore", "-p").AssertFailure(t)
			sb.RunCLIInDir(root, "restore", "missing", "-p").AssertFailure(t)
			sb.RunCLIInDir(root, "restore", entry, "-p", "--from", "2024-01-16_14-30-45", "--force").AssertFailure(t)
		})
	}
}

func TestRestore_ProjectCwdKeepsGlobalDefault(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
		sb.SetEnv(key, "")
	}
	root := filepath.Join(sb.Root, "project")
	sb.WriteProjectConfig(root, "targets:\n  - goose\n")
	dest := sb.CreateTarget("global-goose")
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  goose:\n    path: " + dest + "\n")
	stamp := "2024-01-15_14-30-45"
	saved := filepath.Join(sb.Home, ".local", "share", "skillshare", "backups", stamp, "goose")
	os.MkdirAll(saved, 0755)
	os.WriteFile(filepath.Join(saved, "saved.md"), []byte("global"), 0644)
	for _, flags := range [][]string{nil, {"-g"}, {"--all", "-p"}} {
		args := append([]string{"restore", "goose", "--force"}, flags...)
		sb.RunCLIInDir(root, args...).AssertSuccess(t)
		if data, err := os.ReadFile(filepath.Join(dest, "saved.md")); err != nil || string(data) != "global" {
			t.Fatalf("global restore = %q, %v", data, err)
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills")); !os.IsNotExist(err) {
			t.Fatalf("global restore touched project: %v", err)
		}
	}
}

func TestRestore_ProjectReadOnlyResolution(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
		sb.SetEnv(key, "")
	}
	root := filepath.Join(sb.Root, "project")
	raw := "# keep legacy config\ntargets:\n  - name: custom\n    path: target\n"
	sb.WriteProjectConfig(root, raw)
	configPath := filepath.Join(root, ".skillshare", "config.yaml")
	for _, args := range [][]string{
		{"restore", "custom", "-p", "--dry-run"},
		{"restore", "missing", "-p"},
		{"restore", "custom", "-p"},
	} {
		sb.RunCLIInDir(root, args...).AssertFailure(t)
		if data, err := os.ReadFile(configPath); err != nil || string(data) != raw {
			t.Fatalf("recovery rewrote configuration: %q, %v", data, err)
		}
		for _, rel := range []string{"target", ".skillshare/backups", ".skillshare/skills"} {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
				t.Fatalf("failed recovery initialized %s: %v", rel, err)
			}
		}
	}
}
