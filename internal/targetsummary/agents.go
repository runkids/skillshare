package targetsummary

import (
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	ssync "skillshare/internal/sync"
)

const defaultAgentMode = "merge"

// AgentSummary describes the effective agent configuration and sync counts for
// a single target. ManagedCount maps to linked agents in merge/symlink mode and
// managed copied agents in copy mode.
type AgentSummary struct {
	DisplayPath   string
	Path          string
	Mode          string
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
		}
	}
	if displayPath == "" {
		return nil, nil
	}

	return b.buildSummary(config.ExpandPath(displayPath), displayPath, ac.Mode, ac.Include, ac.Exclude)
}

// ProjectTarget returns the effective agents summary for a project target.
func (b *Builder) ProjectTarget(entry config.ProjectTargetEntry) (*AgentSummary, error) {
	ac := entry.AgentsConfig()
	displayPath := ac.Path
	if displayPath == "" {
		if builtin, ok := b.builtinAgents[entry.Name]; ok {
			displayPath = builtin.Path
		}
	}
	if displayPath == "" {
		return nil, nil
	}

	return b.buildSummary(resolveProjectPath(b.projectRoot, displayPath), displayPath, ac.Mode, ac.Include, ac.Exclude)
}

func (b *Builder) buildSummary(path, displayPath, mode string, include, exclude []string) (*AgentSummary, error) {
	if mode == "" {
		mode = defaultAgentMode
	}

	summary := &AgentSummary{
		Path:        path,
		DisplayPath: displayPath,
		Mode:        mode,
		Include:     append([]string(nil), include...),
		Exclude:     append([]string(nil), exclude...),
	}

	expectedAgents := b.activeAgents
	if b.sourceExists && mode != "symlink" {
		filtered, err := ssync.FilterAgents(expectedAgents, include, exclude)
		if err != nil {
			return nil, err
		}
		expectedAgents = filtered
	}

	if b.sourceExists {
		summary.ExpectedCount = len(expectedAgents)
	}
	summary.ManagedCount = countManagedAgents(path, mode, b.sourcePath, summary.ExpectedCount)
	summary.LocalCount = countLocalAgents(path, b.sourcePath)

	return summary, nil
}

func countManagedAgents(targetPath, mode, sourcePath string, expectedCount int) int {
	switch mode {
	case "copy":
		_, managed, _ := ssync.CheckStatusCopy(targetPath)
		return managed
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
		if entry.Type()&os.ModeSymlink == 0 {
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
