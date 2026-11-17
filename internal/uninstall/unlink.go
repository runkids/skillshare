package uninstall

import (
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/sourcelink"
)

// UnlinkResult separates a completed link removal from metadata cleanup faults.
// Store is the fresh global store, reloaded after a cleanup failure when possible.
// A nil Store means metadata was not loaded; callers retain their current cache.
type UnlinkResult struct {
	Store       *install.MetadataStore
	MetadataErr error
	ReloadErr   error
}

// Unlink moves only the source link to trash. Global cleanup loads metadata
// after the move, saves its undo copy before pruning, and reloads on failure.
// Project mode leaves metadata alone. A returned error means the link did not
// move; MetadataErr is a warning after a successful move.
func Unlink(sourceDir, trashDir, name string, global bool) (UnlinkResult, error) {
	trashPath, err := sourcelink.Remove(sourceDir, trashDir, name)
	if err != nil {
		return UnlinkResult{}, err
	}
	var result UnlinkResult
	if !global {
		return result, nil
	}
	store, err := install.LoadMetadataWithMigration(sourceDir, "")
	if err != nil {
		result.MetadataErr = err
		return result, nil
	}
	result.Store = store
	result.MetadataErr = ForgetUnlinkedName(sourceDir, trashPath, strings.TrimRight(name, `/\`), result.Store)
	if result.MetadataErr != nil {
		var fresh *install.MetadataStore
		fresh, result.ReloadErr = install.LoadMetadataWithMigration(sourceDir, "")
		if result.ReloadErr == nil {
			result.Store = fresh
		}
	}
	return result, nil
}
