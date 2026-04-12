package managed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/resources/adapters"
)

type managedRuleSyncState struct {
	Outputs map[string]managedRuleSyncOutput `json:"outputs,omitempty"`
}

type managedRuleSyncOutput struct {
	Target    string    `json:"target,omitempty"`
	Checksum  string    `json:"checksum,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func loadManagedRuleSyncState(projectRoot string) (*managedRuleSyncState, error) {
	state := &managedRuleSyncState{Outputs: map[string]managedRuleSyncOutput{}}
	if strings.TrimSpace(projectRoot) == "" {
		return state, nil
	}

	path := managedRuleSyncStatePath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if errorsIsNotExist(err) {
			return state, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, state); err != nil {
		return &managedRuleSyncState{Outputs: map[string]managedRuleSyncOutput{}}, nil
	}
	normalizeManagedRuleSyncState(state)
	return state, nil
}

func recordManagedRuleSyncState(projectRoot, target string, files []adapters.CompiledFile, state *managedRuleSyncState) error {
	if strings.TrimSpace(projectRoot) == "" || state == nil {
		return nil
	}
	if state.Outputs == nil {
		state.Outputs = map[string]managedRuleSyncOutput{}
	}

	rootAgentsPath := filepath.Clean(filepath.Join(projectRoot, "AGENTS.md"))
	target = strings.TrimSpace(target)
	for _, file := range files {
		if filepath.Clean(file.Path) != rootAgentsPath {
			continue
		}
		if target == "" {
			return nil
		}
		state.Outputs[target] = managedRuleSyncOutput{
			Target:    target,
			Checksum:  checksumForContent([]byte(file.Content)),
			UpdatedAt: time.Now().UTC(),
		}
		return saveManagedRuleSyncState(projectRoot, state)
	}

	if target != "" {
		delete(state.Outputs, target)
	}
	return saveManagedRuleSyncState(projectRoot, state)
}

func managedRuleProjectRootAgentsOwned(state *managedRuleSyncState, root string) bool {
	if strings.TrimSpace(root) == "" || state == nil {
		return false
	}
	if state.Outputs == nil {
		return false
	}
	return len(state.Outputs) > 0
}

func pruneManagedProjectRootAgents(root string, keep map[string]struct{}, state *managedRuleSyncState, dryRun bool, pruned *[]string) error {
	if strings.TrimSpace(root) == "" || state == nil {
		return nil
	}

	agentsPath := filepath.Clean(filepath.Join(root, "AGENTS.md"))
	if _, ok := keep[agentsPath]; ok {
		return nil
	}

	ownership, ok := managedRuleProjectRootAgentsClaim(state)
	if !ok {
		return nil
	}

	data, err := os.ReadFile(agentsPath)
	if err != nil {
		if errorsIsNotExist(err) {
			if dryRun {
				return nil
			}
			clearManagedRuleProjectRootAgentsClaims(state)
			return saveManagedRuleSyncState(root, state)
		}
		return err
	}
	if checksumForContent(data) != ownership.Checksum {
		if dryRun {
			return nil
		}
		clearManagedRuleProjectRootAgentsClaims(state)
		return saveManagedRuleSyncState(root, state)
	}

	*pruned = append(*pruned, agentsPath)
	if dryRun {
		return nil
	}
	if err := os.Remove(agentsPath); err != nil && !errorsIsNotExist(err) {
		return err
	}
	clearManagedRuleProjectRootAgentsClaims(state)
	return saveManagedRuleSyncState(root, state)
}

func saveManagedRuleSyncState(projectRoot string, state *managedRuleSyncState) error {
	if strings.TrimSpace(projectRoot) == "" || state == nil {
		return nil
	}
	normalizeManagedRuleSyncState(state)
	if len(state.Outputs) == 0 {
		err := os.Remove(managedRuleSyncStatePath(projectRoot))
		if err != nil && !errorsIsNotExist(err) {
			return err
		}
		return nil
	}

	path := managedRuleSyncStatePath(projectRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}

func managedRuleSyncStatePath(projectRoot string) string {
	cleaned := filepath.Clean(strings.TrimSpace(projectRoot))
	sum := sha256.Sum256([]byte(cleaned))
	return filepath.Join(config.StateDir(), "managed", "rules", hex.EncodeToString(sum[:])+".json")
}

func checksumForContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}

func normalizeManagedRuleSyncState(state *managedRuleSyncState) {
	if state.Outputs == nil {
		state.Outputs = map[string]managedRuleSyncOutput{}
	}

	legacy, ok := state.Outputs["AGENTS.md"]
	if !ok {
		return
	}
	delete(state.Outputs, "AGENTS.md")
	target := strings.TrimSpace(legacy.Target)
	if target == "" {
		target = "legacy"
	}
	if _, exists := state.Outputs[target]; !exists {
		state.Outputs[target] = legacy
	}
}

func managedRuleProjectRootAgentsClaim(state *managedRuleSyncState) (managedRuleSyncOutput, bool) {
	if state == nil {
		return managedRuleSyncOutput{}, false
	}
	for _, output := range state.Outputs {
		if strings.TrimSpace(output.Checksum) == "" {
			continue
		}
		return output, true
	}
	return managedRuleSyncOutput{}, false
}

func clearManagedRuleProjectRootAgentsClaims(state *managedRuleSyncState) {
	if state == nil {
		return
	}
	for key := range state.Outputs {
		delete(state.Outputs, key)
	}
}
