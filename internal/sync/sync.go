package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/skillignore"
	"skillshare/internal/utils"
)

// DiagOutput controls where diagnostic messages (dry-run notices, skip notices)
// are written. Defaults to stdout. Set to io.Discard for silent operation (e.g. JSON mode).
var DiagOutput io.Writer = os.Stdout

// DiscoveredSkill represents a skill found during recursive source directory scan.
type DiscoveredSkill struct {
	SourcePath  string      // Full path: ~/.config/skillshare/skills/_team/frontend/ui
	RelPath     string      // Relative path from source: _team/frontend/ui
	FlatName    string      // Flat name for target: _team__frontend__ui
	IsInRepo    bool        // Whether this skill is inside a tracked repo (_-prefixed directory)
	Targets     []string    // From SKILL.md frontmatter; nil = all targets
	DescChars   int         // Rune count of name + description (populated when collectContext)
	BodyChars   int         // Rune count of body after frontmatter (populated when collectContext)
	Description string      // Frontmatter description text (populated when collectContext)
	LintIssues  []LintIssue // Lint issues (populated when collectContext)
	Disabled    bool        // Whether this skill is ignored by .skillignore
}

// isSkillIgnored checks whether a skill inside a tracked repo should be
// skipped based on the repo's .skillignore matcher.
// parts is strings.Split(relPath, "/"), where relPath is relative to source root.
func isSkillIgnored(parts []string, walkRoot string, ignoreMatchers map[string]*skillignore.Matcher) bool {
	if len(parts) < 2 {
		return false
	}
	repoAbsPath := filepath.Join(walkRoot, parts[0])
	m, ok := ignoreMatchers[repoAbsPath]
	if !ok {
		return false
	}
	return m.Match(strings.Join(parts[1:], "/"), false)
}

// DiscoverSourceSkillsLite recursively scans the source directory for skills
// without parsing SKILL.md frontmatter. Targets is always nil for each skill.
// It also collects tracked repo paths (directories starting with _ that contain
// .git) during the same walk, eliminating the need for a separate GetTrackedRepos call.
//
// Use this for commands like list/uninstall that don't need per-skill target filtering.
func DiscoverSourceSkillsLite(sourcePath string) ([]DiscoveredSkill, []string, error) {
	skills, trackedRepos, _, err := discoverSourceSkillsInternal(sourcePath, discoverOptions{
		parseFrontmatter: false,
		collectIgnored:   false,
		collectTracked:   true,
	})
	return skills, trackedRepos, err
}

// DiscoverSourceSkills recursively scans the source directory for skills.
// A skill is identified by the presence of a SKILL.md file.
// Returns all discovered skills with their metadata for syncing.
func DiscoverSourceSkills(sourcePath string) ([]DiscoveredSkill, error) {
	skills, _, _, err := discoverSourceSkillsInternal(sourcePath, discoverOptions{
		parseFrontmatter: true,
		collectIgnored:   false,
		collectTracked:   false,
	})
	return skills, err
}

// DiscoverSourceSkillsForAnalyze scans skills and computes context usage
// (name+description chars, body chars) in a single pass. Avoids re-reading
// SKILL.md files in a separate analysis phase.
func DiscoverSourceSkillsForAnalyze(sourcePath string) ([]DiscoveredSkill, error) {
	skills, _, _, err := discoverSourceSkillsInternal(sourcePath, discoverOptions{
		parseFrontmatter: true,
		collectContext:   true,
	})
	return skills, err
}

// DiscoverSourceSkillsWithStats recursively scans the source directory for skills
// and collects .skillignore statistics (which files are active, patterns, ignored paths).
// Use this for commands like doctor/status that need to report on .skillignore state.
func DiscoverSourceSkillsWithStats(sourcePath string) ([]DiscoveredSkill, *skillignore.IgnoreStats, error) {
	skills, _, stats, err := discoverSourceSkillsInternal(sourcePath, discoverOptions{
		parseFrontmatter: true,
		collectIgnored:   true,
		collectTracked:   false,
	})
	return skills, stats, err
}

