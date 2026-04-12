package rules

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestManagedRulesDir_GlobalAndProject(t *testing.T) {
	xdgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	globalRules := config.ManagedRulesDir("")
	wantGlobalRules := filepath.Join(xdgHome, "skillshare", "rules")
	if globalRules != wantGlobalRules {
		t.Fatalf("ManagedRulesDir(\"\") = %q, want %q", globalRules, wantGlobalRules)
	}

	globalHooks := config.ManagedHooksDir("")
	wantGlobalHooks := filepath.Join(xdgHome, "skillshare", "hooks")
	if globalHooks != wantGlobalHooks {
		t.Fatalf("ManagedHooksDir(\"\") = %q, want %q", globalHooks, wantGlobalHooks)
	}

	projectRoot := t.TempDir()

	projectRules := config.ManagedRulesDir(projectRoot)
	wantProjectRules := filepath.Join(projectRoot, ".skillshare", "rules")
	if projectRules != wantProjectRules {
		t.Fatalf("ManagedRulesDir(project) = %q, want %q", projectRules, wantProjectRules)
	}

	projectHooks := config.ManagedHooksDir(projectRoot)
	wantProjectHooks := filepath.Join(projectRoot, ".skillshare", "hooks")
	if projectHooks != wantProjectHooks {
		t.Fatalf("ManagedHooksDir(project) = %q, want %q", projectHooks, wantProjectHooks)
	}
}

