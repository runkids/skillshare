//go:build !windows

package hooks

import "os"

func (s *Store) replaceHookFile(tempPath, fullPath string) error {
	return os.Rename(tempPath, fullPath)
}
