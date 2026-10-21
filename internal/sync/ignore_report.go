package sync

import (
	"os"
	"path/filepath"

	"skillshare/internal/resource"
	"skillshare/internal/skillignore"
	"skillshare/internal/utils"
)

// IgnoreReport is the .skillignore/.agentignore envelope that the dashboard
// and CLI sync responses share. Lists are never nil so JSON emits [].
type IgnoreReport struct {
	IgnoredCount       int      `json:"ignored_count"`
	IgnoredSkills      []string `json:"ignored_skills"`
	IgnoreRoot         string   `json:"ignore_root"`
	IgnoreRepos        []string `json:"ignore_repos"`
	AgentIgnoreRoot    string   `json:"agent_ignore_root"`
	AgentIgnoredCount  int      `json:"agent_ignored_count"`
	AgentIgnoredSkills []string `json:"agent_ignored_skills"`
}

// NewIgnoreReport summarizes skill ignore stats and the agent source's
// .agentignore state. Pass pre-discovered agents (including disabled ones) to
// avoid a second traversal; nil triggers discovery.
func NewIgnoreReport(stats *skillignore.IgnoreStats, agentsSource string, agents []resource.DiscoveredResource) IgnoreReport {
	r := IgnoreReport{IgnoredSkills: []string{}, IgnoreRepos: []string{}, AgentIgnoredSkills: []string{}}
	if stats != nil {
		if len(stats.IgnoredSkills) > 0 {
			r.IgnoredSkills = stats.IgnoredSkills
		}
		r.IgnoreRoot = stats.RootFile
		if stats.RepoFiles != nil {
			r.IgnoreRepos = stats.RepoFiles
		}
	}
	r.IgnoredCount = len(r.IgnoredSkills)
	if agentsSource != "" {
		ignoreFile := filepath.Join(utils.ResolveSymlink(agentsSource), ".agentignore")
		if _, err := os.Stat(ignoreFile); err == nil {
			r.AgentIgnoreRoot = ignoreFile
		}
		if agents == nil {
			agents, _ = resource.AgentKind{}.Discover(agentsSource)
		}
		for _, a := range agents {
			if a.Disabled {
				r.AgentIgnoredSkills = append(r.AgentIgnoredSkills, a.RelPath)
			}
		}
	}
	r.AgentIgnoredCount = len(r.AgentIgnoredSkills)
	return r
}
