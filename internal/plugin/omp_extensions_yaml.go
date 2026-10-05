package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

func ompPlanRevision(revision string, changes []OMPExtensionChange) string {
	sorted := slices.Clone(changes)
	slices.SortFunc(sorted, func(a, b OMPExtensionChange) int { return strings.Compare(a.Key, b.Key) })
	raw, _ := json.Marshal([]any{revision, sorted})
	return hash(raw)
}

// Replace just the disabledExtensions entry. Unknown values and unrelated
// comments keep their original bytes rather than round-tripping the whole file.
func ompRenderDisabled(raw []byte, root, disabled *yaml.Node) ([]byte, error) {
	index := -1
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "disabledExtensions" {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("missing disabledExtensions node")
	}
	key := *root.Content[index]
	key.HeadComment = ""
	key.FootComment = ""
	pair := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{&key, disabled}}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(pair); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	lines := strings.SplitAfter(string(raw), "\n")
	if key.Line == 0 {
		prefix := string(raw)
		if prefix != "" && !strings.HasSuffix(prefix, "\n") {
			prefix += "\n"
		}
		return []byte(prefix + out.String()), nil
	}
	start := key.Line - 1
	end := len(lines)
	if index+2 < len(root.Content) {
		end = root.Content[index+2].Line - 1
	}
	// Blank lines/comments immediately before the next key belong to untouched
	// content, not to the disabled list. Preserve their spelling and position.
	for end > start+1 {
		text := strings.TrimSpace(lines[end-1])
		if text != "" && !strings.HasPrefix(text, "#") {
			break
		}
		end--
	}
	return []byte(strings.Join(lines[:start], "") + out.String() + strings.Join(lines[end:], "")), nil
}

func (s *Service) ompBackup(path string, before, after []byte, id string) (string, error) {
	selection := func(raw []byte) (map[string]any, error) {
		var doc map[string]any
		if len(bytes.TrimSpace(raw)) > 0 {
			if err := yaml.Unmarshal(raw, &doc); err != nil {
				return nil, err
			}
		}
		value, exists := doc["disabledExtensions"]
		return map[string]any{"present": exists, "disabledExtensions": value}, nil
	}
	old, err := selection(before)
	if err != nil {
		return "", err
	}
	next, err := selection(after)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(s.StateDir, "omp-extensions", "backups")
	if err := ompSafePath(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(map[string]any{"path": path, "beforeHash": hash(before), "afterHash": hash(after), "before": old, "after": next}, "", "  ")
	if err != nil {
		return "", err
	}
	record := filepath.Join(dir, id+".json")
	if err := atomicNativeWrite(s.StateDir, record, append(raw, '\n'), 0600); err != nil {
		return "", err
	}
	return id, nil
}
