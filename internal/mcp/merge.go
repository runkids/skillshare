package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// MergeResult reports what changed for a single merge operation.
type MergeResult struct {
	Target  string   `json:"target"`
	Added   []string `json:"added,omitempty"`
	Updated []string `json:"updated,omitempty"`
	Removed []string `json:"removed,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// MergeToTarget reads the target's JSON file, merges skillshare-managed servers,
// removes previously-synced entries no longer in source, and writes back.
// All non-MCP keys and user-managed entries are preserved.
func MergeToTarget(
	configPath string,
	servers map[string]MCPServer,
	previousServers []string,
	target MCPTargetSpec,
	dryRun bool,
) (*MergeResult, error) {
	result := &MergeResult{Target: target.Name}

	// 1. Read existing JSON (empty map if file not found).
	doc, err := readJSONFile(configPath)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	// 2. Get or create the MCP section under target.Key.
	var section map[string]any
	if raw, ok := doc[target.Key]; ok {
		if m, ok := raw.(map[string]any); ok {
			section = m
		} else {
			section = make(map[string]any)
		}
	} else {
		section = make(map[string]any)
	}

	// Build a set of previousServers for O(1) lookup.
	prevSet := make(map[string]bool, len(previousServers))
	for _, name := range previousServers {
		prevSet[name] = true
	}

	// 3. Remove entries that were previously managed but are no longer in servers.
	for _, name := range previousServers {
		if _, stillPresent := servers[name]; !stillPresent {
			if _, inSection := section[name]; inSection {
				delete(section, name)
				result.Removed = append(result.Removed, name)
			}
		}
	}

	// 4. Upsert current entries.
	for name, srv := range servers {
		transformed := transformServer(srv, target)
		if _, exists := section[name]; exists && prevSet[name] {
			// Previously managed — update.
			section[name] = transformed
			result.Updated = append(result.Updated, name)
		} else if _, exists := section[name]; !exists {
			// New entry — add.
			section[name] = transformed
			result.Added = append(result.Added, name)
		} else {
			// Entry exists but was not previously managed (user entry) — update anyway
			// since the user's config file already has it; treat as updated to keep
			// skillshare's view consistent.
			section[name] = transformed
			result.Updated = append(result.Updated, name)
		}
	}

	// 5. Sort result slices for deterministic output.
	sort.Strings(result.Added)
	sort.Strings(result.Updated)
	sort.Strings(result.Removed)

	// Write the section back into the document.
	doc[target.Key] = section

	// 6. Write back unless dry run or nothing changed.
	if !dryRun && len(result.Added)+len(result.Updated)+len(result.Removed) > 0 {
		if err := writeJSONFile(configPath, doc); err != nil {
			result.Error = err.Error()
			return result, err
		}
	}

	return result, nil
}

// readJSONFile reads a JSON file and returns its contents as map[string]any.
// Returns an empty map (not an error) if the file does not exist.
func readJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]any), nil
		}
		return nil, fmt.Errorf("read json file %q: %w", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse json file %q: %w", path, err)
	}
	if doc == nil {
		doc = make(map[string]any)
	}
	return doc, nil
}

// writeJSONFile marshals doc as indented JSON (2-space) with a trailing newline
// and writes it to path, creating parent directories as needed.
func writeJSONFile(path string, doc map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create dir for %q: %w", path, err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json for %q: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write json file %q: %w", path, err)
	}
	return nil
}
