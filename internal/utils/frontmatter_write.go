package utils

import (
	"os"
	"strings"

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

	data, err = RewriteFrontmatterList(data, field, values)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// RewriteFrontmatterList returns updated frontmatter while preserving the body.
// It has the same field and removal semantics as SetFrontmatterList.
func RewriteFrontmatterList(data []byte, field string, values []string) ([]byte, error) {
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

	parts := strings.SplitN(field, ".", 2)

	if len(parts) == 2 && parts[0] == "metadata" {
		subField := parts[1]
		md, _ := fm["metadata"].(map[string]any)
		if md == nil {
			md = make(map[string]any)
		}

		if values == nil {
			delete(md, subField)
		} else {
			list := make([]any, len(values))
			for i, v := range values {
				list[i] = v
			}
			md[subField] = list
		}

		if len(md) == 0 {
			delete(fm, "metadata")
		} else {
			fm["metadata"] = md
		}

		// Always remove legacy top-level field when operating on metadata.<field>
		delete(fm, subField)
	} else {
		topField := parts[0]
		if values == nil {
			delete(fm, topField)
		} else {
			list := make([]any, len(values))
			for i, v := range values {
				list[i] = v
			}
			fm[topField] = list
		}
	}

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
// body are never touched.
func ToggleFrontmatterFlag(filePath, key string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}
	fm := splitFrontmatterLines(string(data))
	flagLine := key + ": true" + fm.cr
	if fm.end < 0 {
		return true, os.WriteFile(filePath, []byte(fm.prepend(flagLine)), 0644)
	}

	lines := fm.lines
	on := true
	i := fm.keyLine(key)
	switch {
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
	} else if i, j := fm.keySpan(m, key); i >= 0 {
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
	bom       string // a leading UTF-8 BOM, kept in front of whatever is written back
	content   string
	cr        string
	lines     []string
	open, end int
}

func splitFrontmatterLines(content string) frontmatterLines {
	bom, content := SplitBOM(content)
	fm := frontmatterLines{bom: bom, content: content, lines: strings.Split(content, "\n"), open: -1, end: -1}
	if strings.Contains(content, "\r\n") {
		fm.cr = "\r"
	}
	isDelim := func(l string) bool { return strings.TrimRight(l, " \t\r") == "---" }
	for i, l := range fm.lines {
		if fm.open < 0 {
			if isDelim(l) {
				fm.open = i
			} else if strings.TrimSpace(l) != "" {
				break // body starts before any delimiter: no frontmatter
			}
		} else if isDelim(l) {
			fm.end = i
			break
		}
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
func (fm frontmatterLines) keySpan(mapping *yaml.Node, key string) (int, int) {
	if mapping == nil {
		i := fm.keyLine(key)
		return i, i + 1
	}
	m := mapping.Content
	for k := 0; k+1 < len(m); k += 2 {
		if m[k].Value != key {
			continue
		}
		start, end := fm.open+m[k].Line, fm.end // Line is 1-based from the line after "---"
		if k+2 < len(m) {
			end = fm.open + m[k+2].Line
		}
		for end > start+1 {
			if l := strings.TrimSpace(fm.lines[end-1]); l != "" && !strings.HasPrefix(fm.lines[end-1], "#") {
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
