package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	managedhooks "skillshare/internal/resources/hooks"
	managedrules "skillshare/internal/resources/rules"
	"skillshare/internal/ui"
)

func TestCmdSync_DefaultStillSyncsSkillsOnly(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	sourceDir := filepath.Join(t.TempDir(), "source")
	mustAddSkill(t, sourceDir, "alpha")

	cfg := &config.Config{
		Source: sourceDir,
		Targets: map[string]config.TargetConfig{
			"claude": {Path: filepath.Join(home, ".claude", "skills")},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	putManagedRule(t, "", "claude/manual.md", "# Managed rule\n")
	putManagedHook(t, "", "claude/pre-tool-use/bash.yaml", "./bin/check")

	resetModeLabel(t)
	if err := cmdSync(nil); err != nil {
		t.Fatalf("cmdSync() error = %v", err)
	}

	mustExist(t, filepath.Join(home, ".claude", "skills", "alpha"))
	mustNotExist(t, filepath.Join(home, ".claude", "rules", "manual.md"))
	mustNotExist(t, filepath.Join(home, ".claude", "settings.json"))
}

func TestCmdSync_AllIncludesManagedResources(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	sourceDir := filepath.Join(t.TempDir(), "source")
	mustAddSkill(t, sourceDir, "alpha")

	cfg := &config.Config{
		Source: sourceDir,
		Targets: map[string]config.TargetConfig{
			"claude": {Path: filepath.Join(home, ".claude", "skills")},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	putManagedRule(t, "", "claude/manual.md", "# Managed rule\n")
	putManagedHook(t, "", "claude/pre-tool-use/bash.yaml", "./bin/check")

	resetModeLabel(t)
	if err := cmdSync([]string{"--all"}); err != nil {
		t.Fatalf("cmdSync(--all) error = %v", err)
	}

	mustExist(t, filepath.Join(home, ".claude", "skills", "alpha"))
	mustExist(t, filepath.Join(home, ".claude", "rules", "manual.md"))
	mustExist(t, filepath.Join(home, ".claude", "settings.json"))
}

func TestCmdSync_AllRendersManagedResourceOutput(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	sourceDir := filepath.Join(t.TempDir(), "source")
	mustAddSkill(t, sourceDir, "alpha")

	cfg := &config.Config{
		Source: sourceDir,
		Targets: map[string]config.TargetConfig{
			"claude": {Path: filepath.Join(home, ".claude", "skills")},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	putManagedRule(t, "", "claude/manual.md", "# Managed rule\n")
	putManagedHook(t, "", "claude/pre-tool-use/bash.yaml", "./bin/check")

	resetModeLabel(t)
	output := captureStdout(t, func() {
		if err := cmdSync([]string{"--all"}); err != nil {
			t.Fatalf("cmdSync(--all) error = %v", err)
		}
	})

	if !strings.Contains(output, "rules: 1 updated") {
		t.Fatalf("combined sync output missing rules detail:\n%s", output)
	}
	if !strings.Contains(output, "hooks: 1 updated") {
		t.Fatalf("combined sync output missing hooks detail:\n%s", output)
	}
}

func TestCmdSync_ManagedOnlyDoesNotRequireSkillsSource(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)

	cfg := &config.Config{
		Source: filepath.Join(t.TempDir(), "missing-source"),
		Targets: map[string]config.TargetConfig{
			"claude": {Path: filepath.Join(home, ".claude", "skills")},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	putManagedRule(t, "", "claude/manual.md", "# Managed rule\n")
	putManagedHook(t, "", "claude/pre-tool-use/bash.yaml", "./bin/check")

	resetModeLabel(t)
	if err := cmdSync([]string{"--resources", "rules,hooks"}); err != nil {
		t.Fatalf("cmdSync(--resources rules,hooks) error = %v", err)
	}

	mustExist(t, filepath.Join(home, ".claude", "rules", "manual.md"))
	mustExist(t, filepath.Join(home, ".claude", "settings.json"))
}

func TestCmdSync_ManagedRulesFailureStillAttemptsHooks(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	cfg := &config.Config{
		Source: filepath.Join(t.TempDir(), "unused-source"),
		Targets: map[string]config.TargetConfig{
			"claude": {Path: filepath.Join(home, ".claude", "skills")},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	putManagedRule(t, "", "claude/manual.md", "# Managed rule\n")
	putManagedHook(t, "", "claude/pre-tool-use/bash.yaml", "./bin/check")
	mustWriteFile(t, filepath.Join(home, ".claude", "rules"), "not-a-directory")

	resetModeLabel(t)
	var syncErr error
	output := captureStdout(t, func() {
		syncErr = cmdSync([]string{"--resources", "rules,hooks"})
	})
	if syncErr == nil {
		t.Fatal("expected cmdSync(--resources rules,hooks) to report partial failure")
	}

	mustExist(t, filepath.Join(home, ".claude", "settings.json"))
	if !strings.Contains(output, "hooks: 1 updated") {
		t.Fatalf("partial managed sync output missing hooks detail:\n%s", output)
	}
	if !strings.Contains(output, "apply managed rules") {
		t.Fatalf("partial managed sync output missing rules failure:\n%s", output)
	}
}

func TestCmdCollect_ManagedDryRunReturnsPartialFailure(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	mustWriteFile(t, filepath.Join(home, ".claude", "CLAUDE.md"), "# Root rule\n")
	mustWriteFile(t, filepath.Join(home, ".claude", "rules", "CLAUDE.md"), "# Nested rule\n")
	mustWriteFile(t, filepath.Join(home, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./bin/check"}]}]}}`)

	resetModeLabel(t)
	var collectErr error
	output := captureStdout(t, func() {
		collectErr = cmdCollect([]string{"--resources", "rules,hooks", "--dry-run"})
	})
	if collectErr == nil {
		t.Fatal("expected cmdCollect(--resources rules,hooks --dry-run) to report partial failure")
	}
	if !strings.Contains(output, "would collect") {
		t.Fatalf("dry-run collect output missing planned hook/rule details:\n%s", output)
	}
}

func TestCmdSync_SkillsFailureStillAttemptsManagedResources(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	sourceDir := filepath.Join(t.TempDir(), "source")
	mustAddSkill(t, sourceDir, "alpha")

	cfg := &config.Config{
		Source: sourceDir,
		Targets: map[string]config.TargetConfig{
			"claude": {Path: filepath.Join(home, ".claude", "skills")},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	putManagedRule(t, "", "claude/manual.md", "# Managed rule\n")
	putManagedHook(t, "", "claude/pre-tool-use/bash.yaml", "./bin/check")
	mustWriteFile(t, filepath.Join(home, ".claude", "skills"), "not-a-directory")

	resetModeLabel(t)
	var syncErr error
	output := captureStdout(t, func() {
		syncErr = cmdSync([]string{"--all"})
	})
	if syncErr == nil {
		t.Fatal("expected cmdSync(--all) to report partial failure")
	}

	mustExist(t, filepath.Join(home, ".claude", "rules", "manual.md"))
	mustExist(t, filepath.Join(home, ".claude", "settings.json"))
	if !strings.Contains(output, "rules: 1 updated") {
		t.Fatalf("combined sync output missing rules detail after skills failure:\n%s", output)
	}
	if !strings.Contains(output, "hooks: 1 updated") {
		t.Fatalf("combined sync output missing hooks detail after skills failure:\n%s", output)
	}
}

func TestCmdSync_ManagedOnlyCreatesBackupBeforeMutating(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	sourceDir := filepath.Join(t.TempDir(), "source")
	mustAddSkill(t, sourceDir, "alpha")

	cfg := &config.Config{
		Source: sourceDir,
		Targets: map[string]config.TargetConfig{
			"claude": {Path: filepath.Join(home, ".claude", "skills")},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	mustAddSkill(t, filepath.Join(home, ".claude", "skills"), "local")
	putManagedRule(t, "", "claude/manual.md", "# Managed rule\n")
	putManagedHook(t, "", "claude/pre-tool-use/bash.yaml", "./bin/check")

	resetModeLabel(t)
	if err := cmdSync([]string{"--resources", "rules,hooks"}); err != nil {
		t.Fatalf("cmdSync(--resources rules,hooks) error = %v", err)
	}

	if _, err := os.Stat(backup.BackupDir()); err != nil {
		t.Fatalf("expected sync backup directory to exist: %v", err)
	}
}

func TestSyncBackupPathsForTarget_RebasesToSafeToolRoot(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	sourceDir := filepath.Join(t.TempDir(), "source")
	mustAddSkill(t, sourceDir, "alpha")

	targetPath := filepath.Join(home, ".claude", "skills")
	cfg := &config.Config{
		Source: sourceDir,
		Targets: map[string]config.TargetConfig{
			"claude": {Path: targetPath},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	mustAddSkill(t, targetPath, "local")
	mustWriteFile(t, filepath.Join(targetPath, ".skillshare-manifest.json"), `{"managed":{"local":"abc123"}}`)
	mustWriteFile(t, filepath.Join(home, ".claude", "rules", "manual.md"), "# Old managed rule\n")
	mustWriteFile(t, filepath.Join(home, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[]}}`)

	putManagedRule(t, "", "claude/manual.md", "# New managed rule\n")
	putManagedHook(t, "", "claude/pre-tool-use/bash.yaml", "./bin/check")

	paths, errs := syncBackupPathsForTarget(syncTargetEntry{
		name:   "claude",
		target: config.TargetConfig{Path: targetPath},
	}, resourceSelection{skills: true, rules: true, hooks: true})
	if len(errs) != 0 {
		t.Fatalf("syncBackupPathsForTarget errors = %v", errs)
	}
	if len(paths) == 0 {
		t.Fatal("expected snapshot paths")
	}

	got := make([]string, 0, len(paths))
	for _, path := range paths {
		got = append(got, path.RelativePath)
		if strings.Contains(path.RelativePath, "..") {
			t.Fatalf("relative path %q should not escape backup base", path.RelativePath)
		}
	}

	if !containsString(got, "skills") {
		t.Fatalf("snapshot paths %v missing skills entry", got)
	}
	if !containsString(got, "rules") {
		t.Fatalf("snapshot paths %v missing rules entry", got)
	}
	if !containsString(got, "settings.json") {
		t.Fatalf("snapshot paths %v missing settings entry", got)
	}
}

func TestSyncBackupPathsForTarget_UniversalHooksUseSharedHomeAncestor(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	targetPath := filepath.Join(home, ".agents", "skills")

	mustWriteFile(t, filepath.Join(home, ".codex", "config.toml"), "[features]\ncodex_hooks = true\n")
	mustWriteFile(t, filepath.Join(home, ".codex", "hooks.json"), `{"hooks":{"PreToolUse":[]}}`)
	putManagedHookForTool(t, "", "codex/pre-tool-use/bash.yaml", "codex", "PreToolUse", "Bash", "./bin/check")

	paths, errs := syncBackupPathsForTarget(syncTargetEntry{
		name:   "universal",
		target: config.TargetConfig{Path: targetPath},
	}, resourceSelection{hooks: true})
	if len(errs) != 0 {
		t.Fatalf("syncBackupPathsForTarget errors = %v", errs)
	}
	if len(paths) == 0 {
		t.Fatal("expected snapshot paths")
	}

	got := make([]string, 0, len(paths))
	for _, path := range paths {
		got = append(got, path.RelativePath)
		if strings.Contains(path.RelativePath, "..") {
			t.Fatalf("relative path %q should stay within the shared ancestor", path.RelativePath)
		}
	}

	if !containsString(got, filepath.Join(".codex", "config.toml")) {
		t.Fatalf("snapshot paths %v missing codex config", got)
	}
	if !containsString(got, filepath.Join(".codex", "hooks.json")) {
		t.Fatalf("snapshot paths %v missing codex hooks", got)
	}
}

func TestCmdSync_ManagedOnlyBackupRestoreRoundTripRestoresManagedFiles(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	sourceDir := filepath.Join(t.TempDir(), "source")
	mustAddSkill(t, sourceDir, "alpha")

	targetPath := filepath.Join(home, ".claude", "skills")
	cfg := &config.Config{
		Source: sourceDir,
		Targets: map[string]config.TargetConfig{
			"claude": {Path: targetPath},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	mustAddSkill(t, targetPath, "local")
	originalRule := "# Old managed rule\n"
	originalSettings := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./bin/original"}]}]}}`
	mustWriteFile(t, filepath.Join(home, ".claude", "rules", "manual.md"), originalRule)
	mustWriteFile(t, filepath.Join(home, ".claude", "settings.json"), originalSettings)

	putManagedRule(t, "", "claude/manual.md", "# New managed rule\n")
	putManagedHook(t, "", "claude/pre-tool-use/bash.yaml", "./bin/check")

	resetModeLabel(t)
	if err := cmdSync([]string{"--resources", "rules,hooks"}); err != nil {
		t.Fatalf("cmdSync(--resources rules,hooks) error = %v", err)
	}

	backups, err := backup.FindBackupsForTarget("claude")
	if err != nil {
		t.Fatalf("FindBackupsForTarget(claude) error = %v", err)
	}
	if len(backups) == 0 {
		t.Fatal("expected sync-created backup for claude")
	}

	mustWriteFile(t, filepath.Join(home, ".claude", "rules", "manual.md"), "# Post-sync mutation\n")
	mustWriteFile(t, filepath.Join(home, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./bin/mutated"}]}]}}`)
	mustAddSkill(t, targetPath, "keep-me")

	if err := backup.RestoreToPath(backups[0].Path, "claude", targetPath, backup.RestoreOptions{Force: true}); err != nil {
		t.Fatalf("RestoreToPath(managed backup) error = %v", err)
	}

	assertFileContent(t, filepath.Join(home, ".claude", "rules", "manual.md"), originalRule)
	assertFileContent(t, filepath.Join(home, ".claude", "settings.json"), originalSettings)
	mustExist(t, filepath.Join(targetPath, "keep-me", "SKILL.md"))
}

func TestCmdSync_ManagedOnlyBackupRestoreRoundTripRestoresAbsence(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	sourceDir := filepath.Join(t.TempDir(), "source")
	mustAddSkill(t, sourceDir, "alpha")

	targetPath := filepath.Join(home, ".claude", "skills")
	cfg := &config.Config{
		Source: sourceDir,
		Targets: map[string]config.TargetConfig{
			"claude": {Path: targetPath},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	putManagedRule(t, "", "claude/manual.md", "# Managed rule\n")
	putManagedHook(t, "", "claude/pre-tool-use/bash.yaml", "./bin/check")

	resetModeLabel(t)
	if err := cmdSync([]string{"--resources", "rules,hooks"}); err != nil {
		t.Fatalf("cmdSync(--resources rules,hooks) error = %v", err)
	}

	backups, err := backup.FindBackupsForTarget("claude")
	if err != nil {
		t.Fatalf("FindBackupsForTarget(claude) error = %v", err)
	}
	if len(backups) == 0 {
		t.Fatal("expected sync-created backup for claude")
	}

	mustExist(t, filepath.Join(home, ".claude", "rules", "manual.md"))
	mustExist(t, filepath.Join(home, ".claude", "settings.json"))

	if err := backup.RestoreToPath(backups[0].Path, "claude", targetPath, backup.RestoreOptions{Force: true}); err != nil {
		t.Fatalf("RestoreToPath(absence backup) error = %v", err)
	}

	mustNotExist(t, filepath.Join(home, ".claude", "rules", "manual.md"))
	mustNotExist(t, filepath.Join(home, ".claude", "settings.json"))
}

func TestCmdSync_AllAttemptsManagedResourcesWhenGlobalSourceMissing(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	cfg := &config.Config{
		Source: filepath.Join(t.TempDir(), "missing-source"),
		Targets: map[string]config.TargetConfig{
			"claude": {Path: filepath.Join(home, ".claude", "skills")},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	putManagedRule(t, "", "claude/manual.md", "# Managed rule\n")
	putManagedHook(t, "", "claude/pre-tool-use/bash.yaml", "./bin/check")

	resetModeLabel(t)
	var syncErr error
	output := captureStdout(t, func() {
		syncErr = cmdSync([]string{"--all"})
	})
	if syncErr == nil {
		t.Fatal("expected cmdSync(--all) to report partial failure")
	}

	mustExist(t, filepath.Join(home, ".claude", "rules", "manual.md"))
	mustExist(t, filepath.Join(home, ".claude", "settings.json"))
	if !strings.Contains(output, "source directory does not exist") {
		t.Fatalf("combined sync output missing source failure:\n%s", output)
	}
	if !strings.Contains(output, "rules: 1 updated") {
		t.Fatalf("combined sync output missing rules detail after source failure:\n%s", output)
	}
	if !strings.Contains(output, "hooks: 1 updated") {
		t.Fatalf("combined sync output missing hooks detail after source failure:\n%s", output)
	}
}

func TestCmdSync_AllCreatesBackupAfterSkillsSourceFailure(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	cfg := &config.Config{
		Source: filepath.Join(t.TempDir(), "missing-source"),
		Targets: map[string]config.TargetConfig{
			"claude": {Path: filepath.Join(home, ".claude", "skills")},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	mustAddSkill(t, filepath.Join(home, ".claude", "skills"), "local")
	putManagedRule(t, "", "claude/manual.md", "# Managed rule\n")
	putManagedHook(t, "", "claude/pre-tool-use/bash.yaml", "./bin/check")

	resetModeLabel(t)
	var syncErr error
	output := captureStdout(t, func() {
		syncErr = cmdSync([]string{"--all"})
	})
	if syncErr == nil {
		t.Fatal("expected cmdSync(--all) to report partial failure")
	}

	if _, err := os.Stat(backup.BackupDir()); err != nil {
		t.Fatalf("expected sync backup directory to exist after source failure: %v", err)
	}
	if !strings.Contains(output, "source directory does not exist") {
		t.Fatalf("combined sync output missing source failure:\n%s", output)
	}
}

func TestCmdSyncProject_ResourcesOnlyTargetsCanonicalRepoFiles(t *testing.T) {
	projectRoot := t.TempDir()
	setupProjectResourceTestEnv(t, projectRoot)
	mustChdir(t, projectRoot)

	mustWriteProjectConfig(t, projectRoot)
	mustAddSkill(t, filepath.Join(projectRoot, ".skillshare", "skills"), "alpha")

	putManagedRule(t, projectRoot, "claude/manual.md", "# Managed rule\n")
	putManagedHook(t, projectRoot, "claude/pre-tool-use/bash.yaml", "./bin/check")

	resetModeLabel(t)
	if err := cmdSync([]string{"-p", "--resources", "rules,hooks"}); err != nil {
		t.Fatalf("cmdSync(-p --resources rules,hooks) error = %v", err)
	}

	mustExist(t, filepath.Join(projectRoot, ".claude", "rules", "manual.md"))
	mustExist(t, filepath.Join(projectRoot, ".claude", "settings.json"))
	mustNotExist(t, filepath.Join(projectRoot, ".claude", "skills", "alpha"))
}

func TestCmdSyncProject_AllAttemptsManagedResourcesWhenSourceMissing(t *testing.T) {
	projectRoot := t.TempDir()
	setupProjectResourceTestEnv(t, projectRoot)
	mustChdir(t, projectRoot)

	mustWriteProjectConfig(t, projectRoot)
	if err := os.RemoveAll(filepath.Join(projectRoot, ".skillshare", "skills")); err != nil {
		t.Fatalf("remove project source: %v", err)
	}

	putManagedRule(t, projectRoot, "claude/manual.md", "# Managed rule\n")
	putManagedHook(t, projectRoot, "claude/pre-tool-use/bash.yaml", "./bin/check")

	resetModeLabel(t)
	var syncErr error
	output := captureStdout(t, func() {
		syncErr = cmdSync([]string{"-p", "--all"})
	})
	if syncErr == nil {
		t.Fatal("expected cmdSync(-p --all) to report partial failure")
	}

	mustExist(t, filepath.Join(projectRoot, ".claude", "rules", "manual.md"))
	mustExist(t, filepath.Join(projectRoot, ".claude", "settings.json"))
	if !strings.Contains(output, "source directory does not exist") {
		t.Fatalf("project sync output missing source failure:\n%s", output)
	}
	if !strings.Contains(output, "rules: 1 updated") {
		t.Fatalf("project sync output missing rules detail after source failure:\n%s", output)
	}
	if !strings.Contains(output, "hooks: 1 updated") {
		t.Fatalf("project sync output missing hooks detail after source failure:\n%s", output)
	}
}

func TestCmdCollectProject_ResourcesCollectIntoManagedStores(t *testing.T) {
	projectRoot := t.TempDir()
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	setupProjectResourceTestEnv(t, projectRoot)
	mustChdir(t, projectRoot)

	mustWriteProjectConfig(t, projectRoot)
	mustWriteFile(t, filepath.Join(projectRoot, ".claude", "rules", "backend.md"), "# Backend rule\n")
	mustWriteFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./bin/check"}]}]}}`)

	resetModeLabel(t)
	if err := cmdCollect([]string{"-p", "--resources", "rules,hooks", "--force"}); err != nil {
		t.Fatalf("cmdCollect(-p --resources rules,hooks --force) error = %v", err)
	}

	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Get("claude/backend.md"); err != nil {
		t.Fatalf("managed rule not collected: %v", err)
	}

	hookStore := managedhooks.NewStore(projectRoot)
	records, err := hookStore.List()
	if err != nil {
		t.Fatalf("list managed hooks: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("managed hooks = %#v, want exactly one collected hook", records)
	}
	if records[0].Tool != "claude" || records[0].Event != "PreToolUse" || records[0].Matcher != "Bash" {
		t.Fatalf("managed hook = %#v, want claude PreToolUse Bash", records[0])
	}
	if len(records[0].Handlers) != 1 || records[0].Handlers[0].Command != "./bin/check" {
		t.Fatalf("managed hook handlers = %#v, want collected command handler", records[0].Handlers)
	}
}

func TestCmdCollect_SkillAndRuleFailuresStillAttemptManagedHooks(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	sourcePath := filepath.Join(t.TempDir(), "source-file")
	mustWriteFile(t, sourcePath, "not-a-directory")
	mustAddSkill(t, filepath.Join(home, ".claude", "skills"), "alpha")

	cfg := &config.Config{
		Source: sourcePath,
		Targets: map[string]config.TargetConfig{
			"claude": {Path: filepath.Join(home, ".claude", "skills")},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	mustWriteFile(t, filepath.Join(home, ".claude", "rules", "backend.md"), "# Backend rule\n")
	mustWriteFile(t, filepath.Join(home, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./bin/check"}]}]}}`)
	mustWriteFile(t, config.ManagedRulesDir(""), "not-a-directory")

	resetModeLabel(t)
	var collectErr error
	output := captureStdout(t, func() {
		collectErr = cmdCollect([]string{"--resources", "skills,rules,hooks", "--force"})
	})
	if collectErr == nil {
		t.Fatal("expected cmdCollect(--resources skills,rules,hooks --force) to report partial failure")
	}

	hookStore := managedhooks.NewStore("")
	records, err := hookStore.List()
	if err != nil {
		t.Fatalf("list managed hooks: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("managed hooks = %#v, want exactly one collected hook after partial failure", records)
	}
	if records[0].Tool != "claude" || records[0].Event != "PreToolUse" || records[0].Matcher != "Bash" {
		t.Fatalf("managed hook = %#v, want claude PreToolUse Bash", records[0])
	}
	if !strings.Contains(output, "collected into managed store") {
		t.Fatalf("collect output missing managed hook success after partial failure:\n%s", output)
	}
}

func setupGlobalResourceTestEnv(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	configHome := filepath.Join(root, "xdg-config")
	dataHome := filepath.Join(root, "xdg-data")
	stateHome := filepath.Join(root, "xdg-state")
	cacheHome := filepath.Join(root, "xdg-cache")
	for _, dir := range []string{home, configHome, dataHome, stateHome, cacheHome} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(configHome, "skillshare", "config.yaml"))
	return home
}

func setupProjectResourceTestEnv(t *testing.T, projectRoot string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	configHome := filepath.Join(root, "xdg-config")
	dataHome := filepath.Join(root, "xdg-data")
	stateHome := filepath.Join(root, "xdg-state")
	cacheHome := filepath.Join(root, "xdg-cache")
	for _, dir := range []string{home, configHome, dataHome, stateHome, cacheHome} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(configHome, "skillshare", "config.yaml"))
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}
}

func mustWriteProjectConfig(t *testing.T, projectRoot string) {
	t.Helper()
	cfg := &config.ProjectConfig{
		Targets: []config.ProjectTargetEntry{{Name: "claude"}},
	}
	if err := cfg.Save(projectRoot); err != nil {
		t.Fatalf("save project config: %v", err)
	}
	if err := (&config.Registry{}).Save(filepath.Join(projectRoot, ".skillshare")); err != nil {
		t.Fatalf("save project registry: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, ".skillshare", "skills"), 0o755); err != nil {
		t.Fatalf("mkdir project source: %v", err)
	}
}

func mustAddSkill(t *testing.T, sourceDir, name string) {
	t.Helper()
	mustWriteFile(t, filepath.Join(sourceDir, name, "SKILL.md"), "# "+name+"\n")
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func putManagedRule(t *testing.T, projectRoot, id, content string) {
	t.Helper()
	store := managedrules.NewStore(projectRoot)
	if _, err := store.Put(managedrules.Save{ID: id, Content: []byte(content)}); err != nil {
		t.Fatalf("put managed rule %s: %v", id, err)
	}
}

func putManagedHook(t *testing.T, projectRoot, id, command string) {
	t.Helper()
	putManagedHookForTool(t, projectRoot, id, "claude", "PreToolUse", "Bash", command)
}

func putManagedHookForTool(t *testing.T, projectRoot, id, tool, event, matcher, command string) {
	t.Helper()
	store := managedhooks.NewStore(projectRoot)
	if _, err := store.Put(managedhooks.Save{
		ID:      id,
		Tool:    tool,
		Event:   event,
		Matcher: matcher,
		Handlers: []managedhooks.Handler{{
			Type:    "command",
			Command: command,
		}},
	}); err != nil {
		t.Fatalf("put managed hook %s: %v", id, err)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to not exist, err=%v", path, err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s content = %q, want %q", path, string(data), want)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func mustChdir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
}

func resetModeLabel(t *testing.T) {
	t.Helper()
	ui.ModeLabel = ""
	t.Cleanup(func() {
		ui.ModeLabel = ""
	})
}
