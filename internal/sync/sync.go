package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/skillignore"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
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
	DescTokens  int         // Estimated tokens of name + description (populated when collectContext)
	BodyTokens  int         // Estimated tokens of body (populated when collectContext)
	Description string      // Frontmatter description text (populated when collectContext)
	LintIssues  []LintIssue // Lint issues (populated when collectContext)
	Disabled    bool        // Whether this skill is ignored by .skillignore
	Local       bool        // Found in a target folder rather than the source; see TargetSkills
}

// repoIgnoreMatcher returns the .skillignore matcher of the innermost tracked
// repo strictly above relPath (relative to the source root, slash-separated),
// and relPath made relative to that repo. A repo may sit at any depth, such as
// one installed with --into or one inside a followed group.
func repoIgnoreMatcher(relPath, walkRoot string, ignoreMatchers map[string]*skillignore.Matcher) (*skillignore.Matcher, string) {
	parts := strings.Split(relPath, "/")
	for i := len(parts) - 1; i > 0; i-- {
		if m, ok := ignoreMatchers[filepath.Join(walkRoot, filepath.FromSlash(strings.Join(parts[:i], "/")))]; ok {
			return m, strings.Join(parts[i:], "/")
		}
	}
	return nil, ""
}

// isSkillIgnored checks whether a skill inside a tracked repo should be
// skipped based on the repo's .skillignore matcher.
func isSkillIgnored(relPath, walkRoot string, ignoreMatchers map[string]*skillignore.Matcher) bool {
	m, repoRelPath := repoIgnoreMatcher(relPath, walkRoot, ignoreMatchers)
	return m != nil && m.Match(repoRelPath, false)
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
		// Source exists - merge files through the source handle
		src, err := sourcefs.Open(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to open source: %w", err)
		}
		defer src.Close()
		if err := mergeDirectories(src.Writer(), targetPath, sourcePath); err != nil {
			return fmt.Errorf("failed to merge directories: %w", err)
		}
		// Remove original target
		if err := os.RemoveAll(targetPath); err != nil {
			return fmt.Errorf("failed to remove target after merge: %w", err)
		}
	} else {
		// Source doesn't exist - just move
		if err := os.Rename(targetPath, sourcePath); err != nil {
			// Cross-device? Try copy then delete. The copy creates the
			// missing source root itself, so nothing in it can be a link yet.
			if err := copyDirectory(sourcefs.OS, targetPath, sourcePath); err != nil {
				return fmt.Errorf("failed to copy to source: %w", err)
			}
			if err := os.RemoveAll(targetPath); err != nil {
				return fmt.Errorf("failed to remove original after copy: %w", err)
			}
		}
	}

	return nil
}

// CreateSymlink creates a symlink (or junction on Windows) from target to source.
// When projectRoot is non-empty and both paths reside under it, a relative symlink is created.
func CreateSymlink(targetPath, sourcePath, projectRoot string) error {
	relative := shouldUseRelative(projectRoot, sourcePath, targetPath)
	// Ensure target parent exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create target parent: %w", err)
	}

	// Create link (uses junction on Windows, symlink on Unix)
	if err := createSkillLink(targetPath, sourcePath, skillLinkPath{root: sourcePath}, relative); err != nil {
		return fmt.Errorf("failed to create link: %w", err)
	}

	return nil
}

