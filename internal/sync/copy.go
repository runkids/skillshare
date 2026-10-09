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
	"skillshare/internal/skillignore"
	"skillshare/internal/utils"
)

// CopyResult holds the result of a copy sync operation.
type CopyResult struct {
	Copied     []string // newly copied skills
	Skipped    []string // checksum unchanged, or a user's folder kept
	KeptLocal  []string // the Skipped that are a user's folder on a skill's name; --force replaces them
	Updated    []string // checksum changed, overwritten
	DirCreated string   // Non-empty if target directory was auto-created (or would be in dry-run)
	// UnmatchedIncludes are the include patterns that select no skill.
	UnmatchedIncludes []UnmatchedInclude
}

// CopyOptions controls copy-mode sync behavior.
type CopyOptions struct {
	IgnorePatterns []string
}

// SyncTargetCopy performs copy mode sync — copies each skill individually
// while preserving target-specific (unmanaged) skills.
func SyncTargetCopy(name string, target config.TargetConfig, sourcePath string, dryRun, force bool) (*CopyResult, error) {
	return SyncTargetCopyWithOptions(name, target, sourcePath, dryRun, force, CopyOptions{
		IgnorePatterns: DefaultFileIgnorePatterns(),
	})
}

// SyncTargetCopyWithOptions is SyncTargetCopy with explicit copy behavior options.
func SyncTargetCopyWithOptions(name string, target config.TargetConfig, sourcePath string, dryRun, force bool, opts CopyOptions) (*CopyResult, error) {
	skills, err := DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}
	return SyncTargetCopyWithSkillsOptions(name, target, skills, sourcePath, dryRun, force, nil, opts)
}

// SyncTargetCopyWithSkills is like SyncTargetCopy but accepts pre-discovered skills
// and an optional progress callback for per-skill UI updates.
// sourcePath is the skills source directory, used to detect symlink-mode targets.
func SyncTargetCopyWithSkills(name string, target config.TargetConfig, allSkills []DiscoveredSkill, sourcePath string, dryRun, force bool, onProgress func(current, total int, skill string)) (*CopyResult, error) {
	return SyncTargetCopyWithSkillsOptions(name, target, allSkills, sourcePath, dryRun, force, onProgress, CopyOptions{
		IgnorePatterns: DefaultFileIgnorePatterns(),
	})
}

