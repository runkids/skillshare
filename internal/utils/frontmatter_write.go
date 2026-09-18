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
	content := string(data)

	cr := ""
	if strings.Contains(content, "\r\n") {
		cr = "\r"
	}
	flagLine := key + ": true" + cr
	isDelim := func(l string) bool { return strings.TrimRight(l, " \t\r") == "---" }

	// Split on "\n" only, so CRLF files keep their "\r" on every untouched line.
	lines := strings.Split(content, "\n")
	open, end := -1, -1
	for i, l := range lines {
		if open < 0 {
			if isDelim(l) {
				open = i
			} else if strings.TrimSpace(l) != "" {
				break // body starts before any delimiter: no frontmatter
			}
		} else if isDelim(l) {
			end = i
			break
		}
	}
	if end < 0 {
		out := "---" + cr + "\n" + flagLine + "\n" + "---" + cr + "\n" + content
		return true, os.WriteFile(filePath, []byte(out), 0644)
	}

	on := true
	found := false
	for i := open + 1; i < end; i++ {
		rest, ok := strings.CutPrefix(lines[i], key+":") // column 0 only: nested keys are not the flag
		if !ok {
			continue
		}
		found = true
		if v, _, _ := strings.Cut(rest, " #"); strings.EqualFold(strings.Trim(v, " \t\r\"'"), "true") {
			lines = append(lines[:i], lines[i+1:]...)
			on = false
		} else {
			lines[i] = flagLine
		}
		break
	}
	if !found {
		lines = append(lines[:end], append([]string{flagLine}, lines[end:]...)...)
	}
	return on, os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}
