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
	Path      string    `json:"path,omitempty"`
	Checksum  string    `json:"checksum,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func loadManagedRuleSyncState(root string) (*managedRuleSyncState, error) {
	state := &managedRuleSyncState{Outputs: map[string]managedRuleSyncOutput{}}
	if strings.TrimSpace(root) == "" {
		return state, nil
	}

	path := managedRuleSyncStatePath(root)
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
	normalizeManagedRuleSyncState(root, state)
	return state, nil
}

func recordManagedRuleSyncState(targetName, family, root string, files []adapters.CompiledFile, state *managedRuleSyncState) error {
	if strings.TrimSpace(root) == "" || state == nil {
		return nil
	}
	if state.Outputs == nil {
		state.Outputs = map[string]managedRuleSyncOutput{}
	}
	targetName = strings.TrimSpace(targetName)
	if targetName == "" {
		return nil
	}

	clearManagedRuleTrackedOutputsForTarget(state, targetName)
	for _, output := range managedRuleTrackedOutputs(targetName, family, root, files) {
		state.Outputs[managedRuleSyncOutputKey(output.Target, output.Path)] = output
	}
	return saveManagedRuleSyncState(root, state)
}

func managedRuleHasTrackedOutputs(state *managedRuleSyncState) bool {
	return state != nil && len(state.Outputs) > 0
}

func pruneTrackedManagedRuleOutputs(root string, keep map[string]struct{}, state *managedRuleSyncState, dryRun bool, pruned *[]string) error {
	if state == nil || len(state.Outputs) == 0 {
		return nil
	}

	changed := false
	for path, claims := range managedRuleTrackedOutputsByPath(state) {
		if _, ok := keep[path]; ok {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if errorsIsNotExist(err) {
				if !dryRun {
					clearManagedRuleTrackedOutputsForPath(state, path)
					changed = true
				}
				continue
			}
			return err
		}

		currentChecksum := checksumForContent(data)
		claimed := false
		for _, claim := range claims {
			if claim.Checksum == currentChecksum {
				claimed = true
				break
			}
		}
		if !claimed {
			if !dryRun {
				clearManagedRuleTrackedOutputsForPath(state, path)
				changed = true
			}
			continue
		}

		*pruned = append(*pruned, path)
		if dryRun {
			continue
		}
		if err := os.Remove(path); err != nil && !errorsIsNotExist(err) {
			return err
		}
		clearManagedRuleTrackedOutputsForPath(state, path)
		changed = true
	}

	if !changed || dryRun {
		return nil
	}

	return saveManagedRuleSyncState(root, state)
}

func saveManagedRuleSyncState(root string, state *managedRuleSyncState) error {
	if strings.TrimSpace(root) == "" || state == nil {
		return nil
	}
	normalizeManagedRuleSyncState(root, state)
	if len(state.Outputs) == 0 {
		err := os.Remove(managedRuleSyncStatePath(root))
		if err != nil && !errorsIsNotExist(err) {
			return err
		}
		return nil
	}

	path := managedRuleSyncStatePath(root)
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

func managedRuleSyncStatePath(root string) string {
	cleaned := filepath.Clean(strings.TrimSpace(root))
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

func normalizeManagedRuleSyncState(root string, state *managedRuleSyncState) {
	if state.Outputs == nil {
		state.Outputs = map[string]managedRuleSyncOutput{}
	}

	normalized := make(map[string]managedRuleSyncOutput, len(state.Outputs))
	rootAgentsPath := filepath.Clean(filepath.Join(root, "AGENTS.md"))
	for key, output := range state.Outputs {
		target := strings.TrimSpace(output.Target)
		rawPath := strings.TrimSpace(output.Path)
		path := ""
		if rawPath != "" {
			path = filepath.Clean(rawPath)
		}
		switch {
		case target == "" && key == "AGENTS.md":
			target = "legacy"
			path = rootAgentsPath
		case target == "" && path == "":
			target = strings.TrimSpace(key)
			path = rootAgentsPath
		case path == "":
			path = rootAgentsPath
		}
		if target == "" || path == "" || output.Checksum == "" {
			continue
		}
		output.Target = target
		output.Path = path
		normalized[managedRuleSyncOutputKey(target, path)] = output
	}
	state.Outputs = normalized
}

func managedRuleSyncOutputKey(target, path string) string {
	return strings.TrimSpace(target) + "\x00" + filepath.Clean(strings.TrimSpace(path))
}

func managedRuleTrackedOutputs(targetName, family, root string, files []adapters.CompiledFile) []managedRuleSyncOutput {
	ownedDir, hasOwnedDir := managedRuleOwnedDir(family, root)
	ownedFileSet := make(map[string]struct{})
	for _, path := range managedRuleOwnedFiles(family, root) {
		ownedFileSet[filepath.Clean(path)] = struct{}{}
	}

	outputs := make([]managedRuleSyncOutput, 0, len(files))
	for _, file := range files {
		cleanedPath := filepath.Clean(file.Path)
		if hasOwnedDir && pathWithinDir(cleanedPath, ownedDir) {
			continue
		}
		if _, ok := ownedFileSet[cleanedPath]; ok {
			continue
		}
		outputs = append(outputs, managedRuleSyncOutput{
			Target:    strings.TrimSpace(targetName),
			Path:      cleanedPath,
			Checksum:  checksumForContent([]byte(file.Content)),
			UpdatedAt: time.Now().UTC(),
		})
	}
	return outputs
}

func managedRuleTrackedOutputsByPath(state *managedRuleSyncState) map[string][]managedRuleSyncOutput {
	byPath := make(map[string][]managedRuleSyncOutput)
	if state == nil {
		return byPath
	}
	for _, output := range state.Outputs {
		if output.Path == "" || output.Checksum == "" {
			continue
		}
		path := filepath.Clean(output.Path)
		byPath[path] = append(byPath[path], output)
	}
	return byPath
}

func clearManagedRuleTrackedOutputsForTarget(state *managedRuleSyncState, target string) {
	if state == nil {
		return
	}
	target = strings.TrimSpace(target)
	for key, output := range state.Outputs {
		if output.Target != target {
			continue
		}
		delete(state.Outputs, key)
	}
}

func clearManagedRuleTrackedOutputsForPath(state *managedRuleSyncState, path string) {
	if state == nil {
		return
	}
	path = filepath.Clean(strings.TrimSpace(path))
	for key, output := range state.Outputs {
		if filepath.Clean(output.Path) != path {
			continue
		}
		delete(state.Outputs, key)
	}
}
