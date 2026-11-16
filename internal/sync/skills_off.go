package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// SkillsOffResult is what switching skills off for a target does to its skills
// folder. Removed and Kept are entry names in the folder, or the folder path
// itself when the whole folder is a link. Copies are the entries copy mode
// made: kept like the user's own, but the tool loads them next to wherever it
// now reads skills from, so they show up twice until deleted. SharedWith names
// an enabled target writing to the same folder; the folder is then left alone.
type SkillsOffResult struct {
	Removed    []string `json:"removed"`
	Kept       []string `json:"kept"`
	Copies     []string `json:"copies"`
	SharedWith string   `json:"sharedWith,omitempty"`
}

// DetachSkills removes the links target name's skills folder holds into the
// skills source: the folder itself when it is such a link (the link, never what
// it points at), else each entry linking into the source. Copies, local files
// and links elsewhere are kept and reported. Content is never copied back. The
// manifest loses only the removed entries. dryRun reports without writing.
func DetachSkills(targets map[string]config.TargetConfig, name, sourcePath string, dryRun bool) (*SkillsOffResult, error) {
	res := &SkillsOffResult{Removed: []string{}, Kept: []string{}, Copies: []string{}}
	tc, ok := targets[name]
	if !ok {
		return nil, fmt.Errorf("target %q not found", name)
	}
	raw := tc.SkillsConfig().Path
	if raw == "" {
		return res, nil
	}
	if keeper := config.SkillsPathKeptBy(targets, name, nil); keeper != "" {
		res.SharedWith = keeper
		return res, nil
	}
	folder := filepath.Clean(config.ExpandPath(raw))

	info, err := os.Lstat(folder)
	if os.IsNotExist(err) {
		return res, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", folder, err)
	}
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, err
	}

	if utils.IsLinkMode(folder, info.Mode()) {
		if !linksIntoSource(folder, absSource) {
			res.Kept = append(res.Kept, folder)
			return res, nil
		}
		if !dryRun {
			if err := os.Remove(folder); err != nil {
				return nil, fmt.Errorf("remove link %s: %w", folder, err)
			}
		}
		res.Removed = append(res.Removed, folder)
		return res, nil
	}
	if !info.IsDir() {
		res.Kept = append(res.Kept, folder)
		return res, nil
	}

	manifest, err := ReadManifest(folder)
	if err != nil {
		return nil, fmt.Errorf("read manifest in %s: %w", folder, err)
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", folder, err)
	}
	manifestChanged := false
	for _, entry := range entries {
		entryName := entry.Name()
		if entryName == ManifestFile || manifest.SkipsHidden(entryName) {
			continue
		}
		entryPath := filepath.Join(folder, entryName)
		if !utils.IsSymlinkOrJunction(entryPath) || !linksIntoSource(entryPath, absSource) {
			if _, copied := manifest.Managed[entryName]; copied && !utils.IsSymlinkOrJunction(entryPath) {
				res.Copies = append(res.Copies, entryName)
			} else {
				res.Kept = append(res.Kept, entryName)
			}
			continue
		}
		if !dryRun {
			if err := os.Remove(entryPath); err != nil {
				return nil, fmt.Errorf("remove link %s: %w", entryPath, err)
			}
		}
		res.Removed = append(res.Removed, entryName)
		if _, managed := manifest.Managed[entryName]; managed {
			manifest.Remove(entryName)
			manifestChanged = true
		}
	}
	sort.Strings(res.Removed)
	sort.Strings(res.Kept)
	sort.Strings(res.Copies)

	if manifestChanged && !dryRun {
		if len(manifest.Managed) == 0 {
			err = RemoveManifest(folder)
		} else {
			err = WriteManifest(folder, manifest)
		}
		if err != nil {
			return nil, fmt.Errorf("update manifest in %s: %w", folder, err)
		}
	}
	return res, nil
}

// linksIntoSource reports whether the link at path points at the source or
// inside it, comparing canonical paths so an aliased source still matches. A
// broken link is compared by where it would be: its missing tail is kept below
// the nearest folder that exists.
func linksIntoSource(path, absSource string) bool {
	dest, err := utils.ResolveLinkTarget(path)
	if err != nil {
		return false
	}
	within := func(p, root string) bool {
		return utils.PathsEqual(p, root) || utils.PathHasPrefix(p, root+string(filepath.Separator))
	}
	if within(dest, absSource) {
		return true
	}
	return within(sourcewalk.Canonical(dest), sourcewalk.Canonical(absSource))
}
