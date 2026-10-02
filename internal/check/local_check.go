package check

import (
	"maps"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/install"
)

// LocalSourceStatus compares the files at a local install's recorded source
// path with the hashes recorded at install/update time. A relative source
// resolves against baseDir: project mode records sources as typed, relative to
// the project root. Pass "" when relative sources have no known base.
//
// Returns "update_available" when the source changed, "error" with a message
// when the source cannot be read, and "local" when there is nothing to
// compare against (not a local install, no recorded hashes, or a relative
// source without a base directory).
func LocalSourceStatus(entry *install.MetadataEntry, baseDir string) (status, message string) {
	if entry == nil || entry.Type != "local" || len(entry.FileHashes) == 0 {
		return "local", ""
	}
	source, err := install.ParseSource(entry.Source)
	if err != nil || source.Type != install.SourceTypeLocalPath {
		return "local", ""
	}
	path := source.Path
	// ParseSource resolves a relative source against the current cwd, which
	// need not be the directory the install ran in.
	if !filepath.IsAbs(entry.Source) && !strings.HasPrefix(entry.Source, "~") {
		if baseDir == "" {
			return "local", ""
		}
		path = filepath.Join(baseDir, entry.Source)
	}

	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return "error", "local source not found: " + path
	}
	hashes, err := install.ComputeFileHashes(path)
	if err != nil {
		return "error", "cannot read local source: " + err.Error()
	}

	if maps.Equal(installedFiles(entry, hashes), entry.FileHashes) {
		return "up_to_date", ""
	}
	return "update_available", ""
}

// installedFiles narrows the source hashes to the files an install copies.
func installedFiles(entry *install.MetadataEntry, source map[string]string) map[string]string {
	if !install.CopiesSkillFileOnly(entry, source) {
		return source
	}
	if hash, ok := source["SKILL.md"]; ok {
		return map[string]string{"SKILL.md": hash}
	}
	return map[string]string{}
}
