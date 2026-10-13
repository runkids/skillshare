package config

import (
	"bytes"
	"errors"
	"os"
	"reflect"
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

// marshalConfigFile encodes v to replace the file at path, keeping that file's
// comments. A file that cannot be read for any reason other than being absent
// is an error: writing over it would silently drop its comments.
func marshalConfigFile(path string, v any, header []byte) ([]byte, error) {
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return marshalConfig(v, header, existing)
}

// keepComments copies the comments of existing onto the same keys of m. No
// comment is dropped: one on a key that was removed moves in front of the next
// key that is still present, or to the end of its section. Comments m already
// carries (raw sections such as hooks) win, and the schema directive is
// dropped, since marshalConfig writes its own header. Anchors and aliases are
// restored the same way, by relinkAliases.
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
	pairs := map[*yaml.Node]*yaml.Node{}
	copyComments(top, m, pairs)
	relinkAliases(m, pairs)
}

// copyComments copies the comments of old onto cur and records every old node
// with the node that replaced it in pairs.
func copyComments(old, cur *yaml.Node, pairs map[*yaml.Node]*yaml.Node) {
	pairs[old] = cur
	// A target that changed between `- claude` and `- name: claude` keeps its
	// line comment on the name, the only place a block mapping can show one.
	if old.Kind != cur.Kind {
		if o, c := nameScalar(old), nameScalar(cur); o != nil && c != nil && c.LineComment == "" {
			c.LineComment = o.LineComment
		}
	}
	if cur.HeadComment == "" {
		cur.HeadComment = old.HeadComment
	}
	if cur.LineComment == "" {
		cur.LineComment = old.LineComment
	}
	if cur.FootComment == "" {
		cur.FootComment = old.FootComment
	}
	// orphaned collects the comments of removed entries until a surviving one
	// takes them in front, or the section takes them at its end.
	var orphaned string
	switch {
	case old.Kind == yaml.MappingNode && cur.Kind == yaml.MappingNode:
		for i := 0; i+1 < len(old.Content); i += 2 {
			j := 0
			for ; j+1 < len(cur.Content); j += 2 {
				if cur.Content[j].Value == old.Content[i].Value {
					break
				}
			}
			if j+1 >= len(cur.Content) {
				orphaned = joinComments(orphaned, pairComments(old.Content[i], old.Content[i+1]))
				continue
			}
			copyComments(old.Content[i], cur.Content[j], pairs)
			copyComments(old.Content[i+1], cur.Content[j+1], pairs)
			cur.Content[j].HeadComment = joinComments(orphaned, cur.Content[j].HeadComment)
			orphaned = ""
		}
	case old.Kind == yaml.SequenceNode && cur.Kind == yaml.SequenceNode:
		used := make([]bool, len(cur.Content))
		for _, item := range old.Content {
			j := 0
			for ; j < len(cur.Content); j++ {
				if !used[j] && itemIdentity(item) != "" && itemIdentity(item) == itemIdentity(cur.Content[j]) {
					break
				}
			}
			if j == len(cur.Content) {
				orphaned = joinComments(orphaned, allComments(item))
				continue
			}
			used[j] = true
			copyComments(item, cur.Content[j], pairs)
			cur.Content[j].HeadComment = joinComments(orphaned, cur.Content[j].HeadComment)
			orphaned = ""
		}
	default:
		return
	}
	// A trailing orphan goes on the last entry: yaml.v3 prints a container's
	// own foot comment after the next key instead.
	switch {
	case orphaned == "":
	case cur.Kind == yaml.MappingNode && len(cur.Content) >= 2:
		last := cur.Content[len(cur.Content)-2]
		last.FootComment = joinComments(last.FootComment, orphaned)
	case cur.Kind == yaml.SequenceNode && len(cur.Content) >= 1:
		last := cur.Content[len(cur.Content)-1]
		last.FootComment = joinComments(last.FootComment, orphaned)
	default:
		cur.FootComment = joinComments(cur.FootComment, orphaned)
	}
}

// aliasGroup is an anchor of the replaced file with the nodes that still show
// its value: the anchored node itself and every alias that was not edited.
type aliasGroup struct {
	name  string
	src   *yaml.Node // the anchored node, whose current content the group shows
	first *yaml.Node // the group's first node in document order, which gets the anchor
}

