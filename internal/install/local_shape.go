package install

import (
	"path/filepath"
	"strings"
)

// Layouts a local-path install records in MetadataEntry.Layout.
const (
	// LayoutSkillFile: only SKILL.md was copied. The dashboard installs the
	// root of a directory that also holds child skills this way.
	LayoutSkillFile = "skill-file"
	// LayoutDirectory: the whole source directory was copied, as
	// `install <path>` does.
	LayoutDirectory = "directory"
)

// CopiesSkillFileOnly reports whether a local install copies only SKILL.md
// from its source. source holds the current hashes of the source directory.
//
// The recorded layout decides. Entries written before the layout was recorded
// fall back to the recorded files: exactly SKILL.md, from a source that holds
// child skills. Anything else copied the whole directory, and updating it as
// SKILL.md alone would delete files.
func CopiesSkillFileOnly(entry *MetadataEntry, source map[string]string) bool {
	if entry.Layout != "" {
		return entry.Layout == LayoutSkillFile
	}
	if _, ok := entry.FileHashes["SKILL.md"]; !ok || len(entry.FileHashes) != 1 {
		return false
	}
	for rel := range source {
		if strings.HasSuffix(rel, "/SKILL.md") {
			return true
		}
	}
	return false
}

// discoveredLocalLayout returns the layout a discovery install of skill
// records for a local-path source, or "" when the copy is neither shape
// (a child skill with nested skills excluded) or the source is not local.
func discoveredLocalLayout(discovery *DiscoveryResult, skill SkillInfo) string {
	if discovery.Source == nil || discovery.Source.Type != SourceTypeLocalPath {
		return ""
	}
	excludes := descendantSkillPaths(discovery, skill)
	switch {
	case excludes == nil:
		return LayoutDirectory
	case skill.Path == "." && !discovery.Source.HasSubdir():
		return LayoutSkillFile
	}
	return ""
}

// isSkillFileOnlyLocalInstall reports whether the local skill at destPath
// was installed as its SKILL.md alone, so an update keeps that shape.
func isSkillFileOnlyLocalInstall(source *Source, destPath, sourceDir string) bool {
	if source.Type != SourceTypeLocalPath {
		return false
	}
	if sourceDir == "" {
		sourceDir = filepath.Dir(destPath)
	}
	rel, err := filepath.Rel(sourceDir, destPath)
	if err != nil {
		return false
	}
	entry := LoadMetadataOrNew(sourceDir).GetByPath(filepath.ToSlash(rel))
	if entry == nil {
		return false
	}
	if entry.Layout != "" {
		return entry.Layout == LayoutSkillFile
	}
	if len(entry.FileHashes) == 0 {
		return false
	}
	hashes, err := ComputeFileHashes(source.Path)
	if err != nil {
		return false
	}
	return CopiesSkillFileOnly(entry, hashes)
}
