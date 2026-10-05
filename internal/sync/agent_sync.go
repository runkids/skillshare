package sync

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/resource"
	"skillshare/internal/utils"
)

// AgentSyncResult holds the result of syncing agents to a target.
type AgentSyncResult struct {
	Linked  []string // Agents that were symlinked (merge) or copied (copy)
	Skipped []string // Agents that already exist in target (kept local)
	Updated []string // Agents that had broken symlinks fixed or content updated
}

// AgentCollision represents two agents that flatten to the same filename.
type AgentCollision struct {
	FlatName string // The colliding flat name (e.g. "helper.md")
	PathA    string // First agent relative path
	PathB    string // Second agent relative path
}

// LocalAgentInfo describes a local agent file in a target directory.
type LocalAgentInfo struct {
	Name       string
	Path       string
	TargetName string
}

// CheckAgentCollisions detects agents that flatten to the same filename.
func CheckAgentCollisions(agents []resource.DiscoveredResource) []AgentCollision {
	seen := make(map[string]string) // flatName → first relPath
	var collisions []AgentCollision

	for _, a := range agents {
		if prev, ok := seen[a.FlatName]; ok {
			collisions = append(collisions, AgentCollision{
				FlatName: a.FlatName,
				PathA:    prev,
				PathB:    a.RelPath,
			})
		} else {
			seen[a.FlatName] = a.RelPath
		}
	}

	return collisions
}

// SyncAgents dispatches to the appropriate sync mode for agents.
// mode: "merge" (per-file symlinks), "symlink" (whole dir), "copy" (file copy).
// projectRoot enables relative symlinks when non-empty.
func SyncAgents(agents []resource.DiscoveredResource, sourceDir, targetDir, mode string, dryRun, force bool, projectRoot ...string) (*AgentSyncResult, error) {
	root := ""
	if len(projectRoot) > 0 {
		root = projectRoot[0]
	}

	switch mode {
	case "symlink":
		return syncAgentsSymlink(sourceDir, targetDir, dryRun, force, root)
	case "copy":
		return syncAgentsCopy(agents, targetDir, dryRun, force)
	default: // "merge" or ""
		if !canCreateFileLink() {
			return syncAgentsMergeCopy(agents, targetDir, dryRun, force)
		}
		return syncAgentsMerge(agents, sourceDir, targetDir, dryRun, force, root)
	}
}

// linkResolvesToSource reports whether a resolved symlink target (absLink) and a
// source path (absSource) reference the same file. The fast path is a direct
// string compare; the slow path canonicalizes both sides through EvalSymlinks so
// symlinked ancestors (e.g. macOS /var → /private/var) do not make a stable
// relative link look like it points elsewhere. Mirrors isSymlinkToSource in sync.go.
func linkResolvesToSource(absLink, absSource string) bool {
	if utils.PathsEqual(absLink, absSource) {
		return true
	}
	return utils.PathsEqual(utils.ResolveSymlink(absLink), utils.ResolveSymlink(absSource))
}

// prunableLink reports whether an orphan link at path may be removed: it is
// broken, or it resolves inside sourceDir. Live links to other locations, which
// skillshare never creates, survive.
func prunableLink(path, sourceDir string) bool {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return true
	} else if err != nil {
		return false // e.g. permission denied: not proven broken
	}
	absDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return false
	}
	sep := string(filepath.Separator)
	under := func(p string) bool {
		return utils.PathHasPrefix(p, absDir+sep) || utils.PathHasPrefix(p, utils.ResolveSymlink(absDir)+sep)
	}
	if absLink, err := utils.ResolveLinkTarget(path); err == nil && (under(absLink) || under(utils.ResolveSymlink(absLink))) {
		return true
	}
	// createLink writes relative links from the real parent directory, which
	// differs from the lexical one when a parent is itself a symlink.
	if dest, err := os.Readlink(path); err == nil && !filepath.IsAbs(dest) {
		return under(filepath.Join(utils.ResolveSymlink(filepath.Dir(path)), dest))
	}
	return false
}

