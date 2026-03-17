package skillignore

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadPatterns reads a .skillignore file from dir.
// Returns nil if no .skillignore exists.
// Skips blank lines and lines starting with #.
func ReadPatterns(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, ".skillignore"))
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, strings.TrimRight(line, "/"))
	}
	return patterns
}

// Match returns true if skillPath matches any pattern.
// Supports: exact path, group prefix (path/sub), trailing wildcard (prefix-*).
func Match(skillPath string, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasSuffix(p, "*") {
			if strings.HasPrefix(skillPath, strings.TrimSuffix(p, "*")) {
				return true
			}
		} else if skillPath == p || strings.HasPrefix(skillPath, p+"/") {
			return true
		}
	}
	return false
}
