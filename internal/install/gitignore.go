package install

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	gitignoreMarkerStart = "# BEGIN SKILLSHARE MANAGED - DO NOT EDIT"
	gitignoreMarkerEnd   = "# END SKILLSHARE MANAGED"
)

// UpdateGitIgnore adds an entry to the .gitignore file in the given directory.
// If the entry already exists, it does nothing.
// Creates the .gitignore file if it doesn't exist.
func UpdateGitIgnore(dir, entry string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")

	// Normalize to forward slashes (Windows paths use backslashes)
	entry = strings.ReplaceAll(entry, "\\", "/")

	// Ensure entry ends with / for directory
	if !strings.HasSuffix(entry, "/") {
		entry = entry + "/"
	}

	lines, err := readGitignoreLines(gitignorePath)
	if err != nil {
		return err
	}

	lines, startIdx, endIdx := ensureMarkerBlock(lines)
	if containsGitignoreEntry(lines[startIdx+1:endIdx], entry) {
		return nil
	}

	updated := make([]string, 0, len(lines)+1)
	updated = append(updated, lines[:endIdx]...)
	updated = append(updated, entry)
	updated = append(updated, lines[endIdx:]...)

	return writeGitignoreLines(gitignorePath, updated)
}

// UpdateGitIgnoreBatch adds multiple entries to the .gitignore file in one read/write pass.
func UpdateGitIgnoreBatch(dir string, entries []string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")

	lines, err := readGitignoreLines(gitignorePath)
	if err != nil {
		return err
	}

	lines, startIdx, endIdx := ensureMarkerBlock(lines)
	managed := lines[startIdx+1 : endIdx]

	// Build set of existing entries for O(1) lookup.
	existing := make(map[string]struct{}, len(managed))
	for _, line := range managed {
		existing[strings.TrimSpace(line)] = struct{}{}
	}

	// Collect genuinely new entries.
	var newEntries []string
	for _, entry := range entries {
		entry = strings.ReplaceAll(entry, "\\", "/")
		if !strings.HasSuffix(entry, "/") {
			entry += "/"
		}
		entryNoSlash := strings.TrimSuffix(entry, "/")
		if _, ok := existing[entry]; ok {
			continue
		}
		if _, ok := existing[entryNoSlash]; ok {
			continue
		}
		newEntries = append(newEntries, entry)
	}

	if len(newEntries) == 0 {
		return nil
	}

	updated := make([]string, 0, len(lines)+len(newEntries))
	updated = append(updated, lines[:endIdx]...)
	updated = append(updated, newEntries...)
	updated = append(updated, lines[endIdx:]...)

	return writeGitignoreLines(gitignorePath, updated)
}

// RemoveFromGitIgnore removes an entry from the .gitignore file.
// Returns true if the entry was found and removed.
func RemoveFromGitIgnore(dir, entry string) (bool, error) {
	gitignorePath := filepath.Join(dir, ".gitignore")

	// Normalize to forward slashes (Windows paths use backslashes)
	entry = strings.ReplaceAll(entry, "\\", "/")

	// Ensure entry ends with / for directory
	if !strings.HasSuffix(entry, "/") {
		entry = entry + "/"
	}

	lines, err := readGitignoreLines(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	startIdx, endIdx := findMarkerBlock(lines)
	if startIdx == -1 || endIdx == -1 || startIdx >= endIdx {
		return false, nil
	}

	found := false
	for i := startIdx + 1; i < endIdx; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == entry || trimmed == strings.TrimSuffix(entry, "/") {
			found = true
			continue
		}
	}

	if !found {
		return false, nil
	}

	updated := make([]string, 0, len(lines))
	updated = append(updated, lines[:startIdx+1]...)
	for i := startIdx + 1; i < endIdx; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == entry || trimmed == strings.TrimSuffix(entry, "/") {
			continue
		}
		updated = append(updated, lines[i])
	}
	updated = append(updated, lines[endIdx:]...)

	if err := writeGitignoreLines(gitignorePath, updated); err != nil {
		return false, err
	}

	return true, nil
}

// RemoveFromGitIgnoreBatch removes multiple entries from .gitignore in one read/write pass.
// Returns the number of entries actually removed.
func RemoveFromGitIgnoreBatch(dir string, entries []string) (int, error) {
	gitignorePath := filepath.Join(dir, ".gitignore")

	lines, err := readGitignoreLines(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	startIdx, endIdx := findMarkerBlock(lines)
	if startIdx == -1 || endIdx == -1 || startIdx >= endIdx {
		return 0, nil
	}

	// Build set of entries to remove (both with and without trailing slash).
	removeSet := make(map[string]struct{}, len(entries)*2)
	for _, entry := range entries {
		entry = strings.ReplaceAll(entry, "\\", "/")
		if !strings.HasSuffix(entry, "/") {
			entry += "/"
		}
		removeSet[entry] = struct{}{}
		removeSet[strings.TrimSuffix(entry, "/")] = struct{}{}
	}

	removed := 0
	updated := make([]string, 0, len(lines))
	updated = append(updated, lines[:startIdx+1]...)
	for i := startIdx + 1; i < endIdx; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if _, ok := removeSet[trimmed]; ok {
			removed++
			continue
		}
		updated = append(updated, lines[i])
	}
	updated = append(updated, lines[endIdx:]...)

	if removed == 0 {
		return 0, nil
	}

	if err := writeGitignoreLines(gitignorePath, updated); err != nil {
		return 0, err
	}
	return removed, nil
}

// gitignoreContains checks if an entry exists in .gitignore
func gitignoreContains(path, entry string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	// Also check without trailing slash
	entryNoSlash := strings.TrimSuffix(entry, "/")

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == entry || line == entryNoSlash {
			return true, nil
		}
	}

	return false, scanner.Err()
}

func readGitignoreLines(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read .gitignore: %w", err)
	}

	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines, nil
}

func writeGitignoreLines(path string, lines []string) error {
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}
	return nil
}

func findMarkerBlock(lines []string) (int, int) {
	startIdx := -1
	endIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == gitignoreMarkerStart {
			startIdx = i
			break
		}
	}
	if startIdx == -1 {
		return -1, -1
	}
	for i := startIdx + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == gitignoreMarkerEnd {
			endIdx = i
			break
		}
	}
	return startIdx, endIdx
}

func ensureMarkerBlock(lines []string) ([]string, int, int) {
	startIdx, endIdx := findMarkerBlock(lines)
	if startIdx != -1 && endIdx != -1 && startIdx < endIdx {
		return lines, startIdx, endIdx
	}

	if len(lines) > 0 {
		lines = append(lines, "")
	}
	startIdx = len(lines)
	lines = append(lines, gitignoreMarkerStart, gitignoreMarkerEnd)
	endIdx = startIdx + 1
	return lines, startIdx, endIdx
}

func containsGitignoreEntry(lines []string, entry string) bool {
	entryNoSlash := strings.TrimSuffix(entry, "/")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == entry || trimmed == entryNoSlash {
			return true
		}
	}
	return false
}
