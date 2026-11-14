package utils

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// CanonicalPath resolves symlink and junction ancestors to an absolute path,
// retaining a missing tail below its nearest existing parent.
func CanonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	var missing []string
	for dir := abs; ; dir = filepath.Dir(dir) {
		if resolved, err := physical(dir); err == nil {
			return filepath.Join(append([]string{resolved}, missing...)...)
		}
		if filepath.Dir(dir) == dir {
			return abs
		}
		missing = append([]string{filepath.Base(dir)}, missing...)
	}
}

// physical resolves every link component of an existing absolute path.
// filepath.EvalSymlinks handles symlinks but, since Go 1.23, leaves a Windows
// junction component in place, so each remaining junction is replaced by its
// target and the result resolved again, bounded like a symlink chain.
func physical(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	for hops := 0; hops < 64; hops++ {
		link, ok := firstLink(resolved)
		if !ok {
			return canonicalSpelling(resolved), nil
		}
		// firstLink guarantees no unresolved link in this parent; bypass
		// parent canonicalization to avoid recursing into the same chain.
		target, err := resolveLinkTarget(link, false)
		if err != nil {
			return "", err
		}
		if resolved, err = filepath.EvalSymlinks(target + resolved[len(link):]); err != nil {
			return "", err
		}
	}
	return "", errors.New("too many links: " + path)
}

// firstLink returns the shortest prefix of path that is itself a link.
func firstLink(path string) (string, bool) {
	vol := filepath.VolumeName(path)
	rest := strings.Split(strings.TrimPrefix(path[len(vol):], string(filepath.Separator)), string(filepath.Separator))
	prefix := vol
	for _, part := range rest {
		if part == "" {
			continue
		}
		prefix += string(filepath.Separator) + part
		if info, err := os.Lstat(prefix); err == nil && IsLinkMode(prefix, info.Mode()) {
			return prefix, true
		}
	}
	return "", false
}

// SamePhysicalPath compares absolute physical spellings, including junctions
// and missing leaves. It does not rewrite the link's stored logical target.
func SamePhysicalPath(a, b string) bool {
	if PathsEqual(a, b) {
		return true
	}
	// Ask the filesystem when both paths exist. Windows can expose the same
	// file through junction/symlink combinations with different path spellings.
	if ai, err := os.Stat(a); err == nil {
		if bi, err := os.Stat(b); err == nil && os.SameFile(ai, bi) {
			return true
		}
	}
	return PathsEqual(CanonicalPath(a), CanonicalPath(b))
}

// PathWithinRoot recognizes strict descendants through equivalent root aliases.
// Resolve each possible root separately to retain logical followed descendants
// rather than losing their ownership when they point outside the root.
func PathWithinRoot(link, source string) bool {
	root, err := filepath.Abs(source)
	if err != nil {
		return false
	}
	prefix := func(path string) string {
		return strings.TrimSuffix(path, string(filepath.Separator)) + string(filepath.Separator)
	}
	if PathHasPrefix(link, prefix(root)) {
		return true
	}
	root = CanonicalPath(root)
	if PathHasPrefix(link, prefix(root)) || PathHasPrefix(CanonicalPath(link), prefix(root)) {
		return true
	}
	// A source-root alias can have a different spelling (including Windows
	// short paths) from the root embedded in a relative link. Resolve each
	// possible root separately so followed descendants remain logical paths.
	for parent := filepath.Dir(link); ; parent = filepath.Dir(parent) {
		if PathsEqual(CanonicalPath(parent), root) {
			return true
		}
		if filepath.Dir(parent) == parent {
			return false
		}
	}
}
