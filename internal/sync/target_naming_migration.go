package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

// selectActiveTargetNameForSync returns the entry name a skill syncs to. When
// the skill still has a managed entry under the name an earlier target naming
// gave it, that entry is renamed in place, unless the new name is taken.
// taken holds the names other skills sync to in this run.
func selectActiveTargetNameForSync(mode, targetPath string, skill ResolvedTargetSkill, taken map[string]bool, manifest *Manifest, dryRun bool) (string, error) {
	desiredName := skill.TargetName
	legacy, err := findLegacyTargetEntry(mode, targetPath, skill, taken, manifest)
	if err != nil || legacy.name == "" {
		return desiredName, err
	}
	legacyName, legacyNaming := legacy.name, legacy.naming
	legacyPath := filepath.Join(targetPath, legacyName)

	desiredPath := filepath.Join(targetPath, desiredName)
	if _, err := os.Lstat(desiredPath); err == nil {
		fmt.Fprintf(DiagOutput,
			"Warning: kept legacy managed entry %s for target name %q because %s already exists\n",
			legacyName, desiredName, desiredName)
		return legacyName, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to inspect target entry %s: %w", desiredName, err)
	}

	if dryRun {
		fmt.Fprintf(DiagOutput, "[dry-run] Would rename managed target entry: %s -> %s\n", legacyName, desiredName)
		return legacyName, nil
	}

	if err := os.Rename(legacyPath, desiredPath); err != nil {
		return "", fmt.Errorf("failed to rename managed target entry %s -> %s: %w", legacyName, desiredName, err)
	}
	renameManifestEntry(manifest, legacyName, desiredName, legacyNaming)
	// Record the rename now: if a later skill fails, the manifest must still
	// name this entry, or the next sync would take it for a user folder.
	if err := WriteManifest(targetPath, manifest); err != nil {
		renameManifestEntry(manifest, desiredName, legacyName, legacyNaming)
		if rbErr := os.Rename(desiredPath, legacyPath); rbErr != nil {
			return "", fmt.Errorf("failed to record rename %s -> %s: %w (rename back also failed: %v)", legacyName, desiredName, err, rbErr)
		}
		return "", fmt.Errorf("failed to record rename %s -> %s: %w", legacyName, desiredName, err)
	}
	return desiredName, nil
}

// findLegacyTargetEntry returns the managed entry a skill holds under the name
// another target naming gave it, with that naming, or a zero value when there is none:
// when the skill already has its current name, or the old name is one another
// skill syncs to (taken), there is nothing to migrate.
func findLegacyTargetEntry(mode, targetPath string, skill ResolvedTargetSkill, taken map[string]bool, manifest *Manifest) (namedTarget, error) {
	prev := ""
	for _, c := range targetNameCandidates(skill.Skill) {
		if c.name == "" || c.name == prev || c.name == skill.TargetName || taken[c.name] {
			continue
		}
		prev = c.name
		legacyPath := filepath.Join(targetPath, c.name)
		info, err := os.Lstat(legacyPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return namedTarget{}, fmt.Errorf("failed to inspect legacy target entry %s: %w", c.name, err)
		}
		if !isManagedTargetEntry(mode, legacyPath, info, skill, manifest, c) {
			continue
		}
		desiredPath := filepath.Join(targetPath, skill.TargetName)
		if info, err := os.Lstat(desiredPath); err == nil && isManagedTargetEntry(mode, desiredPath, info, skill, manifest, namedTarget{name: skill.TargetName}) {
			return namedTarget{}, nil
		}
		return c, nil
	}
	return namedTarget{}, nil
}

// isManagedTargetEntry reports whether the entry c.name at entryPath is the
// skill's managed entry. With c.naming set, a copy must also have been made
// under that naming.
func isManagedTargetEntry(mode, entryPath string, info os.FileInfo, skill ResolvedTargetSkill, manifest *Manifest, c namedTarget) bool {
	switch mode {
	case "merge":
		return utils.IsSymlinkOrJunction(entryPath) && isSymlinkToSource(entryPath, skill.Skill.SourcePath)
	case "copy":
		if manifest == nil {
			return false
		}
		// A link merge mode made (recorded as "symlink") migrates like a managed
		// copy; copy sync replaces it with a copy. A user's own link stays put.
		if utils.IsLinkMode(entryPath, info.Mode()) {
			return manifest.Managed[c.name] == manifestSymlink && isSymlinkToSource(entryPath, skill.Skill.SourcePath)
		}
		if !info.IsDir() {
			return false
		}
		if _, managed := manifest.Managed[c.name]; !managed {
			return false
		}
		// A recorded naming says exactly which naming made the entry; older
		// manifests have none, so the first managed candidate wins.
		recorded := manifest.Naming[c.name]
		return c.naming == "" || recorded == "" || recorded == c.naming
	default:
		return false
	}
}

// renameManifestEntry moves an entry's records to newName. The naming record
// becomes oldNaming, the naming that produced the entry, so copy sync sees the
// naming change and refreshes the copy.
func renameManifestEntry(manifest *Manifest, oldName, newName, oldNaming string) {
	if manifest == nil || oldName == newName {
		return
	}
	if manifest.Managed == nil {
		manifest.Managed = make(map[string]string)
	}
	if manifest.Mtimes == nil {
		manifest.Mtimes = make(map[string]int64)
	}
	if manifest.Naming == nil {
		manifest.Naming = make(map[string]string)
	}

	if managed, ok := manifest.Managed[oldName]; ok {
		manifest.Managed[newName] = managed
		delete(manifest.Managed, oldName)
	}
	if mtime, ok := manifest.Mtimes[oldName]; ok {
		manifest.Mtimes[newName] = mtime
		delete(manifest.Mtimes, oldName)
	}
	delete(manifest.Naming, oldName)
	manifest.Naming[newName] = oldNaming
}
