//go:build !windows

package rules

import "os"

func replaceRuleFile(tempPath, fullPath string) error {
	return os.Rename(tempPath, fullPath)
}
