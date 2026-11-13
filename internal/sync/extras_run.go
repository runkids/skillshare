package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/resource"
)

// ExtraRunOptions holds what differs between the callers of RunExtraTargets.
type ExtraRunOptions struct {
	DryRun bool
	Force  bool
	// ProjectRoot is passed to the sync; empty in global mode.
	ProjectRoot string
	// ResolvePath turns a configured target path into the path to sync.
	ResolvePath func(path string) string
	// ResolveExtension loads a target's transform extension by its configured value.
	ResolveExtension func(ext string) (*ExtensionSpec, error)
	// AgentTargetPaths holds cleaned agents sync target paths. Targets of the
	// extra named "agents" at these paths are skipped. Nil skips nothing.
	AgentTargetPaths map[string]bool
}

// AgentTargetPaths returns the cleaned agentPaths, the folders the agents sync
// writes to, for ExtraRunOptions.AgentTargetPaths. It returns nil when
// agentsSource holds no agents, since the agents sync then writes nothing.
func AgentTargetPaths(agentsSource string, agentPaths []string) map[string]bool {
	if _, err := os.Stat(agentsSource); err != nil {
		return nil
	}
	if agents, err := (resource.AgentKind{}).Discover(agentsSource); err != nil || len(agents) == 0 {
		return nil
	}
	var paths map[string]bool
	for _, p := range agentPaths {
		if p == "" {
			continue
		}
		if paths == nil {
			paths = make(map[string]bool)
		}
		paths[filepath.Clean(p)] = true
	}
	return paths
}

// ExtraRun is the outcome of RunExtraTargets for one extra.
type ExtraRun struct {
	SourceMissing bool // the source directory is missing, so no target was synced
	Targets       []ExtraTargetRun
	Warnings      []string
}

// ExtraTargetRun is the outcome of one target. At most one of SkippedBy,
// ModeErr/ExtensionErr (both may be set), Err and Result describes it.
type ExtraTargetRun struct {
	Target config.ExtraTargetConfig // the target as configured
	Path   string                   // the resolved target path
	// Mode is the effective configured mode when the target is skipped or
	// ModeErr is set, "copy" when only ExtensionErr is set, and the mode the
	// target synced with once the sync ran.
	Mode         string
	SkippedBy    string // "agents" when the agents sync already manages the path
	ModeErr      error  // the target has an extension but a non-copy mode
	ExtensionErr error  // the target's extension could not be resolved
	Err          error  // the sync failed
	Result       *ExtraResult
}

// RunExtraTargets syncs every target of extra from sourceDir. A missing
// sourceDir syncs nothing and sets SourceMissing. It only syncs and reports;
// callers print, log and count the results.
func RunExtraTargets(extra config.ExtraConfig, sourceDir string, opts ExtraRunOptions) ExtraRun {
	run := inspectExtraSync(extra, sourceDir, opts.ProjectRoot)
	if run.SourceMissing || len(run.Warnings) > 0 {
		return run
	}

	run.Targets = make([]ExtraTargetRun, 0, len(extra.Targets))
	for _, t := range extra.Targets {
		run.Targets = append(run.Targets, runExtraTarget(extra, t, sourceDir, opts))
	}
	return run
}

func runExtraTarget(extra config.ExtraConfig, t config.ExtraTargetConfig, sourceDir string, opts ExtraRunOptions) ExtraTargetRun {
	tr := ExtraTargetRun{Target: t, Path: opts.ResolvePath(t.Path), Mode: EffectiveMode(t.Mode)}

	if extra.Name == config.AgentsExtraName && opts.AgentTargetPaths[filepath.Clean(tr.Path)] {
		tr.SkippedBy = config.AgentsExtraName
		return tr
	}

	var spec *ExtensionSpec
	if t.Extension != "" {
		mode, modeErr := ResolveExtensionMode(t.Mode)
		if modeErr == nil {
			tr.Mode = mode
		}
		tr.ModeErr = modeErr
		spec, tr.ExtensionErr = opts.ResolveExtension(t.Extension)
		if tr.ModeErr != nil || tr.ExtensionErr != nil {
			return tr
		}
	}

	tr.Result, tr.Err = SyncExtraTarget(extra, t, sourceDir, tr.Path, tr.Mode, spec, ExtraSyncOptions{DryRun: opts.DryRun, Force: opts.Force, ProjectRoot: opts.ProjectRoot})
	tr.Mode = ExtraTargetMode(tr.Mode, extra.File != "")
	return tr
}

// SyncExtraTarget syncs one target of an extra. Single-file extras (file:)
// sync just that file to <target>/<as or file>; others sync the directory.
func SyncExtraTarget(extra config.ExtraConfig, target config.ExtraTargetConfig, sourceDir, targetPath, mode string, spec *ExtensionSpec, opts ExtraSyncOptions) (*ExtraResult, error) {
	if err := CheckExtraSyncInterruption(extra, sourceDir, opts.ProjectRoot); err != nil {
		return nil, err
	}
	return syncExtraTarget(extra, target, sourceDir, targetPath, mode, spec, opts)
}

func syncExtraTarget(extra config.ExtraConfig, target config.ExtraTargetConfig, sourceDir, targetPath, mode string, spec *ExtensionSpec, opts ExtraSyncOptions) (*ExtraResult, error) {
	if extra.File == "" {
		return syncExtraDir(sourceDir, targetPath, mode, target, spec, opts)
	}
	if spec != nil {
		return nil, fmt.Errorf("extensions are not supported for single-file extras")
	}
	return SyncExtraFile(NewExtraFile(sourceDir, extra.File, targetPath, target.As, mode), opts.DryRun, opts.ProjectRoot)
}

func inspectExtraSync(extra config.ExtraConfig, sourceDir, projectRoot string) ExtraRun {
	if err := CheckExtraSyncInterruption(extra, sourceDir, projectRoot); err != nil {
		return ExtraRun{Warnings: []string{err.Error()}}
	}
	_, err := os.Stat(sourceDir)
	return ExtraRun{SourceMissing: os.IsNotExist(err)}
}

// CheckExtraSyncInterruption observes state without repairing, fetching or locking.
func CheckExtraSyncInterruption(extra config.ExtraConfig, sourceDir, projectRoot string) error {
	if extra.Remote != "" && install.InspectRemoteExtra(filepath.Dir(sourceDir), extra.Name).Interrupted {
		return fmt.Errorf("extra %q has an interrupted publication; run: %s", extra.Name, config.ExtraRestoreCommand(extra, projectRoot != ""))
	}
	return nil
}