// syncAgentsMerge creates per-file symlinks in targetDir for each discovered agent.
// Existing non-symlink files are preserved (skipped) unless force is true.
func syncAgentsMerge(agents []resource.DiscoveredResource, sourceDir, targetDir string, dryRun, force bool, projectRoot string) (*AgentSyncResult, error) {
	result := &AgentSyncResult{}
	relative := shouldUseRelative(projectRoot, sourceDir, targetDir)

	if !dryRun {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create agent target directory: %w", err)
		}
	}

	// Copies made while file links were unavailable are relinked.
	copies := loadCopyTracker(targetDir)

	for _, agent := range agents {
		targetPath := filepath.Join(targetDir, agent.FlatName)

		_, err := os.Lstat(targetPath)
		if err == nil {
			if utils.IsSymlinkOrJunction(targetPath) {
				absLink, linkErr := utils.ResolveLinkTarget(targetPath)
				if linkErr != nil {
					return nil, fmt.Errorf("failed to resolve link for %s: %w", agent.FlatName, linkErr)
				}
				absSource, _ := filepath.Abs(agent.AbsPath)

				if linkResolvesToSource(absLink, absSource) && fileLinkUsable(targetPath) {
					dest, _ := os.Readlink(targetPath)
					if !linkNeedsReformat(dest, relative) {
						result.Linked = append(result.Linked, agent.FlatName)
						continue
					}
					if !dryRun {
						if err := reformatLink(targetPath, agent.AbsPath, relative); err != nil {
							return nil, fmt.Errorf("failed to reformat symlink for %s: %w", agent.FlatName, err)
						}
					}
					result.Updated = append(result.Updated, agent.FlatName)
					continue
				}

				if !dryRun {
					os.Remove(targetPath)
					if err := createLink(targetPath, agent.AbsPath, relative); err != nil {
						return nil, fmt.Errorf("failed to create symlink for %s: %w", agent.FlatName, err)
					}
				}
				result.Updated = append(result.Updated, agent.FlatName)
			} else {
				if force || copies.owns(agent.FlatName) {
					if !dryRun {
						os.Remove(targetPath)
						if err := createLink(targetPath, agent.AbsPath, relative); err != nil {
							return nil, fmt.Errorf("failed to create symlink for %s: %w", agent.FlatName, err)
						}
						copies.forget(agent.FlatName)
					}
					result.Updated = append(result.Updated, agent.FlatName)
				} else {
					result.Skipped = append(result.Skipped, agent.FlatName)
				}
			}
		} else if os.IsNotExist(err) {
			if !dryRun {
				if err := createLink(targetPath, agent.AbsPath, relative); err != nil {
					return nil, fmt.Errorf("failed to create symlink for %s: %w", agent.FlatName, err)
				}
			}
			result.Linked = append(result.Linked, agent.FlatName)
		} else {
			return nil, fmt.Errorf("failed to check target path for %s: %w", agent.FlatName, err)
		}
	}

	if !dryRun {
		if err := copies.save(); err != nil {
			return nil, fmt.Errorf("failed to update agent manifest: %w", err)
		}
	}
	return result, nil
}

// syncAgentsMergeCopy is merge mode where file links are unavailable: each
// agent is copied and tracked, so later syncs keep the copies updated and
// pruned while local agent files stay preserved as in merge mode.
func syncAgentsMergeCopy(agents []resource.DiscoveredResource, targetDir string, dryRun, force bool) (*AgentSyncResult, error) {
	result := &AgentSyncResult{}

	if !dryRun {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create agent target directory: %w", err)
		}
	}
	copies := loadCopyTracker(targetDir)

	for _, agent := range agents {
		name := agent.FlatName
		targetPath := filepath.Join(targetDir, name)

		info, err := os.Lstat(targetPath)
		exists := err == nil
		switch {
		case os.IsNotExist(err):
		case err != nil:
			return nil, fmt.Errorf("failed to check target path for %s: %w", name, err)
		case utils.IsSymlinkOrJunction(targetPath):
			// A merge-mode link, or a junction to a file that cannot be read.
		case info.IsDir():
			result.Skipped = append(result.Skipped, name)
			continue
		case contentEqual(agent.AbsPath, targetPath):
			if !copies.owns(name) {
				result.Skipped = append(result.Skipped, name)
				continue
			}
			result.Linked = append(result.Linked, name)
			continue
		case !force && !copies.owns(name):
			result.Skipped = append(result.Skipped, name)
			continue
		}

		if !dryRun {
			if exists {
				if err := os.Remove(targetPath); err != nil {
					return nil, fmt.Errorf("failed to replace %s: %w", name, err)
				}
			}
			if err := copyFile(agent.AbsPath, targetPath); err != nil {
				return nil, fmt.Errorf("failed to copy %s: %w", name, err)
			}
			copies.record(name)
		}
		if exists {
			result.Updated = append(result.Updated, name)
		} else {
			result.Linked = append(result.Linked, name)
		}
	}

	if !dryRun {
		if err := copies.save(); err != nil {
			return nil, fmt.Errorf("failed to update agent manifest: %w", err)
		}
	}
	return result, nil
}

