package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/utils"
)

// DiscoveredSkill represents a skill found during recursive source directory scan.
type DiscoveredSkill struct {
	SourcePath string   // Full path: ~/.config/skillshare/skills/_team/frontend/ui
	RelPath    string   // Relative path from source: _team/frontend/ui
	FlatName   string   // Flat name for target: _team__frontend__ui
	IsInRepo   bool     // Whether this skill is inside a tracked repo (_-prefixed directory)
	Targets    []string // From SKILL.md frontmatter; nil = all targets
}

// DiscoverSourceSkills recursively scans the source directory for skills.
// A skill is identified by the presence of a SKILL.md file.
// Returns all discovered skills with their metadata for syncing.
func DiscoverSourceSkills(sourcePath string) ([]DiscoveredSkill, error) {
	var skills []DiscoveredSkill

	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		// Skip .git directory only — other hidden directories (e.g., .curated/, .system/)
		// may contain skills (like openai/skills repo structure)
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Look for SKILL.md files
		if !info.IsDir() && info.Name() == "SKILL.md" {
			skillDir := filepath.Dir(path)
			relPath, err := filepath.Rel(sourcePath, skillDir)
			if err != nil {
				return nil // Skip if we can't get relative path
			}

			// Skip root level (source directory itself)
			if relPath == "." {
				return nil
			}

			// Normalize path separators
			relPath = strings.ReplaceAll(relPath, "\\", "/")

			// Check if this skill is inside a tracked repo
			isInRepo := false
			parts := strings.Split(relPath, "/")
			if len(parts) > 0 && utils.IsTrackedRepoDir(parts[0]) {
				isInRepo = true
			}

			targets := utils.ParseFrontmatterList(filepath.Join(skillDir, "SKILL.md"), "targets")

			skills = append(skills, DiscoveredSkill{
				SourcePath: skillDir,
				RelPath:    relPath,
				FlatName:   utils.PathToFlatName(relPath),
				IsInRepo:   isInRepo,
				Targets:    targets,
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk source directory: %w", err)
	}

	return skills, nil
}

// TargetStatus represents the state of a target
type TargetStatus int

const (
	StatusUnknown  TargetStatus = iota
	StatusLinked                // Target is a symlink pointing to source
	StatusNotExist              // Target doesn't exist
	StatusHasFiles              // Target exists with files (needs migration)
	StatusConflict              // Target is a symlink pointing elsewhere
	StatusBroken                // Target is a broken symlink
	StatusMerged                // Target uses merge mode (individual skill symlinks)
	StatusCopied                // Target uses copy mode (individual skill copies + manifest)
)

func (s TargetStatus) String() string {
	switch s {
	case StatusLinked:
		return "linked"
	case StatusNotExist:
		return "not exist"
	case StatusHasFiles:
		return "has files"
	case StatusConflict:
		return "conflict"
	case StatusBroken:
		return "broken"
	case StatusMerged:
		return "merged"
	case StatusCopied:
		return "copied"
	default:
		return "unknown"
	}
}

// CheckStatus checks the status of a target
func CheckStatus(targetPath, sourcePath string) TargetStatus {
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return StatusNotExist
		}
		return StatusUnknown
	}

	// Check if it's a symlink/junction
	if utils.IsSymlinkOrJunction(targetPath) {
		absLink, err := utils.ResolveLinkTarget(targetPath)
		if err != nil {
			return StatusUnknown
		}

		// Check if link points to our source
		absSource, _ := filepath.Abs(sourcePath)

		if utils.PathsEqual(absLink, absSource) {
			// Verify the link is not broken
			if _, err := os.Stat(targetPath); err != nil {
				return StatusBroken
			}
			return StatusLinked
		}
		return StatusConflict
	}

	// It's a directory with files
	if info.IsDir() {
		return StatusHasFiles
	}

	return StatusUnknown
}

// MigrateToSource moves files from target to source, then creates symlink
func MigrateToSource(targetPath, sourcePath string) error {
	// Ensure source parent directory exists
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0755); err != nil {
		return fmt.Errorf("failed to create source parent: %w", err)
	}

	// Check if source already exists
	if _, err := os.Stat(sourcePath); err == nil {
		// Source exists - merge files
		if err := mergeDirectories(targetPath, sourcePath); err != nil {
			return fmt.Errorf("failed to merge directories: %w", err)
		}
		// Remove original target
		if err := os.RemoveAll(targetPath); err != nil {
			return fmt.Errorf("failed to remove target after merge: %w", err)
		}
	} else {
		// Source doesn't exist - just move
		if err := os.Rename(targetPath, sourcePath); err != nil {
			// Cross-device? Try copy then delete
			if err := copyDirectory(targetPath, sourcePath); err != nil {
				return fmt.Errorf("failed to copy to source: %w", err)
			}
			if err := os.RemoveAll(targetPath); err != nil {
				return fmt.Errorf("failed to remove original after copy: %w", err)
			}
		}
	}

	return nil
}

