package managed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPruneTrackedManagedRuleOutputs_RemovesCompileRootStateAfterLastClaim(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	root := t.TempDir()
	nestedAgentsPath := filepath.Join(root, "nested", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(nestedAgentsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(nestedAgentsPath), err)
	}

	content := []byte("# Nested Pi Agents\n")
	if err := os.WriteFile(nestedAgentsPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", nestedAgentsPath, err)
	}

	state := &managedRuleSyncState{
		Outputs: map[string]managedRuleSyncOutput{
			managedRuleSyncOutputKey("pi", nestedAgentsPath): {
				Target:   "pi",
				Path:     nestedAgentsPath,
				Checksum: checksumForContent(content),
			},
		},
	}
	if err := saveManagedRuleSyncState(root, state); err != nil {
		t.Fatalf("saveManagedRuleSyncState() error = %v", err)
	}
	if _, err := os.Stat(managedRuleSyncStatePath(root)); err != nil {
		t.Fatalf("expected state file before prune, got err=%v", err)
	}

	pruned := make([]string, 0, 1)
	if err := pruneTrackedManagedRuleOutputs(root, map[string]struct{}{}, state, false, &pruned); err != nil {
		t.Fatalf("pruneTrackedManagedRuleOutputs() error = %v", err)
	}

	if !containsAll(pruned, nestedAgentsPath) {
		t.Fatalf("pruned = %v, want %q", pruned, nestedAgentsPath)
	}
	if _, err := os.Stat(managedRuleSyncStatePath(root)); !os.IsNotExist(err) {
		t.Fatalf("expected compile-root state file to be removed, got err=%v", err)
	}
}

func TestLoadManagedRuleSyncState_MigratesLegacyPathlessClaimsToProjectRootAgents(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	root := t.TempDir()
	legacy := map[string]any{
		"outputs": map[string]any{
			"pi": map[string]any{
				"target":   "pi",
				"checksum": "legacy-checksum",
			},
		},
	}

	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(managedRuleSyncStatePath(root)), 0o755); err != nil {
		t.Fatalf("MkdirAll(state dir) error = %v", err)
	}
	if err := os.WriteFile(managedRuleSyncStatePath(root), data, 0o644); err != nil {
		t.Fatalf("WriteFile(state) error = %v", err)
	}

	state, err := loadManagedRuleSyncState(root)
	if err != nil {
		t.Fatalf("loadManagedRuleSyncState() error = %v", err)
	}

	wantPath := filepath.Join(root, "AGENTS.md")
	output, ok := state.Outputs[managedRuleSyncOutputKey("pi", wantPath)]
	if !ok {
		t.Fatalf("state outputs = %#v, want migrated claim for %q", state.Outputs, wantPath)
	}
	if output.Path != wantPath {
		t.Fatalf("output.Path = %q, want %q", output.Path, wantPath)
	}
}
