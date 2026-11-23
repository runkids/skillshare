package utils

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// SetFrontmatterList writes a YAML list field into a SKILL.md file's frontmatter.
// The field parameter uses dot notation: "metadata.targets" operates on fm["metadata"]["targets"].
// When values is nil, the field is removed. When non-nil, the field is set.
// For "metadata.targets", any legacy top-level "targets" field is also removed.
// All other frontmatter fields and the body content are preserved.
func SetFrontmatterList(filePath string, field string, values []string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	out, err := RewriteFrontmatterList(data, field, values)
	if err != nil {
		return err
	}
	if bytes.Equal(out, data) {
		return nil
	}
	return os.WriteFile(filePath, out, 0644)
}

// RewriteFrontmatterList returns updated frontmatter while preserving the body.
// It has the same field and removal semantics as SetFrontmatterList.
func RewriteFrontmatterList(data []byte, field string, values []string) ([]byte, error) {
	if field == "metadata.targets" || field == "targets" {
		return rewriteTargets(data, field, values)
	}
	return rewriteFrontmatterListLegacy(data, field, values)
}

func rewriteFrontmatterListLegacy(data []byte, field string, values []string) ([]byte, error) {
	bom, rest := SplitBOM(string(data))
	fmRaw, body := splitFrontmatterAndBody([]byte(rest))

	var fm map[string]any
	if len(fmRaw) > 0 {
		if err := yaml.Unmarshal(fmRaw, &fm); err != nil {
			return nil, err
		}
	}
	if fm == nil {
		fm = make(map[string]any)
	}

	applyListField(fm, field, values, false)

	fmBytes, err := MarshalYAML(fm)
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(bom)
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n")
	sb.Write(body)

	return []byte(sb.String()), nil
}

// applyListField describes the intended change independently of its source layout.
func applyListField(fm map[string]any, field string, values []string, preserveAbsent bool) {
	set := func(parent map[string]any, key string) {
		if values == nil {
			delete(parent, key)
			return
		}
		list := make([]any, len(values))
		for i, v := range values {
			list[i] = v
		}
		parent[key] = list
	}
	if key, ok := strings.CutPrefix(field, "metadata."); ok {
		md, _ := fm["metadata"].(map[string]any)
		if _, present := md[key]; !preserveAbsent || present || values != nil {
			if md == nil {
				md = make(map[string]any)
			}
			set(md, key)
			if len(md) == 0 {
				delete(fm, "metadata")
			} else {
				fm["metadata"] = md
			}
		}
		delete(fm, key) // migrate the legacy root field
		return
	}
	key, _, _ := strings.Cut(field, ".")
	set(fm, key)
}

// ErrTargetFrontmatter identifies a file layout the target editor cannot safely
// change. The remedy also reaches folder-batch clients via their errors list.
var ErrTargetFrontmatter = errors.New("cannot safely edit target frontmatter: edit targets in the file directly or use an unindented YAML mapping with explicit target values instead of aliases")

// errUnlocatedHeader tells the user what to fix when a writer will not put a header over
// one it cannot locate.
var errUnlocatedHeader = errors.New("frontmatter --- lines are indented, unclosed or below other text")

// hidesHeader reports whether a new header on top of content would hide one the readers
// use but the writer could not locate: unclosed, indented or below other text. opened says
// the writer saw an opening delimiter that never closes; that counts even when the text is
// not valid YAML, since the readers that scan line by line still read it.
func hidesHeader(content []byte, opened bool) bool {
	return opened || len(decodeFrontmatter(locateFrontmatter(content, lenientBlock).withoutLastNewline())) != 0
}

// refuseTargetEdit keeps the reason next to the remedy for API clients.
func refuseTargetEdit(reason any) error {
	return fmt.Errorf("%w: %v", ErrTargetFrontmatter, reason)
}

