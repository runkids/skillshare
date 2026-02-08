//go:build windows

package utils

import "syscall"

const fileAttributeReparsePoint = 0x0400

func isWindowsReparsePoint(path string) bool {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}

	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return false
	}

	return attrs&fileAttributeReparsePoint != 0
}