// SyncTargetCopyWithSkillsOptions is SyncTargetCopyWithSkills with explicit copy behavior options.
func SyncTargetCopyWithSkillsOptions(name string, target config.TargetConfig, allSkills []DiscoveredSkill, sourcePath string, dryRun, force bool, onProgress func(current, total int, skill string), opts CopyOptions) (*CopyResult, error) {
	sc := target.SkillsConfig()
	result := &CopyResult{}
	if !sc.IsEnabled() {
		return result, nil
	}
	ignorePatterns := opts.IgnorePatterns
	if ignorePatterns == nil {
		ignorePatterns = DefaultFileIgnorePatterns()
	}

	// Convert from symlink mode if needed, auto-create if missing.
	dirCreated, err := ensureRealTargetDir(sc.Path, sourcePath, "copy", dryRun)
	if err != nil {
		return nil, err
	}
	if dirCreated {
		result.DirCreated = sc.Path
	}
	// When dry-run would create the directory, suppress per-skill diagnostic
	// messages (they flood the terminal). Counts are still populated.
	quietDryRun := dirCreated && dryRun

	resolution, err := ResolveTargetSkillsForTarget(name, target.SkillsConfig(), allSkills)
	if err != nil {
		return nil, err
	}
	printResolutionSummary(resolution)
	result.UnmatchedIncludes = resolution.UnmatchedIncludes

	// Read existing manifest
	manifest, err := ReadManifest(sc.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}
	naming := resolution.Naming

	taken := resolution.ValidTargetNames()
	for i, resolved := range resolution.Skills {
		skill := resolved.Skill
		activeName, err := selectActiveTargetNameForSync("copy", sc.Path, resolved, taken, manifest, dryRun)
		if err != nil {
			return nil, err
		}
		if onProgress != nil {
			onProgress(i+1, len(resolution.Skills), activeName)
		}

		targetSkillPath := filepath.Join(sc.Path, activeName)

		// Compute source mtime for fast-path skip
		currentMtime, mtimeErr := DirMaxMtimeWithIgnore(skill.SourcePath, ignorePatterns)

		// mtime fast-path: if source mtime is unchanged AND target is still a valid dir, skip checksum
		oldChecksum, isManaged := manifest.Managed[activeName]
		oldMtime := manifest.Mtimes[activeName] // 0 if missing
		// A copy made under another naming carries another name:, so it is
		// refreshed even when the source is unchanged. A legacy entry kept
		// under its old name keeps its old naming.
		onDesiredName := activeName == resolved.TargetName
		recordedNaming := manifest.Naming[activeName]
		namingChanged := onDesiredName && recordedNaming != "" && recordedNaming != naming
		// Under prefixed naming the copy's name: becomes the entry name, so it
		// matches its folder. A legacy entry kept under its old name is copied as is.
		rewriteName := ""
		if naming == "prefixed" && onDesiredName && resolved.TargetName != resolved.SkillName {
			rewriteName = resolved.TargetName
		}
		recordNaming := func() {
			if onDesiredName && !dryRun {
				manifest.Naming[activeName] = naming
			}
		}
		if mtimeErr == nil && isManaged && !force && !namingChanged && oldMtime > 0 && currentMtime == oldMtime {
			// Verify target still exists as a directory (user may have replaced it)
			if ti, err := os.Lstat(targetSkillPath); err == nil && ti.IsDir() {
				recordNaming()
				result.Skipped = append(result.Skipped, activeName)
				continue
			}
		}

		// mtime changed or no record — compute full checksum
		srcChecksum, err := DirChecksumWithIgnore(skill.SourcePath, ignorePatterns)
		if err != nil {
			return nil, fmt.Errorf("failed to checksum source skill %s: %w", activeName, err)
		}

		// Check what exists at the target path
		targetInfo, lstatErr := os.Lstat(targetSkillPath)
		exists := lstatErr == nil

		if exists {
			// If it's a symlink (leftover from merge mode), remove it
			if utils.IsSymlinkOrJunction(targetSkillPath) {
				if dryRun {
					if !quietDryRun {
						fmt.Fprintf(DiagOutput, "[dry-run] Would replace symlink with copy: %s\n", activeName)
					}
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
							if !quietDryRun {
								fmt.Fprintf(DiagOutput, "[dry-run] Would replace non-directory entry with copy: %s\n", activeName)
							}
						} else {
							if err := os.RemoveAll(targetSkillPath); err != nil {
								return nil, fmt.Errorf("failed to remove invalid entry %s: %w", activeName, err)
							}
							if err := copySkillToTarget(skill.SourcePath, targetSkillPath, rewriteName, ignorePatterns); err != nil {
								return nil, fmt.Errorf("failed to copy skill %s: %w", activeName, err)
							}
							manifest.Managed[activeName] = srcChecksum
							recordNaming()
							if mtimeErr == nil {
								manifest.Mtimes[activeName] = currentMtime
							}
						}
						result.Updated = append(result.Updated, activeName)
						continue
					}

					// Local non-directory entry — preserve unless --force.
					result.Skipped = append(result.Skipped, activeName)
					result.KeptLocal = append(result.KeptLocal, activeName)
					continue
				}

				if !force && isManaged && !namingChanged && oldChecksum == srcChecksum {
					// Unchanged — skip (but update mtime record if it changed)
					if mtimeErr == nil && currentMtime != oldMtime && !dryRun {
						manifest.Mtimes[activeName] = currentMtime
					}
					recordNaming()
					result.Skipped = append(result.Skipped, activeName)
					continue
				}

				if isManaged || force {
					// Managed or forced — overwrite
					if dryRun {
						if !quietDryRun {
							fmt.Fprintf(DiagOutput, "[dry-run] Would update copy: %s\n", activeName)
						}
					} else {
						if err := os.RemoveAll(targetSkillPath); err != nil {
							return nil, fmt.Errorf("failed to remove old copy %s: %w", activeName, err)
						}
					}
					if !dryRun {
						if err := copySkillToTarget(skill.SourcePath, targetSkillPath, rewriteName, ignorePatterns); err != nil {
							return nil, fmt.Errorf("failed to copy skill %s: %w", activeName, err)
						}
						manifest.Managed[activeName] = srcChecksum
						recordNaming()
						if mtimeErr == nil {
							manifest.Mtimes[activeName] = currentMtime
						}
					}
					result.Updated = append(result.Updated, activeName)
					continue
				}

				// Not managed (local skill) — preserve
				result.Skipped = append(result.Skipped, activeName)
				result.KeptLocal = append(result.KeptLocal, activeName)
				continue
			}
		}

		// Copy skill to target
		if dryRun {
			if !quietDryRun {
				dest := targetSkillPath
				// A dry run leaves an entry it would rename at its old name.
				if !onDesiredName {
					newPath := filepath.Join(sc.Path, resolved.TargetName)
					if _, err := os.Lstat(newPath); os.IsNotExist(err) {
						dest = newPath
					}
				}
				fmt.Fprintf(DiagOutput, "[dry-run] Would copy: %s -> %s\n", skill.SourcePath, dest)
			}
		} else {
			if err := copySkillToTarget(skill.SourcePath, targetSkillPath, rewriteName, ignorePatterns); err != nil {
				return nil, fmt.Errorf("failed to copy skill %s: %w", activeName, err)
			}
			manifest.Managed[activeName] = srcChecksum
			recordNaming()
			if mtimeErr == nil {
				manifest.Mtimes[activeName] = currentMtime
			}
		}
		result.Copied = append(result.Copied, activeName)
	}

	// Write updated manifest
	if !dryRun {
		if err := WriteManifest(sc.Path, manifest); err != nil {
			return nil, fmt.Errorf("failed to write manifest: %w", err)
		}
	}

	return result, nil
}

