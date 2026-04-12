package managed

import (
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