// SyncTarget performs the sync operation for a single target.
// projectRoot enables relative symlinks when non-empty.
func SyncTarget(name string, target config.TargetConfig, sourcePath string, dryRun bool, projectRoot string) error {
	sc := target.SkillsConfig()
	if !sc.IsEnabled() {
		return nil
	}

	// Remove manifest if present (merge/copy → symlink conversion)
	if !dryRun {
		RemoveManifest(sc.Path) //nolint:errcheck
	}

	status := CheckStatus(sc.Path, sourcePath)

	switch status {
	case StatusLinked:
		relative := shouldUseRelative(projectRoot, sourcePath, sc.Path)
		dest, _ := os.Readlink(sc.Path)
		if !linkNeedsReformat(dest, relative) {
			return nil
		}
		// Correct target but wrong format — recreate
		if dryRun {
			fmt.Fprintf(DiagOutput, "[dry-run] Would reformat symlink: %s\n", sc.Path)
			return nil
		}
		return reformatSkillLink(sc.Path, sourcePath, skillLinkPath{root: sourcePath}, relative)

	case StatusNotExist:
		if dryRun {
			fmt.Fprintf(DiagOutput, "[dry-run] Would create symlink: %s -> %s\n", sc.Path, sourcePath)
			return nil
		}
		return CreateSymlink(sc.Path, sourcePath, projectRoot)

	case StatusHasFiles:
		if dryRun {
			fmt.Fprintf(DiagOutput, "[dry-run] Would migrate files from %s to %s, then create symlink\n", sc.Path, sourcePath)
			return nil
		}
		if err := MigrateToSource(sc.Path, sourcePath); err != nil {
			return err
		}
		return CreateSymlink(sc.Path, sourcePath, projectRoot)

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
		return CreateSymlink(sc.Path, sourcePath, projectRoot)

	default:
		return fmt.Errorf("unknown target status: %s", status)
	}
}

// mergeDirectories copies files from src to dst through w, skipping existing files
func mergeDirectories(w sourcefs.Writer, src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return w.MkdirAll(dstPath, info.Mode())
		}

		// Skip if destination exists
		if _, err := os.Stat(dstPath); err == nil {
			fmt.Fprintf(DiagOutput, "  skip (exists): %s\n", relPath)
			return nil
		}

		return copyFile(w, path, dstPath)
	})
}

// copyDirectory copies a directory recursively through w
func copyDirectory(w sourcefs.Writer, src, dst string) error {
	return copyDirectoryWithState(w, src, dst, map[string]bool{}, nil)
}

// copyDirectoryOpts controls behavior of copyDirectoryWithState.
type copyDirectoryOpts struct {
	SkipGit bool // skip .git directories
	Ignore  *skillignore.Matcher
}

// copyDirectorySkipGit copies a directory recursively through w, skipping .git directories.
// Use this for collect/pull operations where .git is not wanted in the destination.
func copyDirectorySkipGit(w sourcefs.Writer, src, dst string) error {
	return copyDirectoryWithState(w, src, dst, map[string]bool{}, &copyDirectoryOpts{SkipGit: true})
}

// copyDirectoryWithIgnore copies a skill into a copy-mode target.
func copyDirectoryWithIgnore(src, dst string, ignorePatterns []string) error {
	return copyDirectoryWithState(sourcefs.OS, src, dst, map[string]bool{}, &copyDirectoryOpts{
		SkipGit: true,
		Ignore:  compileFileIgnore(ignorePatterns),
	})
}

// copyDirectoryWithState copies recursively and dereferences directory symlinks.
// active tracks real paths in the current recursion stack to prevent cycles.
func copyDirectoryWithState(w sourcefs.Writer, src, dst string, active map[string]bool, opts *copyDirectoryOpts) error {
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
	var ignore *skillignore.Matcher
	if opts != nil {
		ignore = opts.Ignore
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		relPath = strings.ReplaceAll(relPath, "\\", "/")
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			if skipGit && info.Name() == ".git" {
				return filepath.SkipDir
			}
			if relPath != "." && isFileIgnored(ignore, relPath, true) {
				return filepath.SkipDir
			}
			return w.MkdirAll(dstPath, info.Mode())
		}
		if isFileIgnored(ignore, relPath, false) {
			return nil
		}

		// In copy mode we need real files/dirs, so directory symlinks are
		// dereferenced and copied as concrete directories.
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
				return copyDirectoryWithState(w, resolvedDir, dstPath, active, opts)
			}
		}

		return copyFile(w, path, dstPath)
	})
}

