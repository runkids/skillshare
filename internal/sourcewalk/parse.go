package sourcewalk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type declarations struct {
	err      error
	names    []string
	warnings []string
	active   bool
	local    bool
}

// readDeclarations merges portable first-level names, preserving file order.
func readDeclarations(root string) declarations {
	var out declarations
	seen := make(map[string]bool)
	for _, file := range []string{".skillfollow", ".skillfollow.local"} {
		path := filepath.Join(root, file)
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				out.warnings = append(out.warnings, fmt.Sprintf("%s: %v", path, err))
				out.err = errors.Join(out.err, fmt.Errorf("read skillfollow declaration %s: %w", path, err))
			}
			continue
		}
		out.active = true
		if file == ".skillfollow.local" {
			out.local = true
		}
		for i, line := range strings.Split(string(data), "\n") {
			name := strings.TrimSpace(line)
			if name == "" || strings.HasPrefix(name, "#") {
				continue
			}
			if !validEntryName(name) {
				out.warnings = append(out.warnings, fmt.Sprintf("%s:%d: invalid first-level entry %q", path, i+1, name))
				continue
			}
			// Keep the first spelling of names the platform treats as equal.
			if key := entryNameKey(name); !seen[key] {
				out.names = append(out.names, name)
				seen[key] = true
			}
		}
	}
	return out
}

// ValidEntryName reports whether name is an accepted first-level declaration.
func ValidEntryName(name string) bool { return validEntryName(name) }

// SameEntryName reports whether two declared names identify the same
// first-level entry: case-insensitively on Windows, exactly elsewhere.
func SameEntryName(a, b string) bool { return sameEntryName(a, b) }

func validEntryName(name string) bool {
	// Check Windows separators and volumes on every host, not only Windows.
	volume := len(name) >= 2 && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) && name[1] == ':'
	return name != "" && name != "." && name != ".." && !volume &&
		!filepath.IsAbs(name) && filepath.Clean(name) == name &&
		!strings.ContainsAny(name, "/\\*?[]{}!\x00")
}
