package sync

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/utils"
)

// CopyResult holds the result of a copy sync operation.
type CopyResult struct {
	Copied  []string // newly copied skills
	Skipped []string // checksum unchanged, skipped
	Updated []string // checksum changed, overwritten
}

// SyncTargetCopy performs copy mode sync — copies each skill individually
// while preserving target-specific (unmanaged) skills.
func SyncTargetCopy(name string, target config.TargetConfig, sourcePath string, dryRun, force bool) (*CopyResult, error) {
	skills, err := DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}
	return SyncTargetCopyWithSkills(name, target, skills, sourcePath, dryRun, force, nil)
}

// SyncTargetCopyWithSkills is like SyncTargetCopy but accepts pre-discovered skills
// and an optional progress callback for per-skill UI updates.
// sourcePath is the skills source directory, used to detect symlink-mode targets.
func SyncTargetCopyWithSkills(name string, target config.TargetConfig, allSkills []DiscoveredSkill, sourcePath string, dryRun, force bool, onProgress func(current, total int, skill string)) (*CopyResult, error) {
	result := &CopyResult{}

	// If target is currently a symlink pointing to our source (symlink mode),
	// remove it to convert to copy mode. External symlinks (e.g., dotfiles
	// manager) are preserved.
	info, err := os.Lstat(target.Path)
	if err == nil && info != nil && utils.IsSymlinkOrJunction(target.Path) {
		if isSymlinkToSource(target.Path, sourcePath) {
			if dryRun {
				fmt.Printf("[dry-run] Would convert from symlink mode to copy mode: %s\n", target.Path)
			} else {
				if err := os.Remove(target.Path); err != nil {
					return nil, fmt.Errorf("failed to remove symlink for copy conversion: %w", err)
				}
			}
		}
		// else: target is an external symlink (dotfiles manager, etc.) — keep it
	}

	// Ensure target directory exists
	if !dryRun {
		if err := os.MkdirAll(target.Path, 0755); err != nil {
			return nil, fmt.Errorf("failed to create target directory: %w", err)
		}
	}

	// Filter skills for this target
	discoveredSkills, err := FilterSkills(allSkills, target.Include, target.Exclude)
	if err != nil {
		return nil, fmt.Errorf("failed to apply filters for target %s: %w", name, err)
	}
	discoveredSkills = FilterSkillsByTarget(discoveredSkills, name)

	// Read existing manifest
	manifest, err := ReadManifest(target.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	for i, skill := range discoveredSkills {
		if onProgress != nil {
			onProgress(i+1, len(discoveredSkills), skill.FlatName)
		}

		targetSkillPath := filepath.Join(target.Path, skill.FlatName)

		// Compute source mtime for fast-path skip
		currentMtime, mtimeErr := DirMaxMtime(skill.SourcePath)

		// mtime fast-path: if source mtime is unchanged AND target is still a valid dir, skip checksum
		oldChecksum, isManaged := manifest.Managed[skill.FlatName]
		oldMtime := manifest.Mtimes[skill.FlatName] // 0 if missing
		if mtimeErr == nil && isManaged && !force && oldMtime > 0 && currentMtime == oldMtime {
			// Verify target still exists as a directory (user may have replaced it)
			if ti, err := os.Lstat(targetSkillPath); err == nil && ti.IsDir() {
				result.Skipped = append(result.Skipped, skill.FlatName)
				continue
			}
		}

		// mtime changed or no record — compute full checksum
		srcChecksum, err := DirChecksum(skill.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to checksum source skill %s: %w", skill.FlatName, err)
		}

		// Check what exists at the target path
		targetInfo, lstatErr := os.Lstat(targetSkillPath)
		exists := lstatErr == nil

		if exists {
			// If it's a symlink (leftover from merge mode), remove it
			if utils.IsSymlinkOrJunction(targetSkillPath) {
				if dryRun {
					fmt.Printf("[dry-run] Would replace symlink with copy: %s\n", skill.FlatName)
				} else {
					os.Remove(targetSkillPath)
				}
				// Fall through to copy below
			} else {
				// Non-directory entries are invalid for managed skills.
				// Managed/forced entries should be replaced with a proper skill directory.
				if !targetInfo.IsDir() {
					if isManaged || force {
						if dryRun {
							fmt.Printf("[dry-run] Would replace non-directory entry with copy: %s\n", skill.FlatName)
						} else {
							if err := os.RemoveAll(targetSkillPath); err != nil {
								return nil, fmt.Errorf("failed to remove invalid entry %s: %w", skill.FlatName, err)
							}
							if err := copyDirectory(skill.SourcePath, targetSkillPath); err != nil {
								return nil, fmt.Errorf("failed to copy skill %s: %w", skill.FlatName, err)
							}
							manifest.Managed[skill.FlatName] = srcChecksum
							if mtimeErr == nil {
								manifest.Mtimes[skill.FlatName] = currentMtime
							}
						}
						result.Updated = append(result.Updated, skill.FlatName)
						continue
					}

					// Local non-directory entry — preserve unless --force.
					result.Skipped = append(result.Skipped, skill.FlatName)
					continue
				}

				if !force && isManaged && oldChecksum == srcChecksum {
					// Unchanged — skip (but update mtime record if it changed)
					if mtimeErr == nil && currentMtime != oldMtime && !dryRun {
						manifest.Mtimes[skill.FlatName] = currentMtime
					}
					result.Skipped = append(result.Skipped, skill.FlatName)
					continue
				}

				if isManaged || force {
					// Managed or forced — overwrite
					if dryRun {
						fmt.Printf("[dry-run] Would update copy: %s\n", skill.FlatName)
					} else {
						if err := os.RemoveAll(targetSkillPath); err != nil {
							return nil, fmt.Errorf("failed to remove old copy %s: %w", skill.FlatName, err)
						}
					}
					if !dryRun {
						if err := copyDirectory(skill.SourcePath, targetSkillPath); err != nil {
							return nil, fmt.Errorf("failed to copy skill %s: %w", skill.FlatName, err)
						}
						manifest.Managed[skill.FlatName] = srcChecksum
						if mtimeErr == nil {
							manifest.Mtimes[skill.FlatName] = currentMtime
						}
					}
					result.Updated = append(result.Updated, skill.FlatName)
					continue
				}

				// Not managed (local skill) — preserve
				result.Skipped = append(result.Skipped, skill.FlatName)
				continue
			}
		}

		// Copy skill to target
		if dryRun {
			fmt.Printf("[dry-run] Would copy: %s -> %s\n", skill.SourcePath, targetSkillPath)
		} else {
			if err := copyDirectory(skill.SourcePath, targetSkillPath); err != nil {
				return nil, fmt.Errorf("failed to copy skill %s: %w", skill.FlatName, err)
			}
			manifest.Managed[skill.FlatName] = srcChecksum
			if mtimeErr == nil {
				manifest.Mtimes[skill.FlatName] = currentMtime
			}
		}
		result.Copied = append(result.Copied, skill.FlatName)
	}

	// Write updated manifest
	if !dryRun {
		if err := WriteManifest(target.Path, manifest); err != nil {
			return nil, fmt.Errorf("failed to write manifest: %w", err)
		}
	}

	return result, nil
}

// PruneOrphanCopies removes managed copies that no longer exist in source.
func PruneOrphanCopies(targetPath, sourcePath string, include, exclude []string, targetName string, dryRun bool) (*PruneResult, error) {
	allSourceSkills, err := DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills for pruning: %w", err)
	}
	return PruneOrphanCopiesWithSkills(targetPath, allSourceSkills, include, exclude, targetName, dryRun)
}

