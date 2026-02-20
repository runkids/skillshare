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
	result := &CopyResult{}

	// If target is currently a symlink (symlink mode), remove it to convert
	info, err := os.Lstat(target.Path)
	if err == nil && info != nil && utils.IsSymlinkOrJunction(target.Path) {
		if dryRun {
			fmt.Printf("[dry-run] Would convert from symlink mode to copy mode: %s\n", target.Path)
		} else {
			if err := os.Remove(target.Path); err != nil {
				return nil, fmt.Errorf("failed to remove symlink for copy conversion: %w", err)
			}
		}
	}

	// Ensure target directory exists
	if !dryRun {
		if err := os.MkdirAll(target.Path, 0755); err != nil {
			return nil, fmt.Errorf("failed to create target directory: %w", err)
		}
	}

	// Discover and filter source skills
	discoveredSkills, err := DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}
	discoveredSkills, err = FilterSkills(discoveredSkills, target.Include, target.Exclude)
	if err != nil {
		return nil, fmt.Errorf("failed to apply filters for target %s: %w", name, err)
	}
	discoveredSkills = FilterSkillsByTarget(discoveredSkills, name)

	// Read existing manifest
	manifest, err := ReadManifest(target.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	for _, skill := range discoveredSkills {
		targetSkillPath := filepath.Join(target.Path, skill.FlatName)

		// Compute source checksum
		srcChecksum, err := DirChecksum(skill.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to checksum source skill %s: %w", skill.FlatName, err)
		}

		// Check what exists at the target path
		_, lstatErr := os.Lstat(targetSkillPath)
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
				// It's a real directory — check if managed by us
				oldChecksum, isManaged := manifest.Managed[skill.FlatName]

				if !force && isManaged && oldChecksum == srcChecksum {
					// Unchanged — skip
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
	result := &PruneResult{}

	manifest, err := ReadManifest(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Discover current source skills
	allSourceSkills, err := DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills for pruning: %w", err)
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
		if _, err := os.Stat(filepath.Join(targetPath, name)); err == nil {
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

// DirChecksum computes a deterministic SHA256 checksum of a directory.
// It hashes sorted relative paths and file contents.
func DirChecksum(dir string) (string, error) {
	type entry struct {
		relPath string
		content []byte
	}
	var entries []entry

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible
		}
		if info.IsDir() {
			// Skip .git directories
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		// Normalize path separators for cross-platform consistency
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		entries = append(entries, entry{relPath: relPath, content: content})
		return nil
	})
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
