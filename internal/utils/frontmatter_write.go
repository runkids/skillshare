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

	content := string(data)
	fmRaw, body := splitFrontmatterAndBody(content)

	var fm map[string]any
	if fmRaw != "" {
		if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
			return err
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

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n")
	if body != "" {
		sb.WriteString(body)
	}

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// splitFrontmatterAndBody splits SKILL.md content into raw frontmatter YAML
// and the remaining body. Returns ("", fullContent) if no frontmatter found.
func splitFrontmatterAndBody(content string) (string, string) {
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		return "", content
	}

	lines := strings.Split(content, "\n")
	inFrontmatter := false
	fmStart := -1
	fmEnd := -1

	for i, line := range lines {
		// Only match "---" at column 0 (with optional trailing whitespace).
		// TrimRight preserves leading whitespace so indented "---" inside
		// a YAML block scalar is not mistaken for the frontmatter delimiter.
		if strings.TrimRight(line, " \t") == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				fmStart = i + 1
			} else {
				fmEnd = i
				break
			}
		}
	}

	if fmEnd < 0 {
		return "", content
	}

	fmRaw := strings.Join(lines[fmStart:fmEnd], "\n")
	body := ""
	if fmEnd+1 < len(lines) {
		body = strings.Join(lines[fmEnd+1:], "\n")
	}

	return fmRaw, body
}
