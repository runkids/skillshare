//go:build !windows

package apply

import "os"

func replaceFile(tempPath, fullPath string) error {
	return os.Rename(tempPath, fullPath)
}
