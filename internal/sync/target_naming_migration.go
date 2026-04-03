package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

func selectActiveTargetNameForSync(mode, targetPath string, skill ResolvedTargetSkill, manifest *Manifest, dryRun bool) (string, error) {
	desiredName := skill.TargetName
	legacyName := skill.Skill.FlatName
	if desiredName == "" || desiredName == legacyName {
		return desiredName, nil
	}

	legacyPath := filepath.Join(targetPath, legacyName)
	info, err := os.Lstat(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return desiredName, nil
		}
		return "", fmt.Errorf("failed to inspect legacy target entry %s: %w", legacyName, err)
	}

	if !isManagedLegacyTargetEntry(mode, legacyPath, info, skill, manifest) {
		return desiredName, nil
	}

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
	renameManifestEntry(manifest, legacyName, desiredName)
	return desiredName, nil
}

func isManagedLegacyTargetEntry(mode, legacyPath string, info os.FileInfo, skill ResolvedTargetSkill, manifest *Manifest) bool {
	switch mode {
	case "merge":
		return utils.IsSymlinkOrJunction(legacyPath) && isSymlinkToSource(legacyPath, skill.Skill.SourcePath)
	case "copy":
		if manifest == nil || !info.IsDir() || utils.IsSymlinkOrJunction(legacyPath) {
			return false
		}
		_, managed := manifest.Managed[skill.Skill.FlatName]
		return managed
	default:
		return false
	}
}

func renameManifestEntry(manifest *Manifest, oldName, newName string) {
	if manifest == nil || oldName == newName {
		return
	}
	if manifest.Managed == nil {
		manifest.Managed = make(map[string]string)
	}
	if manifest.Mtimes == nil {
		manifest.Mtimes = make(map[string]int64)
	}

	if managed, ok := manifest.Managed[oldName]; ok {
		manifest.Managed[newName] = managed
		delete(manifest.Managed, oldName)
	}
	if mtime, ok := manifest.Mtimes[oldName]; ok {
		manifest.Mtimes[newName] = mtime
		delete(manifest.Mtimes, oldName)
	}
}
