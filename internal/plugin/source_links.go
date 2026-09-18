package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Resolve each component ourselves so an internal link cannot hide an absolute
// or out-of-tree intermediate link. .git is excluded from every snapshot.
func resolveSourcePath(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("link escapes plugin source: %s", path)
	}
	parts := strings.Split(rel, string(filepath.Separator))
	current := root
	hops := 0
	for len(parts) > 0 {
		part := parts[0]
		parts = parts[1:]
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if current == root {
				return "", fmt.Errorf("link escapes plugin source: %s", path)
			}
			current = filepath.Dir(current)
			continue
		}
		if part == ".git" {
			return "", fmt.Errorf("link references excluded .git directory: %s", path)
		}
		next := filepath.Join(current, part)
		info, err := os.Lstat(next)
		if err != nil {
			return "", fmt.Errorf("invalid plugin link %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			if len(parts) > 0 && !info.IsDir() {
				return "", fmt.Errorf("non-directory in plugin link: %s", path)
			}
			current = next
			continue
		}
		hops++
		if hops > 256 {
			return "", fmt.Errorf("cyclic or excessively deep plugin link: %s", path)
		}
		link, err := os.Readlink(next)
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(link) {
			return "", fmt.Errorf("absolute plugin link: %s", path)
		}
		// Resolve parent components only after preceding links, just like the OS.
		parts = append(strings.Split(link, string(filepath.Separator)), parts...)
	}
	return current, nil
}

func validateSourceLinks(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	active := map[string]bool{}
	count := 0
	var visit func(string) error
	visit = func(path string) error {
		count++
		if count > 20000 {
			return fmt.Errorf("plugin source exceeds 20,000 entries including links")
		}
		resolved, err := resolveSourcePath(root, path)
		if err != nil {
			return err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			return nil
		}
		if !info.IsDir() {
			return fmt.Errorf("unsupported special file: %s", path)
		}
		if active[resolved] {
			return fmt.Errorf("cyclic plugin directory link: %s", path)
		}
		active[resolved] = true
		defer delete(active, resolved)
		entries, err := os.ReadDir(resolved)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Name() == ".git" && entry.IsDir() {
				continue
			}
			if err := visit(filepath.Join(resolved, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(root)
}