// SyncedAgentCopies counts regular files matching their source, independently
// of ownership. Status equality does not authorize overwriting or pruning.
func SyncedAgentCopies(targetDir string, agents []resource.DiscoveredResource, preserved ...*int) int {
	copies := loadCopyTracker(targetDir)
	n := 0
	for _, a := range agents {
		if canCreateFileLink() {
			if _, tracked := copies.m.Managed[a.FlatName]; !tracked {
				continue
			}
		}
		target := filepath.Join(targetDir, a.FlatName)
		if info, err := os.Lstat(target); err == nil && info.Mode().IsRegular() && contentEqual(a.AbsPath, target) {
			if len(preserved) > 0 && !copies.owns(a.FlatName) {
				*preserved[0]++
			}
			n++
		}
	}
	return n
}

// SyncedExtensionOutputs counts agents whose extension output is tracked in
// the target's manifest, unchanged since sync, and converted from the
// source's current content. See ExtensionOutputStatus. Refs #391.
func SyncedExtensionOutputs(targetDir string, agents []resource.DiscoveredResource) int {
	synced, _ := ExtensionOutputStatus(targetDir, agents)
	n := 0
	for _, ok := range synced {
		if ok {
			n++
		}
	}
	return n
}

// ExtensionOutputStatus compares agents with an extension target's manifest.
// synced[i] reports whether agents[i] has a tracked output that is unchanged
// since sync and was converted from the source's current content. orphans are
// unchanged tracked outputs that no agent converts to; sync prunes them.
//
// An extension renames the output per output_ext and transforms its content,
// so neither the source name nor the source content can be compared with the
// output; the manifest's output hash and source fingerprint are the record of
// what sync wrote and from what. Any output_ext is matched by stem, so callers
// need not load the extension spec.
func ExtensionOutputStatus(targetDir string, agents []resource.DiscoveredResource) (synced []bool, orphans []string) {
	copies := loadCopyTracker(targetDir)
	synced = make([]bool, len(agents))
	stems := make(map[string]bool, len(agents))
	used := make(map[string]bool) // one output satisfies one agent, even when flat names collide
	for i, a := range agents {
		stem := strings.TrimSuffix(a.FlatName, filepath.Ext(a.FlatName))
		stems[stem] = true
		for key := range copies.m.Managed {
			if used[key] || strings.TrimSuffix(key, filepath.Ext(key)) != stem {
				continue
			}
			rel := filepath.FromSlash(key)
			if copies.owns(rel) && copies.sourceMatches(rel, a.AbsPath) {
				used[key] = true
				synced[i] = true
				break
			}
		}
	}
	for key := range copies.m.Managed {
		if !stems[strings.TrimSuffix(key, filepath.Ext(key))] && copies.owns(filepath.FromSlash(key)) {
			orphans = append(orphans, key)
		}
	}
	slices.Sort(orphans)
	return synced, orphans
}

// syncAgentsSymlink creates a single directory symlink from targetDir to sourceDir.
// If targetDir already exists as a real directory, it's replaced only with force.
func syncAgentsSymlink(sourceDir, targetDir string, dryRun, force bool, projectRoot string) (*AgentSyncResult, error) {
	result := &AgentSyncResult{}
	relative := shouldUseRelative(projectRoot, sourceDir, targetDir)

	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return nil, fmt.Errorf("failed to create target parent: %w", err)
	}

	info, err := os.Lstat(targetDir)
	if err == nil {
		if utils.IsLinkMode(targetDir, info.Mode()) {
			// Already a symlink — check if correct
			absLink, linkErr := utils.ResolveLinkTarget(targetDir)
			if linkErr != nil {
				return nil, fmt.Errorf("failed to resolve link: %w", linkErr)
			}
			absSource, _ := filepath.Abs(sourceDir)

			if linkResolvesToSource(absLink, absSource) {
				dest, _ := os.Readlink(targetDir)
				if !linkNeedsReformat(dest, relative) {
					result.Linked = append(result.Linked, "(directory)")
					return result, nil
				}
				if !dryRun {
					if err := reformatLink(targetDir, sourceDir, relative); err != nil {
						return nil, fmt.Errorf("failed to reformat directory symlink: %w", err)
					}
				}
				result.Updated = append(result.Updated, "(directory)")
				return result, nil
			}

			// Wrong target
			if !dryRun {
				os.Remove(targetDir)
				if err := createLink(targetDir, sourceDir, relative); err != nil {
					return nil, fmt.Errorf("failed to create directory symlink: %w", err)
				}
			}
			result.Updated = append(result.Updated, "(directory)")
		} else {
			// Real directory
			if force {
				if !dryRun {
					os.RemoveAll(targetDir)
					if err := createLink(targetDir, sourceDir, relative); err != nil {
						return nil, fmt.Errorf("failed to create directory symlink: %w", err)
					}
				}
				result.Updated = append(result.Updated, "(directory)")
			} else {
				result.Skipped = append(result.Skipped, "(directory)")
			}
		}
	} else if os.IsNotExist(err) {
		if !dryRun {
			if err := createLink(targetDir, sourceDir, relative); err != nil {
				return nil, fmt.Errorf("failed to create directory symlink: %w", err)
			}
		}
		result.Linked = append(result.Linked, "(directory)")
	} else {
		return nil, fmt.Errorf("failed to check target path: %w", err)
	}

	return result, nil
}

