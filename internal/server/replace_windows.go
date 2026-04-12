//go:build windows

package server

import "os"

func replaceFile(tempPath, fullPath string) error {
	return os.Rename(tempPath, fullPath)
}
