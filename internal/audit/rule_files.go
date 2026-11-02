package audit

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// files uses case-insensitive path.Match globs: basename globs match at any depth; globs with
// slashes are rooted at the skill directory. Recursive ** and escapes are not
// supported, so their meaning cannot silently differ across platforms.
func validateRuleFiles(files []string) error {
	for _, glob := range files {
		if glob == "" || strings.ContainsAny(glob, `\:`) || strings.Contains(glob, "**") || strings.HasPrefix(glob, "/") {
			return fmt.Errorf("invalid files glob %q: use relative slash-separated path.Match globs without ** or escapes", glob)
		}
		for _, part := range strings.Split(glob, "/") {
			if part == "" || part == "." || part == ".." {
				return fmt.Errorf("invalid files glob %q: empty, dot and parent segments are not allowed", glob)
			}
		}
		if _, err := path.Match(glob, ""); err != nil {
			return fmt.Errorf("invalid files glob %q: %w", glob, err)
		}
	}
	return nil
}

func rulesForFile(rules []rule, filename string) []rule {
	name := strings.ToLower(filepath.ToSlash(filename))
	filtered := make([]rule, 0, len(rules))
	for _, r := range rules {
		if len(r.Files) == 0 {
			filtered = append(filtered, r)
			continue
		}
		for _, glob := range r.Files {
			candidate := name
			if !strings.Contains(glob, "/") {
				candidate = path.Base(name)
			}
			if ok, _ := path.Match(strings.ToLower(glob), candidate); ok {
				filtered = append(filtered, r)
				break
			}
		}
	}
	return filtered
}

// Preserve an explicit [] when toggles or severity edits rewrite YAML.
type fileGlobs []string

func (f fileGlobs) IsZero() bool { return f == nil }