// CreateSymlink creates a symlink (or junction on Windows) from target to source
func CreateSymlink(targetPath, sourcePath string) error {
	// Ensure target parent exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create target parent: %w", err)
	}

	// Create link (uses junction on Windows, symlink on Unix)
	if err := createLink(targetPath, sourcePath); err != nil {
		return fmt.Errorf("failed to create link: %w", err)
	}

	return nil
}

// SyncTarget performs the sync operation for a single target
func SyncTarget(name string, target config.TargetConfig, sourcePath string, dryRun bool) error {
	// Remove manifest if present (merge/copy → symlink conversion)
	if !dryRun {
		RemoveManifest(target.Path) //nolint:errcheck
	}

	status := CheckStatus(target.Path, sourcePath)

	switch status {
	case StatusLinked:
		// Already correct
		return nil

	case StatusNotExist:
		if dryRun {
			fmt.Printf("[dry-run] Would create symlink: %s -> %s\n", target.Path, sourcePath)
			return nil
		}
		return CreateSymlink(target.Path, sourcePath)

	case StatusHasFiles:
		if dryRun {
			fmt.Printf("[dry-run] Would migrate files from %s to %s, then create symlink\n", target.Path, sourcePath)
			return nil
		}
		if err := MigrateToSource(target.Path, sourcePath); err != nil {
			return err
		}
		return CreateSymlink(target.Path, sourcePath)

	case StatusConflict:
		link, err := utils.ResolveLinkTarget(target.Path)
		if err != nil {
			link = "(unable to resolve target)"
		}
		return fmt.Errorf("target is symlink to different location: %s -> %s", target.Path, link)

	case StatusBroken:
		if dryRun {
			fmt.Printf("[dry-run] Would remove broken symlink and recreate: %s\n", target.Path)
			return nil
		}
		os.Remove(target.Path)
		return CreateSymlink(target.Path, sourcePath)

	default:
		return fmt.Errorf("unknown target status: %s", status)
	}
}

// mergeDirectories copies files from src to dst, skipping existing files
func mergeDirectories(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Skip if destination exists
		if _, err := os.Stat(dstPath); err == nil {
			fmt.Printf("  skip (exists): %s\n", relPath)
			return nil
		}

		return copyFile(path, dstPath)
	})
}

// copyDirectory copies a directory recursively
func copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// MergeResult holds the result of a merge sync operation
type MergeResult struct {
	Linked  []string // Skills that were symlinked
	Skipped []string // Skills that already exist in target (kept local)
	Updated []string // Skills that had broken symlinks fixed
}