// DiscoverSourceSkillsAll scans the source directory and returns ALL skills
// including those ignored by .skillignore. Ignored skills have Disabled=true.
// Use this for list/UI commands that need to show disabled skills.
func DiscoverSourceSkillsAll(sourcePath string) ([]DiscoveredSkill, error) {
	skills, _, _, err := discoverSourceSkillsInternal(sourcePath, discoverOptions{
		parseFrontmatter: true,
		includeIgnored:   true,
	})
	return skills, err
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
	if err := createLink(targetPath, sourcePath, false); err != nil {
		return fmt.Errorf("failed to create link: %w", err)
	}

	return nil
}

// SyncTarget performs the sync operation for a single target
func SyncTarget(name string, target config.TargetConfig, sourcePath string, dryRun bool) error {
	sc := target.SkillsConfig()

	// Remove manifest if present (merge/copy → symlink conversion)
	if !dryRun {
		RemoveManifest(sc.Path) //nolint:errcheck
	}

	status := CheckStatus(sc.Path, sourcePath)

	switch status {
	case StatusLinked:
		// Already correct
		return nil

	case StatusNotExist:
		if dryRun {
			fmt.Fprintf(DiagOutput, "[dry-run] Would create symlink: %s -> %s\n", sc.Path, sourcePath)
			return nil
		}
		return CreateSymlink(sc.Path, sourcePath)

	case StatusHasFiles:
		if dryRun {
			fmt.Fprintf(DiagOutput, "[dry-run] Would migrate files from %s to %s, then create symlink\n", sc.Path, sourcePath)
			return nil
		}
		if err := MigrateToSource(sc.Path, sourcePath); err != nil {
			return err
		}
		return CreateSymlink(sc.Path, sourcePath)

	case StatusConflict:
		link, err := utils.ResolveLinkTarget(sc.Path)
		if err != nil {
			link = "(unable to resolve target)"
		}
		return fmt.Errorf("target is symlink to different location: %s -> %s", sc.Path, link)

	case StatusBroken:
		if dryRun {
			fmt.Fprintf(DiagOutput, "[dry-run] Would remove broken symlink and recreate: %s\n", sc.Path)
			return nil
		}
		os.Remove(sc.Path)
		return CreateSymlink(sc.Path, sourcePath)

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
			fmt.Fprintf(DiagOutput, "  skip (exists): %s\n", relPath)
			return nil
		}

		return copyFile(path, dstPath)
	})
}

// copyDirectory copies a directory recursively
func copyDirectory(src, dst string) error {
	return copyDirectoryWithState(src, dst, map[string]bool{}, nil)
}

// copyDirectoryOpts controls behavior of copyDirectoryWithState.
type copyDirectoryOpts struct {
	SkipGit bool // skip .git directories
}

// copyDirectorySkipGit copies a directory recursively, skipping .git directories.
// Use this for collect/pull operations where .git is not wanted in the destination.
func copyDirectorySkipGit(src, dst string) error {
	return copyDirectoryWithState(src, dst, map[string]bool{}, &copyDirectoryOpts{SkipGit: true})
}

// copyDirectoryWithState copies recursively and dereferences directory symlinks.
// active tracks real paths in the current recursion stack to prevent cycles.
func copyDirectoryWithState(src, dst string, active map[string]bool, opts *copyDirectoryOpts) error {
	resolvedSrc, err := filepath.EvalSymlinks(src)
	if err != nil {
		return fmt.Errorf("failed to resolve source directory %s: %w", src, err)
	}
	if active[resolvedSrc] {
		return fmt.Errorf("detected symlink directory cycle while copying: %s", src)
	}
	active[resolvedSrc] = true
	defer delete(active, resolvedSrc)

	skipGit := opts != nil && opts.SkipGit

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			if skipGit && info.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(dstPath, info.Mode())
		}

		// In copy mode we need real files/dirs, so directory symlinks are
		// dereferenced and copied as concrete directories.
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
				return copyDirectoryWithState(resolvedDir, dstPath, active, opts)
			}
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
	Linked     []string // Skills that were symlinked
	Skipped    []string // Skills that already exist in target (kept local)
	Updated    []string // Skills that had broken symlinks fixed
	DirCreated string   // Non-empty if target directory was auto-created (or would be in dry-run)
}

