// Package adopt detects and migrates skills that external CLI tools (e.g.
// firecrawl/cli, googleworkspace/cli) drop into skillshare's "universal" target
// (~/.agents/skills) and track in ~/.agents/.skill-lock.json, bypassing the
// source-of-truth model. The lockfile is treated as READ-ONLY: we detect and
// warn, but never write or prune it.
package adopt

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// lockFileName is the lockfile external tools maintain in the agents dir.
const lockFileName = ".skill-lock.json"

// LockFileName returns the name of the external tool lockfile we detect.
func LockFileName() string { return lockFileName }

// LockEntry describes a single skill recorded in the external tool's lockfile.
// SourceTool is the owning tool (firecrawl, googleworkspace, ...) used for
// provenance reporting. Raw preserves the original fields so callers can
// inspect anything else the lockfile carried without us guessing its schema.
type LockEntry struct {
	Name       string
	SourceTool string
	Raw        map[string]any
}

// The exact lockfile schema is not standardized across tools, so we parse
// defensively. Two shapes are supported:
//
//	(a) nested: {"skills": {"<name>": {<fields>}}}
//	(b) flat:   {"<name>": {<fields>}}
//
// For each entry the source tool is read from the first present of:
// "sourceTool", "source", "tool", "owner". Unknown shapes degrade gracefully
// to an empty SourceTool rather than failing.
func sourceToolFromRaw(raw map[string]any) string {
	for _, key := range []string{"sourceTool", "source", "tool", "owner"} {
		if v, ok := raw[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// ReadLock reads agentsDir/.skill-lock.json and returns its entries keyed by
// skill name. The lockfile is never modified.
//
// Behavior:
//   - file does not exist  => empty map, nil error
//   - malformed JSON       => empty (non-nil) map + non-fatal error (caller may ignore)
//   - valid                => populated map, nil error
func ReadLock(agentsDir string) (map[string]LockEntry, error) {
	entries := make(map[string]LockEntry)

	data, err := os.ReadFile(filepath.Join(agentsDir, lockFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return entries, err
	}

	// Parse into a tolerant top-level map first.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return entries, errors.New("malformed lockfile: " + err.Error())
	}

	// Shape (a): nested "skills" object. Shape (b): flat top-level map.
	if skillsRaw, ok := top["skills"]; ok {
		var nested map[string]map[string]any
		if err := json.Unmarshal(skillsRaw, &nested); err != nil {
			return entries, errors.New("malformed lockfile skills: " + err.Error())
		}
		for name, raw := range nested {
			entries[name] = LockEntry{
				Name:       name,
				SourceTool: sourceToolFromRaw(raw),
				Raw:        raw,
			}
		}
		return entries, nil
	}

	for name, rawMsg := range top {
		var raw map[string]any
		if err := json.Unmarshal(rawMsg, &raw); err != nil {
			// Not an object value (e.g. metadata field) — skip leniently.
			continue
		}
		entries[name] = LockEntry{
			Name:       name,
			SourceTool: sourceToolFromRaw(raw),
			Raw:        raw,
		}
	}

	return entries, nil
}

// Provenance returns the source tool recorded for a skill name, or "" if the
// name is not present in the lockfile entries.
func Provenance(entries map[string]LockEntry, name string) string {
	if e, ok := entries[name]; ok {
		return e.SourceTool
	}
	return ""
}
