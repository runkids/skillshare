package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseSkillName reads the SKILL.md and extracts the top-level "name" from frontmatter.
// It reads line by line and stops at a plain top-level name line ("name", 'name' and
// name : count; top-level keys share the first key's indent). Any other form YAML
// allows (flow mapping, block scalar, alias, merge key, explicit key, ...) leaves the
// scan without a name, and then the whole block is decoded.
func ParseSkillName(skillPath string) (string, error) {
	name, literal, indent := "", "", -1
	path := filepath.Join(skillPath, "SKILL.md")
	err := scanLenientBlock(path, func(raw []byte) bool {
		line := string(raw)
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" || trimmed[0] == '#' {
			return true
		}
		if indent < 0 {
			indent = len(line) - len(trimmed)
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok || len(line)-len(trimmed) != indent || strings.Trim(strings.TrimSpace(key), `"'`) != "name" {
			return true
		}
		if v := strings.TrimSpace(value); v != "" && strings.ContainsRune("*|>&!", rune(v[0])) {
			return false // alias, block scalar, anchor or tag: decode the whole block
		}
		// The line alone decodes quotes and a trailing comment. When it does not decode to
		// a string (a multi-line quoted scalar, a number), the whole block is decoded, and
		// the line read as is is kept for frontmatter YAML rejects.
		var fm map[string]any
		if yaml.Unmarshal([]byte("name:"+value), &fm) == nil {
			if v, isString := fm["name"].(string); isString {
				name = v
				return false
			}
		}
		literal = strings.Trim(strings.TrimSpace(value), `"'`)
		return false
	})
	if err != nil || name != "" {
		return name, err
	}
	raw, err := readLenientBlock(path)
	if err != nil {
		return "", err
	}
	if name, isString := decodeFrontmatter(raw)["name"].(string); isString {
		return name, nil
	}
	return literal, nil
}

// isYAMLBlockIndicator returns true for YAML block scalar indicators (>, >-, >+, |, |-, |+).
func isYAMLBlockIndicator(s string) bool {
	switch s {
	case ">", ">-", ">+", "|", "|-", "|+":
		return true
	}
	return false
}

// resolveField looks up a field in the frontmatter map.
// Priority: metadata.<field> > top-level <field>.
// Returns nil when the field is absent in both locations.
func resolveField(fm map[string]any, field string) any {
	if md, ok := fm["metadata"]; ok {
		if mdMap, ok := md.(map[string]any); ok {
			if val, ok := mdMap[field]; ok {
				return val
			}
		}
	}
	val, ok := fm[field]
	if !ok {
		return nil
	}
	return val
}

// ParseFrontmatterList reads a SKILL.md file and extracts a YAML list field from frontmatter.
// Supports both inline [a, b] and block (- a\n- b) formats.
// Returns nil when the field is absent or the file cannot be read.
func ParseFrontmatterList(filePath, field string) []string {
	raw, err := readLenientBlock(filePath)
	if err != nil {
		return nil
	}
	return stringList(decodeFrontmatter(raw), field)
}

// ParseFrontmatterListFromBytes parses a YAML list field from pre-read content.
// Same as ParseFrontmatterList but avoids re-reading the file.
func ParseFrontmatterListFromBytes(content []byte, field string) []string {
	raw := locateFrontmatter(content, lenientBlock).withoutLastNewline()
	return stringList(decodeFrontmatter(raw), field)
}

// stringList returns the string items of a list field, resolved by resolveField.
func stringList(fm map[string]any, field string) []string {
	list, _ := resolveField(fm, field).([]any)
	var result []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// decodeFrontmatter decodes the block a lenient reader found. It is nil when the block
// is empty or not a YAML mapping.
func decodeFrontmatter(raw []byte) map[string]any {
	var fm map[string]any
	if err := yaml.Unmarshal(raw, &fm); err != nil {
		return nil
	}
	return fm
}

// ParseFrontmatterMap returns the complete YAML frontmatter of SKILL.md content.
// The Agent Skills format requires the file to begin with it, closed by a second ---.
func ParseFrontmatterMap(content []byte) (map[string]any, error) {
	block := locateFrontmatter(content, strictBlock)
	if !block.open {
		return nil, fmt.Errorf("no frontmatter at the start")
	}
	if !block.closed {
		return nil, fmt.Errorf("unclosed frontmatter: no closing ---")
	}
	if len(bytes.TrimSpace(block.raw)) == 0 {
		return nil, fmt.Errorf("no frontmatter")
	}
	var fm map[string]any
	if err := yaml.Unmarshal(block.raw, &fm); err != nil {
		return nil, fmt.Errorf("invalid frontmatter: %w", err)
	}
	// A non-string key decodes to map[any]any, which JSON cannot carry.
	if _, err := json.Marshal(fm); err != nil {
		return nil, fmt.Errorf("frontmatter JSON cannot carry: %w", err)
	}
	return fm, nil
}

// ParseFrontmatterFields reads a SKILL.md file once and returns the values of
// multiple frontmatter fields. This avoids opening the same file repeatedly
// when multiple fields are needed (e.g. description + license).
// Note: does not resolve metadata.<field> — only reads top-level scalar fields.
// Use ParseFrontmatterMap for the complete structured frontmatter, including nested values.
func ParseFrontmatterFields(filePath string, fields []string) map[string]string {
	result := make(map[string]string, len(fields))
	if len(fields) == 0 {
		return result
	}

	raw, err := readLenientBlock(filePath)
	if err != nil {
		return result
	}
	fm := decodeFrontmatter(raw)

	for _, field := range fields {
		val, ok := fm[field]
		if !ok || val == nil {
			continue
		}
		switch v := val.(type) {
		case string:
			result[field] = v
		case int:
			result[field] = fmt.Sprintf("%d", v)
		case float64:
			result[field] = fmt.Sprintf("%g", v)
		case bool:
			result[field] = fmt.Sprintf("%t", v)
		}
	}

	return result
}

// ReadSkillBody reads a file and returns everything after the YAML frontmatter.
// If no frontmatter is present, the entire content is returned.
// Returns "" on read error.
func ReadSkillBody(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	block := locateFrontmatter(scanLines(data), bodyBlock)
	if !block.open {
		return strings.TrimSpace(string(data))
	}
	if !block.closed {
		return ""
	}
	return strings.TrimSpace(string(block.body))
}

// ParseFrontmatterField reads a SKILL.md file and extracts the value of a given frontmatter field.
// It supports both inline values and YAML block scalars (>, >-, |, |-).
func ParseFrontmatterField(filePath, field string) string {
	prefix := field + ":"
	indent := -1
	val := ""
	var blockParts []string // set once val is a block scalar indicator
	err := scanLenientBlock(filePath, func(raw []byte) bool {
		if blockParts != nil {
			// The block scalar continues while lines are indented
			if len(raw) > 0 && (raw[0] == ' ' || raw[0] == '\t') {
				blockParts = append(blockParts, strings.TrimSpace(string(raw)))
				return true
			}
			return false
		}
		line := strings.TrimSpace(string(raw))
		if line == "" || strings.HasPrefix(line, "#") {
			return true
		}
		lineIndent := len(raw) - len(bytes.TrimLeft(raw, " \t"))
		if indent < 0 {
			indent = lineIndent
		}
		if lineIndent != indent || !strings.HasPrefix(line, prefix) {
			return true
		}
		val = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		if isYAMLBlockIndicator(val) {
			blockParts = []string{}
			return true
		}
		return false
	})
	if err != nil {
		return ""
	}
	if blockParts != nil {
		return strings.Join(blockParts, " ")
	}
	return strings.Trim(val, `"'`)
}
