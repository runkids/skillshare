//go:build !windows

package utils

func isWindowsReparsePoint(path string) bool {
	return false
}