// Verify that a bounded edit did not alter any unowned data before callers write it.
func rewriteTargets(data []byte, field string, values []string) ([]byte, error) {
	beforeBlock := locateFrontmatter(data, rewriteBlock)
	if !beforeBlock.closed {
		if slices.Equal(ParseFrontmatterListFromBytes(data, "targets"), values) {
			return data, nil // readers already see the requested targets
		}
		if hidesHeader(data, beforeBlock.open) {
			return nil, refuseTargetEdit(errUnlocatedHeader)
		}
		return rewriteFrontmatterListLegacy(data, field, values)
	}
	var before, after map[string]any
	if err := yaml.Unmarshal(beforeBlock.raw, &before); err != nil {
		return nil, refuseTargetEdit(err)
	}
	if before == nil {
		before = make(map[string]any)
	}
	// applyListField cannot describe an edit inside a mapping with non-string keys.
	if mixed, ok := before["metadata"].(map[any]any); ok && field == "metadata.targets" {
		if _, nested := mixed["targets"]; nested || values != nil {
			return nil, refuseTargetEdit("metadata has non-string keys")
		}
	}
	want := maps.Clone(before)
	if md, ok := want["metadata"].(map[string]any); ok {
		want["metadata"] = maps.Clone(md)
	}
	applyListField(want, field, values, true)
	if reflect.DeepEqual(before, want) {
		return data, nil // Includes absent targets under aliases and merge keys.
	}
	out, block := data, beforeBlock
	var err error
	if _, legacy := before["targets"]; legacy && field == "metadata.targets" {
		out, err = spliceTarget(out, block, "targets", nil)
		if err == nil {
			block = locateFrontmatter(out, rewriteBlock)
		}
	}
	if err == nil {
		out, err = spliceTarget(out, block, field, values)
	}
	if err != nil {
		return nil, refuseTargetEdit(err)
	}
	afterBlock := locateFrontmatter(out, rewriteBlock)
	if !afterBlock.closed || !bytes.Equal(beforeBlock.body, afterBlock.body) || yaml.Unmarshal(afterBlock.raw, &after) != nil {
		return nil, refuseTargetEdit("the edit would break the header or change the body")
	}
	if after == nil {
		after = make(map[string]any)
	}
	if !reflect.DeepEqual(want, after) {
		return nil, refuseTargetEdit("the edit would change other fields")
	}
	return out, nil
}

// spliceTarget changes one owned entry. The caller verifies the final document.
func spliceTarget(data []byte, block frontmatterBlock, field string, values []string) ([]byte, error) {
	bom, rest := SplitBOM(string(data))
	var doc yaml.Node
	if err := yaml.Unmarshal(block.raw, &doc); err != nil {
		return nil, err
	}
	fm := splitKnownBlock(rest, "", block, true)
	cr := ""
	if strings.HasSuffix(fm.lines[fm.open], "\r") {
		cr = "\r"
	}
	root := &yaml.Node{Kind: yaml.MappingNode}
	if len(doc.Content) > 0 {
		if doc.Content[0].Kind != yaml.MappingNode {
			// The caller decoded a mapping, so this is an explicit null with nothing to preserve.
			return rewriteFrontmatterListLegacy(data, field, values)
		}
		root = doc.Content[0]
	}
	if root.Style&yaml.FlowStyle == 0 && len(root.Content) != 0 && root.Column != 1 {
		return nil, errors.New("use an unindented root mapping")
	}
	nested := field == "metadata.targets"
	md := mappingValue(root, "metadata")
	if nested && md != nil && md.Kind == yaml.AliasNode && values != nil {
		return nil, errors.New("metadata is a YAML alias")
	}
	// Choose the owned entry once, regardless of block or flow syntax.
	parent, key := root, "targets"
	if nested {
		switch {
		case md == nil || md.Kind != yaml.MappingNode:
			key = "metadata"
			if values == nil {
				return data, nil
			}
		case values == nil && len(md.Content) == 2 && mappingValue(md, "targets") != nil:
			key = "metadata" // its only field will be removed
		default:
			parent = md
		}
	}
	lines := fm.lines
	if parent.Style&yaml.FlowStyle != 0 {
		entry := targetEntry(values)
		if key == "metadata" && entry != nil {
			wrapped := "metadata: {" + *entry + "}"
			entry = &wrapped
		}
		out, err := editFlowMap(string(block.raw), parent, key, entry)
		if err != nil {
			return nil, err
		}
		lines = append(lines[:fm.open+1], append(strings.Split(strings.TrimSuffix(out, "\n"), "\n"), lines[fm.end:]...)...)
		return []byte(bom + strings.Join(lines, "\n")), nil
	}
	limit, insert, indent := fm.end, fm.end, ""
	var added []string
	if key == "metadata" && values != nil {
		added, indent = []string{"metadata:" + cr}, "  "
	} else if parent != root {
		i, j := fm.keySpan(root, "metadata", fm.end)
		if i < 0 || len(parent.Content) == 0 {
			return nil, errors.New("cannot locate the metadata mapping")
		}
		limit, insert = j, lfLine(fm, parent.Content[0])
		if insert <= i || insert >= j {
			return nil, errors.New("cannot locate the first metadata key")
		}
		indent = lines[insert][:len(lines[insert])-len(strings.TrimLeft(lines[insert], " \t"))]
	}
	i, j := fm.keySpan(parent, key, limit)
	if i < 0 {
		if mappingValue(parent, key) != nil {
			return nil, fmt.Errorf("cannot locate the lines of %q", key)
		}
		i, j = insert, insert
	}
	if values == nil {
		if i < j {
			lines = removeTargetSpan(lines, i, j)
		}
	} else {
		encoded, err := MarshalYAML(values)
		if err != nil {
			return nil, err
		}
		added = append(added, indent+"targets:"+cr)
		for _, line := range strings.Split(strings.TrimSuffix(string(encoded), "\n"), "\n") {
			added = append(added, indent+"  "+line+cr)
		}
		lines = append(lines[:i], append(added, lines[j:]...)...)
	}
	return []byte(bom + strings.Join(lines, "\n")), nil
}