// PruneOrphanCopiesWithSkills is like PruneOrphanCopies but accepts pre-discovered skills.
func PruneOrphanCopiesWithSkills(targetPath string, allSourceSkills []DiscoveredSkill, include, exclude []string, targetName string, dryRun bool) (*PruneResult, error) {
	result := &PruneResult{}

	manifest, err := ReadManifest(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	managedSkills, err := FilterSkills(allSourceSkills, include, exclude)
	if err != nil {
		return nil, fmt.Errorf("failed to apply filters for pruning: %w", err)
	}
	managedSkills = FilterSkillsByTarget(managedSkills, targetName)

	// Build set of valid flat names
	validFlatNames := make(map[string]bool)
	for _, skill := range managedSkills {
		validFlatNames[skill.FlatName] = true
	}

	// Remove manifest entries that are no longer in source
	for flatName := range manifest.Managed {
		if validFlatNames[flatName] {
			continue
		}

		entryPath := filepath.Join(targetPath, flatName)
		if dryRun {
			fmt.Printf("[dry-run] Would remove orphan copy: %s\n", entryPath)
		} else {
			if err := os.RemoveAll(entryPath); err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s: failed to remove: %v", flatName, err))
				continue
			}
			delete(manifest.Managed, flatName)
			delete(manifest.Mtimes, flatName)
		}
		result.Removed = append(result.Removed, flatName)
	}

	// Write updated manifest (only if we actually removed something)
	if !dryRun && len(result.Removed) > 0 {
		if err := WriteManifest(targetPath, manifest); err != nil {
			return result, fmt.Errorf("failed to write manifest: %w", err)
		}
	}

	return result, nil
}