// syncAgentsCopy copies agent .md files to targetDir.
// Existing files are overwritten if content differs; force replaces all.
func syncAgentsCopy(agents []resource.DiscoveredResource, targetDir string, dryRun, force bool) (*AgentSyncResult, error) {
	result := &AgentSyncResult{}

	if !dryRun {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create agent target directory: %w", err)
		}
	}

	// Copies are tracked so prune removes only what skillshare wrote.
	copies := loadCopyTracker(targetDir)

	for _, agent := range agents {
		targetPath := filepath.Join(targetDir, agent.FlatName)

		srcData, err := os.ReadFile(agent.AbsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read source %s: %w", agent.FlatName, err)
		}

		// A link left by merge mode (or an unreadable junction) is replaced;
		// writing through it would edit the source.
		if utils.IsSymlinkOrJunction(targetPath) {
			if !dryRun {
				if err := os.Remove(targetPath); err != nil {
					return nil, fmt.Errorf("failed to replace %s: %w", agent.FlatName, err)
				}
				if err := os.WriteFile(targetPath, srcData, 0644); err != nil {
					return nil, fmt.Errorf("failed to write %s: %w", agent.FlatName, err)
				}
				copies.record(agent.FlatName)
			}
			result.Updated = append(result.Updated, agent.FlatName)
			continue
		}

		if _, statErr := os.Stat(targetPath); statErr == nil {
			// File exists — check if content matches
			tgtData, readErr := os.ReadFile(targetPath)
			if readErr == nil && string(tgtData) == string(srcData) && !force {
				if !dryRun {
					copies.record(agent.FlatName) // adopts copies made before tracking
				}
				result.Linked = append(result.Linked, agent.FlatName)
				continue
			}
			// Content differs or force — overwrite
			if !dryRun {
				if err := os.WriteFile(targetPath, srcData, 0644); err != nil {
					return nil, fmt.Errorf("failed to write %s: %w", agent.FlatName, err)
				}
				copies.record(agent.FlatName)
			}
			result.Updated = append(result.Updated, agent.FlatName)
		} else {
			// New file
			if !dryRun {
				if err := os.WriteFile(targetPath, srcData, 0644); err != nil {
					return nil, fmt.Errorf("failed to write %s: %w", agent.FlatName, err)
				}
				copies.record(agent.FlatName)
			}
			result.Linked = append(result.Linked, agent.FlatName)
		}
	}

	if !dryRun {
		if err := copies.save(); err != nil {
			return nil, fmt.Errorf("failed to update agent manifest: %w", err)
		}
	}
	return result, nil
}