// lfLine converts parser coordinates to the physical LF-delimited line index.
func lfLine(fm frontmatterLines, n *yaml.Node) int {
	if !fm.targetEdit {
		return fm.open + n.Line // scalar line editor uses the original parser-index policy
	}
	raw := strings.Join(fm.lines[fm.open+1:fm.end], "\n")
	at := nodeOffset(raw, n)
	if at < 0 {
		return -1
	}
	return fm.open + 1 + strings.Count(raw[:at], "\n")
}

// Blank lines after a removed entry belong to that entry's position, not to a
// preceding keep-chomp scalar. Preserve comments while dropping those blanks.
func removeTargetSpan(lines []string, start, end int) []string {
	next := end
	for next < len(lines) {
		trimmed := strings.TrimSpace(lines[next])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			break
		}
		next++
	}
	kept := make([]string, 0, next-end)
	for _, line := range lines[end:next] {
		if strings.TrimSpace(line) != "" {
			kept = append(kept, line)
		}
	}
	return append(lines[:start], append(kept, lines[next:]...)...)
}

func targetEntry(values []string) *string {
	if values == nil {
		return nil
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
	for _, value := range values {
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
	}
	out, _ := MarshalYAML(seq)
	entry := "targets: " + strings.TrimSpace(string(out))
	return &entry
}

// nodeOffset converts YAML's one-based rune positions into byte offsets.
func nodeOffset(text string, n *yaml.Node) int {
	line, col := 1, 1
	for i := 0; i < len(text); {
		if line == n.Line && col == n.Column {
			return i
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size
		switch r {
		case '\r':
			if i < len(text) && text[i] == '\n' {
				i++
			}
			line++
			col = 1
		case '\n', '\u0085', '\u2028', '\u2029':
			line++
			col = 1
		default:
			col++
		}
	}
	return -1
}

// yaml.v3 gives the start but not the end of a flow value. Only consume a
// spelling its encoder can reproduce exactly, without adjacent comments.
func flowValueEnd(text string, n *yaml.Node) int {
	at := nodeOffset(text, n)
	if at < 0 || n.Kind == yaml.AliasNode || n.Anchor != "" || n.Style&yaml.TaggedStyle != 0 {
		return -1
	}
	v := *n
	v.HeadComment, v.LineComment, v.FootComment = "", "", ""
	raw, err := MarshalYAML(&v)
	token := strings.TrimSuffix(string(raw), "\n")
	if err != nil || token == "" || !strings.HasPrefix(text[at:], token) {
		return -1
	}
	return at + len(token)
}

// Editing a flow entry never requires scanning siblings. A trailing comma is
// valid YAML, so removing the last entry can leave preceding comments intact.
func editFlowMap(text string, m *yaml.Node, key string, replacement *string) (string, error) {
	fail := errors.New("use a plain target list or a block mapping")
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value != key {
			continue
		}
		from, to := nodeOffset(text, m.Content[i]), flowValueEnd(text, m.Content[i+1])
		if from < 0 || to <= from {
			return "", fail
		}
		if replacement != nil {
			return text[:from] + *replacement + text[to:], nil
		}
		tail := strings.TrimLeft(text[to:], " \t")
		if strings.HasPrefix(tail, ",") {
			to = len(text) - len(tail) + 1
		}
		return text[:from] + text[to:], nil
	}
	if replacement == nil {
		return text, nil
	}
	at := nodeOffset(text, m)
	if at < 0 || text[at] != '{' {
		return "", fail
	}
	sep := ""
	if len(m.Content) > 0 {
		sep = ", "
	}
	return text[:at+1] + *replacement + sep + text[at+1:], nil
}

// splitFrontmatterAndBody splits SKILL.md content into raw frontmatter YAML
// and the remaining body. Returns (nil, content) if no frontmatter found.
func splitFrontmatterAndBody(content []byte) (fmRaw, body []byte) {
	block := locateFrontmatter(content, rewriteBlock)
	if !block.closed {
		return nil, content
	}
	return block.withoutLastNewline(), block.body
}

// ToggleFrontmatterFlag flips a top-level boolean frontmatter key and reports the new state.
// Absent or false becomes "key: true"; true removes the line, so toggling twice hands back
// the original file byte for byte — which keeps a tracked repo clean once the flag is off again.
// It edits one line instead of round-tripping YAML: key order, comments, line endings and the
// body are never touched. In a flow root, other existing true flags become false in place.
func ToggleFrontmatterFlag(filePath, key string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}
	fm := splitFrontmatterLines(string(data))
	flagLine := key + ": true" + fm.cr
	if fm.end < 0 {
		if hidesHeader(data, fm.open >= 0) {
			return false, errUnlocatedHeader
		}
		return true, os.WriteFile(filePath, []byte(fm.prepend(flagLine)), 0644)
	}

	lines := fm.lines
	on := true
	i := fm.keyLine(key)
	switch m := fm.mapping(); {
	case m != nil && m.Style&yaml.FlowStyle != 0:
		lines, on, err = toggleFlowFlag(fm, m, key)
		if err != nil {
			return false, err
		}
	case i < 0:
		lines = append(lines[:fm.end], append([]string{flagLine}, lines[fm.end:]...)...)
	case isTrueValue(lines[i][len(key)+1:]):
		lines = append(lines[:i], lines[i+1:]...)
		on = false
	default:
		lines[i] = flagLine
	}
	return on, os.WriteFile(filePath, []byte(fm.join(lines)), 0644)
}

