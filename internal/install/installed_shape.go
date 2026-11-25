package install

import (
	"os"
	"path/filepath"
	"strings"
)

// keepInstalledShape trims a fresh copy of a git skill in dir to the shape its
// install recorded in entry. A discovery install leaves out the skills nested
// inside the one it installs, since each installs on its own, and copies only
// SKILL.md from a repository root that holds other skills. A reinstall from
// the recorded source copies everything, so without this an update or a
// rebuilt base would gain those files. It reports whether it removed any.
func keepInstalledShape(entry *MetadataEntry, source *Source, dir string) (bool, error) {
	if entry == nil || len(entry.FileHashes) == 0 || !source.IsGit() {
		return false, nil
	}
	nested := discoverSkillsByDirectory(dir, false)
	if len(nested) == 0 {
		return false, nil
	}
	if _, ok := entry.FileHashes["SKILL.md"]; ok && len(entry.FileHashes) == 1 && !source.HasSubdir() {
		return true, keepOnlySkillFile(dir)
	}
	removed := false
	for _, skill := range nested {
		if recordsFilesUnder(entry.FileHashes, skill.Path) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, filepath.FromSlash(skill.Path))); err != nil {
			return removed, err
		}
		removed = true
	}
	return removed, nil
}

func keepOnlySkillFile(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == "SKILL.md" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func recordsFilesUnder(hashes map[string]string, dir string) bool {
	for rel := range hashes {
		if strings.HasPrefix(rel, dir+"/") {
			return true
		}
	}
	return false
}

// recordedEntry returns the metadata entry of the skill installed at destPath,
// or nil when none is recorded.
func recordedEntry(destPath, sourceDir string) *MetadataEntry {
	if sourceDir == "" {
		sourceDir = filepath.Dir(destPath)
	}
	rel, err := filepath.Rel(sourceDir, destPath)
	if err != nil {
		return nil
	}
	return LoadMetadataOrNew(sourceDir).GetByPath(filepath.ToSlash(rel))
}