// isSymlinkToSource checks whether targetPath is a symlink pointing to sourcePath.
// Both sides are canonicalized (EvalSymlinks) so that symlink aliases of the same
// physical directory are recognized as equal.
func isSymlinkToSource(targetPath, sourcePath string) bool {
	absLink, err := utils.ResolveLinkTarget(targetPath)
	if err != nil {
		return false
	}
	absSource, _ := filepath.Abs(sourcePath)

	// Fast path: direct string comparison covers the common case.
	if utils.PathsEqual(absLink, absSource) {
		return true
	}

	// Slow path: canonicalize both sides to detect symlink aliases.
	canonLink := utils.ResolveSymlink(absLink)
	canonSource := utils.ResolveSymlink(absSource)
	return utils.PathsEqual(canonLink, canonSource)
}

// ensureRealTargetDir handles symlink→merge/copy conversion and verifies the
// target directory exists. Returns an error if the directory is missing or
// inaccessible (never auto-creates to catch typos).
// ensureRealTargetDir handles symlink→merge/copy conversion and auto-creates
// missing target directories. Returns (true, nil) when a directory was created
// (or would be in dry-run), (false, nil) when it already existed.
func ensureRealTargetDir(targetPath, sourcePath, modeName string, dryRun bool) (created bool, err error) {
	info, lstatErr := os.Lstat(targetPath)
	if lstatErr == nil && info != nil && utils.IsSymlinkOrJunction(targetPath) {
		if isSymlinkToSource(targetPath, sourcePath) {
			if dryRun {
				fmt.Fprintf(DiagOutput, "[dry-run] Would convert from symlink mode to %s mode: %s\n", modeName, targetPath)
				return false, nil
			}
			if err := os.Remove(targetPath); err != nil {
				return false, fmt.Errorf("failed to remove symlink for %s conversion: %w", modeName, err)
			}
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return false, fmt.Errorf("failed to create target directory after symlink conversion: %w", err)
			}
			return false, nil // converted — directory exists
		}
		// else: target is an external symlink (dotfiles manager, etc.) — keep it
	}

	// Auto-create target directory if missing, with a visible notification.
	// This handles fresh CLI installs (e.g., ~/.claude/ exists but skills/ doesn't)
	// and built-in targets like universal (~/.agents/skills) where the entire
	// path tree may not exist yet.
	fi, statErr := os.Stat(targetPath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			if dryRun {
				return true, nil
			}
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return false, fmt.Errorf("failed to create target directory: %w", err)
			}
			return true, nil
		}
		return false, fmt.Errorf("cannot access target directory: %w", statErr)
	}
	if !fi.IsDir() {
		return false, fmt.Errorf("target path is not a directory: %s", targetPath)
	}
	return false, nil
}

// SyncTargetMerge performs merge mode sync - creates symlinks for each skill individually
// while preserving target-specific skills.
// Supports nested skills: source path "personal/writing/email" becomes target symlink "personal__writing__email"
// If force is true, local copies will be replaced with symlinks.
func SyncTargetMerge(name string, target config.TargetConfig, sourcePath string, dryRun, force bool) (*MergeResult, error) {
	skills, err := DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}
	return SyncTargetMergeWithSkills(name, target, skills, sourcePath, dryRun, force)
}