// copySkillToTarget copies a skill into dst and, when newName is set, rewrites
// name: in the copied SKILL.md to it. The source is never touched.
func copySkillToTarget(src, dst, newName string, ignorePatterns []string) error {
	if err := copyDirectoryWithIgnore(src, dst, ignorePatterns); err != nil {
		return err
	}
	if newName == "" {
		return nil
	}
	skillFile := filepath.Join(dst, "SKILL.md")
	// The copy keeps the source's mode; lift a read-only one for the rewrite only.
	if info, err := os.Stat(skillFile); err == nil && info.Mode().Perm()&0o200 == 0 {
		os.Chmod(skillFile, info.Mode().Perm()|0o200)
		defer os.Chmod(skillFile, info.Mode().Perm())
	}
	if err := utils.SetFrontmatterValue(skillFile, "name", newName); err != nil {
		// dst was just created by this copy; left behind without a manifest
		// entry, it would pass for a user's folder and never be refreshed.
		os.RemoveAll(dst)
		return fmt.Errorf("failed to set prefixed name: %w", err)
	}
	return nil
}

// PruneOrphanCopies removes managed copies that no longer exist in source.
func PruneOrphanCopies(targetPath, sourcePath string, include, exclude []string, targetName, targetNaming string, dryRun bool) (*PruneResult, error) {
	allSourceSkills, err := DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills for pruning: %w", err)
	}
	return PruneOrphanCopiesWithSkills(targetPath, allSourceSkills, include, exclude, targetName, targetNaming, dryRun)
}

