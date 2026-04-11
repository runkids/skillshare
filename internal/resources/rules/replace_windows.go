//go:build windows

package rules

import "golang.org/x/sys/windows"

func replaceRuleFile(tempPath, fullPath string) error {
	return windows.Rename(tempPath, fullPath)
}