// SyncTargetMergeWithSkills is like SyncTargetMerge but accepts pre-discovered skills,
// avoiding redundant filesystem walks when syncing multiple targets.
// sourcePath is the skills source directory, used to detect symlink-mode targets.
func SyncTargetMergeWithSkills(name string, target config.TargetConfig, allSkills []DiscoveredSkill, sourcePath string, dryRun, force bool) (*MergeResult, error) {
	sc := target.SkillsConfig()
	result := &MergeResult{}

	// Convert from symlink mode if needed, auto-create if missing.
	dirCreated, err := ensureRealTargetDir(sc.Path, sourcePath, "merge", dryRun)
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
	if n := len(resolution.Warnings); n > 0 {
		fmt.Fprintf(DiagOutput, "  %d skill(s) skipped (naming validation)\n", n)
	}
	if n := len(resolution.Collisions); n > 0 {
		fmt.Fprintf(DiagOutput, "  %d name collision(s) excluded\n", n)
	}

	manifest, err := ReadManifest(sc.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	for _, resolved := range resolution.Skills {
		skill := resolved.Skill
		activeName, err := selectActiveTargetNameForSync("merge", sc.Path, resolved, manifest, dryRun)
		if err != nil {
			return nil, err
		}
		targetSkillPath := filepath.Join(sc.Path, activeName)

		// Check if skill exists in target
		_, err = os.Lstat(targetSkillPath)
		if err == nil {
			// Something exists at target path
			if utils.IsSymlinkOrJunction(targetSkillPath) {
				// It's a symlink/junction - check if it points to source
				absLink, err := utils.ResolveLinkTarget(targetSkillPath)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve link target for %s: %w", activeName, err)
				}
				absSource, _ := filepath.Abs(skill.SourcePath)

				if utils.PathsEqual(absLink, absSource) {
					// Already correctly linked
					result.Linked = append(result.Linked, activeName)
					continue
				}

				// Symlink points elsewhere - broken or wrong
				if dryRun {
					if !quietDryRun {
						fmt.Fprintf(DiagOutput, "[dry-run] Would fix symlink: %s\n", activeName)
					}
				} else {
					os.Remove(targetSkillPath)
					if err := createLink(targetSkillPath, skill.SourcePath, false); err != nil {
						return nil, fmt.Errorf("failed to create link for %s: %w", activeName, err)
					}
				}
				result.Updated = append(result.Updated, activeName)
			} else {
				// It's a real directory
				if force {
					// Force: replace local copy with symlink
					if dryRun {
						if !quietDryRun {
							fmt.Fprintf(DiagOutput, "[dry-run] Would replace local copy: %s\n", activeName)
						}
					} else {
						if err := os.RemoveAll(targetSkillPath); err != nil {
							return nil, fmt.Errorf("failed to remove local copy %s: %w", activeName, err)
						}
						if err := createLink(targetSkillPath, skill.SourcePath, false); err != nil {
							return nil, fmt.Errorf("failed to create link for %s: %w", activeName, err)
						}
					}
					result.Updated = append(result.Updated, activeName)
				} else {
					// Preserve local skill
					result.Skipped = append(result.Skipped, activeName)
				}
			}
		} else if os.IsNotExist(err) {
			// Doesn't exist - create link
			if dryRun {
				if !quietDryRun {
					fmt.Fprintf(DiagOutput, "[dry-run] Would create link: %s -> %s\n", targetSkillPath, skill.SourcePath)
				}
			} else {
				if err := createLink(targetSkillPath, skill.SourcePath, false); err != nil {
					return nil, fmt.Errorf("failed to create link for %s: %w", activeName, err)
				}
			}
			result.Linked = append(result.Linked, activeName)
		} else {
			return nil, fmt.Errorf("failed to check target skill %s: %w", activeName, err)
		}
	}

	// Write manifest (additive: merge with existing entries)
	if !dryRun {
		for _, name := range result.Linked {
			manifest.Managed[name] = "symlink"
		}
		for _, name := range result.Updated {
			manifest.Managed[name] = "symlink"
		}
		// Skipped items are NOT added — they are user-local copies
		WriteManifest(sc.Path, manifest) //nolint:errcheck
	}

	return result, nil
}

// PruneResult holds the result of a prune operation
type PruneResult struct {
	Removed   []string // Items that were removed
	Warnings  []string // Items that were kept with warnings
	LocalDirs []string // User-created directories not managed by skillshare
}

