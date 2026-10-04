package skillignore

import (
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/sourcefs"
)

// AddPattern is AddPatternWith for an ignore file outside the skills source,
// such as .agentignore.
func AddPattern(filePath, pattern string) (bool, error) {
	return AddPatternWith(sourcefs.OS, filePath, pattern)
}

// AddPatternWith appends a pattern to an ignore file through w.
// Creates the file (and parent dirs) if it doesn't exist.
// Returns true if the pattern was added, false if it already existed.
func AddPatternWith(w sourcefs.Writer, filePath, pattern string) (bool, error) {
	if err := w.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
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

	return true, w.WriteFile(filePath, []byte(content), 0644)
}

// RemovePattern is RemovePatternWith for an ignore file outside the skills
// source, such as .agentignore.
func RemovePattern(filePath, pattern string) (bool, error) {
	return RemovePatternWith(sourcefs.OS, filePath, pattern)
}

// RemovePatternWith removes all lines matching the exact pattern from an ignore file through w.
// Returns true if the pattern was found and removed, false if not found.
func RemovePatternWith(w sourcefs.Writer, filePath, pattern string) (bool, error) {
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

	return true, w.WriteFile(filePath, []byte(result), 0644)
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

// OpenWriter returns the writer for the ignore file at path. For a
// .skillignore it is a handle at the skills source, the file's folder,
// created when missing, so a linked .skillignore is refused instead of
// written through. For an .agentignore it is the plain writer. close
// releases the handle.
func OpenWriter(path string, skillsSource bool) (w sourcefs.Writer, close func(), err error) {
	if !skillsSource {
		return sourcefs.OS, func() {}, nil
	}
	root, err := sourcefs.Create(filepath.Dir(path))
	if err != nil {
		return nil, nil, err
	}
	return root.Writer(), func() { root.Close() }, nil
}
