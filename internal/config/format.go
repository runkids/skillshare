package config

import (
	"bytes"
	"cmp"
	"errors"
	"maps"
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
// dropped, since marshalConfig writes its own header. Anchors, aliases and
// merge keys are restored the same way, by relinkAliases.
func keepComments(m *yaml.Node, existing []byte) {
	var old yaml.Node
	if yaml.Unmarshal(existing, &old) != nil || len(old.Content) == 0 || old.Content[0].Kind != yaml.MappingNode {
		return
	}
	top := old.Content[0]
	fixAnchorComments(top)
	top.HeadComment = withoutSchemaDirective(joinComments(old.HeadComment, top.HeadComment))
	for i := 0; i < len(top.Content); i += 2 {
		top.Content[i].HeadComment = withoutSchemaDirective(top.Content[i].HeadComment)
	}
	top.FootComment = joinComments(top.FootComment, old.FootComment)
	l := &links{pairs: map[*yaml.Node]*yaml.Node{}}
	copyComments(top, m, l)
	relinkAliases(m, l)
	settleLineComments(m, false)
}

// fixAnchorComments gives the comment that follows an anchor on a block mapping
// (`base: &base # shared`) a line of its own. yaml.v3 parses it as the line
// comment of the mapping's first key, where it collides with the line comment
// of that key's value, and only the value's is written.
func fixAnchorComments(n *yaml.Node) {
	if n.Kind == yaml.MappingNode && n.Anchor != "" && len(n.Content) >= 2 &&
		n.Content[0].LineComment != "" && n.Content[1].Kind != yaml.MappingNode && n.Content[1].Kind != yaml.SequenceNode {
		first := n.Content[0]
		first.HeadComment = joinComments(first.LineComment, first.HeadComment)
		first.LineComment = ""
	}
	for _, c := range n.Content {
		fixAnchorComments(c)
	}
}

// settleLineComments moves comments to where yaml.v3 writes them and the next
// parse reads them back from the same place:
//   - a block collection's line comment (what the comment of an alias written
//     out as a copy, or of an anchor that moved onto it, becomes) is dropped, so
//     it goes in front of the first entry;
//   - so does the head comment of an anchored one, which would be written in
//     front of its `- ` or key and parsed into it on the next write;
//   - a key's line comment is lost or lands on the next key when its value is
//     an empty collection, so it goes to the value; with an anchor on the value
//     it is written after the anchor name's line, so it goes in front of the
//     first entry.
//
// Inside a flow collection, which yaml.v3 writes for any node under it, comments
// stay on their node.
func settleLineComments(n *yaml.Node, inFlow bool) {
	inFlow = inFlow || n.Style&yaml.FlowStyle != 0
	for i, c := range n.Content {
		isCollection := c.Kind == yaml.MappingNode || c.Kind == yaml.SequenceNode
		empty := isCollection && len(c.Content) == 0
		block := isCollection && !empty && c.Style&yaml.FlowStyle == 0 && !inFlow
		if n.Kind == yaml.MappingNode && i%2 == 1 && (empty || block && c.Anchor != "") {
			k := n.Content[i-1]
			c.LineComment = joinComments(k.LineComment, c.LineComment)
			k.LineComment = ""
		}
		if block {
			lead := c.LineComment
			if c.Anchor != "" {
				lead, c.HeadComment = joinComments(c.HeadComment, lead), ""
			}
			c.Content[0].HeadComment = joinComments(lead, c.Content[0].HeadComment)
			c.LineComment = ""
		}
		settleLineComments(c, inFlow)
	}
}

// links records how the nodes of the replaced file map onto the new ones.
type links struct {
	pairs  map[*yaml.Node]*yaml.Node // old node -> the node that replaced it
	merges []mergeEntry              // merge keys of the old file, for relinkMerge
}

// mergeEntry is a merge key (`<<: *base`) of an old mapping and the mapping
// that replaced it, which has the merged keys written out.
type mergeEntry struct{ old, cur, key, val *yaml.Node }

// copyComments copies the comments of old onto cur and records every old node
// with the node that replaced it in l.
func copyComments(old, cur *yaml.Node, l *links) {
	l.pairs[old] = cur
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
				if old.Content[i].Tag == "!!merge" {
					// relinkMerge writes it back with its comments, or hands them on.
					l.merges = append(l.merges, mergeEntry{old, cur, old.Content[i], old.Content[i+1]})
					continue
				}
				orphaned = joinComments(orphaned, pairComments(old.Content[i], old.Content[i+1]))
				continue
			}
			copyComments(old.Content[i], cur.Content[j], l)
			copyComments(old.Content[i+1], cur.Content[j+1], l)
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
			copyComments(item, cur.Content[j], l)
			cur.Content[j].HeadComment = joinComments(orphaned, cur.Content[j].HeadComment)
			orphaned = ""
		}
	default:
		return
	}
	footOrphans(cur, orphaned)
}

