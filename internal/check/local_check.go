package check

import (
	"maps"
	"os"

	"skillshare/internal/install"
)

// LocalSourceStatus compares the files at a local install's recorded source
// path with the hashes recorded at install/update time. The path is resolved
// the same way `update` resolves it before re-copying, so a successful update
// brings the status back to "up_to_date".
//
// Returns "update_available" when the source changed, "error" with a message
// when the source cannot be read, and "local" when there is nothing to
// compare against (not a local install, or no recorded hashes).
func LocalSourceStatus(entry *install.MetadataEntry) (status, message string) {
	if entry == nil || entry.Type != "local" || len(entry.FileHashes) == 0 {
		return "local", ""
	}
	source, err := install.ParseSource(entry.Source)
	if err != nil || source.Type != install.SourceTypeLocalPath {
		return "local", ""
	}

	if info, err := os.Stat(source.Path); err != nil || !info.IsDir() {
		return "error", "local source not found: " + source.Path
	}
	hashes, err := install.ComputeFileHashes(source.Path)
	if err != nil {
		return "error", "cannot read local source: " + err.Error()
	}

	if maps.Equal(hashes, entry.FileHashes) {
		return "up_to_date", ""
	}
	return "update_available", ""
}