// SyncTargetMerge performs merge mode sync - creates symlinks for each skill individually
// while preserving target-specific skills.
// Supports nested skills: source path "personal/writing/email" becomes target symlink "personal__writing__email"
// If force is true, local copies will be replaced with symlinks.
func SyncTargetMerge(name string, target config.TargetConfig, sourcePath string, dryRun, force bool) (*MergeResult, error) {
	result := &MergeResult{}

	// Check if target is currently a symlink/junction (symlink mode) - need to convert to merge mode
	info, err := os.Lstat(target.Path)
	if err == nil && info != nil && utils.IsSymlinkOrJunction(target.Path) {
		// Target is a symlink - remove it to convert to merge mode
		if dryRun {
			fmt.Printf("[dry-run] Would convert from symlink mode to merge mode: %s\n", target.Path)
		} else {
			if err := os.Remove(target.Path); err != nil {
				return nil, fmt.Errorf("failed to remove symlink for merge conversion: %w", err)
			}
		}
	}

	// Ensure target directory exists
	if !dryRun {
		if err := os.MkdirAll(target.Path, 0755); err != nil {
			return nil, fmt.Errorf("failed to create target directory: %w", err)
		}
	}

	// Discover all skills recursively from source
	discoveredSkills, err := DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}
	discoveredSkills, err = FilterSkills(discoveredSkills, target.Include, target.Exclude)
	if err != nil {
		return nil, fmt.Errorf("failed to apply filters for target %s: %w", name, err)
	}
	discoveredSkills = FilterSkillsByTarget(discoveredSkills, name)

	for _, skill := range discoveredSkills {
		// Use flat name in target (e.g., "personal__writing__email")
		targetSkillPath := filepath.Join(target.Path, skill.FlatName)

		// Check if skill exists in target
		_, err := os.Lstat(targetSkillPath)
		if err == nil {
			// Something exists at target path
			if utils.IsSymlinkOrJunction(targetSkillPath) {
				// It's a symlink/junction - check if it points to source
				absLink, err := utils.ResolveLinkTarget(targetSkillPath)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve link target for %s: %w", skill.FlatName, err)
				}
				absSource, _ := filepath.Abs(skill.SourcePath)

				if utils.PathsEqual(absLink, absSource) {
					// Already correctly linked
					result.Linked = append(result.Linked, skill.FlatName)
					continue
				}

				// Symlink points elsewhere - broken or wrong
				if dryRun {
					fmt.Printf("[dry-run] Would fix symlink: %s\n", skill.FlatName)
				} else {
					os.Remove(targetSkillPath)
					if err := createLink(targetSkillPath, skill.SourcePath); err != nil {
						return nil, fmt.Errorf("failed to create link for %s: %w", skill.FlatName, err)
					}
				}
				result.Updated = append(result.Updated, skill.FlatName)
			} else {
				// It's a real directory
				if force {
					// Force: replace local copy with symlink
					if dryRun {
						fmt.Printf("[dry-run] Would replace local copy: %s\n", skill.FlatName)
					} else {
						if err := os.RemoveAll(targetSkillPath); err != nil {
							return nil, fmt.Errorf("failed to remove local copy %s: %w", skill.FlatName, err)
						}
						if err := createLink(targetSkillPath, skill.SourcePath); err != nil {
							return nil, fmt.Errorf("failed to create link for %s: %w", skill.FlatName, err)
						}
					}
					result.Updated = append(result.Updated, skill.FlatName)
				} else {
					// Preserve local skill
					result.Skipped = append(result.Skipped, skill.FlatName)
				}
			}
		} else if os.IsNotExist(err) {
			// Doesn't exist - create link
			if dryRun {
				fmt.Printf("[dry-run] Would create link: %s -> %s\n", targetSkillPath, skill.SourcePath)
			} else {
				if err := createLink(targetSkillPath, skill.SourcePath); err != nil {
					return nil, fmt.Errorf("failed to create link for %s: %w", skill.FlatName, err)
				}
			}
			result.Linked = append(result.Linked, skill.FlatName)
		} else {
			return nil, fmt.Errorf("failed to check target skill %s: %w", skill.FlatName, err)
		}
	}

	// Write manifest (additive: merge with existing entries)
	if !dryRun {
		manifest, _ := ReadManifest(target.Path)
		for _, name := range result.Linked {
			manifest.Managed[name] = "symlink"
		}
		for _, name := range result.Updated {
			manifest.Managed[name] = "symlink"
		}
		// Skipped items are NOT added — they are user-local copies
		WriteManifest(target.Path, manifest) //nolint:errcheck
	}

	return result, nil
}

// PruneResult holds the result of a prune operation
type PruneResult struct {
	Removed  []string // Items that were removed
	Warnings []string // Items that were kept with warnings
}