// CheckStatusCopy checks the status of a target in copy mode.
// Returns: status, managed count, local count.
func CheckStatusCopy(targetPath string) (TargetStatus, int, int) {
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return StatusNotExist, 0, 0
		}
		return StatusUnknown, 0, 0
	}

	// If it's a symlink, it's using symlink mode, not copy
	if utils.IsSymlinkOrJunction(targetPath) {
		return StatusLinked, 0, 0
	}

	if !info.IsDir() {
		return StatusUnknown, 0, 0
	}

	manifest, err := ReadManifest(targetPath)
	if err != nil {
		return StatusUnknown, 0, 0
	}

	// Count managed entries that actually exist on disk
	managedCount := 0
	for name := range manifest.Managed {
		if info, err := os.Stat(filepath.Join(targetPath, name)); err == nil && info.IsDir() {
			managedCount++
		}
	}

	// Count local (non-managed) entries
	localCount := 0
	entries, _ := os.ReadDir(targetPath)
	for _, entry := range entries {
		if utils.IsHidden(entry.Name()) {
			continue
		}
		if !entry.IsDir() {
			continue
		}
		if _, isManaged := manifest.Managed[entry.Name()]; !isManaged {
			localCount++
		}
	}

	if managedCount > 0 || len(manifest.Managed) > 0 {
		return StatusCopied, managedCount, localCount
	}

	return StatusHasFiles, 0, localCount
}

// DirMaxMtime returns the latest ModTime (UnixNano) among all files in dir.
// Skips .git directories. Only uses os.Stat (via filepath.Walk), never reads file content.
func DirMaxMtime(dir string) (int64, error) {
	var maxMtime int64
	hasSymlink := false
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		// Symlink targets can change without updating the link mtime.
		// Disable mtime fast-path for symlink-containing skills.
		if info.Mode()&os.ModeSymlink != 0 {
			hasSymlink = true
			return nil
		}
		if mt := info.ModTime().UnixNano(); mt > maxMtime {
			maxMtime = mt
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if hasSymlink {
		return 0, fmt.Errorf("mtime fast-path not supported for directories containing symlinks")
	}
	return maxMtime, nil
}

// DirChecksum computes a deterministic SHA256 checksum of a directory.
// It hashes sorted relative paths and file contents.
func DirChecksum(dir string) (string, error) {
	var entries []checksumEntry
	err := collectChecksumEntries(dir, "", &entries, map[string]bool{})
	if err != nil {
		return "", err
	}

	// Sort for deterministic output
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})

	h := sha256.New()
	for _, e := range entries {
		io.WriteString(h, e.relPath)
		h.Write([]byte{0}) // separator
		h.Write(e.content)
		h.Write([]byte{0}) // separator
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

type checksumEntry struct {
	relPath string
	content []byte
}

// collectChecksumEntries recursively collects file entries for checksumming.
// Directory symlinks are dereferenced to hash effective copied content.
func collectChecksumEntries(root, relPrefix string, entries *[]checksumEntry, active map[string]bool) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("failed to resolve checksum root %s: %w", root, err)
	}
	if active[resolvedRoot] {
		return fmt.Errorf("detected symlink directory cycle while checksumming: %s", root)
	}
	active[resolvedRoot] = true
	defer delete(active, resolvedRoot)

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip .git directories
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// Normalize path separators for cross-platform consistency
		relPath = strings.ReplaceAll(relPath, "\\", "/")
		if relPath == "." {
			relPath = ""
		}

		fullRelPath := relPath
		if relPrefix != "" {
			if fullRelPath == "" {
				fullRelPath = relPrefix
			} else {
				fullRelPath = relPrefix + "/" + fullRelPath
			}
		}

		// A symlink can point to a directory. In copy mode we dereference
		// directory symlinks, so checksum should include effective contents.
		if info.Mode()&os.ModeSymlink != 0 {
			targetInfo, statErr := os.Stat(path)
			if statErr != nil {
				return fmt.Errorf("failed to stat symlink target %s: %w", path, statErr)
			}
			if targetInfo.IsDir() {
				resolvedDir, resolveErr := filepath.EvalSymlinks(path)
				if resolveErr != nil {
					return fmt.Errorf("failed to resolve symlink directory %s: %w", path, resolveErr)
				}
				return collectChecksumEntries(resolvedDir, fullRelPath, entries, active)
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		*entries = append(*entries, checksumEntry{relPath: fullRelPath, content: content})
		return nil
	})
}
