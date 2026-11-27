package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/utils"
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
		if !utils.IsLinkMode(next, info.Mode()) {
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

// validateSourceLinks returns the links, relative to root, that are broken, point outside the
// source or into .git, or close a cycle. Callers leave them out of digests and snapshots, so an
// unrelated bad link does not block the rest of a source and is never followed.
func validateSourceLinks(root string, ignore func(string) bool) (map[string]bool, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	skipped := map[string]bool{}
	active := map[string]bool{}
	count := 0
	var visit func(string) error
	visit = func(path string) error {
		if rel, _ := filepath.Rel(root, path); ignore != nil && rel != "." && ignore(filepath.ToSlash(rel)) {
			return nil
		}
		count++
		if count > 20000 {
			return fmt.Errorf("plugin source exceeds 20,000 entries including links")
		}
		// path always sits in a real directory, so only its last component can be a link.
		skip := func(err error) error {
			if info, e := os.Lstat(path); e == nil && utils.IsLinkMode(path, info.Mode()) {
				rel, _ := filepath.Rel(root, path)
				skipped[rel] = true
				return nil
			}
			return err
		}
		resolved, err := resolveSourcePath(root, path)
		if err != nil {
			return skip(err)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return skip(err)
		}
		if info.Mode().IsRegular() {
			return nil
		}
		if !info.IsDir() {
			return fmt.Errorf("unsupported special file: %s", path)
		}
		if active[resolved] {
			return skip(fmt.Errorf("cyclic plugin directory link: %s", path))
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
	return skipped, visit(root)
}
