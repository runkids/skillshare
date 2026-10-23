package sync

import (
	"os"
	"path/filepath"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/resource"
)

// DiscoverTargetAgents preserves agent discovery's empty-source/prune behavior.
// all also holds .agentignore'd agents, so NewIgnoreReport needs no second scan.
func DiscoverTargetAgents(source string) (active, all []resource.DiscoveredResource, available bool) {
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		all, _ = (resource.AgentKind{}).Discover(source)
		if all == nil {
			// NewIgnoreReport treats nil as a rescan request; preserve an empty snapshot.
			all = []resource.DiscoveredResource{}
		}
		return resource.ActiveAgents(all), all, true
	}
	return nil, nil, false
}

// BackupTargetsOptions describes the adapter's backup location and policy.
type BackupTargetsOptions struct {
	Dir                 string
	Retention           backup.CleanupConfig
	ProjectMode         bool
	ProjectRoot         string
	BuiltinAgentTargets map[string]config.TargetConfig
}

// BackupTargets snapshots folders before execution and returns nonfatal warnings.
func BackupTargets(targets map[string]config.TargetConfig, skills, agents bool, opts BackupTargetsOptions) []string {
	var warnings []string
	dir := opts.Dir
	if opts.ProjectMode {
		skills = false
	}
	snapshot := func(entry, path string) {
		if _, err := backup.CreateInDir(dir, entry, path); err != nil {
			warnings = append(warnings, "backup failed for "+entry+": "+err.Error())
		}
	}
	builtinAgents := opts.BuiltinAgentTargets
	for name, target := range targets {
		if skills && target.SkillsConfig().IsEnabled() {
			snapshot(name, target.SkillsConfig().Path)
		}
		if p := ResolveAgentTargetPath(target, builtinAgents, name, opts.ProjectMode); agents && p != "" {
			snapshot(name+"-agents", resolveTargetPath(opts.ProjectRoot, p))
		}
	}
	if _, err := backup.CleanupInDir(dir, opts.Retention); err != nil {
		warnings = append(warnings, "backup cleanup failed: "+err.Error())
	}
	return warnings
}

func resolveTargetPath(root, path string) string {
	path = config.ExpandPath(path)
	if root != "" && !filepath.IsAbs(path) {
		return filepath.Join(root, filepath.FromSlash(path))
	}
	return path
}

// ResolveAgentTargetPath resolves explicit, builtin and tool-default agent paths.
func ResolveAgentTargetPath(target config.TargetConfig, builtinAgents map[string]config.TargetConfig, name string, projectMode bool) string {
	if ac := target.AgentsConfig(); ac.Path != "" {
		return config.ExpandPath(ac.Path)
	}
	if builtin, ok := builtinAgents[name]; ok {
		return config.ExpandPath(builtin.Path)
	}
	if projectMode {
		if builtin, ok := config.LookupProjectAgentTarget(name); ok {
			return config.ExpandPath(builtin.Path)
		}
	} else if builtin, ok := config.LookupGlobalAgentTarget(name); ok {
		return config.ExpandPath(builtin.Path)
	}
	return ""
}
