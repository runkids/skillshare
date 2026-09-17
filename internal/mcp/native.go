package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/tailscale/hujson"
)

// Native is a parsed target document; edits preserve unrelated source bytes.
type Native struct {
	Target  string
	Entries map[string]map[string]any
	data    []byte
	json    hujson.Value
}

func isTOMLTarget(target string) bool { return target == "codex" || target == "grok" }

func nativeKey(target string) string {
	if isTOMLTarget(target) {
		return "mcp_servers"
	}
	if target == "opencode" {
		return "mcp"
	}
	if target == "vscode" {
		return "servers"
	}
	return "mcpServers"
}

func uniqueJSON(v *hujson.Value) error {
	if obj, ok := v.Value.(*hujson.Object); ok {
		seen := map[string]bool{}
		for i := range obj.Members {
			member := &obj.Members[i]
			var name string
			literal, ok := member.Name.Value.(hujson.Literal)
			if !ok {
				return fmt.Errorf("invalid JSON property")
			}
			if err := json.Unmarshal(literal, &name); err != nil {
				return fmt.Errorf("invalid JSON property")
			}
			if seen[name] {
				return fmt.Errorf("duplicate JSON property")
			}
			seen[name] = true
			if err := uniqueJSON(&member.Value); err != nil {
				return err
			}
		}
	}
	if arr, ok := v.Value.(*hujson.Array); ok {
		for i := range arr.Elements {
			if err := uniqueJSON(&arr.Elements[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

// ParseNative fails closed on malformed files and duplicate JSON properties.
func ParseNative(target string, data []byte) (*Native, error) {
	if !validTarget(target) {
		return nil, fmt.Errorf("unsupported MCP target")
	}
	n := &Native{Target: target, data: data, Entries: map[string]map[string]any{}}
	var document map[string]any
	if isTOMLTarget(target) {
		if err := toml.Unmarshal(data, &document); err != nil {
			return nil, fmt.Errorf("invalid TOML; target was not changed")
		}
	} else {
		if len(data) == 0 {
			data = []byte("{}\n")
			n.data = data
		}
		var err error
		n.json, err = hujson.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("invalid JSON/JSONC; target was not changed")
		}
		if err := uniqueJSON(&n.json); err != nil {
			return nil, err
		}
		standard := n.json.Clone()
		standard.Standardize()
		if err := json.Unmarshal(standard.Pack(), &document); err != nil || document == nil {
			return nil, fmt.Errorf("target configuration must be an object")
		}
	}
	if raw, ok := document[nativeKey(target)]; ok {
		servers, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("native MCP section must be an object")
		}
		for name, raw := range servers {
			entry, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("native MCP entry must be an object")
			}
			n.Entries[name] = entry
		}
	}
	return n, nil
}

func pointerKey(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// Edit replaces or removes whole owned entries, preserving all other entries.
func (n *Native) Edit(changes map[string]map[string]any) ([]byte, error) {
	if len(changes) == 0 {
		return append([]byte(nil), n.data...), nil
	}
	if isTOMLTarget(n.Target) {
		return n.editTOML(changes)
	}
	v := n.json.Clone()
	key := "/" + nativeKey(n.Target)
	var patches []map[string]any
	if v.Find(key) == nil {
		patches = append(patches, map[string]any{"op": "add", "path": key, "value": map[string]any{}})
	}
	names := make([]string, 0, len(changes))
	for name := range changes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := changes[name]
		if entry == nil {
			if _, exists := n.Entries[name]; exists {
				patches = append(patches, map[string]any{"op": "remove", "path": key + "/" + pointerKey(name)})
			}
		} else {
			patches = append(patches, map[string]any{"op": "add", "path": key + "/" + pointerKey(name), "value": entry})
		}
	}
	if len(patches) == 0 {
		return append([]byte(nil), n.data...), nil
	}
	// Keep URLs such as ?a=1&b=2 readable instead of \u0026-escaped.
	var patch bytes.Buffer
	encoder := json.NewEncoder(&patch)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(patches); err != nil {
		return nil, err
	}
	if err := v.Patch(patch.Bytes()); err != nil {
		return nil, fmt.Errorf("cannot safely edit native MCP entries")
	}
	out := v.Pack()
	if _, err := ParseNative(n.Target, out); err != nil {
		return nil, err
	}
	return out, nil
}

func entryHash(entry map[string]any) string {
	if entry == nil {
		return ""
	}
	data, _ := json.Marshal(entry)
	return digest(data)
}

func sameEntry(a, b map[string]any) bool { return entryHash(a) == entryHash(b) }

// managedFields are the native fields Render writes, plus the switches that turn
// a server off. Anything else, such as a timeout or envFile, belongs to the
// Agent and survives synchronization.
var managedFields = map[string][]string{
	"claude":   {"type", "command", "args", "env", "url", "headers", "enabled", "disabled"},
	"cursor":   {"type", "command", "args", "env", "url", "headers", "enabled", "disabled"},
	"vscode":   {"type", "command", "args", "env", "url", "headers", "enabled", "disabled"},
	"opencode": {"type", "command", "environment", "url", "headers", "enabled", "disabled"},
	"codex":    {"command", "args", "env", "env_vars", "url", "http_headers", "env_http_headers", "bearer_token_env_var", "enabled", "disabled"},
	"grok":     {"command", "args", "env", "url", "headers", "enabled", "disabled"},
}

// managedEntry is the part of a native entry that ownership hashes cover. It
// drops defaults Render adds or omits (stdio/http type, empty collections,
// enabled: true) and header name case, so hand-written equivalents match.
func managedEntry(target string, entry map[string]any) map[string]any {
	if entry == nil {
		return nil
	}
	subset := map[string]any{}
	for _, key := range managedFields[target] {
		if value, ok := entry[key]; ok {
			subset[key] = value
		}
	}
	// Rendered entries use typed maps and slices; compare their JSON shape.
	data, _ := json.Marshal(subset)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	for key, value := range out {
		switch v := value.(type) {
		case map[string]any:
			if len(v) == 0 {
				delete(out, key)
			} else if strings.Contains(key, "headers") {
				lower := map[string]any{}
				for name, item := range v {
					lower[strings.ToLower(name)] = item
				}
				out[key] = lower
			}
		case []any:
			if len(v) == 0 {
				delete(out, key)
			}
		}
	}
	if out["enabled"] == true {
		delete(out, "enabled")
	}
	if out["disabled"] == false {
		delete(out, "disabled")
	}
	if kind := out["type"]; out["command"] != nil && kind == "stdio" || out["url"] != nil && (kind == "http" || kind == "streamable-http") {
		delete(out, "type")
	}
	return out
}

// sectionDigest identifies only the MCP entries, so an Agent may rewrite its
// other settings between preview and apply.
func sectionDigest(n *Native) string {
	data, _ := json.Marshal(n.Entries)
	return digest(data)
}

// withAgentFields keeps the Agent-only fields of the current entry in a rewrite.
func withAgentFields(target string, current, want map[string]any) map[string]any {
	out := maps.Clone(want)
	for key, value := range current {
		if !slices.Contains(managedFields[target], key) {
			out[key] = value
		}
	}
	return out
}

func trimTrailingComments(data []byte, start, end int) int {
	for end > start {
		lineEnd := end
		if data[lineEnd-1] == '\n' {
			lineEnd--
		}
		lineStart := bytes.LastIndexByte(data[start:lineEnd], '\n') + start + 1
		line := bytes.TrimSpace(data[lineStart:lineEnd])
		if len(line) != 0 && line[0] != '#' {
			break
		}
		end = lineStart
	}
	return end
}
