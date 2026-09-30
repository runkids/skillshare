package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tailscale/hujson"
)

// nativeDoc is one shared hooks file of a command Agent: Claude, Gemini and Qwen
// settings.json, Codex and Cursor hooks.json under "hooks", and Droid's hooks.json,
// whose top level is the event map. Edits touch only the event arrays they change,
// so comments, order and every other setting keep their bytes.
type nativeDoc struct {
	target string
	data   []byte
	value  hujson.Value
	// events holds each event's elements in their JSON shape, in file order.
	events map[string][]any
	// hasSection reports an existing event map: a "hooks" key, or Droid's whole file.
	hasSection bool
}

// wrapped reports whether the event map sits under a "hooks" key.
func wrapped(target string) bool { return target != "droid" }

func uniqueJSON(v *hujson.Value) error {
	switch value := v.Value.(type) {
	case *hujson.Object:
		seen := map[string]bool{}
		for i := range value.Members {
			name := literalString(value.Members[i].Name)
			if seen[name] {
				return fmt.Errorf("duplicate JSON property %q", name)
			}
			seen[name] = true
			if err := uniqueJSON(&value.Members[i].Value); err != nil {
				return err
			}
		}
	case *hujson.Array:
		for i := range value.Elements {
			if err := uniqueJSON(&value.Elements[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func literalString(v hujson.Value) string {
	if lit, ok := v.Value.(hujson.Literal); ok {
		return lit.String()
	}
	return ""
}

// parseNative fails closed on malformed files, duplicate properties and event
// values that are not arrays; the file is then never edited.
func parseNative(target string, data []byte) (*nativeDoc, error) {
	n := &nativeDoc{target: target, data: data, events: map[string][]any{}}
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}\n")
		n.data = data
	}
	var err error
	if n.value, err = hujson.Parse(data); err != nil {
		return nil, fmt.Errorf("invalid JSON/JSONC; file was not changed")
	}
	if err := uniqueJSON(&n.value); err != nil {
		return nil, err
	}
	standard := n.value.Clone()
	standard.Standardize()
	var document map[string]any
	if err := json.Unmarshal(standard.Pack(), &document); err != nil || document == nil {
		return nil, fmt.Errorf("configuration must be a JSON object")
	}
	if target == "cursor" {
		if version, ok := document["version"]; ok && version != float64(1) {
			return nil, fmt.Errorf("unsupported Cursor hooks version %v; only version 1 is supported", version)
		}
	}
	section := document
	n.hasSection = true
	if wrapped(target) {
		raw, ok := document["hooks"]
		if !ok {
			n.hasSection = false
			return n, nil
		}
		if section, ok = raw.(map[string]any); !ok {
			return nil, fmt.Errorf("hooks must be an object")
		}
	}
	for event, raw := range section {
		items, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("hooks event %q must be an array", event)
		}
		n.events[event] = items
	}
	return n, nil
}

// elementHash identifies element content; ownership is never inferred from it alone.
func elementHash(v any) string {
	data, _ := json.Marshal(v)
	return digest(data)
}

// sectionDigest identifies only the hooks, so an Agent may rewrite its other
// settings between preview and apply.
func (n *nativeDoc) sectionDigest() string {
	data, _ := json.Marshal(n.events)
	return digest(data)
}

// elementOp changes one array element. Index is the element's position in the file
// as read; -1 appends. A nil Value removes the element at Index.
type elementOp struct {
	Key    string `json:"key"`
	Event  string `json:"event"`
	Index  int    `json:"index"`
	Value  any    `json:"value,omitempty"`
	Before any    `json:"before,omitempty"`
}

// edit applies ops and returns the new bytes plus the final index of each op key
// and each tracked key (event -> original index -> key). With drop, a hooks key the
// edit leaves empty is removed. The output keeps the file's style: a single-line file
// stays compact, an indented one keeps its indent unit. It fails when an op no longer
// fits the file, which callers have already ruled out by comparing section digests.
func (n *nativeDoc) edit(ops []elementOp, track map[string]map[int]string, drop bool) ([]byte, map[string]int, error) {
	positions := map[string]int{}
	if len(ops) == 0 {
		return append([]byte(nil), n.data...), positions, nil
	}
	v := n.value.Clone()
	root, ok := v.Value.(*hujson.Object)
	if !ok {
		return nil, nil, fmt.Errorf("configuration must be a JSON object")
	}
	indent := detectIndent(root)
	fresh := len(root.Members) == 0
	compact := !fresh && !bytes.Contains(bytes.TrimSpace(n.data), []byte("\n"))
	section := root
	depth := 1
	if wrapped(n.target) {
		if n.target == "cursor" && member(root, "version") == nil {
			addMember(root, "version", hujson.Value{Value: hujson.Int(1)}, indent)
		}
		m := member(root, "hooks")
		if m == nil {
			m = addMember(root, "hooks", hujson.Value{Value: &hujson.Object{}}, indent)
		}
		if section, ok = m.Value.Value.(*hujson.Object); !ok {
			return nil, nil, fmt.Errorf("hooks must be an object")
		}
		depth = 2
	}
	byEvent := map[string][]elementOp{}
	for _, op := range ops {
		byEvent[op.Event] = append(byEvent[op.Event], op)
	}
	for _, event := range sortedKeys(byEvent) {
		m := member(section, event)
		if m == nil {
			m = addMember(section, event, hujson.Value{Value: &hujson.Array{}}, strings.Repeat(indent, depth))
		}
		arr, ok := m.Value.Value.(*hujson.Array)
		if !ok {
			return nil, nil, fmt.Errorf("hooks event %q must be an array", event)
		}
		// ids follow each original element through the edit to its final index.
		ids := make([]string, len(arr.Elements))
		for i, key := range track[event] {
			if i < len(ids) {
				ids[i] = key
			}
		}
		removed := map[int]bool{}
		var appended []elementOp
		for _, op := range byEvent[event] {
			switch {
			case op.Index < 0:
				appended = append(appended, op)
			case op.Index >= len(arr.Elements):
				return nil, nil, fmt.Errorf("hooks event %q changed; preview again", event)
			case op.Value == nil:
				removed[op.Index] = true
			default:
				value, err := formatted(op.Value, indent, depth+1)
				if err != nil {
					return nil, nil, err
				}
				value.BeforeExtra = arr.Elements[op.Index].BeforeExtra
				arr.Elements[op.Index] = value
				ids[op.Index] = op.Key
			}
		}
		var kept []hujson.Value
		var keptIDs []string
		for i, element := range arr.Elements {
			if !removed[i] {
				kept = append(kept, element)
				keptIDs = append(keptIDs, ids[i])
			}
		}
		for _, op := range appended {
			value, err := formatted(op.Value, indent, depth+1)
			if err != nil {
				return nil, nil, err
			}
			value.BeforeExtra = hujson.Extra("\n" + strings.Repeat(indent, depth+1))
			kept = append(kept, value)
			keptIDs = append(keptIDs, op.Key)
		}
		if len(kept) == 0 && len(arr.Elements) > 0 {
			// Skillshare removed the last element, so it drops the event it emptied.
			removeMember(section, event)
			continue
		}
		arr.Elements = kept
		if len(appended) > 0 && !bytes.Contains(arr.AfterExtra, []byte("\n")) {
			arr.AfterExtra = hujson.Extra("\n" + strings.Repeat(indent, depth))
		}
		for i, id := range keptIDs {
			if id != "" {
				positions[id] = i
			}
		}
	}
	switch {
	case len(section.Members) == 0 && drop && section != root:
		removeMember(root, "hooks")
	case len(section.Members) == 0:
		section.AfterExtra = nil
	case !bytes.Contains(section.AfterExtra, []byte("\n")):
		section.AfterExtra = hujson.Extra("\n" + strings.Repeat(indent, depth-1))
	}
	if len(root.Members) == 0 {
		root.AfterExtra = nil
	} else if fresh || !bytes.Contains(root.AfterExtra, []byte("\n")) {
		root.AfterExtra = hujson.Extra("\n")
	}
	if compact {
		v.Minimize()
	}
	out := v.Pack()
	if !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	if _, err := parseNative(n.target, out); err != nil {
		return nil, nil, fmt.Errorf("cannot safely edit hooks: %w", err)
	}
	return out, positions, nil
}

func member(obj *hujson.Object, name string) *hujson.ObjectMember {
	for i := range obj.Members {
		if literalString(obj.Members[i].Name) == name {
			return &obj.Members[i]
		}
	}
	return nil
}

func addMember(obj *hujson.Object, name string, value hujson.Value, indent string) *hujson.ObjectMember {
	key := hujson.Value{BeforeExtra: hujson.Extra("\n" + indent), Value: hujson.String(name)}
	value.BeforeExtra = hujson.Extra(" ")
	obj.Members = append(obj.Members, hujson.ObjectMember{Name: key, Value: value})
	return &obj.Members[len(obj.Members)-1]
}

func removeMember(obj *hujson.Object, name string) {
	for i := range obj.Members {
		if literalString(obj.Members[i].Name) == name {
			obj.Members = append(obj.Members[:i], obj.Members[i+1:]...)
			return
		}
	}
}

// detectIndent reuses the file's indent unit, two spaces for a new file.
func detectIndent(root *hujson.Object) string {
	if len(root.Members) > 0 {
		before := string(root.Members[0].Name.BeforeExtra)
		if i := strings.LastIndex(before, "\n"); i >= 0 {
			if ws := before[i+1:]; ws != "" && strings.TrimLeft(ws, " \t") == "" {
				return ws
			}
		}
	}
	return "  "
}

// formatted encodes a value laid out at depth, so written elements stay readable.
func formatted(value any, indent string, depth int) (hujson.Value, error) {
	var text bytes.Buffer
	encoder := json.NewEncoder(&text)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(strings.Repeat(indent, depth), indent)
	if err := encoder.Encode(value); err != nil {
		return hujson.Value{}, err
	}
	return hujson.Parse(bytes.TrimSpace(text.Bytes()))
}

// safeRead refuses symlinks and non-regular files at native destinations.
func safeRead(path string) ([]byte, bool, os.FileMode, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, 0, nil
	}
	if err != nil {
		return nil, false, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, 0, fmt.Errorf("hooks file must be a regular file, not a symlink or directory: %s", path)
	}
	data, err := os.ReadFile(path)
	return data, true, info.Mode().Perm(), err
}
