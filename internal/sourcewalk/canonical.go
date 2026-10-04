package sourcewalk

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/utils"
)

// linkOps is an operation-local seam for testing junctions on non-Windows hosts.
// Production always uses the platform-aware utils implementations.
type linkOps struct {
	isLink  func(string, fs.FileMode) bool
	resolve func(string) (string, error)
}

func platformLinks() linkOps { return linkOps{utils.IsLinkMode, utils.ResolveLinkTarget} }

// Canonicalize resolves link components, including Windows junctions. Missing
// suffixes are retained beneath their nearest existing canonical ancestor.
func Canonicalize(path string) (string, error) { return canonicalize(path, platformLinks(), 0) }

func canonicalize(path string, links linkOps, hops int) (string, error) {
	if hops > 255 {
		return "", fmt.Errorf("too many links resolving %s", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	volume := filepath.VolumeName(abs)
	current := volume + string(filepath.Separator)
	parts := strings.Split(strings.TrimPrefix(abs[len(volume):], string(filepath.Separator)), string(filepath.Separator))
	for i, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return filepath.Join(append([]string{current}, parts[i+1:]...)...), nil
		}
		if err != nil {
			return "", err
		}
		if links.isLink(current, info.Mode()) {
			target, err := links.resolve(current)
			if err != nil {
				return "", err
			}
			resolved, err := canonicalize(target, links, hops+1)
			if err != nil {
				return "", err
			}
			current = resolved
		}
	}
	return filepath.Clean(current), nil
}

func containsPath(root, path string) bool {
	return utils.PathsEqual(root, path) || utils.PathHasPrefix(path, strings.TrimRight(root, string(filepath.Separator))+string(filepath.Separator))
}
func overlaps(a, b string) bool { return containsPath(a, b) || containsPath(b, a) }