// SyncAgentsTransform pipes each agent through spec and writes the output into
// targetDir, renamed per spec.OutputExt. Like copy mode, the target is owned by
// skillshare, so differing files are overwritten. A failing agent is reported in
// the returned error without stopping the others.
func SyncAgentsTransform(agents []resource.DiscoveredResource, sourceDir, targetDir, mode string, spec *ExtensionSpec, dryRun, force bool) (*AgentSyncResult, error) {
	if _, err := ResolveExtensionMode(mode); err != nil {
		return nil, err
	}
	result := &AgentSyncResult{}
	var errs []error

	// A symlink-mode leftover points the whole target at the source; writing
	// through it would overwrite source agents.
	if !dryRun && utils.IsSymlinkOrJunction(targetDir) {
		if absLink, err := utils.ResolveLinkTarget(targetDir); err == nil && linkResolvesToSource(absLink, sourceDir) {
			if err := os.Remove(targetDir); err != nil {
				return nil, fmt.Errorf("failed to remove directory symlink: %w", err)
			}
		}
	}
	// Outputs are tracked so prune removes only what skillshare wrote.
	copies := loadCopyTracker(targetDir)

	for _, agent := range agents {
		name := ApplyOutputExt(agent.FlatName, spec.OutputExt)
		// A preview must never execute extension code, so dry-run can't know
		// whether the output would change.
		if dryRun {
			result.Linked = append(result.Linked, name)
			continue
		}

		tgtFile := filepath.Join(targetDir, name)
		// Merge-mode leftovers link to the source; drop the link, never write through it.
		if info, err := os.Lstat(tgtFile); err == nil && utils.IsLinkMode(tgtFile, info.Mode()) {
			if err := os.Remove(tgtFile); err != nil {
				errs = append(errs, fmt.Errorf("%s: remove symlink: %w", agent.FlatName, err))
				continue
			}
		}
		existing, readErr := os.ReadFile(tgtFile)
		out, err := runExtension(spec, agent.AbsPath, map[string]string{
			"SS_SRC_PATH":   agent.AbsPath,
			"SS_REL_PATH":   agent.RelPath,
			"SS_TARGET_DIR": targetDir,
			"SS_MODE":       "sync",
		})
		if err != nil {
			// Keeping the old output would leave an unconverted agent active.
			if readErr == nil {
				os.Remove(tgtFile)
				copies.forget(name)
			}
			errs = append(errs, fmt.Errorf("%s: %w", agent.FlatName, err))
			continue
		}
		if readErr == nil && bytes.Equal(existing, out) && !force {
			copies.record(name) // adopts outputs made before tracking
			copies.recordSource(name, agent.AbsPath)
			result.Linked = append(result.Linked, name)
			continue
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create agent target directory: %w", err)
		}
		if err := os.WriteFile(tgtFile, out, 0644); err != nil {
			errs = append(errs, fmt.Errorf("%s: write target: %w", agent.FlatName, err))
			continue
		}
		copies.record(name)
		copies.recordSource(name, agent.AbsPath)
		if readErr == nil {
			result.Updated = append(result.Updated, name)
		} else {
			result.Linked = append(result.Linked, name)
		}
	}

	if !dryRun {
		if err := copies.save(); err != nil {
			errs = append(errs, fmt.Errorf("failed to update agent manifest: %w", err))
		}
	}
	return result, errors.Join(errs...)
}

// SyncAgentsToTarget creates file symlinks in targetDir for each discovered agent.
// Uses merge semantics. Kept for backward compatibility; prefer SyncAgents().
func SyncAgentsToTarget(agents []resource.DiscoveredResource, targetDir string, dryRun, force bool) (*AgentSyncResult, error) {
	return syncAgentsMerge(agents, "", targetDir, dryRun, force, "")
}

// PruneOrphanAgentLinks removes file symlinks in targetDir that point into
// sourceDir but don't correspond to any discovered agent. Links to other
// locations are kept. For merge mode only.
func PruneOrphanAgentLinks(targetDir, sourceDir string, agents []resource.DiscoveredResource, dryRun bool) (removed []string, _ error) {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read agent target directory: %w", err)
	}

	expected := make(map[string]bool, len(agents))
	for _, a := range agents {
		expected[a.FlatName] = true
	}

	// Copies stand in for links when file links are unavailable.
	copies := loadCopyTracker(targetDir)
	var errs []error
	removed, err = copies.pruneOrphans(expected, dryRun)
	if err != nil {
		errs = append(errs, err)
	}
	if !dryRun {
		if err := copies.save(); err != nil {
			return removed, errors.Join(append(errs, fmt.Errorf("failed to update agent manifest: %w", err))...)
		}
	}

	for _, entry := range entries {
		name := entry.Name()

		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if !utils.IsLinkMode(filepath.Join(targetDir, name), info.Mode()) {
			continue
		}

		if expected[name] || !prunableLink(filepath.Join(targetDir, name), sourceDir) {
			continue
		}

		if !dryRun {
			if err := os.Remove(filepath.Join(targetDir, name)); err != nil {
				errs = append(errs, fmt.Errorf("failed to remove orphaned link %s: %w", name, err))
				continue
			}
		}
		removed = append(removed, name)
	}

	return removed, errors.Join(errs...)
}

