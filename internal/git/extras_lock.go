package git

import (
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/gitenv"
	"skillshare/internal/utils"
)

// extraRootLockPathspec excludes only a known root's empty administrative lock
// for this invocation. Ordinary same-name content and Git ignore files remain
// untouched. With no configured root, callers retain their normal Git scope.
func extraRootLockPathspec(dir string, extrasRoots ...string) []string {
	var locks []string
	for _, root := range extrasRoots {
		if root == "" {
			continue
		}
		canonical, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue
		}
		lock := filepath.Join(canonical, utils.ExtrasRootLockFileName)
		info, err := os.Lstat(lock)
		if err == nil && info.Mode().IsRegular() && !utils.IsLinkMode(lock, info.Mode()) && info.Size() == 0 {
			locks = append(locks, lock)
		}
	}
	if len(locks) == 0 {
		return nil
	}
	cmd := gitenv.Command("rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	top, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		return nil
	}
	var paths []string
	for _, lock := range locks {
		rel, err := filepath.Rel(top, lock)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		paths = append(paths, ":(top,literal,exclude)"+filepath.ToSlash(rel))
	}
	if len(paths) == 0 {
		return nil
	}
	// Exclude-only pathspecs preserve the command's existing whole-repo scope.
	return append([]string{"--"}, paths...)
}
