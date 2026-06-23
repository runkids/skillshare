package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// RejectProjectRootLocalInstall returns an error when source is a local path
// that resolves to the project root itself. Installing the project root into
// its own .skillshare/skills/ subtree would recursively copy the destination
// back into itself (see discussion #227). Callers should surface the returned
// error to the user.
//
// Returns nil for non-local sources, an empty root, or any local path that is
// not the project root.
func RejectProjectRootLocalInstall(source *Source, root string) error {
	if source == nil || source.Type != SourceTypeLocalPath || root == "" {
		return nil
	}
	sourcePath, err := filepath.Abs(source.Path)
	if err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}
	projectRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("invalid project root: %w", err)
	}
	sourcePath, err = filepath.EvalSymlinks(sourcePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot resolve source path: %w", err)
		}
	} else if resolvedRoot, rootErr := filepath.EvalSymlinks(projectRoot); rootErr == nil {
		projectRoot = resolvedRoot
	}
	if sourcePath == projectRoot {
		return fmt.Errorf("cannot install the project root into itself\nRun 'skillshare install ./my-skill -p' to install a specific skill subdirectory instead")
	}
	return nil
}
