package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

	for _, agent := range agents {
		targetPath := filepath.Join(targetDir, agent.FlatName)

		info, err := os.Lstat(targetPath)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				absLink, linkErr := utils.ResolveLinkTarget(targetPath)
				if linkErr != nil {
					return nil, fmt.Errorf("failed to resolve link for %s: %w", agent.FlatName, linkErr)
				}
				absSource, _ := filepath.Abs(agent.AbsPath)

				if linkResolvesToSource(absLink, absSource) {
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
				if force {
					if !dryRun {
						os.Remove(targetPath)
						if err := createLink(targetPath, agent.AbsPath, relative); err != nil {
							return nil, fmt.Errorf("failed to create symlink for %s: %w", agent.FlatName, err)
						}
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

	return result, nil
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
		if info.Mode()&os.ModeSymlink != 0 {
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

	for _, agent := range agents {
		targetPath := filepath.Join(targetDir, agent.FlatName)

		srcData, err := os.ReadFile(agent.AbsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read source %s: %w", agent.FlatName, err)
		}

		if _, statErr := os.Stat(targetPath); statErr == nil {
			// File exists — check if content matches
			tgtData, readErr := os.ReadFile(targetPath)
			if readErr == nil && string(tgtData) == string(srcData) && !force {
				result.Linked = append(result.Linked, agent.FlatName)
				continue
			}
			// Content differs or force — overwrite
			if !dryRun {
				if err := os.WriteFile(targetPath, srcData, 0644); err != nil {
					return nil, fmt.Errorf("failed to write %s: %w", agent.FlatName, err)
				}
			}
			result.Updated = append(result.Updated, agent.FlatName)
		} else {
			// New file
			if !dryRun {
				if err := os.WriteFile(targetPath, srcData, 0644); err != nil {
					return nil, fmt.Errorf("failed to write %s: %w", agent.FlatName, err)
				}
			}
			result.Linked = append(result.Linked, agent.FlatName)
		}
	}

	return result, nil
}

// SyncAgentsToTarget creates file symlinks in targetDir for each discovered agent.
// Uses merge semantics. Kept for backward compatibility; prefer SyncAgents().
func SyncAgentsToTarget(agents []resource.DiscoveredResource, targetDir string, dryRun, force bool) (*AgentSyncResult, error) {
	return syncAgentsMerge(agents, "", targetDir, dryRun, force, "")
}

// PruneOrphanAgentLinks removes file symlinks in targetDir that don't
// correspond to any discovered agent. For merge mode only.
func PruneOrphanAgentLinks(targetDir string, agents []resource.DiscoveredResource, dryRun bool) (removed []string, _ error) {
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

	for _, entry := range entries {
		name := entry.Name()

		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		if expected[name] {
			continue
		}

		if !dryRun {
			os.Remove(filepath.Join(targetDir, name))
		}
		removed = append(removed, name)
	}

	return removed, nil
}

// PruneOrphanAgentCopies removes copied .md files in targetDir that don't
// correspond to any discovered agent. For copy mode only.
func PruneOrphanAgentCopies(targetDir string, agents []resource.DiscoveredResource, dryRun bool) (removed []string, _ error) {
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

	for _, entry := range entries {
		name := entry.Name()

		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}

		// Skip conventional excludes (user might have README.md etc.)
		if resource.ConventionalExcludes[name] {
			continue
		}

		if expected[name] {
			continue
		}

		if !dryRun {
			os.Remove(filepath.Join(targetDir, name))
		}
		removed = append(removed, name)
	}

	return removed, nil
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

	if info.Mode()&os.ModeSymlink != 0 {
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

	for _, entry := range entries {
		name := entry.Name()

		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		if utils.IsHidden(name) || resource.ConventionalExcludes[name] {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
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