// toggleFlowFlag makes one bounded source edit and verifies its decoded result.
func toggleFlowFlag(fm frontmatterLines, m *yaml.Node, key string) ([]string, bool, error) {
	fail := func() ([]string, bool, error) { return nil, false, fmt.Errorf("cannot safely edit flow flag %q", key) }
	raw := strings.Join(fm.lines[fm.open+1:fm.end], "\n")
	start := len(strings.Join(fm.lines[:fm.open+1], "\n")) + 1
	at := func(n *yaml.Node) int {
		if i := nodeOffset(raw, n); i >= 0 {
			return start + i
		}
		return -1
	}
	// The verification reader requires exact delimiters. Tell the user how to
	// fix a block that the line editor found but that reader would not see.
	if fm.open != 0 || !strictBlock.delim([]byte(fm.lines[fm.open])) || !strictBlock.delim([]byte(fm.lines[fm.end])) {
		return nil, false, fmt.Errorf("flow frontmatter needs exact --- delimiters starting on line 1")
	}
	expected, err := ParseFrontmatterMap([]byte(fm.join(fm.lines)))
	if err != nil {
		return fail()
	}
	text, inserted := fm.content, key+": true"
	from, to, replacement, on := -1, -1, "", true
	v := mappingValue(m, key)
	if v == nil {
		if len(m.Content) > 0 {
			from, replacement = at(m.Content[0]), inserted+", "
		} else if start := at(m); start >= 0 {
			if brace := strings.IndexByte(text[start:], '{'); brace >= 0 {
				from = start + brace + 1
				replacement = inserted
			}
		}
		to = from
		expected[key] = true
	} else {
		start := at(m.Content[0])
		first := m.Content[0].Value == key
		if start < 0 {
			return fail()
		}
		// Only the exact prefix inserted by this helper can be removed without
		// locating separators or disturbing comments.
		tail := text[start:]
		if first && isTrueValue(v.Value) && strings.HasPrefix(tail, inserted+", ") {
			from, to = start, start+len(inserted)+2
			delete(expected, key)
			on = false
		} else if first && len(m.Content) == 2 && isTrueValue(v.Value) && strings.HasPrefix(tail, inserted) && !strings.HasPrefix(strings.TrimLeft(tail[len(inserted):], " \t\r\n"), ",") {
			from, to = start, start+len(inserted)
			delete(expected, key)
			on = false
		} else {
			from = at(v)
			to = from + len(v.Value)
			if v.Style&(yaml.SingleQuotedStyle|yaml.DoubleQuotedStyle) != 0 {
				to += 2
			}
			on = !isTrueValue(v.Value)
			replacement = "false"
			if on {
				replacement = "true"
			}
			expected[key] = on
		}
	}
	if from < 0 || to < from || to > len(text) {
		return fail()
	}
	candidate := text[:from] + replacement + text[to:]
	after, err := ParseFrontmatterMap([]byte(fm.bom + candidate))
	if err != nil || !reflect.DeepEqual(after, expected) {
		return fail()
	}
	return strings.Split(candidate, "\n"), on, nil
}

