package install

import (
	"path/filepath"
	"strings"
)

// IsSkillFileOnlyInstall reports whether recorded file hashes describe a
// local install that copied only SKILL.md from a source directory that also
// holds child skills. The dashboard installs the root of such a collection
// this way; `install <path>` copies the whole directory, child skills
// included. source holds the current hashes of the source directory.
//
// The recorded files decide: an install that recorded anything besides
// SKILL.md copied the whole directory, even if child skills were added to
// the source later, and updating it as SKILL.md alone would delete files.
func IsSkillFileOnlyInstall(source, recorded map[string]string) bool {
	if _, ok := recorded["SKILL.md"]; !ok || len(recorded) != 1 {
		return false
	}
	for rel := range source {
		if strings.HasSuffix(rel, "/SKILL.md") {
			return true
		}
	}
	return false
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
	if entry == nil || len(entry.FileHashes) == 0 {
		return false
	}
	hashes, err := ComputeFileHashes(source.Path)
	if err != nil {
		return false
	}
	return IsSkillFileOnlyInstall(hashes, entry.FileHashes)
}