// relinkAliases restores the anchors and aliases the struct encoding expanded.
// An anchored node keeps its anchor. An alias stays an alias while its value is
// the anchor's old value or equals the anchor's current one, so editing an
// anchored node reaches its aliases, as the file said it should; an alias
// whose value was edited to something else keeps its own copy. When the
// anchored node was removed, its aliases keep their own copies. An anchor
// stays where its node survives even if no alias is left, and a merge key
// (`<<: *base`) is not restored: the merged keys are written out.
func relinkAliases(cur *yaml.Node, pairs map[*yaml.Node]*yaml.Node) {
	members := map[*yaml.Node]*aliasGroup{}
	for old, c := range pairs {
		if c.Kind == yaml.AliasNode {
			continue
		}
		if old.Anchor != "" {
			c.Anchor = old.Anchor
		}
		if old.Kind != yaml.AliasNode {
			continue
		}
		src := pairs[old.Alias]
		if src == nil || !(sameValue(old.Alias, c) || sameValue(src, c)) {
			continue
		}
		g := members[src]
		if g == nil {
			g = &aliasGroup{name: old.Value, src: src}
			members[src] = g
		}
		members[c] = g
	}
	var place func(n *yaml.Node)
	place = func(n *yaml.Node) {
		if g := members[n]; g != nil {
			g.place(n)
		}
		for _, child := range n.Content {
			place(child)
		}
	}
	place(cur)
}

// place makes n the group's anchor if it is the first node reached, and an
// alias of it otherwise. Output order is not the old file's order, so the
// anchor moves to wherever the group appears first: an alias may not precede
// its anchor.
func (g *aliasGroup) place(n *yaml.Node) {
	head, line, foot := n.HeadComment, n.LineComment, n.FootComment
	if g.first != nil {
		*n = yaml.Node{Kind: yaml.AliasNode, Value: g.name, Alias: g.first}
	} else {
		g.first = n
		if n != g.src {
			*n = *g.src
		}
		n.Anchor = g.name
	}
	n.HeadComment, n.LineComment, n.FootComment = head, line, foot
}

// sameValue reports whether two nodes decode to the same data, with aliases
// resolved and mapping order ignored.
func sameValue(a, b *yaml.Node) bool {
	var x, y any
	return a.Decode(&x) == nil && b.Decode(&y) == nil && reflect.DeepEqual(x, y)
}

// pairComments gathers the comments of a mapping entry as they appeared: the
// key's head and line, the value's, then the key's foot, which yaml.v3 parses
// from the lines after the entry.
func pairComments(key, val *yaml.Node) string {
	return joinComments(key.HeadComment, key.LineComment, allComments(val), key.FootComment)
}

// allComments gathers every comment in a subtree, in document order.
func allComments(n *yaml.Node) string {
	out := joinComments(n.HeadComment, n.LineComment)
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			out = joinComments(out, pairComments(n.Content[i], n.Content[i+1]))
		}
	} else {
		for _, c := range n.Content {
			out = joinComments(out, allComments(c))
		}
	}
	return joinComments(out, n.FootComment)
}

// itemIdentity names a sequence item: its value for a scalar, and for a
// mapping its `name` value (so `- claude` and `- name: claude` are the same
// target) or, failing that, its first scalar key and value. Items without one
// ("" for a nested list) never match.
func itemIdentity(n *yaml.Node) string {
	switch n.Kind {
	case yaml.AliasNode:
		return itemIdentity(n.Alias)
	case yaml.ScalarNode:
		return n.Value
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == "name" && n.Content[i+1].Kind == yaml.ScalarNode {
				return n.Content[i+1].Value
			}
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i+1].Kind == yaml.ScalarNode {
				return n.Content[i].Value + "=" + n.Content[i+1].Value
			}
		}
	}
	return ""
}

// nameScalar is the node a sequence item's identity comes from: the scalar
// itself, or a mapping's `name` value.
func nameScalar(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.ScalarNode {
		return n
	}
	for i := 0; n.Kind == yaml.MappingNode && i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "name" && n.Content[i+1].Kind == yaml.ScalarNode {
			return n.Content[i+1]
		}
	}
	return nil
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
