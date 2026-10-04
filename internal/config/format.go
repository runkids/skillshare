package config

import (
	"bytes"
	"sort"

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
func marshalConfig(v any, header []byte) ([]byte, error) {
	var doc yaml.Node
	if err := doc.Encode(v); err != nil {
		return nil, err
	}
	if doc.Kind == yaml.MappingNode {
		sortSections(&doc)
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