// PruneOrphanLinks removes target entries that are no longer managed by sync.
// This includes:
// 1. Source-linked entries excluded by include/exclude filters (remove from target)
// 2. Orphan links/directories that no longer exist in source
// 3. Unknown local directories (kept with warning)
func PruneOrphanLinks(targetPath, sourcePath string, include, exclude []string, targetName string, dryRun, force bool) (*PruneResult, error) {
	result := &PruneResult{}

	// Read manifest for managed-directory detection (may be empty/absent)
	manifest, _ := ReadManifest(targetPath)
	manifestChanged := false

	// Discover all skills from source, then filter to target-managed skills.
	allSourceSkills, err := DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills for pruning: %w", err)
	}
	managedSkills, err := FilterSkills(allSourceSkills, include, exclude)
	if err != nil {
		return nil, fmt.Errorf("failed to apply filters for pruning: %w", err)
	}
	managedSkills = FilterSkillsByTarget(managedSkills, targetName)
	includePatterns, err := normalizePatterns(include)
	if err != nil {
		return nil, fmt.Errorf("invalid include pattern for pruning: %w", err)
	}
	excludePatterns, err := normalizePatterns(exclude)
	if err != nil {
		return nil, fmt.Errorf("invalid exclude pattern for pruning: %w", err)
	}

	// Build a set of valid flat names
	validFlatNames := make(map[string]bool)
	for _, skill := range managedSkills {
		validFlatNames[skill.FlatName] = true
	}
	// Scan target directory
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil // Target doesn't exist, nothing to prune
		}
		return nil, fmt.Errorf("failed to read target directory: %w", err)
	}

	absSource, _ := filepath.Abs(sourcePath)

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files
		if utils.IsHidden(name) {
			continue
		}

		entryPath := filepath.Join(targetPath, name)
		info, err := os.Lstat(entryPath)
		if err != nil {
			continue
		}

		// Check if this entry is still valid
		if validFlatNames[name] {
			continue // Still exists in source, keep it
		}
		managedByFilter := shouldSyncFlatName(name, includePatterns, excludePatterns)

		// For names outside current filter scope:
		// - remove only symlinks/junctions that point to source (historical sync artifacts)
		// - preserve local directories/files owned by users
		if !managedByFilter {
			// This entry is outside current filter scope, so it should no longer
			// be treated as skillshare-managed for this target.
			_, inManifest := manifest.Managed[name]
			if inManifest {
				delete(manifest.Managed, name)
				manifestChanged = true
			}
			if utils.IsSymlinkOrJunction(entryPath) {
				absLink, err := utils.ResolveLinkTarget(entryPath)
				if err != nil {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("%s: unable to resolve excluded link target, kept", name))
					continue
				}
				if utils.PathHasPrefix(absLink, absSource+string(filepath.Separator)) {
					if dryRun {
						fmt.Printf("[dry-run] Would remove excluded symlink: %s\n", entryPath)
					} else if err := os.RemoveAll(entryPath); err != nil {
						result.Warnings = append(result.Warnings,
							fmt.Sprintf("%s: failed to remove excluded symlink: %v", name, err))
						continue
					}
					result.Removed = append(result.Removed, name)
				}
			} else if inManifest && info.IsDir() {
				// Real directory previously managed by skillshare, now excluded by filter — remove it
				if dryRun {
					fmt.Printf("[dry-run] Would remove excluded managed directory: %s\n", entryPath)
				} else if err := os.RemoveAll(entryPath); err != nil {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("%s: failed to remove excluded managed directory: %v", name, err))
					continue
				}
				result.Removed = append(result.Removed, name)
			}
			continue
		}

		// Entry is orphan - determine if we should remove it
		shouldRemove := false
		reason := ""

		if utils.IsSymlinkOrJunction(entryPath) {
			absLink, err := utils.ResolveLinkTarget(entryPath)
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s: unable to resolve link target, kept", name))
				continue
			}

			targetExists := false
			if _, err := os.Stat(absLink); err == nil {
				targetExists = true
			}

			if utils.PathHasPrefix(absLink, absSource+string(filepath.Separator)) {
				if !targetExists {
					shouldRemove = true
					reason = "broken symlink to source"
				} else {
					shouldRemove = true
					reason = "orphan symlink to source"
				}
			} else if !targetExists {
				// External symlink whose target no longer exists (e.g. after data migration)
				shouldRemove = true
				reason = "broken external symlink"
			} else if force {
				// Valid external symlink, but force mode requested
				shouldRemove = true
				reason = "external symlink (force)"
			} else {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s: symlink to external location (%s), kept", name, absLink))
			}
		} else if info.IsDir() {
			// Check manifest first (most reliable)
			if _, inManifest := manifest.Managed[name]; inManifest {
				shouldRemove = true
				reason = "orphan skillshare-managed directory (manifest)"
			} else if utils.HasNestedSeparator(name) || utils.IsTrackedRepoDir(name) {
				// Fallback: naming pattern heuristic
				shouldRemove = true
				reason = "orphan skillshare-managed directory"
			} else {
				// Unknown directory - warn and keep
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s: unknown directory (not from skillshare), kept", name))
			}
		}

		if shouldRemove {
			if dryRun {
				fmt.Printf("[dry-run] Would remove %s: %s\n", reason, entryPath)
			} else {
				if err := os.RemoveAll(entryPath); err != nil {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("%s: failed to remove: %v", name, err))
					continue
				}
			}
			result.Removed = append(result.Removed, name)
			// Track manifest changes for cleanup
			if _, inManifest := manifest.Managed[name]; inManifest {
				delete(manifest.Managed, name)
				manifestChanged = true
			}
		}
	}

	// Write back manifest if entries were removed (skip in dry-run)
	if manifestChanged && !dryRun {
		WriteManifest(targetPath, manifest) //nolint:errcheck
	}

	return result, nil
}

