package utils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseSkillName reads the SKILL.md and extracts the "name" from frontmatter.
func ParseSkillName(skillPath string) (string, error) {
	skillFile := filepath.Join(skillPath, "SKILL.md")
	file, err := os.Open(skillFile)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inFrontmatter := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Detect frontmatter delimiters
		if line == "---" {
			if inFrontmatter {
				break // End of frontmatter
			}
			inFrontmatter = true
			continue
		}

		if inFrontmatter {
			if strings.HasPrefix(line, "name:") {
				// Extract value: "name: my-skill" -> "my-skill"
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[1])
					// Remove quotes if present
					name = strings.Trim(name, `"'`)
					return name, nil
				}
			}
		}
	}

	return "", nil // Name not found
}

// isYAMLBlockIndicator returns true for YAML block scalar indicators (>, >-, >+, |, |-, |+).
func isYAMLBlockIndicator(s string) bool {
	switch s {
	case ">", ">-", ">+", "|", "|-", "|+":
		return true
	}
	return false
}

// ParseFrontmatterList reads a SKILL.md file and extracts a YAML list field from frontmatter.
// Supports both inline [a, b] and block (- a\n- b) formats.
// Returns nil when the field is absent or the file cannot be read.
func ParseFrontmatterList(filePath, field string) []string {
	raw := extractFrontmatterRaw(filePath)
	if raw == "" {
		return nil
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(raw), &fm); err != nil {
		return nil
	}

	val, ok := fm[field]
	if !ok || val == nil {
		return nil
	}

	switch v := val.(type) {
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	default:
		return nil
	}
}

// extractFrontmatterRaw reads the raw frontmatter text between --- delimiters.
func extractFrontmatterRaw(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inFrontmatter := false
	var lines []string

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			if inFrontmatter {
				break
			}
			inFrontmatter = true
			continue
		}

		if inFrontmatter {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// ParseFrontmatterFields reads a SKILL.md file once and returns the values of
// multiple frontmatter fields. This avoids opening the same file repeatedly
// when multiple fields are needed (e.g. description + license).
func ParseFrontmatterFields(filePath string, fields []string) map[string]string {
	result := make(map[string]string, len(fields))
	if len(fields) == 0 {
		return result
	}

	raw := extractFrontmatterRaw(filePath)
	if raw == "" {
		return result
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(raw), &fm); err != nil {
		return result
	}

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

// ParseFrontmatterField reads a SKILL.md file and extracts the value of a given frontmatter field.
// It supports both inline values and YAML block scalars (>, >-, |, |-).
func ParseFrontmatterField(filePath, field string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inFrontmatter := false
	prefix := field + ":"

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "---" {
			if inFrontmatter {
				break
			}
			inFrontmatter = true
			continue
		}

		if inFrontmatter && strings.HasPrefix(line, prefix) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				// Handle YAML block scalar indicators — read indented continuation lines
				if isYAMLBlockIndicator(val) {
					var blockParts []string
					for scanner.Scan() {
						next := scanner.Text()
						trimmed := strings.TrimSpace(next)
						if trimmed == "---" {
							break
						}
						// Block continues while lines are indented
						if len(next) > 0 && (next[0] == ' ' || next[0] == '\t') {
							blockParts = append(blockParts, trimmed)
						} else {
							break
						}
					}
					return strings.Join(blockParts, " ")
				}
				val = strings.Trim(val, `"'`)
				return val
			}
		}
	}

	return ""
}
