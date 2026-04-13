//go:build windows

package hooks

import "golang.org/x/sys/windows"

func (s *Store) replaceHookFile(tempPath, fullPath string) error {
	return windows.Rename(tempPath, fullPath)
}
