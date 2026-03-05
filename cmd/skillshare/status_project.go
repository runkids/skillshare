package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/audit"
	"skillshare/internal/git"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func cmdStatusProject(root string) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return err
	}

	sp := ui.StartSpinner("Discovering skills...")
	discovered, discoverErr := sync.DiscoverSourceSkills(runtime.sourcePath)
	if discoverErr != nil {
		discovered = nil
	}
	trackedRepos := extractTrackedRepos(discovered)
	sp.Stop()

	printProjectSourceStatus(runtime.sourcePath, len(discovered))
	printProjectTrackedReposStatus(runtime.sourcePath, discovered, trackedRepos)
	if err := printProjectTargetsStatus(runtime, discovered); err != nil {
		return err
	}
	printAuditStatus(runtime.config.Audit)

	return nil
}

func cmdStatusProjectJSON(root string) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return writeJSONError(err)
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return writeJSONError(err)
	}

	discovered, _ := sync.DiscoverSourceSkills(runtime.sourcePath)
	trackedRepos := extractTrackedRepos(discovered)

	output := statusJSONOutput{
		Source: statusJSONSource{
			Path:   runtime.sourcePath,
			Exists: dirExists(runtime.sourcePath),
		},
		SkillCount: len(discovered),
		Version:    version,
	}

	// Tracked repos (parallel dirty checks)
	output.TrackedRepos = buildTrackedRepoJSON(runtime.sourcePath, trackedRepos, discovered)

	// Targets
	for _, entry := range runtime.config.Targets {
		target, ok := runtime.targets[entry.Name]
		if !ok {
			continue
		}
		mode := target.Mode
		if mode == "" {
			mode = "merge"
		}
		res := getTargetStatusDetail(target, runtime.sourcePath, mode)
		output.Targets = append(output.Targets, statusJSONTarget{
			Name:        entry.Name,
			Path:        target.Path,
			Mode:        mode,
			Status:      res.statusStr,
			SyncedCount: res.syncedCount,
			Include:     target.Include,
			Exclude:     target.Exclude,
		})
	}

	// Audit
	policy := audit.ResolvePolicy(audit.PolicyInputs{
		ConfigProfile:   runtime.config.Audit.Profile,
		ConfigThreshold: runtime.config.Audit.BlockThreshold,
		ConfigDedupe:    runtime.config.Audit.DedupeMode,
		ConfigAnalyzers: runtime.config.Audit.EnabledAnalyzers,
	})
	output.Audit = statusJSONAudit{
		Profile:   string(policy.Profile),
		Threshold: policy.Threshold,
		Dedupe:    string(policy.DedupeMode),
		Analyzers: policy.EffectiveAnalyzers(),
	}

	return writeJSON(&output)
}

func printProjectSourceStatus(sourcePath string, skillCount int) {
	ui.Header("Source (project)")
	info, err := os.Stat(sourcePath)
	if err != nil {
		ui.Error(".skillshare/skills/ (not found)")
		return
	}

	ui.Success(".skillshare/skills/ (%d skills, %s)", skillCount, info.ModTime().Format("2006-01-02 15:04"))
}

func printProjectTrackedReposStatus(sourcePath string, discovered []sync.DiscoveredSkill, trackedRepos []string) {
	if len(trackedRepos) == 0 {
		return
	}

	ui.Header("Tracked Repositories")
	for _, repoName := range trackedRepos {
		repoPath := filepath.Join(sourcePath, repoName)

		skillCount := 0
		for _, d := range discovered {
			if d.IsInRepo && strings.HasPrefix(d.RelPath, repoName+"/") {
				skillCount++
			}
		}

		statusStr := "up-to-date"
		statusIcon := "✓"
		if isDirty, _ := git.IsDirty(repoPath); isDirty {
			statusStr = "has uncommitted changes"
			statusIcon = "!"
		}

		ui.Status(repoName, statusIcon, fmt.Sprintf("%d skills, %s", skillCount, statusStr))
	}
}

func printProjectTargetsStatus(runtime *projectRuntime, discovered []sync.DiscoveredSkill) error {
	ui.Header("Targets (project)")
	driftTotal := 0
	for _, entry := range runtime.config.Targets {
		target, ok := runtime.targets[entry.Name]
		if !ok {
			ui.Error("%s: target not found", entry.Name)
			continue
		}

		mode := target.Mode
		if mode == "" {
			mode = "merge"
		}

		res := getTargetStatusDetail(target, runtime.sourcePath, mode)
		ui.Status(entry.Name, res.statusStr, res.detail)

		if mode == "merge" || mode == "copy" {
			filtered, err := sync.FilterSkills(discovered, target.Include, target.Exclude)
			if err != nil {
				return fmt.Errorf("target %s has invalid include/exclude config: %w", entry.Name, err)
			}
			filtered = sync.FilterSkillsByTarget(filtered, entry.Name)
			expectedCount := len(filtered)

			if res.syncedCount < expectedCount {
				drift := expectedCount - res.syncedCount
				if drift > driftTotal {
					driftTotal = drift
				}
			}
		} else if len(target.Include) > 0 || len(target.Exclude) > 0 {
			ui.Warning("%s: include/exclude ignored in symlink mode", entry.Name)
		}
	}
	if driftTotal > 0 {
		ui.Warning("%d skill(s) not synced — run 'skillshare sync'", driftTotal)
	}
	return nil
}
