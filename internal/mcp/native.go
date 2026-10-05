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
	"gopkg.in/yaml.v3"
)

// Native is a parsed target document; edits preserve unrelated source bytes.
type Native struct {
	Target  string
	Entries map[string]map[string]any
	data    []byte
	off     []string // claude-off: the list in file order, for index-based removal
	json    hujson.Value
	yaml    yaml.Node
}

// claudeOffPrefix names the one destination that is not a map of servers: the list of
// server names Claude Code keeps per project in ~/.claude.json, which /mcp edits. The
// project root rides in the target so a backup can find the list again. Internal only:
// validTarget rejects it, so no source can select it.
const claudeOffPrefix = "claude-off:"

// shownTarget is the Agent a plan reports for a native target.
func shownTarget(target string) string {
	if strings.HasPrefix(target, claudeOffPrefix) {
		return "claude"
	}
	return target
}

func claudeProject(root string) string { return "/projects/" + pointerKey(root) }

func isTOMLTarget(target string) bool { return target == "codex" || target == "grok" }

func nativeKey(target string) string {
	if format, ok := clientFormats[target]; ok {
		return format.key
	}
	if isTOMLTarget(target) {
		return "mcp_servers"
	}
	if openCodeFormat(target) {
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
	root, offList := strings.CutPrefix(target, claudeOffPrefix)
	if !validTarget(target) && !offList {
		return nil, fmt.Errorf("unsupported MCP target")
	}
	n := &Native{Target: target, data: data, Entries: map[string]map[string]any{}}
	var document map[string]any
	if target == "goose" {
		if len(data) == 0 {
			data = []byte("{}\n")
			n.data = data
		}
		if err := parseYAML(data, &n.yaml); err != nil {
			return nil, err
		}
		if err := plainYAML(&n.yaml); err != nil {
			return nil, err
		}
		if err := n.yaml.Decode(&document); err != nil {
			return nil, fmt.Errorf("invalid Goose YAML configuration")
		}
	} else if isTOMLTarget(target) {
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
	if offList {
		projects, _ := document["projects"].(map[string]any)
		project, _ := projects[root].(map[string]any)
		list, _ := project["disabledMcpServers"].([]any)
		for _, raw := range list {
			if name, ok := raw.(string); ok {
				n.off = append(n.off, name)
				n.Entries[name] = map[string]any{}
			}
		}
		return n, nil
	}
	var piEntries map[string]json.RawMessage
	if target == "pi" {
		standard := n.json.Clone()
		standard.Standardize()
		var doc struct {
			Servers map[string]json.RawMessage `json:"mcpServers"`
		}
		_ = json.Unmarshal(standard.Pack(), &doc)
		piEntries = doc.Servers
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
			if target == "pi" {
				var options PiOptions
				if json.Unmarshal(piEntries[name], &options) == nil && options["toolExposure"] != nil {
					entry["toolExposure"] = options["toolExposure"]
				}
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
	if n.Target == "goose" {
		return n.editYAML(changes)
	}
	v := n.json.Clone()
	if root, ok := strings.CutPrefix(n.Target, claudeOffPrefix); ok {
		return n.editOffList(v, root, changes)
	}
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
	layOut(&v, nativeKey(n.Target), changes)
	out := v.Pack()
	if _, err := ParseNative(n.Target, out); err != nil {
		return nil, err
	}
	return out, nil
}

// editOffList adds and removes names in Claude Code's per-project off list. Everything
// else in ~/.claude.json, Claude Code's own state, stays byte for byte.
func (n *Native) editOffList(v hujson.Value, root string, changes map[string]map[string]any) ([]byte, error) {
	var patches []map[string]any
	list := claudeProject(root) + "/disabledMcpServers"
	for _, step := range []struct {
		path  string
		value any
	}{{"/projects", map[string]any{}}, {claudeProject(root), map[string]any{}}, {list, []any{}}} {
		if v.Find(step.path) == nil {
			patches = append(patches, map[string]any{"op": "add", "path": step.path, "value": step.value})
		}
	}
	// Remove from the end so earlier indexes stay valid.
	for i := len(n.off) - 1; i >= 0; i-- {
		if entry, changed := changes[n.off[i]]; changed && entry == nil {
			patches = append(patches, map[string]any{"op": "remove", "path": fmt.Sprintf("%s/%d", list, i)})
		}
	}
	for _, name := range sortedKeys(changes) {
		if _, exists := n.Entries[name]; changes[name] != nil && !exists {
			patches = append(patches, map[string]any{"op": "add", "path": list + "/-", "value": name})
		}
	}
	patch, err := json.Marshal(patches)
	if err != nil {
		return nil, err
	}
	if len(patches) > 0 {
		if err := v.Patch(patch); err != nil {
			return nil, fmt.Errorf("cannot safely edit Claude Code's project settings")
		}
	}
	if len(bytes.TrimSpace(n.data)) <= 2 {
		v.Format() // a new file, or the dashboard's preview: nobody's layout to keep
	}
	out := v.Pack()
	if _, err := ParseNative(n.Target, out); err != nil {
		return nil, err
	}
	return out, nil
}

// cramped reports an entry that sits on one line: what Skillshare wrote before it laid
// entries out, or a minifier's work. An entry someone formatted by hand has line breaks
// and is left alone.
func (n *Native) cramped(name string) bool {
	if isTOMLTarget(n.Target) || n.Target == "goose" || n.json.Value == nil || strings.HasPrefix(n.Target, claudeOffPrefix) {
		return false
	}
	v := n.json.Find("/" + nativeKey(n.Target) + "/" + pointerKey(name))
	if v == nil {
		return false
	}
	object, ok := v.Value.(*hujson.Object)
	return ok && len(object.Members) > 0 && !bytes.Contains(v.Pack(), []byte("\n"))
}

// layOut gives the entries this edit wrote one line per field, at the indent the file already
// uses. Patch inserts values without any whitespace, which left every server on a single line.
// Only members named in changes are touched, so the rest of the file keeps the owner's layout.
func layOut(v *hujson.Value, key string, changes map[string]map[string]any) {
	root, ok := v.Value.(*hujson.Object)
	if !ok || len(root.Members) == 0 {
		return
	}
	indent := "  "
	if before := string(root.Members[0].Name.BeforeExtra); strings.Contains(before, "\n") {
		if ws := before[strings.LastIndex(before, "\n")+1:]; ws != "" && strings.TrimLeft(ws, " \t") == "" {
			indent = ws
		}
	}
	for i := range root.Members {
		member := &root.Members[i]
		section, ok := member.Value.Value.(*hujson.Object)
		if !ok || literalString(member.Name) != key {
			continue
		}
		if len(member.Name.BeforeExtra) == 0 { // the section itself was just created
			member.Name.BeforeExtra = []byte("\n" + indent)
			member.Value.BeforeExtra = []byte(" ")
		}
		for j := range section.Members {
			entry := changes[literalString(section.Members[j].Name)]
			if entry == nil {
				continue
			}
			var text bytes.Buffer
			encoder := json.NewEncoder(&text)
			encoder.SetEscapeHTML(false)
			encoder.SetIndent(indent+indent, indent)
			if encoder.Encode(entry) != nil {
				continue
			}
			parsed, err := hujson.Parse(bytes.TrimSpace(text.Bytes()))
			if err != nil {
				continue
			}
			section.Members[j].Name.BeforeExtra = []byte("\n" + indent + indent)
			parsed.BeforeExtra = []byte(" ")
			section.Members[j].Value = parsed
		}
		if len(section.Members) > 0 && !bytes.Contains(section.AfterExtra, []byte("\n")) {
			section.AfterExtra = []byte("\n" + indent)
		}
	}
	if !bytes.Contains(root.AfterExtra, []byte("\n")) { // a file that started empty
		root.AfterExtra = []byte("\n")
	}
}

func literalString(v hujson.Value) string {
	if lit, ok := v.Value.(hujson.Literal); ok {
		return lit.String()
	}
	return ""
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
	"antigravity": {"command", "args", "env", "serverUrl", "headers", "enabled", "disabled"},
	"claude":      {"type", "command", "args", "env", "url", "headers", "enabled", "disabled"},
	"cursor":      {"type", "command", "args", "env", "url", "headers", "enabled", "disabled"},
	"vscode":      {"type", "command", "args", "env", "url", "headers", "enabled", "disabled"},
	"opencode":    {"type", "command", "environment", "url", "headers", "enabled", "disabled"},
	"kilocode":    {"type", "command", "environment", "url", "headers", "enabled", "disabled"},
	"codex":       {"command", "args", "env", "env_vars", "url", "http_headers", "env_http_headers", "bearer_token_env_var", "enabled_tools", "disabled_tools", "enabled", "disabled"},
	"grok":        {"command", "args", "env", "url", "headers", "enabled", "disabled"},
}

// managedEntry is the part of a native entry that ownership hashes cover. It
// drops defaults Render adds or omits (stdio/http type, empty collections,
// enabled: true) and header name case, so hand-written equivalents match.
func managedEntry(target string, entry map[string]any) map[string]any {
	if entry == nil {
		return nil
	}
	subset := map[string]any{}
	for _, key := range additionalManagedFields(target) {
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
	if out["enabled"] == true || (target == "pi" || target == "omp") && (out["command"] != nil || out["url"] != nil) {
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
		// A field the config sets wins over the one in the file, e.g. Pi's directTools.
		if _, set := want[key]; !set && (!slices.Contains(additionalManagedFields(target), key) || (target == "pi" || target == "omp") && key == "enabled") {
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