// SetFrontmatterValue sets a top-level frontmatter key to a plain scalar value,
// adding the key (or the whole frontmatter block) when it is missing. Like
// ToggleFrontmatterFlag it edits only the key's lines (a missing key goes before the
// first one), so key order, comments, line endings and the body are kept; a
// flow-style mapping ({...}) is rewritten whole.
// value is written unquoted and must be a plain YAML scalar.
func SetFrontmatterValue(filePath, key, value string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	fm := splitFrontmatterLines(string(data))
	line := key + ": " + value + fm.cr
	if fm.end < 0 {
		// The one caller renames a fresh copy in a target. Refusing would fail the whole sync,
		// so the copy gets a new header holding only the name; a header only readers accept
		// (see hidesHeader) stays below it as body text.
		return os.WriteFile(filePath, []byte(fm.prepend(line)), 0644)
	}

	lines := fm.lines
	m := fm.mapping()
	if m != nil && m.Style&yaml.FlowStyle != 0 {
		// Flow-style keys share lines, so the mapping is written back whole.
		setMappingScalar(m, key, value)
		out, err := yaml.Marshal(m)
		if err != nil {
			return err
		}
		block := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
		for i := range block {
			block[i] += fm.cr
		}
		lines = append(lines[:fm.open+1], append(block, lines[fm.end:]...)...)
	} else if i, j := fm.keySpan(m, key, fm.end); i >= 0 {
		if v := mappingValue(m, key); v != nil {
			// Keep everything before the value as written (the key, its anchor, quoting,
			// spacing and indent) and replace only the value, keeping its own anchor for
			// aliases elsewhere. Column counts characters and points at an anchor or tag.
			i = fm.open + v.Line
			before := []rune(lines[i])
			before = before[:min(v.Column-1, len(before))]
			anchor := ""
			if v.Anchor != "" {
				anchor = "&" + v.Anchor + " "
			}
			line = string(before) + anchor + value + fm.cr
		}
		lines = append(lines[:i], append([]string{line}, lines[j:]...)...)
	} else if m != nil && len(m.Content) > 0 {
		// Before the first key, with that line's indent: inside the mapping whatever
		// follows it (a "..." end marker, comments), and an explicit key still
		// overrides one a merge key (<<) brings in, wherever it stands.
		at := fm.open + m.Content[0].Line
		indent := lines[at][:len(lines[at])-len(strings.TrimLeft(lines[at], " \t"))]
		lines = append(lines[:at], append([]string{indent + line}, lines[at:]...)...)
	} else {
		lines = append(lines[:fm.end], append([]string{line}, lines[fm.end:]...)...)
	}
	return os.WriteFile(filePath, []byte(fm.join(lines)), 0644)
}