// NameCollision represents a conflict where multiple skills share the same name
type NameCollision struct {
	Name  string   // The conflicting SKILL.md name
	Paths []string // All paths that have this name
}

// CheckNameCollisions detects skills with duplicate names in SKILL.md.
// Returns a list of collisions (skills that share the same name).
func CheckNameCollisions(skills []DiscoveredSkill) []NameCollision {
	// Map: skill name -> list of paths
	nameMap := make(map[string][]string)

	for _, skill := range skills {
		// Parse the actual name from SKILL.md
		name, err := utils.ParseSkillName(skill.SourcePath)
		if err != nil || name == "" {
			continue // Skip if we can't parse or no name
		}
		nameMap[name] = append(nameMap[name], skill.RelPath)
	}

	// Find collisions (names with multiple paths)
	var collisions []NameCollision
	for name, paths := range nameMap {
		if len(paths) > 1 {
			collisions = append(collisions, NameCollision{
				Name:  name,
				Paths: paths,
			})
		}
	}

	return collisions
}

// TargetCollision holds name collisions that affect a specific target after filtering.
type TargetCollision struct {
	TargetName string
	Collisions []NameCollision
}

// CheckNameCollisionsForTargets checks name collisions both globally and per-target.
// Global collisions are computed on the unfiltered skill set.
// Per-target collisions apply each target's include/exclude filters first, then check.
// Symlink-mode targets are skipped (filters don't apply).
func CheckNameCollisionsForTargets(
	skills []DiscoveredSkill,
	targets map[string]config.TargetConfig,
) (global []NameCollision, perTarget []TargetCollision) {
	global = CheckNameCollisions(skills)

	for name, target := range targets {
		mode := target.Mode
		if mode == "symlink" {
			continue
		}
		if len(target.Include) == 0 && len(target.Exclude) == 0 {
			continue // no filters — same as global
		}
		filtered, err := FilterSkills(skills, target.Include, target.Exclude)
		if err != nil {
			continue
		}
		collisions := CheckNameCollisions(filtered)
		if len(collisions) > 0 {
			perTarget = append(perTarget, TargetCollision{
				TargetName: name,
				Collisions: collisions,
			})
		}
	}

	return global, perTarget
}

// CheckStatusMerge checks the status of a target in merge mode
func CheckStatusMerge(targetPath, sourcePath string) (TargetStatus, int, int) {
	// Returns: status, linked count, local count

	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return StatusNotExist, 0, 0
		}
		return StatusUnknown, 0, 0
	}

	// If it's a symlink/junction to source, it's using symlink mode not merge
	if utils.IsSymlinkOrJunction(targetPath) {
		absLink, err := utils.ResolveLinkTarget(targetPath)
		if err != nil {
			return StatusUnknown, 0, 0
		}
		absSource, _ := filepath.Abs(sourcePath)
		if utils.PathsEqual(absLink, absSource) {
			return StatusLinked, 0, 0
		}
		return StatusConflict, 0, 0
	}

	if !info.IsDir() {
		return StatusUnknown, 0, 0
	}

	// Count linked vs local skills
	linkedCount := 0
	localCount := 0

	entries, _ := os.ReadDir(targetPath)
	for _, entry := range entries {
		if utils.IsHidden(entry.Name()) {
			continue
		}
		skillPath := filepath.Join(targetPath, entry.Name())

		if utils.IsSymlinkOrJunction(skillPath) {
			// It's a symlink/junction - check if it points to somewhere in source
			absLink, err := utils.ResolveLinkTarget(skillPath)
			if err != nil {
				localCount++
				continue
			}
			absSource, _ := filepath.Abs(sourcePath)

			// Check if the symlink target is within the source directory
			if utils.PathHasPrefix(absLink, absSource+string(filepath.Separator)) || utils.PathsEqual(absLink, absSource) {
				linkedCount++
			} else {
				localCount++
			}
		} else {
			localCount++
		}
	}

	if linkedCount > 0 {
		return StatusMerged, linkedCount, localCount
	}

	return StatusHasFiles, 0, localCount
}
