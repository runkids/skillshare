package sync

import (
	"fmt"

	"skillshare/internal/config"
	"skillshare/internal/resource"
)

// AgentTarget is one target an agent sync run visits.
type AgentTarget struct {
	Name   string
	Path   string // resolved agents path; "" when the target has none
	Config config.ResourceTargetConfig
}

// AgentRunOptions holds what differs between agent sync callers.
type AgentRunOptions struct {
	Source      string // agents source directory
	ProjectRoot string // passed to SyncAgents; "" in global mode
	DryRun      bool
	Force       bool
	// ResolveExtension turns a target's extension value into a transform
	// spec. A nil spec links or copies agents instead of transforming them.
	ResolveExtension func(ext string) (*ExtensionSpec, error)
}

// AgentTargetResult is the outcome of an agent sync for one target.
type AgentTargetResult struct {
	Name string
	Path string // "" when the target has no agents path and was not synced
	// Mode is the configured mode (merge when unset), after the file-link
	// fallback for syncs that do not transform.
	Mode     string
	Linked   []string
	Updated  []string
	Skipped  []string
	Pruned   []string
	Synced   bool  // sync or transform returned results, possibly partial
	PruneErr error // also reported in Warnings; the sync itself stands
	Warnings []string
	Err      error // invalid filter or extension; nothing was synced
	SyncErr  error // sync or transform failure
}

// RunAgentSync filters agents for each target, syncs or transforms them into
// the target's agents path, and prunes orphans left by earlier syncs.
func RunAgentSync(targets []AgentTarget, agents []resource.DiscoveredResource, opts AgentRunOptions) []AgentTargetResult {
	results := make([]AgentTargetResult, 0, len(targets))
	for _, t := range targets {
		results = append(results, runAgentTarget(t, agents, opts))
	}
	return results
}

func runAgentTarget(t AgentTarget, agents []resource.DiscoveredResource, opts AgentRunOptions) AgentTargetResult {
	res := AgentTargetResult{Name: t.Name, Path: t.Path}
	if t.Path == "" {
		return res
	}
	mode := t.Config.Mode
	if mode == "" {
		mode = "merge"
	}
	res.Mode = mode

	filtered, err := FilterAgents(agents, t.Config.Include, t.Config.Exclude)
	if err != nil {
		res.Err = fmt.Errorf("invalid agent filter: %w", err)
		return res
	}
	filtered = FilterAgentsByTarget(filtered, t.Name)

	spec, err := opts.ResolveExtension(t.Config.Extension)
	if err != nil {
		res.Err = err
		return res
	}

	var result *AgentSyncResult
	outputExt := ""
	if spec != nil {
		result, res.SyncErr = SyncAgentsTransform(filtered, opts.Source, t.Path, mode, spec, opts.DryRun, opts.Force)
		outputExt = spec.OutputExt
	} else {
		result, res.SyncErr = SyncAgents(filtered, opts.Source, t.Path, mode, opts.DryRun, opts.Force, opts.ProjectRoot)
		res.Mode = EffectiveAgentMode(mode)
		if res.Mode != mode {
			res.Warnings = append(res.Warnings, t.Name+": agents "+FileLinkFallbackWarning)
		}
	}
	// A transform returns partial results when only some agents failed;
	// the rest still count and orphans still get pruned.
	if result == nil {
		return res
	}
	res.Synced = true
	res.Linked, res.Updated, res.Skipped = result.Linked, result.Updated, result.Skipped

	// Prune even when the source is empty so uninstall-all clears previously
	// synced target entries, matching skills.
	switch mode {
	case "copy":
		res.Pruned, res.PruneErr = PruneOrphanAgentCopies(t.Path, filtered, outputExt, opts.DryRun)
	case "merge":
		res.Pruned, res.PruneErr = PruneOrphanAgentLinks(t.Path, opts.Source, filtered, opts.DryRun)
	}
	if res.PruneErr != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s: agents prune failed: %v", t.Name, res.PruneErr))
	}
	return res
}