// frontmatterLines is a file split on "\n" only, so CRLF files keep their "\r"
// on every untouched line; open and end index the "---" delimiters (end < 0
// when there is no frontmatter block).
type frontmatterLines struct {
	bom        string // a leading UTF-8 BOM, kept in front of whatever is written back
	content    string
	cr         string
	lines      []string
	open, end  int
	targetEdit bool // preserve nested mapping comments before the following key
}

func splitFrontmatterLines(content string) frontmatterLines {
	bom, content := SplitBOM(content)
	return splitKnownBlock(content, bom, locateFrontmatter([]byte(content), lineEditorBlock), false)
}

func splitKnownBlock(content, bom string, block frontmatterBlock, targetEdit bool) frontmatterLines {
	fm := frontmatterLines{bom: bom, content: content, lines: strings.Split(content, "\n"), open: -1, end: -1, targetEdit: targetEdit}
	if strings.Contains(content, "\r\n") {
		fm.cr = "\r"
	}
	if block.open {
		fm.open = block.openLine
	}
	if block.closed {
		fm.end = block.closeLine
	}
	return fm
}

// join returns lines as file content, with the BOM back in front.
func (fm frontmatterLines) join(lines []string) string {
	return fm.bom + strings.Join(lines, "\n")
}

// keyLine returns the index of the line holding key at column 0 (nested keys
// do not count), or -1.
func (fm frontmatterLines) keyLine(key string) int {
	for i := fm.open + 1; i < fm.end; i++ {
		if strings.HasPrefix(fm.lines[i], key+":") {
			return i
		}
	}
	return -1
}

// mappingValue returns key's value node in mapping node m, or nil.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for k := 0; m != nil && k+1 < len(m.Content); k += 2 {
		if m.Content[k].Value == key {
			return m.Content[k+1]
		}
	}
	return nil
}

// mapping parses the frontmatter into its top-level mapping node, or nil when
// it does not parse to one.
func (fm frontmatterLines) mapping() *yaml.Node {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(strings.Join(fm.lines[fm.open+1:fm.end], "\n")), &doc) != nil ||
		len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	return doc.Content[0]
}

// setMappingScalar sets key in mapping node m to a plain scalar, appending it when
// absent. A replaced value keeps its anchor, which aliases elsewhere may point at.
func setMappingScalar(m *yaml.Node, key, value string) {
	v := &yaml.Node{Kind: yaml.ScalarNode, Value: value}
	for k := 0; k+1 < len(m.Content); k += 2 {
		if m.Content[k].Value == key {
			v.Anchor = m.Content[k+1].Anchor
			m.Content[k+1] = v
			return
		}
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, v)
}

// keySpan returns the lines [start, end) holding top-level key and its whole
// value in block mapping node mapping, so "key": / key : / multi-line scalars count.
// Blank and column-0 comment lines before the next key stay outside the span.
// A nil mapping (frontmatter YAML cannot parse) falls back to keyLine; start is -1 when absent.
func (fm frontmatterLines) keySpan(mapping *yaml.Node, key string, limit int) (int, int) {
	if mapping == nil {
		i := fm.keyLine(key)
		return i, i + 1
	}
	m := mapping.Content
	for k := 0; k+1 < len(m); k += 2 {
		if m[k].Value != key {
			continue
		}
		start, end := lfLine(fm, m[k]), limit
		if k+2 < len(m) {
			end = lfLine(fm, m[k+2])
		}
		if start < 0 || end <= start || end > limit {
			return -1, -1
		}
		for end > start+1 {
			l := fm.lines[end-1]
			if fm.targetEdit {
				l = strings.TrimSpace(l)
			}
			if strings.TrimSpace(l) != "" && !strings.HasPrefix(l, "#") {
				break
			}
			end--
		}
		return start, end
	}
	return -1, -1
}

// prepend returns the content with a new frontmatter block holding line.
func (fm frontmatterLines) prepend(line string) string {
	return fm.bom + "---" + fm.cr + "\n" + line + "\n" + "---" + fm.cr + "\n" + fm.content
}

func isTrueValue(rest string) bool {
	v, _, _ := strings.Cut(rest, " #")
	return strings.EqualFold(strings.Trim(v, " \t\r\"'"), "true")
}
