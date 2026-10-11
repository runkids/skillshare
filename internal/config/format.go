package config

import (
	"bytes"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"skillshare/internal/utils"
)

// configSections is the top-level key order for config files. It matches
// CONFIG_SECTIONS in ui/src/lib/formatYaml.ts; keep the two lists in sync.
var configSections = []string{
	"sources", "source", "agents_source", "extras_source", "git_root", "mode", "target_naming",
	"targets", "skills", "agents", "extras", "mcp", "plugins", "hooks",
	"projects", "hub", "log", "context_budget", "tui", "gitlab_hosts", "azure_hosts", "ignore", "audit",
}

// marshalConfig encodes v the way the dashboard's Beautify lays out a config:
// the schema header first, top-level sections in configSections order (unknown
// keys last, in their encoded order), and a blank line between sections.
// existing is the file being replaced, or nil; its comments are kept on every
// key that is still present.
func marshalConfig(v any, header []byte, existing []byte) ([]byte, error) {
	var doc yaml.Node
	if err := doc.Encode(v); err != nil {
		return nil, err
	}
	if doc.Kind == yaml.MappingNode {
		sortSections(&doc)
		keepComments(&doc, existing)
	}
	data, err := utils.MarshalYAML(&doc)
	if err != nil {
		return nil, err
	}
	out := append([]byte{}, header...)
	out = append(out, '\n')
	return append(out, spaceSections(data)...), nil
}

func sortSections(m *yaml.Node) {
	rank := func(key string) int {
		for i, s := range configSections {
			if s == key {
				return i
			}
		}
		return len(configSections)
	}
	pairs := make([][2]*yaml.Node, 0, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		pairs = append(pairs, [2]*yaml.Node{m.Content[i], m.Content[i+1]})
	}
	original := m.Content
	sort.SliceStable(pairs, func(i, j int) bool {
		return rank(pairs[i][0].Value) < rank(pairs[j][0].Value)
	})
	m.Content = make([]*yaml.Node, 0, len(original))
	for _, p := range pairs {
		m.Content = append(m.Content, p[0], p[1])
	}
	// Keep the original order if moving sections would put an alias before its anchor.
	if !aliasesFollowAnchors(m, map[string]bool{}) {
		m.Content = original
	}
}

// keepComments copies the comments of existing onto the same keys of m. A key
// that was removed loses its comments; a key that was added has none. Comments
// m already carries (raw sections such as hooks) win, and the schema directive
// is dropped, since marshalConfig writes its own header.
func keepComments(m *yaml.Node, existing []byte) {
	var old yaml.Node
	if yaml.Unmarshal(existing, &old) != nil || len(old.Content) == 0 || old.Content[0].Kind != yaml.MappingNode {
		return
	}
	top := old.Content[0]
	top.HeadComment = withoutSchemaDirective(joinComments(old.HeadComment, top.HeadComment))
	for i := 0; i < len(top.Content); i += 2 {
		top.Content[i].HeadComment = withoutSchemaDirective(top.Content[i].HeadComment)
	}
	top.FootComment = joinComments(top.FootComment, old.FootComment)
	copyComments(top, m)
}

func copyComments(old, cur *yaml.Node) {
	if cur.HeadComment == "" {
		cur.HeadComment = old.HeadComment
	}
	if cur.LineComment == "" {
		cur.LineComment = old.LineComment
	}
	if cur.FootComment == "" {
		cur.FootComment = old.FootComment
	}
	switch {
	case old.Kind == yaml.MappingNode && cur.Kind == yaml.MappingNode:
		for i := 0; i+1 < len(old.Content); i += 2 {
			for j := 0; j+1 < len(cur.Content); j += 2 {
				if cur.Content[j].Value == old.Content[i].Value {
					copyComments(old.Content[i], cur.Content[j])
					copyComments(old.Content[i+1], cur.Content[j+1])
					break
				}
			}
		}
	case old.Kind == yaml.SequenceNode && cur.Kind == yaml.SequenceNode:
		for _, item := range old.Content {
			for _, c := range cur.Content {
				if sameItem(item, c) {
					copyComments(item, c)
					break
				}
			}
		}
	}
}

// sameItem matches a sequence item by its value, or for a mapping item by its
// first key and value (`- name: claude`), so an added or removed item does not
// shift the comments of the others.
func sameItem(a, b *yaml.Node) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case yaml.ScalarNode:
		return a.Value == b.Value
	case yaml.MappingNode:
		return len(a.Content) >= 2 && len(b.Content) >= 2 &&
			a.Content[0].Value == b.Content[0].Value && a.Content[1].Value == b.Content[1].Value
	}
	return false
}

func joinComments(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "\n")
}

func withoutSchemaDirective(comment string) string {
	var kept []string
	for _, line := range strings.Split(comment, "\n") {
		if !strings.HasPrefix(line, "# yaml-language-server:") {
			kept = append(kept, line)
		}
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func aliasesFollowAnchors(n *yaml.Node, anchors map[string]bool) bool {
	if n.Kind == yaml.AliasNode && !anchors[n.Value] {
		return false
	}
	if n.Anchor != "" {
		anchors[n.Anchor] = true
	}
	for _, child := range n.Content {
		if !aliasesFollowAnchors(child, anchors) {
			return false
		}
	}
	return true
}

// spaceSections inserts a blank line before each top-level key, or before the
// comment block that heads it. Nested content is always indented, so a line
// starting in column 0 (other than a sequence item) begins a section.
func spaceSections(data []byte) []byte {
	lines := bytes.SplitAfter(data, []byte("\n"))
	var out bytes.Buffer
	prev := []byte(nil)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		top := line[0] != ' ' && line[0] != '\n' && line[0] != '-'
		if top && prev != nil && prev[0] != '\n' && prev[0] != '#' {
			out.WriteByte('\n')
		}
		out.Write(line)
		prev = line
	}
	return out.Bytes()
}