// footOrphans puts the comments of removed entries that no entry follows on the
// last entry of cur: yaml.v3 prints a container's own foot comment after the
// next key instead.
func footOrphans(cur *yaml.Node, orphaned string) {
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

// aliasGroups maps each node of a group to it, the anchored node included.
type aliasGroups map[*yaml.Node]*aliasGroup

// add makes n show the value of src, creating src's group on first use.
func (m aliasGroups) add(src, n *yaml.Node, name string) {
	g := m[src]
	if g == nil {
		g = &aliasGroup{name: name, src: src}
		m[src] = g
	}
	m[n] = g
}

// relinkAliases restores the anchors and aliases the struct encoding expanded.
// An anchored node keeps its anchor. An alias stays an alias while its value is
// the anchor's old value or equals the anchor's current one, so editing an
// anchored node reaches its aliases, as the file said it should; an alias
// whose value was edited to something else keeps its own copy. When the
// anchored node was removed, its aliases keep their own copies. An anchor
// stays where its node survives even if no alias is left. A merge key
// (`<<: *base`) is restored by relinkMerge, which follows the same rules.
func relinkAliases(cur *yaml.Node, l *links) {
	l.pairAnchoredMerges()
	members := aliasGroups{}
	for old, c := range l.pairs {
		if c.Kind == yaml.AliasNode {
			continue
		}
		if old.Anchor != "" {
			c.Anchor = old.Anchor
		}
		if old.Kind != yaml.AliasNode {
			continue
		}
		src := l.pairs[old.Alias]
		if src == nil || !(sameValue(old.Alias, c) || sameValue(src, c)) {
			continue
		}
		members.add(src, c, old.Value)
	}
	// relinkMerge may add entries: an inline merge source has merge keys of its own.
	for i := 0; i < len(l.merges); i++ {
		l.relinkMerge(l.merges[i], members)
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

// pairAnchoredMerges pairs every mapping that carries an anchor on a merge key
// (`<<: &base {…}`, where an earlier write put it) with the node of an alias of
// it: the first one still showing its value, or if every one was edited, the
// first in the file. The mapping has no key of its own to pair by, and that
// alias is where the anchor was written before it moved.
func (l *links) pairAnchoredMerges() {
	for _, e := range l.merges {
		for _, n := range mergeItems(e.val) {
			if n.Anchor == "" || n.Kind != yaml.MappingNode {
				continue
			}
			var best *yaml.Node
			bestSame := false
			for old, c := range l.pairs {
				if old.Alias != n || c.Kind != yaml.MappingNode {
					continue
				}
				same := sameValue(n, c)
				first := best != nil && cmp.Or(cmp.Compare(old.Line, best.Line), cmp.Compare(old.Column, best.Column)) < 0
				if best == nil || same && !bestSame || same == bestSame && first {
					best, bestSame = old, same
				}
			}
			if best != nil {
				l.pairs[n] = l.pairs[best]
			}
		}
	}
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

// relinkMerge writes the merge key of e.old back onto e.cur in place of the
// merged keys the struct encoding expanded, so what is written decodes to the
// data e.cur holds, except where the edit of an anchored mapping is meant to
// reach this one:
//   - a key the old mapping set itself stays written out, even when it equals
//     the merged value;
//   - a key whose value is the one its merge source gives it is dropped, and so
//     is one left alone while the source that gave it was edited, so an edit of
//     an anchored mapping (a value changed, a key added or removed) reaches the
//     mappings merging it;
//   - any other key stays written out and replaces the merged value as a whole,
//     since merges are shallow;
//   - a source that was removed, or that has a key e.old had and e.cur lost, is
//     not merged: its keys stay written out, because a merge could not drop them.
//
// Comments follow the entry they were on: a merge item's to the alias that
// replaces it, and those of an item no longer merged or a key the merge now
// supplies to the next entry still written, as copyComments does for a removed key.
//
// Decoded data cannot tell a key the command left alone from one it set to the
// old merged value, so an edit that does both to the same key reads as left alone.
func (l *links) relinkMerge(e mergeEntry, groups aliasGroups) {
	items := mergeItems(e.val)
	oldData, curData := mapData(e.old), mapData(e.cur)
	local := map[string]bool{}
	for i := 0; i+1 < len(e.old.Content); i += 2 {
		local[e.old.Content[i].Value] = true
	}
	type source struct {
		node *yaml.Node
		data map[string]any
		item *yaml.Node // the merge item it came from, whose comments it keeps
	}
	var used []source
	var dropped string
	// follows reports for each key whether the first merge item that gave it
	// before is still merged.
	follows := map[string]bool{}
	for _, item := range items {
		src := item
		if item.Kind == yaml.AliasNode {
			src = item.Alias
		}
		kept := false
		if node := l.pairs[src]; node != nil && node.Kind == yaml.MappingNode && curData != nil {
			if d := mapData(node); d != nil && !dropsKey(oldData, curData, d) {
				// The comments of the items dropped in front of it head it.
				item.HeadComment = joinComments(dropped, item.HeadComment)
				used = append(used, source{node, d, item})
				dropped, kept = "", true
				if item == src {
					// An inline source has no key of its own to pair its comments by.
					body := *item
					body.HeadComment, body.LineComment, body.FootComment = "", "", ""
					copyComments(&body, node, l)
				}
			}
		}
		if !kept {
			dropped = joinComments(dropped, allComments(item))
		}
		for k := range mapData(item) {
			if _, seen := follows[k]; !seen {
				follows[k] = kept
			}
		}
	}
	if len(used) == 0 {
		comments := pairComments(e.key, e.val)
		if len(e.cur.Content) > 0 {
			e.cur.Content[0].HeadComment = joinComments(comments, e.cur.Content[0].HeadComment)
		} else {
			e.cur.FootComment = joinComments(e.cur.FootComment, comments)
		}
		return
	}
	// A key is merged when the first source that has it gives its value, or when
	// the command left it alone and its old source is still merged, whatever that
	// source now says, including nothing.
	merged := map[string]any{}
	for i := len(used) - 1; i >= 0; i-- {
		maps.Copy(merged, used[i].data)
	}
	covered := func(k string) bool {
		if local[k] {
			return false
		}
		v, ok := merged[k]
		return ok && reflect.DeepEqual(curData[k], v) ||
			follows[k] && reflect.DeepEqual(curData[k], oldData[k])
	}
	// Every used node has its anchor by now: relinkAliases set it from the old file.
	aliases := make([]*yaml.Node, len(used))
	for n, s := range used {
		aliases[n] = &yaml.Node{
			Kind: yaml.AliasNode, Value: s.node.Anchor, Alias: s.node,
			HeadComment: s.item.HeadComment, LineComment: s.item.LineComment, FootComment: s.item.FootComment,
		}
		groups.add(s.node, aliases[n], s.node.Anchor)
	}
	end := aliases[len(aliases)-1]
	end.FootComment = joinComments(end.FootComment, dropped)
	val := aliases[0]
	if e.val.Kind == yaml.SequenceNode {
		val = &yaml.Node{Kind: yaml.SequenceNode, Style: e.val.Style & yaml.FlowStyle, Content: aliases}
	}
	val.HeadComment, val.LineComment, val.FootComment = e.val.HeadComment, e.val.LineComment, e.val.FootComment
	// yaml.v3 prints a parsed merge key with its !!merge tag; the untagged
	// `<<` is the same key.
	key := *e.key
	key.Tag = ""
	content := []*yaml.Node{&key, val}
	var orphaned string
	for i := 0; i+1 < len(e.cur.Content); i += 2 {
		k, v := e.cur.Content[i], e.cur.Content[i+1]
		if covered(k.Value) {
			orphaned = joinComments(orphaned, pairComments(k, v))
			continue
		}
		k.HeadComment = joinComments(orphaned, k.HeadComment)
		orphaned = ""
		content = append(content, k, v)
	}
	e.cur.Content = content
	if len(content) == 2 {
		// A foot comment would follow the merged mapping when the anchor moves
		// onto it, and be parsed into that mapping on the next write.
		key.HeadComment = joinComments(key.HeadComment, orphaned)
	} else {
		footOrphans(e.cur, orphaned)
	}
}

// mergeItems lists the sources of a merge key: its value, or the items of a list.
func mergeItems(val *yaml.Node) []*yaml.Node {
	if val.Kind == yaml.SequenceNode {
		return val.Content
	}
	return []*yaml.Node{val}
}

// mapData decodes a mapping node, with aliases and merges resolved. It is nil
// for any other node.
func mapData(n *yaml.Node) map[string]any {
	var m map[string]any
	if n.Decode(&m) != nil {
		return nil
	}
	return m
}

// dropsKey reports whether old had a key of src that cur lost. A key src gained
// is not one: old never had it.
func dropsKey(old, cur, src map[string]any) bool {
	for k := range src {
		_, inOld := old[k]
		_, inCur := cur[k]
		if inOld && !inCur {
			return true
		}
	}
	return false
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
