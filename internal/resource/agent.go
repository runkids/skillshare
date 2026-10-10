package resource

import (
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/skillignore"
	"skillshare/internal/utils"
)

// AgentKind handles single-file .md agent resources.
type AgentKind struct{}

var _ ResourceKind = AgentKind{}

func (AgentKind) Kind() string { return "agent" }

// Discover scans sourceDir for .md files, excluding conventional files
// (README.md, LICENSE.md, etc.) and hidden files.
//
// For tracked repos (directories starting with _): if the repo contains an
// agents/ subdirectory, only files inside agents/ are discovered. Otherwise
// the entire repo is walked with conventional excludes applied.
func (AgentKind) Discover(sourceDir string) ([]DiscoveredResource, error) {
	walkRoot := utils.ResolveSymlink(sourceDir)

	// Read .agentignore for filtering
	ignoreMatcher := skillignore.ReadAgentIgnoreMatcher(walkRoot)

	// Pre-scan: find tracked repos that have an agents/ subdirectory.
	// When agents/ exists, only its contents count as agent files.
	reposWithAgentsDir := map[string]bool{}
	if entries, readErr := os.ReadDir(walkRoot); readErr == nil {
		for _, e := range entries {
			if !e.IsDir() || !utils.IsTrackedRepoDir(e.Name()) {
				continue
			}
			agentsPath := filepath.Join(walkRoot, e.Name(), "agents")
			if info, statErr := os.Stat(agentsPath); statErr == nil && info.IsDir() {
				reposWithAgentsDir[e.Name()] = true
			}
		}
	}

	var resources []DiscoveredResource

	err := filepath.Walk(walkRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if info.Name() == ".git" || (path != walkRoot && utils.IsHidden(info.Name())) {
				return filepath.SkipDir
			}
			// Skip ignored directories early
			if ignoreMatcher.HasRules() && info.Name() != "." {
				relDir, relErr := filepath.Rel(walkRoot, path)
				if relErr == nil {
					relDir = strings.ReplaceAll(relDir, "\\", "/")
					if ignoreMatcher.CanSkipDir(relDir) {
						return filepath.SkipDir
					}
				}
			}
			// For tracked repos with agents/ subdir: skip non-agents children
			relDir, relErr := filepath.Rel(walkRoot, path)
			if relErr == nil {
				parts := strings.SplitN(filepath.ToSlash(relDir), "/", 3)
				if len(parts) >= 2 && reposWithAgentsDir[parts[0]] && parts[1] != "agents" {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Only .md files
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		// Skip conventional excludes
		if ConventionalExcludes[info.Name()] {
			return nil
		}

		// Skip hidden files
		if utils.IsHidden(info.Name()) {
			return nil
		}

		relPath, relErr := filepath.Rel(walkRoot, path)
		if relErr != nil {
			return nil
		}
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		// For tracked repos with agents/ subdir: skip files at repo root level
		// (e.g. _repo/CLAUDE.md) — only _repo/agents/** should be discovered.
		parts := strings.SplitN(relPath, "/", 3)
		if len(parts) == 2 && reposWithAgentsDir[parts[0]] {
			return nil
		}

		// Apply .agentignore matching — mark as disabled but still include
		disabled := ignoreMatcher.HasRules() && ignoreMatcher.Match(relPath, false)

		name := agentNameFromFile(path, info.Name())
		targets := utils.ParseFrontmatterList(path, "targets")

		isNested := strings.Contains(relPath, "/")
		repoRelPath := findTrackedRepoRelPath(walkRoot, relPath)

		resources = append(resources, DiscoveredResource{
			Name:        name,
			Kind:        "agent",
			RelPath:     relPath,
			AbsPath:     path,
			IsNested:    isNested,
			IsInRepo:    repoRelPath != "",
			RepoRelPath: repoRelPath,
			Disabled:    disabled,
			FlatName:    AgentFlatName(relPath),
			SourcePath:  filepath.Join(sourceDir, relPath),
			Targets:     targets,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return resources, nil
}

func findTrackedRepoRelPath(root, relPath string) string {
	dir := filepath.Dir(relPath)
	if dir == "." || dir == "" {
		return ""
	}

	parts := strings.Split(filepath.ToSlash(dir), "/")
	for i, part := range parts {
		if !utils.IsTrackedRepoDir(part) {
			continue
		}
		candidate := strings.Join(parts[:i+1], "/")
		gitDir := filepath.Join(root, filepath.FromSlash(candidate), ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return candidate
		}
	}

	return ""
}

// agentNameFromFile resolves an agent name. Checks frontmatter name field
// first, falls back to filename without .md extension.
func agentNameFromFile(filePath, fileName string) string {
	name := utils.ParseFrontmatterField(filePath, "name")
	if name != "" {
		return name
	}
	return strings.TrimSuffix(fileName, ".md")
}

// ResolveName extracts the agent name from an .md file.
// Checks frontmatter name field first, falls back to filename.
func (AgentKind) ResolveName(path string) string {
	return agentNameFromFile(path, filepath.Base(path))
}

// FlatName flattens nested agent paths using the shared __ separator.
// Example: "curriculum/math-tutor.md" → "curriculum__math-tutor.md"
func (AgentKind) FlatName(relPath string) string {
	return AgentFlatName(relPath)
}

// AgentFlatName is the standalone flat name computation for agents.
// Agents must sync into flat target directories, so nested segments are
// encoded using the same path flattening rule as skills.
func AgentFlatName(relPath string) string {
	return utils.PathToFlatName(relPath)
}

// ActiveAgents returns only non-disabled agents from the given slice.
func ActiveAgents(agents []DiscoveredResource) []DiscoveredResource {
	active := make([]DiscoveredResource, 0, len(agents))
	for _, a := range agents {
		if !a.Disabled {
			active = append(active, a)
		}
	}
	return active
}

// CreateLink creates a file symlink from dst pointing to src.
func (AgentKind) CreateLink(src, dst string) error {
	return os.Symlink(src, dst)
}

func (AgentKind) SupportsAudit() bool   { return true }
func (AgentKind) SupportsTrack() bool   { return true }
func (AgentKind) SupportsCollect() bool { return true }