// PruneOrphanAgentCopies removes tracked copies in targetDir that don't
// correspond to any discovered agent. For copy mode only. outputExt is the
// extension a transform renames outputs to ("" keeps .md); a stale tracked .md
// copy of a transformed agent counts as an orphan. Files skillshare never wrote,
// and copies edited since, are preserved.
func PruneOrphanAgentCopies(targetDir string, agents []resource.DiscoveredResource, outputExt string, dryRun bool) (removed []string, _ error) {
	expected := make(map[string]bool, len(agents))
	for _, a := range agents {
		expected[ApplyOutputExt(a.FlatName, outputExt)] = true
	}

	copies := loadCopyTracker(targetDir)
	// A copy made before tracking under the pre-transform name is still
	// identical to its agent; adopt it so the renamed output replaces it.
	for _, a := range agents {
		if ApplyOutputExt(a.FlatName, outputExt) == a.FlatName {
			continue
		}
		stale := filepath.Join(targetDir, a.FlatName)
		if info, err := os.Lstat(stale); err == nil && info.Mode().IsRegular() && contentEqual(a.AbsPath, stale) {
			copies.record(a.FlatName)
		}
	}
	removed, err := copies.pruneOrphans(expected, dryRun)
	if !dryRun {
		if saveErr := copies.save(); saveErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to update agent manifest: %w", saveErr))
		}
	}
	return removed, err
}

// FindLocalAgents finds local (non-symlinked) agent files in a target directory.
// If the target directory itself is a symlink to sourcePath, it returns no local agents.
func FindLocalAgents(targetDir, sourcePath string) ([]LocalAgentInfo, error) {
	var agents []LocalAgentInfo

	info, err := os.Lstat(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return agents, nil
		}
		return nil, fmt.Errorf("failed to read agent target directory: %w", err)
	}

	if utils.IsLinkMode(targetDir, info.Mode()) {
		absLink, err := utils.ResolveLinkTarget(targetDir)
		if err != nil {
			return nil, err
		}
		absSource, _ := filepath.Abs(sourcePath)
		if linkResolvesToSource(absLink, absSource) {
			return agents, nil
		}
		resolved, statErr := os.Stat(targetDir)
		if statErr != nil || !resolved.IsDir() {
			return agents, nil
		}
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return agents, nil
		}
		return nil, fmt.Errorf("failed to read agent target directory: %w", err)
	}

	copies := loadCopyTracker(targetDir)
	for _, entry := range entries {
		name := entry.Name()

		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		if utils.IsHidden(name) || resource.ConventionalExcludes[name] {
			continue
		}
		if copies.owns(name) {
			continue // a copy standing in for a link, not a local agent
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.IsDir() || utils.IsLinkMode(filepath.Join(targetDir, name), info.Mode()) {
			continue
		}

		agents = append(agents, LocalAgentInfo{
			Name: name,
			Path: filepath.Join(targetDir, name),
		})
	}

	return agents, nil
}

// PullAgent copies a single local agent file from target to source.
func PullAgent(agent LocalAgentInfo, sourcePath string, force bool) error {
	destPath := filepath.Join(sourcePath, agent.Name)

	if _, err := os.Stat(destPath); err == nil {
		if !force {
			return ErrAlreadyExists
		}
		if err := os.RemoveAll(destPath); err != nil {
			return fmt.Errorf("failed to remove existing: %w", err)
		}
	}

	data, err := os.ReadFile(agent.Path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", agent.Name, err)
	}
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", agent.Name, err)
	}

	return nil
}

// PullAgents copies multiple local agent files from targets to source.
func PullAgents(agents []LocalAgentInfo, sourcePath string, opts PullOptions) (*PullResult, error) {
	result := &PullResult{
		Failed: make(map[string]error),
	}

	if !opts.DryRun {
		if err := os.MkdirAll(sourcePath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create agent source dir: %w", err)
		}
	}

	for _, agent := range agents {
		if opts.DryRun {
			result.Pulled = append(result.Pulled, agent.Name)
			continue
		}

		err := PullAgent(agent, sourcePath, opts.Force)
		if err != nil {
			if errors.Is(err, ErrAlreadyExists) {
				result.Skipped = append(result.Skipped, agent.Name)
			} else {
				result.Failed[agent.Name] = err
			}
			continue
		}

		result.Pulled = append(result.Pulled, agent.Name)
	}

	return result, nil
}
