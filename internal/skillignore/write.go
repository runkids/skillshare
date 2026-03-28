package skillignore

import (
	"os"
	"path/filepath"
	"strings"
)

// AddPattern appends a pattern to a .skillignore file.
// Creates the file (and parent dirs) if it doesn't exist.
// Does nothing if the exact pattern already exists.
func AddPattern(filePath, pattern string) error {
	if HasPattern(filePath, pattern) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	existing, _ := os.ReadFile(filePath)
	content := string(existing)

	// Ensure trailing newline before appending
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += pattern + "\n"

	return os.WriteFile(filePath, []byte(content), 0644)
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
		trimmed := strings.TrimRight(line, " \t")
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
	// Remove trailing empty lines that result from removal
	for strings.HasSuffix(result, "\n\n") {
		result = strings.TrimSuffix(result, "\n")
	}

	return true, os.WriteFile(filePath, []byte(result), 0644)
}

// HasPattern returns true if the exact pattern exists in a .skillignore file.
func HasPattern(filePath, pattern string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimRight(line, " \t") == pattern {
			return true
		}
	}
	return false
}
