package skillignore

import (
	"os"
	"path/filepath"
	"strings"
)

// AddPattern appends a pattern to a .skillignore file.
// Creates the file (and parent dirs) if it doesn't exist.
// Returns true if the pattern was added, false if it already existed.
func AddPattern(filePath, pattern string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return false, err
	}

	existing, _ := os.ReadFile(filePath)
	content := string(existing)

	// Check for duplicate in a single pass
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimRight(line, " \t\r") == pattern {
			return false, nil
		}
	}

	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += pattern + "\n"

	return true, os.WriteFile(filePath, []byte(content), 0644)
}

// RemovePattern removes all lines matching the exact pattern from a .skillignore file.
// Returns true if the pattern was found and removed, false if not found.
func RemovePattern(filePath, pattern string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	lines := strings.Split(string(data), "\n")
	var kept []string
	found := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		if trimmed == pattern {
			found = true
			continue
		}
		kept = append(kept, line)
	}

	if !found {
		return false, nil
	}

	result := strings.Join(kept, "\n")
	for strings.HasSuffix(result, "\n\n") {
		result = strings.TrimSuffix(result, "\n")
	}

	return true, os.WriteFile(filePath, []byte(result), 0644)
}

// RenamePatterns rewrites every line that is exactly a key of renames, or a
// "!" negation of one, to its value and reports whether any changed. Lines keep
// their place: with negations the last matching rule wins, so a removed and
// re-added line would change what the file means.
func RenamePatterns(content string, renames map[string]string) (string, bool) {
	lines := strings.Split(content, "\n")
	changed := false
	for i, line := range lines {
		pattern := strings.TrimRight(line, " \t\r")
		negation := strings.HasPrefix(pattern, "!")
		if to, ok := renames[strings.TrimPrefix(pattern, "!")]; ok {
			if negation {
				to = "!" + to
			}
			lines[i], changed = to, true
		}
	}
	return strings.Join(lines, "\n"), changed
}

// HasPattern returns true if the exact pattern exists in a .skillignore file.
func HasPattern(filePath, pattern string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimRight(line, " \t\r") == pattern {
			return true
		}
	}
	return false
}