// PruneOptions holds parameters for PruneOrphanLinks / PruneOrphanCopies.
type PruneOptions struct {
	TargetPath   string
	SourcePath   string
	Skills       []DiscoveredSkill // pre-discovered; if nil, will be discovered from SourcePath
	Include      []string
	Exclude      []string
	TargetNaming string
	TargetName   string
	DryRun       bool
	Force        bool
}

// PruneOrphanLinks removes target entries that are no longer managed by sync.
// This includes:
// 1. Source-linked entries excluded by include/exclude filters (remove from target)
// 2. Orphan links/directories that no longer exist in source
// 3. Unknown local directories (kept with warning)
func PruneOrphanLinks(targetPath, sourcePath string, include, exclude []string, targetName, targetNaming string, dryRun, force bool) (*PruneResult, error) {
	allSourceSkills, err := DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills for pruning: %w", err)
	}
	return PruneOrphanLinksWithSkills(PruneOptions{
		TargetPath:   targetPath,
		SourcePath:   sourcePath,
		Skills:       allSourceSkills,
		Include:      include,
		Exclude:      exclude,
		TargetNaming: targetNaming,
		TargetName:   targetName,
		DryRun:       dryRun,
		Force:        force,
	})
}

// PruneOrphanLinksWithSkills is like PruneOrphanLinks but accepts pre-discovered skills
// via PruneOptions, avoiding redundant filesystem walks.
func PruneOrphanLinksWithSkills(opts PruneOptions) (*PruneResult, error) {
	targetPath := opts.TargetPath
	sourcePath := opts.SourcePath
	allSourceSkills := opts.Skills
	include := opts.Include
	exclude := opts.Exclude
	targetNaming := opts.TargetNaming
	targetName := opts.TargetName
	dryRun := opts.DryRun
	force := opts.Force
	result := &PruneResult{}

	// Read manifest for managed-directory detection (may be empty/absent)
	manifest, _ := ReadManifest(targetPath)
	manifestChanged := false

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
	legacyNames := resolution.LegacyFlatNames()
	naming := config.EffectiveTargetNaming(targetNaming)
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
		if validTargetNames[name] {
			continue // Still exists in source, keep it
		}
		if _, keepLegacy := legacyNames[name]; keepLegacy {
			continue // Still exists in source, keep it
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
			} else if naming == "flat" && (utils.HasNestedSeparator(name) || utils.IsTrackedRepoDir(name)) {
				// Fallback: naming pattern heuristic
				shouldRemove = true
				reason = "orphan skillshare-managed directory"
			} else {
				// User-created local directory — record but don't warn
				result.LocalDirs = append(result.LocalDirs, name)
			}
		}

		if shouldRemove {
			if dryRun {
				fmt.Fprintf(DiagOutput, "[dry-run] Would remove %s: %s\n", reason, entryPath)
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
		sc := target.SkillsConfig()
		if sc.Mode == "symlink" {
			continue
		}
		// In flat naming mode without filters, per-target collisions are
		// identical to the global check — skip the redundant resolution.
		naming := config.EffectiveTargetNaming(sc.TargetNaming)
		if naming == "flat" && len(sc.Include) == 0 && len(sc.Exclude) == 0 {
			continue
		}
		resolution, err := ResolveTargetSkillsForTarget(name, sc, skills)
		if err != nil {
			continue
		}
		if len(resolution.Collisions) > 0 {
			perTarget = append(perTarget, TargetCollision{
				TargetName: name,
				Collisions: resolution.Collisions,
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
		// External symlink (e.g., dotfiles manager) — follow it and treat
		// the resolved directory as a normal target directory.
		resolved, statErr := os.Stat(targetPath)
		if statErr != nil || !resolved.IsDir() {
			return StatusConflict, 0, 0
		}
		// Fall through to count linked/local skills in the resolved directory
	} else if !info.IsDir() {
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