// PruneOrphanCopiesWithSkills is like PruneOrphanCopies but accepts pre-discovered skills.
func PruneOrphanCopiesWithSkills(targetPath string, allSourceSkills []DiscoveredSkill, include, exclude []string, targetName, targetNaming string, dryRun bool) (*PruneResult, error) {
	result := &PruneResult{}

	manifest, err := ReadManifest(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	resolution, err := ResolveTargetSkillsForTarget(targetName, config.ResourceTargetConfig{
		Path:         targetPath,
		TargetNaming: targetNaming,
		Include:      include,
		Exclude:      exclude,
	}, allSourceSkills)
	if err != nil {
		return nil, err
	}
	validTargetNames := resolution.ValidTargetNames()
	legacyNames := resolution.LegacyNames("copy", targetPath, manifest)

	// Remove manifest entries that are no longer in source
	for entryName := range manifest.Managed {
		if validTargetNames[entryName] {
			continue
		}
		if _, keepLegacy := legacyNames[entryName]; keepLegacy {
			continue
		}

		entryPath := filepath.Join(targetPath, entryName)
		if dryRun {
			fmt.Fprintf(DiagOutput, "[dry-run] Would remove orphan copy: %s\n", entryPath)
		} else {
			if err := os.RemoveAll(entryPath); err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s: failed to remove: %v", entryName, err))
				continue
			}
			manifest.Remove(entryName)
		}
		result.Removed = append(result.Removed, entryName)
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
	return DirMaxMtimeWithIgnore(dir, nil)
}

// DirMaxMtimeWithIgnore returns the latest ModTime among non-ignored files in dir.
func DirMaxMtimeWithIgnore(dir string, ignorePatterns []string) (int64, error) {
	var maxMtime int64
	hasSymlink := false
	ignore := compileFileIgnore(ignorePatterns)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		relPath = strings.ReplaceAll(relPath, "\\", "/")
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			if relPath != "." && isFileIgnored(ignore, relPath, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if isFileIgnored(ignore, relPath, false) {
			return nil
		}
		// Symlink targets can change without updating the link mtime.
		// Disable mtime fast-path for symlink-containing skills.
		if utils.IsLinkMode(path, info.Mode()) {
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
	return DirChecksumWithIgnore(dir, nil)
}

// DirChecksumWithIgnore computes a deterministic checksum of non-ignored files in dir.
func DirChecksumWithIgnore(dir string, ignorePatterns []string) (string, error) {
	var entries []checksumEntry
	err := collectChecksumEntries(dir, "", &entries, map[string]bool{}, compileFileIgnore(ignorePatterns))
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
func collectChecksumEntries(root, relPrefix string, entries *[]checksumEntry, active map[string]bool, ignore *skillignore.Matcher) error {
	resolvedRoot, err := resolveLinkRoot(root)
	if err != nil {
		return fmt.Errorf("failed to resolve checksum root %s: %w", root, err)
	}
	if active[resolvedRoot] {
		return fmt.Errorf("detected symlink directory cycle while checksumming: %s", root)
	}
	active[resolvedRoot] = true
	defer delete(active, resolvedRoot)

	// The root link has already been resolved and entered in the cycle set.
	root = resolvedRoot
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
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
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			if fullRelPath != "" && isFileIgnored(ignore, fullRelPath, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if isFileIgnored(ignore, fullRelPath, false) {
			return nil
		}

		// A symlink can point to a directory. In copy mode we dereference
		// directory symlinks, so checksum should include effective contents.
		if utils.IsLinkMode(path, info.Mode()) {
			targetInfo, statErr := os.Stat(path)
			if statErr != nil {
				return fmt.Errorf("failed to stat symlink target %s: %w", path, statErr)
			}
			if targetInfo.IsDir() {
				resolvedDir, resolveErr := filepath.EvalSymlinks(path)
				if resolveErr != nil {
					return fmt.Errorf("failed to resolve symlink directory %s: %w", path, resolveErr)
				}
				return collectChecksumEntries(resolvedDir, fullRelPath, entries, active, ignore)
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
