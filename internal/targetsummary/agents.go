package targetsummary

import (
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

const defaultAgentMode = "merge"

// AgentSummary describes the effective agent configuration and sync counts for
// a single target. ManagedCount maps to linked agents in merge/symlink mode and
// managed copied agents in copy mode.
type AgentSummary struct {
	DisplayPath   string
	Path          string
	Mode          string
	Extension     string
	Include       []string
	Exclude       []string
	ManagedCount  int
	LocalCount    int
	ExpectedCount int
}

// Builder caches the discovered agent source so multiple target summaries can
// share the same resolution and counting logic.
type Builder struct {
	sourcePath    string
	projectRoot   string
	builtinAgents map[string]config.TargetConfig
	activeAgents  []resource.DiscoveredResource
	sourceExists  bool
}

// NewGlobalBuilder returns a summary builder for global-mode targets.
func NewGlobalBuilder(cfg *config.Config) (*Builder, error) {
	return newBuilder(cfg.EffectiveAgentsSource(), "", config.DefaultAgentTargets())
}

// NewProjectBuilder returns a summary builder for project-mode targets.
func NewProjectBuilder(agentsSourcePath, projectRoot string) (*Builder, error) {
	return newBuilder(agentsSourcePath, projectRoot, config.ProjectAgentTargets())
}

func newBuilder(sourcePath, projectRoot string, builtinAgents map[string]config.TargetConfig) (*Builder, error) {
	builder := &Builder{
		sourcePath:    sourcePath,
		projectRoot:   projectRoot,
		builtinAgents: builtinAgents,
	}

	if !dirExists(sourcePath) {
		return builder, nil
	}

	discovered, err := resource.AgentKind{}.Discover(sourcePath)
	if err != nil {
		return nil, err
	}
	builder.activeAgents = resource.ActiveAgents(discovered)
	builder.sourceExists = true
	return builder, nil
}

// GlobalTarget returns the effective agents summary for a global target.
func (b *Builder) GlobalTarget(name string, tc config.TargetConfig) (*AgentSummary, error) {
	ac := tc.AgentsConfig()
	displayPath := ac.Path
	if displayPath == "" {
		if builtin, ok := b.builtinAgents[name]; ok {
			displayPath = config.ExpandPath(builtin.Path)
		} else if builtin, ok := config.LookupGlobalAgentTarget(name); ok {
			displayPath = config.ExpandPath(builtin.Path)
		}
	}
	if displayPath == "" {
		return nil, nil
	}

	return b.buildSummary(name, config.ExpandPath(displayPath), displayPath, ac)
}

// ProjectTarget returns the effective agents summary for a project target.
func (b *Builder) ProjectTarget(entry config.ProjectTargetEntry) (*AgentSummary, error) {
	ac := entry.AgentsConfig()
	displayPath := ac.Path
	if displayPath == "" {
		if builtin, ok := b.builtinAgents[entry.Name]; ok {
			displayPath = builtin.Path
		} else if builtin, ok := config.LookupProjectAgentTarget(entry.Name); ok {
			displayPath = builtin.Path
		}
	}
	if displayPath == "" {
		return nil, nil
	}

	return b.buildSummary(entry.Name, resolveProjectPath(b.projectRoot, displayPath), displayPath, ac)
}

func (b *Builder) buildSummary(targetName, path, displayPath string, ac config.ResourceTargetConfig) (*AgentSummary, error) {
	mode, include, exclude := ac.Mode, ac.Include, ac.Exclude
	if mode == "" {
		mode = defaultAgentMode
	}
	if ac.Extension == "" {
		mode = ssync.EffectiveAgentMode(mode)
	}

	summary := &AgentSummary{
		Path:        path,
		DisplayPath: displayPath,
		Mode:        mode,
		Extension:   ac.Extension,
		Include:     append([]string(nil), include...),
		Exclude:     append([]string(nil), exclude...),
	}

	expectedAgents := b.activeAgents
	if b.sourceExists && mode != "symlink" {
		filtered, err := ssync.FilterAgents(expectedAgents, include, exclude)
		if err != nil {
			return nil, err
		}
		expectedAgents = ssync.FilterAgentsByTarget(filtered, targetName)
	}

	if b.sourceExists {
		summary.ExpectedCount = len(expectedAgents)
	}
	summary.ManagedCount = countManagedAgents(path, mode, ac.Extension, b.sourcePath, summary.ExpectedCount, expectedAgents)
	// Extension output is managed, not local, even when it keeps the .md name.
	if ac.Extension == "" {
		summary.LocalCount = countLocalAgents(path, b.sourcePath)
	}

	return summary, nil
}

func countManagedAgents(targetPath, mode, extension, sourcePath string, expectedCount int, agents []resource.DiscoveredResource) int {
	if extension != "" {
		return ssync.SyncedExtensionOutputs(targetPath, agents)
	}
	switch mode {
	case "copy":
		_, managed, _ := ssync.CheckStatusCopy(targetPath)
		return managed + ssync.SyncedAgentCopies(targetPath, agents)
	case "symlink":
		if ssync.CheckStatus(targetPath, sourcePath) == ssync.StatusLinked {
			return expectedCount
		}
		return 0
	default:
		return countHealthyAgentLinks(targetPath)
	}
}

func countHealthyAgentLinks(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	linked := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		if !utils.IsLinkMode(filepath.Join(dir, entry.Name()), entry.Type()) {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, entry.Name())); err == nil {
			linked++
		}
	}

	return linked
}

func countLocalAgents(targetPath, sourcePath string) int {
	if targetPath == "" {
		return 0
	}

	localAgents, err := ssync.FindLocalAgents(targetPath, sourcePath)
	if err != nil {
		return 0
	}
	return len(localAgents)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func resolveProjectPath(projectRoot, path string) string {
	if path == "" {
		return ""
	}

	resolved := config.ExpandPath(path)
	if !filepath.IsAbs(resolved) {
		return filepath.Join(projectRoot, filepath.FromSlash(resolved))
	}
	return resolved
}
