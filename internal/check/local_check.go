package check

import (
	"maps"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/install"
)

// LocalSourceStatus compares the files at a local install's recorded source
// path with the hashes recorded at install/update time.
//
// Returns "update_available" when the source changed, "error" with a message
// when the source cannot be read, and "local" when there is nothing to
// compare against (not a local install, no recorded hashes, or a relative
// source path whose base directory is unknown).
func LocalSourceStatus(entry *install.MetadataEntry) (status, message string) {
	if entry == nil || entry.Type != "local" || len(entry.FileHashes) == 0 {
		return "local", ""
	}
	// A relative source resolves against the cwd of the install, which is
	// not recorded; comparing against the current cwd would report a missing
	// or unrelated directory.
	if !filepath.IsAbs(entry.Source) && !strings.HasPrefix(entry.Source, "~") {
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

	if maps.Equal(installedFiles(hashes, entry.FileHashes), entry.FileHashes) {
		return "up_to_date", ""
	}
	return "update_available", ""
}

// installedFiles narrows the source hashes to the files an install copies.
// `install <path>` copies the whole directory, nested skills included. The
// dashboard installs the root of a directory that also holds child skills
// as its SKILL.md alone; that shape is recognized by recorded hashes that
// contain none of the child skills present in the source.
func installedFiles(source, recorded map[string]string) map[string]string {
	hasChildSkill := false
	for rel := range source {
		if strings.HasSuffix(rel, "/SKILL.md") {
			if _, ok := recorded[rel]; ok {
				return source
			}
			hasChildSkill = true
		}
	}
	if !hasChildSkill {
		return source
	}
	if hash, ok := source["SKILL.md"]; ok {
		return map[string]string{"SKILL.md": hash}
	}
	return map[string]string{}
}
