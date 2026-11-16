//go:build !windows

package utils

import "fmt"

// platformIsJunction returns false: junctions exist only on Windows.
func platformIsJunction(string) bool { return false }

// CreateJunction cannot recreate a Windows junction on another platform.
func CreateJunction(linkPath, sourcePath string) error {
	return fmt.Errorf("cannot restore Windows junction %s to %s on this platform", linkPath, sourcePath)
}

func canonicalSpelling(path string) string { return path }