// copyFile copies a single file through w
func copyFile(w sourcefs.Writer, src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := w.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
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
	Warnings   []string // include patterns that select no skill
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

// SyncTargetMergeWithSkills performs merge mode sync with pre-discovered skills:
// it links each skill individually and preserves target-specific skills. Nested
// source path "personal/writing/email" becomes target link "personal__writing__email".
// sourcePath is the skills source directory, used to detect symlink-mode targets.
func SyncTargetMergeWithSkills(name string, target config.TargetConfig, allSkills []DiscoveredSkill, sourcePath string, dryRun, force bool, projectRoot string) (*MergeResult, error) {
	return SyncTargetMergeWithSkillsOptions(name, target, allSkills, sourcePath, dryRun, force, projectRoot, MergeOptions{})
}

// MergeOptions carries one operation's policy into a merge sync.
type MergeOptions struct {
	// Follow is the operation's .skillfollow snapshot; nil keeps legacy rules.
	Follow *sourcewalk.FollowSet
}

// SyncTargetMergeWithSkillsOptions is SyncTargetMergeWithSkills with an explicit
// follow policy. Link identity accepts links through followed entries, and while
// an entry is unavailable a replaced link is reported with a warning.
func SyncTargetMergeWithSkillsOptions(name string, target config.TargetConfig, allSkills []DiscoveredSkill, sourcePath string, dryRun, force bool, projectRoot string, opts MergeOptions) (*MergeResult, error) {
	sc := target.SkillsConfig()
	result := &MergeResult{}
	if !sc.IsEnabled() {
		return result, nil
	}
	// Checked against sourcePath (root dir), not per-skill paths — all skills
	// are children of sourcePath so the result is the same for every skill.
	relative := shouldUseRelative(projectRoot, sourcePath, sc.Path)

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
	result.Warnings = resolution.UnmatchedIncludeWarnings()
	scope := newFollowScope(sourcePath, opts.Follow)
	paused := len(scope.unavailable()) > 0

	manifest, err := ReadManifest(sc.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	for _, resolved := range resolution.Skills {
		skill := resolved.Skill
		skillPath := scope.skillPath(skill)
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
				// It's a symlink/junction - check if it points to this skill
				if _, err := utils.ResolveLinkTarget(targetSkillPath); err != nil {
					return nil, fmt.Errorf("failed to resolve link target for %s: %w", activeName, err)
				}

				if sameSkillLink(targetSkillPath, skill, scope) {
					rawDest, _ := os.Readlink(targetSkillPath)
					if !linkNeedsReformat(rawDest, relative) {
						// Already correctly linked with correct format
						result.Linked = append(result.Linked, activeName)
						continue
					}
					// Correct target but wrong format (abs↔rel) — recreate
					if !dryRun {
						if err := reformatSkillLink(targetSkillPath, skill.SourcePath, skillPath, relative); err != nil {
							return nil, fmt.Errorf("failed to reformat link for %s: %w", activeName, err)
						}
					}
					result.Updated = append(result.Updated, activeName)
					continue
				}

				// Symlink points elsewhere - broken or wrong
				if paused {
					// The manifest records no origin, so the old link may have
					// served this name from the unavailable entry.
					result.Warnings = append(result.Warnings, fmt.Sprintf(
						"%s: relinked while a followed entry is unavailable; this name may previously have come from that entry and may collide when it returns", activeName))
				}
				if dryRun {
					if !quietDryRun {
						fmt.Fprintf(DiagOutput, "[dry-run] Would fix symlink: %s\n", activeName)
					}
				} else {
					os.Remove(targetSkillPath)
					if err := createSkillLink(targetSkillPath, skill.SourcePath, skillPath, relative); err != nil {
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
						if err := createSkillLink(targetSkillPath, skill.SourcePath, skillPath, relative); err != nil {
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
				if err := createSkillLink(targetSkillPath, skill.SourcePath, skillPath, relative); err != nil {
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
	LocalDirs []string // User-created entries (directories or links) not managed by skillshare
	// Paused names the unavailable followed entries, as "name (state)", that
	// stopped this prune. Nothing is removed while it is non-empty.
	Paused []string
}

// PruneOptions holds parameters for PruneOrphanLinksWithSkills and PruneOrphanCopiesWithOptions.
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
	// ManagedOnly removes only entries that are provably skillshare's: broken
	// links into the source and manifest-tracked links and directories. The name heuristic and
	// broken-external-link cleanup are skipped. For directories that are not a
	// configured target's own path.
	ManagedOnly bool
	// Follow is the operation's .skillfollow snapshot; nil keeps legacy rules.
	// Managed links into a currently followed entry's target are owned too.
	Follow *sourcewalk.FollowSet
}

// PruneOrphanLinksWithSkills removes target entries that sync no longer manages:
// filtered-out source links, orphans missing from opts.Skills, and nothing it
// cannot prove is skillshare's. A link still resolving into the source is
// removed only when the manifest records it (or Force is set).
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
	scope := newFollowScope(sourcePath, opts.Follow)
	// Discovery is incomplete, so no absent name can be attributed. --force
	// does not override this.
	if result.Paused = scope.paused(); len(result.Paused) > 0 {
		return result, nil
	}

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

		// Skip hidden files unless skillshare manages them
		if manifest.SkipsHidden(name) {
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

			_, inManifest := manifest.Managed[name]
			owned := utils.PathHasPrefix(absLink, absSource+string(filepath.Separator))
			if !owned && opts.Follow != nil && targetExists {
				// Relative links read from the canonical parent, a source that
				// is itself a link, and followed entries' targets.
				owned = scope.ownsLink(entryPath)
			}

			if owned {
				if !targetExists {
					shouldRemove = true
					reason = "broken symlink to source"
				} else if inManifest || force {
					shouldRemove = true
					reason = "orphan symlink to source"
				} else {
					// A live link into the source that skillshare never created is the
					// user's own wiring. Keep it, like the directory branch below.
					result.LocalDirs = append(result.LocalDirs, name)
					continue
				}
			} else if !targetExists && !opts.ManagedOnly {
				// External symlink whose target no longer exists (e.g. after data migration)
				shouldRemove = true
				reason = "broken external symlink"
			} else if force {
				// Valid external symlink, but force mode requested
				shouldRemove = true
				reason = "external symlink (force)"
			} else if inManifest && opts.Follow != nil {
				// Links created through a followed entry stay inside the source
				// after unfollow; a physical external path was made by hand.
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s: managed link resolves outside the source after unfollow; remove it or re-run with --force", name))
			} else {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s: symlink to external location (%s), kept", name, absLink))
			}
		} else if info.IsDir() {
			// Check manifest first (most reliable)
			if _, inManifest := manifest.Managed[name]; inManifest {
				shouldRemove = true
				reason = "orphan skillshare-managed directory (manifest)"
			} else if naming == "flat" && !opts.ManagedOnly && (utils.HasNestedSeparator(name) || utils.IsTrackedRepoDir(name)) {
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
		if sc.Mode == "symlink" || !sc.IsEnabled() {
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
	return CheckStatusMergeWithOptions(targetPath, sourcePath, StatusOptions{})
}

// StatusOptions carries one operation's policy into a status check.
type StatusOptions struct {
	// Follow is the operation's .skillfollow snapshot; nil keeps legacy rules.
	Follow *sourcewalk.FollowSet
}

// CheckStatusMergeWithOptions is CheckStatusMerge with an explicit follow
// policy. In-source links count exactly as before. A link that resolves only
// through a followed root counts as linked when the manifest records it and it
// is inside a currently followed entry's resolved target. This measures
// connectivity, not per-name wiring.
func CheckStatusMergeWithOptions(targetPath, sourcePath string, opts StatusOptions) (TargetStatus, int, int) {
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

	manifest, _ := ReadManifest(targetPath)
	entries, _ := os.ReadDir(targetPath)
	for _, entry := range entries {
		if manifest.SkipsHidden(entry.Name()) {
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
			} else if _, managed := manifest.Managed[entry.Name()]; managed && opts.Follow != nil && followedLink(skillPath, sourcePath, opts.Follow) {
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