func TestRuleStore_PutGetListDelete(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	saved, err := store.Put(Save{
		ID:      "claude/backend.md",
		Content: []byte("# backend\n"),
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if saved.ID != "claude/backend.md" {
		t.Fatalf("Put() ID = %q, want %q", saved.ID, "claude/backend.md")
	}

	got, err := store.Get("claude/backend.md")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got.Content) != "# backend\n" {
		t.Fatalf("Get() content = %q, want %q", string(got.Content), "# backend\n")
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("List() len = %d, want 1", len(all))
	}
	if all[0].ID != "claude/backend.md" {
		t.Fatalf("List()[0].ID = %q, want %q", all[0].ID, "claude/backend.md")
	}

	if err := store.Delete("claude/backend.md"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = store.Get("claude/backend.md")
	if !os.IsNotExist(err) {
		t.Fatalf("Get() after Delete error = %v, want not-exist", err)
	}
}

func TestRuleStore_PutOverwritesExistingRule(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	_, err := store.Put(Save{
		ID:      "claude/backend.md",
		Content: []byte("# v1\n"),
	})
	if err != nil {
		t.Fatalf("first Put() error = %v", err)
	}

	_, err = store.Put(Save{
		ID:      "claude/backend.md",
		Content: []byte("# v2\n"),
	})
	if err != nil {
		t.Fatalf("second Put() error = %v", err)
	}

	got, err := store.Get("claude/backend.md")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got.Content) != "# v2\n" {
		t.Fatalf("Get() content = %q, want %q", string(got.Content), "# v2\n")
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("List() len = %d, want 1", len(all))
	}
	if all[0].ID != "claude/backend.md" {
		t.Fatalf("List()[0].ID = %q, want %q", all[0].ID, "claude/backend.md")
	}
	if string(all[0].Content) != "# v2\n" {
		t.Fatalf("List()[0].Content = %q, want %q", string(all[0].Content), "# v2\n")
	}
}

func TestRuleStore_PutAndGet_RoundTripsMetadataSidecar(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	content := []byte("---\npaths: [src/**]\n---\n# Manual rule\n")
	saved, err := store.Put(Save{
		ID:         "claude/manual.md",
		Content:    content,
		Targets:    []string{"claude-work", "claude-personal"},
		SourceType: "local",
		Disabled:   true,
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, err := store.Get(saved.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if string(got.Content) != string(content) {
		t.Fatalf("content changed during round-trip: got %q want %q", string(got.Content), string(content))
	}
	if !reflect.DeepEqual(got.Targets, []string{"claude-work", "claude-personal"}) {
		t.Fatalf("Get() Targets = %#v, want %#v", got.Targets, []string{"claude-work", "claude-personal"})
	}
	if got.SourceType != "local" {
		t.Fatalf("Get() SourceType = %q, want %q", got.SourceType, "local")
	}
	if !got.Disabled {
		t.Fatal("Get() Disabled = false, want true")
	}

	rulePath := filepath.Join(projectRoot, ".skillshare", "rules", "claude", "manual.md")
	raw, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("ReadFile(rule) error = %v", err)
	}
	if string(raw) != string(content) {
		t.Fatalf("rule file content changed: got %q want %q", string(raw), string(content))
	}
}

func TestRuleStore_Get_LegacyRuleWithoutMetadataLoadsAsGlobalEnabled(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	rulePath := filepath.Join(projectRoot, ".skillshare", "rules", "codex", "backend.md")
	if err := os.MkdirAll(filepath.Dir(rulePath), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(rulePath, []byte("# Backend\n"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := store.Get("codex/backend.md")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Disabled {
		t.Fatal("Get() Disabled = true, want false")
	}
	if len(got.Targets) != 0 {
		t.Fatalf("Get() Targets = %#v, want nil/empty", got.Targets)
	}
	if got.SourceType != "" {
		t.Fatalf("Get() SourceType = %q, want empty", got.SourceType)
	}
}

func TestRuleStore_RejectsInvalidIDs(t *testing.T) {
	store := NewStore(t.TempDir())

	invalidIDs := []string{
		"",
		"   ",
		".",
		"..",
		"../outside.md",
		"..\\outside.md",
		"..\\..\\outside.md",
		"/tmp/outside.md",
		`C:\outside.md`,
		"C:/outside.md",
		"claude/C:/outside.md",
		"claude/C:outside.md",
		`\\server\share\file.md`,
		"claude/.backend.md.metadata.yaml",
		"claude/nested/.backend.md.metadata.yaml",
		"claude/.rule-metadata-tmp-123",
		"claude/nested/.rule-metadata-tmp-123",
	}

	for _, id := range invalidIDs {
		id := id
		t.Run(id, func(t *testing.T) {
			if _, err := store.Put(Save{ID: id, Content: []byte("x")}); err == nil {
				t.Fatalf("Put(%q) error = nil, want error", id)
			}
			if _, err := store.Get(id); err == nil {
				t.Fatalf("Get(%q) error = nil, want error", id)
			}
			if err := store.Delete(id); err == nil {
				t.Fatalf("Delete(%q) error = nil, want error", id)
			}
		})
	}
}

func TestRuleStore_Put_RollsBackContentWhenMetadataWriteFails(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	if _, err := store.Put(Save{
		ID:         "claude/backend.md",
		Content:    []byte("# old\n"),
		Targets:    []string{"claude-work"},
		SourceType: "local",
	}); err != nil {
		t.Fatalf("seed Put() error = %v", err)
	}

	originalRuleWriteFile := ruleWriteFile
	ruleWriteFile = func(name string, data []byte, perm os.FileMode) error {
		if isRuleMetadataFile(filepath.Base(name)) || strings.HasPrefix(filepath.Base(name), ruleMetadataTempPrefix) {
			return errors.New("boom")
		}
		return originalRuleWriteFile(name, data, perm)
	}
	defer func() {
		ruleWriteFile = originalRuleWriteFile
	}()

	_, err := store.Put(Save{
		ID:         "claude/backend.md",
		Content:    []byte("# new\n"),
		Targets:    []string{"claude-personal"},
		SourceType: "project",
		Disabled:   true,
	})
	if err == nil {
		t.Fatal("Put() error = nil, want metadata write failure")
	}

	got, getErr := store.Get("claude/backend.md")
	if getErr != nil {
		t.Fatalf("Get() error = %v", getErr)
	}
	if string(got.Content) != "# old\n" {
		t.Fatalf("Get() content = %q, want rollback to %q", string(got.Content), "# old\n")
	}
	if !reflect.DeepEqual(got.Targets, []string{"claude-work"}) {
		t.Fatalf("Get() Targets = %#v, want %#v", got.Targets, []string{"claude-work"})
	}
	if got.SourceType != "local" {
		t.Fatalf("Get() SourceType = %q, want %q", got.SourceType, "local")
	}
	if got.Disabled {
		t.Fatal("Get() Disabled = true, want false")
	}
}

func TestRuleStore_RejectsUnsupportedToolPrefixes(t *testing.T) {
	store := NewStore(t.TempDir())

	unsupportedIDs := []string{
		"foo/bar.md",
		"hooks/rule.md",
		"unknown/nested/rule.md",
	}

	for _, id := range unsupportedIDs {
		id := id
		t.Run(id, func(t *testing.T) {
			if _, err := store.Put(Save{ID: id, Content: []byte("x")}); err == nil {
				t.Fatalf("Put(%q) error = nil, want error", id)
			}
			if _, err := store.Get(id); err == nil {
				t.Fatalf("Get(%q) error = nil, want error", id)
			}
			if err := store.Delete(id); err == nil {
				t.Fatalf("Delete(%q) error = nil, want error", id)
			}
		})
	}
}

func TestNormalizeRuleID_RejectsBareToolPrefixes(t *testing.T) {
	for _, id := range []string{"claude", "codex", "gemini", "pi"} {
		t.Run(id, func(t *testing.T) {
			if _, err := NormalizeRuleID(id); err == nil {
				t.Fatalf("NormalizeRuleID(%q) error = nil, want error", id)
			}
		})
	}
}

func TestNormalizeRuleID_AcceptsPiInstructionPaths(t *testing.T) {
	for _, id := range []string{"pi/AGENTS.md", "pi/SYSTEM.md", "pi/APPEND_SYSTEM.md"} {
		t.Run(id, func(t *testing.T) {
			got, err := NormalizeRuleID(id)
			if err != nil {
				t.Fatalf("NormalizeRuleID(%q) error = %v", id, err)
			}
			if got != id {
				t.Fatalf("NormalizeRuleID(%q) = %q, want %q", id, got, id)
			}
		})
	}
}

func TestNormalizeRuleID_RejectsReservedTempSegments(t *testing.T) {
	for _, id := range []string{
		"claude/.rule-tmp-test.md",
		"claude/rules/.rule-tmp-test.md",
		"codex/.rule-tmp-agents.md",
		"gemini/nested/.rule-tmp-rule.md",
	} {
		t.Run(id, func(t *testing.T) {
			if _, err := NormalizeRuleID(id); err == nil {
				t.Fatalf("NormalizeRuleID(%q) error = nil, want error", id)
			}
		})
	}
}

func TestRuleStore_ListIgnoresTransientTempFiles(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	if _, err := store.Put(Save{
		ID:      "claude/keep.md",
		Content: []byte("# Keep\n"),
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	tempPath := filepath.Join(projectRoot, ".skillshare", "rules", "claude", ".rule-tmp-12345")
	if err := os.WriteFile(tempPath, []byte("# Temp\n"), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", tempPath, err)
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("List() len = %d, want 1", len(all))
	}
	if all[0].ID != "claude/keep.md" {
		t.Fatalf("List()[0].ID = %q, want %q", all[0].ID, "claude/keep.md")
	}
}

func TestRuleStore_ListIgnoresMetadataTempFiles(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	if _, err := store.Put(Save{
		ID:      "claude/keep.md",
		Content: []byte("# Keep\n"),
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	tempPath := filepath.Join(projectRoot, ".skillshare", "rules", "claude", ".rule-metadata-tmp-12345")
	if err := os.WriteFile(tempPath, []byte("targets:\n  - stray\n"), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", tempPath, err)
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("List() len = %d, want 1", len(all))
	}
	if all[0].ID != "claude/keep.md" {
		t.Fatalf("List()[0].ID = %q, want %q", all[0].ID, "claude/keep.md")
	}
}

func TestRuleStore_ListIgnoresNonRegularEntries(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	if _, err := store.Put(Save{
		ID:      "claude/keep.md",
		Content: []byte("# Keep\n"),
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	targetPath := filepath.Join(projectRoot, "external.md")
	if err := os.WriteFile(targetPath, []byte("# Linked\n"), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", targetPath, err)
	}

	linkPath := filepath.Join(projectRoot, ".skillshare", "rules", "claude", "linked.md")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("Symlink(%q, %q) unsupported: %v", targetPath, linkPath, err)
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("List() len = %d, want 1", len(all))
	}
	if all[0].ID != "claude/keep.md" {
		t.Fatalf("List()[0].ID = %q, want %q", all[0].ID, "claude/keep.md")
	}
}

func TestRuleStore_GetRejectsNonRegularEntries(t *testing.T) {
	projectRoot := t.TempDir()
	store := NewStore(projectRoot)

	targetPath := filepath.Join(projectRoot, "external.md")
	if err := os.WriteFile(targetPath, []byte("# Linked\n"), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", targetPath, err)
	}

	managedDir := filepath.Join(projectRoot, ".skillshare", "rules", "claude")
	if err := os.MkdirAll(managedDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", managedDir, err)
	}

	linkPath := filepath.Join(managedDir, "linked.md")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("Symlink(%q, %q) unsupported: %v", targetPath, linkPath, err)
	}

	_, err := store.Get("claude/linked.md")
	if err == nil {
		t.Fatalf("Get() error = nil, want non-regular file error")
	}
	if !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("Get() error = %v, want non-regular file error", err)
	}
}
