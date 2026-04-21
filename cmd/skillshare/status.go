package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	gosync "sync"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/git"
	hookpkg "skillshare/internal/hooks"
	pluginpkg "skillshare/internal/plugins"
	"skillshare/internal/resource"
	"skillshare/internal/skillignore"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	versioncheck "skillshare/internal/version"
)

// statusJSONOutput is the JSON representation for status --json output.
type statusJSONOutput struct {
	Source       statusJSONSource   `json:"source"`
	SkillCount   int                `json:"skill_count"`
	TrackedRepos []statusJSONRepo   `json:"tracked_repos"`
	Targets      []statusJSONTarget `json:"targets"`
	Agents       *statusJSONAgents  `json:"agents,omitempty"`
	Plugins      []pluginpkg.Bundle `json:"plugins,omitempty"`
	Hooks        []hookpkg.Bundle   `json:"hooks,omitempty"`
	Audit        statusJSONAudit    `json:"audit"`
	Version      string             `json:"version"`
}

type statusJSONSource struct {
	Path        string                  `json:"path"`
	Exists      bool                    `json:"exists"`
	Skillignore *statusJSONSourceIgnore `json:"skillignore"`
}

type statusJSONSourceIgnore struct {
	Active        bool     `json:"active"`
	Files         []string `json:"files,omitempty"`
	Patterns      []string `json:"patterns,omitempty"`
	IgnoredCount  int      `json:"ignored_count"`
	IgnoredSkills []string `json:"ignored_skills,omitempty"`
}

type statusJSONRepo struct {
	Name       string `json:"name"`
	SkillCount int    `json:"skill_count"`
	Dirty      bool   `json:"dirty"`
}

type statusJSONTarget struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Mode        string   `json:"mode"`
	Status      string   `json:"status"`
	SyncedCount int      `json:"synced_count"`
	Include     []string `json:"include"`
	Exclude     []string `json:"exclude"`
}

type statusJSONAudit struct {
	Profile   string   `json:"profile"`
	Threshold string   `json:"threshold"`
	Dedupe    string   `json:"dedupe"`
	Analyzers []string `json:"analyzers"`
}

func cmdStatus(args []string) error {
	if wantsHelp(args) {
		printStatusHelp()
		return nil
	}

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	jsonOutput := hasFlag(rest, "--json")

	if mode == modeProject {
		// Reject unexpected positional arguments in project mode
		for _, arg := range rest {
			if arg != "--json" {
				return fmt.Errorf("unexpected arguments: %v", rest)
			}
		}
		if jsonOutput {
			return cmdStatusProjectJSON(cwd)
		}
		return cmdStatusProject(cwd)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if !jsonOutput {
		sp := ui.StartSpinner("Discovering skills...")
		discovered, stats, discoverErr := sync.DiscoverSourceSkillsWithStats(cfg.Source)
		if discoverErr != nil {
			discovered = nil
		}
		trackedRepos := extractTrackedRepos(discovered)
		sp.Stop()

		printSourceStatus(cfg, len(discovered), stats)
		printTrackedReposStatus(cfg, discovered, trackedRepos)
		if err := printTargetsStatus(cfg, discovered); err != nil {
			return err
		}

		// Extras
		if len(cfg.Extras) > 0 {
			ui.Header("Extras")
			printExtrasStatus(cfg.Extras, func(extra config.ExtraConfig) string {
				return config.ResolveExtrasSourceDir(extra, cfg.ExtrasSource, cfg.Source)
			})
		}
		if bundles, bundleErr := pluginpkg.Discover(cfg.EffectivePluginsSource()); bundleErr == nil && len(bundles) > 0 {
			ui.Header("Plugins")
			for _, bundle := range bundles {
				ui.Status(bundle.Name, "plugin", fmt.Sprintf("claude=%t codex=%t", bundle.HasClaude, bundle.HasCodex))
			}
		}
		if bundles, bundleErr := hookpkg.Discover(cfg.EffectiveHooksSource()); bundleErr == nil && len(bundles) > 0 {
			ui.Header("Hooks")
			for _, bundle := range bundles {
				ui.Status(bundle.Name, "hook", fmt.Sprintf("claude=%d codex=%d", bundle.Targets["claude"], bundle.Targets["codex"]))
			}
		}

		printAuditStatus(cfg.Audit)
		checkSkillVersion(cfg)
		return nil
	}

	// JSON mode
	output := statusJSONOutput{
		Version: version,
	}

	discovered, stats, _ := sync.DiscoverSourceSkillsWithStats(cfg.Source)
	trackedRepos := extractTrackedRepos(discovered)

	output.Source = statusJSONSource{
		Path:        cfg.Source,
		Exists:      dirExists(cfg.Source),
		Skillignore: buildSkillignoreJSON(stats),
	}
	output.SkillCount = len(discovered)
	output.TrackedRepos = buildTrackedRepoJSON(cfg.Source, trackedRepos, discovered)

	for name, target := range cfg.Targets {
		sc := target.SkillsConfig()
		tMode := getTargetMode(sc.Mode, cfg.Mode)
		res := getTargetStatusDetail(target, cfg.Source, tMode)
		output.Targets = append(output.Targets, statusJSONTarget{
			Name:        name,
			Path:        sc.Path,
			Mode:        tMode,
			Status:      res.statusStr,
			SyncedCount: res.syncedCount,
			Include:     sc.Include,
			Exclude:     sc.Exclude,
		})
	}

	policy := audit.ResolvePolicy(audit.PolicyInputs{
		ConfigProfile:   cfg.Audit.Profile,
		ConfigThreshold: cfg.Audit.BlockThreshold,
		ConfigDedupe:    cfg.Audit.DedupeMode,
		ConfigAnalyzers: cfg.Audit.EnabledAnalyzers,
	})
	output.Audit = statusJSONAudit{
		Profile:   string(policy.Profile),
		Threshold: policy.Threshold,
		Dedupe:    string(policy.DedupeMode),
		Analyzers: policy.EffectiveAnalyzers(),
	}

	output.Agents = buildAgentStatusJSON(cfg)
	output.Plugins, _ = pluginpkg.Discover(cfg.EffectivePluginsSource())
	output.Hooks, _ = hookpkg.Discover(cfg.EffectiveHooksSource())

	return writeJSON(&output)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func printExtrasStatus(extras []config.ExtraConfig, sourceDirFn func(config.ExtraConfig) string) {
	for _, extra := range extras {
		sourceDir := sourceDirFn(extra)
		files, err := sync.DiscoverExtraFiles(sourceDir)
		if err != nil {
			ui.Warning("  %s: source not found", extra.Name)
			continue
		}
		if len(extra.Targets) == 0 {
			ui.Warning("  %s: no targets configured", extra.Name)
			continue
		}
		for _, t := range extra.Targets {
			detail := fmt.Sprintf("[%s] %s (%d files)", sync.EffectiveMode(t.Mode), t.Path, len(files))
			ui.Status(extra.Name, "has files", detail)
		}
	}
}

func printSourceStatus(cfg *config.Config, skillCount int, stats *skillignore.IgnoreStats) {
	ui.Header("Source")
	info, err := os.Stat(cfg.Source)
	if err != nil {
		ui.Error("%s (not found)", cfg.Source)
		return
	}

	ui.Success("%s (%d skills, %s)", cfg.Source, skillCount, info.ModTime().Format("2006-01-02 15:04"))
	printSkillignoreLine(stats)

	// Agents source
	agentsSource := cfg.EffectiveAgentsSource()
	if agentsInfo, agentsErr := os.Stat(agentsSource); agentsErr == nil {
		agentCount := 0
		if entries, readErr := os.ReadDir(agentsSource); readErr == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
					agentCount++
				}
			}
		}
		ui.Success("%s (%d agents, %s)", agentsSource, agentCount, agentsInfo.ModTime().Format("2006-01-02 15:04"))
	}
}

func printSkillignoreLine(stats *skillignore.IgnoreStats) {
	if stats == nil || !stats.Active() {
		return
	}
	hint := ".skillignore"
	if stats.HasLocal() {
		hint += " (.local active)"
	}
	ui.Info("%s: %d patterns, %d skills ignored", hint, stats.PatternCount(), stats.IgnoredCount())
}

func buildSkillignoreJSON(stats *skillignore.IgnoreStats) *statusJSONSourceIgnore {
	if stats == nil || !stats.Active() {
		return &statusJSONSourceIgnore{Active: false}
	}

	var files []string
	if stats.RootFile != "" {
		files = append(files, stats.RootFile)
	}
	if stats.RootLocalFile != "" {
		files = append(files, stats.RootLocalFile)
	}
	files = append(files, stats.RepoFiles...)
	files = append(files, stats.RepoLocalFiles...)

	return &statusJSONSourceIgnore{
		Active:        true,
		Files:         files,
		Patterns:      stats.Patterns,
		IgnoredCount:  stats.IgnoredCount(),
		IgnoredSkills: stats.IgnoredSkills,
	}
}

func printTrackedReposStatus(cfg *config.Config, discovered []sync.DiscoveredSkill, trackedRepos []string) {
	if len(trackedRepos) == 0 {
		return
	}

	ui.Header("Tracked Repositories")
	for _, repoName := range trackedRepos {
		repoPath := filepath.Join(cfg.Source, repoName)

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

// extractTrackedRepos derives tracked repo names from discovered skills.
// Note: repos with zero skills will not appear (acceptable trade-off).
func extractTrackedRepos(discovered []sync.DiscoveredSkill) []string {
	seen := make(map[string]bool)
	var repos []string
	for _, d := range discovered {
		if !d.IsInRepo {
			continue
		}
		// First path segment is the repo name (e.g. "_team" from "_team/frontend/ui")
		idx := strings.Index(d.RelPath, "/")
		if idx <= 0 {
			continue
		}
		repo := d.RelPath[:idx]
		if !seen[repo] {
			seen[repo] = true
			repos = append(repos, repo)
		}
	}
	sort.Strings(repos)
	return repos
}

// buildTrackedRepoJSON builds statusJSONRepo entries with parallel git.IsDirty checks.
func buildTrackedRepoJSON(sourcePath string, trackedRepos []string, discovered []sync.DiscoveredSkill) []statusJSONRepo {
	results := make([]statusJSONRepo, len(trackedRepos))

	// Count skills per repo (single pass)
	repoSkillCount := make(map[string]int, len(trackedRepos))
	for _, d := range discovered {
		if !d.IsInRepo {
			continue
		}
		idx := strings.Index(d.RelPath, "/")
		if idx > 0 {
			repoSkillCount[d.RelPath[:idx]]++
		}
	}

	// Parallel git.IsDirty checks
	var wg gosync.WaitGroup
	for i, repoName := range trackedRepos {
		wg.Add(1)
		go func(idx int, name string) {
			defer wg.Done()
			repoPath := filepath.Join(sourcePath, name)
			dirty, _ := git.IsDirty(repoPath)
			results[idx] = statusJSONRepo{
				Name:       name,
				SkillCount: repoSkillCount[name],
				Dirty:      dirty,
			}
		}(i, repoName)
	}
	wg.Wait()
	return results
}

// targetStatusResult bundles status detail with synced count to avoid
// duplicate CheckStatusMerge/Copy calls.
type targetStatusResult struct {
	statusStr   string
	detail      string
	syncedCount int // -1 for symlink mode (no drift check)
}

func printTargetsStatus(cfg *config.Config, discovered []sync.DiscoveredSkill) error {
	ui.Header("Targets")

	builtinAgents := config.DefaultAgentTargets()
	agentsSource := cfg.EffectiveAgentsSource()
	agentsExist := dirExists(agentsSource)
	var agentCount int
	if agentsExist {
		agents, _ := resource.AgentKind{}.Discover(agentsSource)
		agentCount = len(agents)
	}

	driftTotal := 0
	for name, target := range cfg.Targets {
		// Target name header
		fmt.Printf("%s%s%s\n", ui.Bold, name, ui.Reset)

		// Skills sub-item
		sc := target.SkillsConfig()
		mode := getTargetMode(sc.Mode, cfg.Mode)
		res := getTargetStatusDetail(target, cfg.Source, mode)
		printTargetSubItem("skills", res.statusStr, res.detail)

		if mode == "merge" || mode == "copy" {
			filtered, err := sync.FilterSkills(discovered, sc.Include, sc.Exclude)
			if err != nil {
				return fmt.Errorf("target %s has invalid include/exclude config: %w", name, err)
			}
			filtered = sync.FilterSkillsByTarget(filtered, name)
			expectedCount := len(filtered)

			if res.syncedCount < expectedCount {
				drift := expectedCount - res.syncedCount
				if drift > driftTotal {
					driftTotal = drift
				}
			}
		} else if len(sc.Include) > 0 || len(sc.Exclude) > 0 {
			ui.Warning("%s: include/exclude ignored in symlink mode", name)
		}

		// Agents sub-item
		if agentsExist {
			agentPath := resolveAgentTargetPath(target, builtinAgents, name)
			if agentPath != "" {
				linked := countLinkedAgents(agentPath)
				agentStatus := "merged"
				driftLabel := ""
				if linked != agentCount && agentCount > 0 {
					agentStatus = "drift"
					driftLabel = ui.Yellow + " (drift)" + ui.Reset
				}
				printTargetSubItem("agents", agentStatus, fmt.Sprintf("[merge] %d/%d linked%s", linked, agentCount, driftLabel))
			}
		}
	}
	if driftTotal > 0 {
		ui.Warning("%d skill(s) not synced — run 'skillshare sync'", driftTotal)
	}
	return nil
}

// printTargetSubItem prints an indented sub-item line under a target.
func printTargetSubItem(kind, status, detail string) {
	statusColor := ui.Gray
	switch status {
	case "merged", "synced", "copied", "linked":
		statusColor = ui.Green
	case "drift", "not exist":
		statusColor = ui.Yellow
	case "conflict", "broken":
		statusColor = ui.Red
	}
	fmt.Printf("  %-8s %s%-12s%s %s\n", kind, statusColor, status, ui.Reset, ui.Dim+detail+ui.Reset)
}

func getTargetMode(targetMode, globalMode string) string {
	if targetMode != "" {
		return targetMode
	}
	if globalMode != "" {
		return globalMode
	}
	return "merge"
}

func getTargetStatusDetail(target config.TargetConfig, source, mode string) targetStatusResult {
	switch mode {
	case "merge":
		return getMergeStatusDetail(target, source, mode)
	case "copy":
		return getCopyStatusDetail(target, mode)
	default:
		return getSymlinkStatusDetail(target, source, mode)
	}
}

func getMergeStatusDetail(target config.TargetConfig, source, mode string) targetStatusResult {
	sc := target.SkillsConfig()
	status, linkedCount, localCount := sync.CheckStatusMerge(sc.Path, source)

	switch status {
	case sync.StatusMerged:
		return targetStatusResult{"merged", fmt.Sprintf("[%s] %s (%d shared, %d local)", mode, sc.Path, linkedCount, localCount), linkedCount}
	case sync.StatusLinked:
		return targetStatusResult{"linked", fmt.Sprintf("[%s->needs sync] %s", mode, sc.Path), linkedCount}
	default:
		return targetStatusResult{status.String(), fmt.Sprintf("[%s] %s (%d local)", mode, sc.Path, localCount), 0}
	}
}

func getCopyStatusDetail(target config.TargetConfig, mode string) targetStatusResult {
	sc := target.SkillsConfig()
	status, managedCount, localCount := sync.CheckStatusCopy(sc.Path)

	switch status {
	case sync.StatusCopied:
		return targetStatusResult{"copied", fmt.Sprintf("[%s] %s (%d managed, %d local)", mode, sc.Path, managedCount, localCount), managedCount}
	case sync.StatusLinked:
		return targetStatusResult{"linked", fmt.Sprintf("[%s->needs sync] %s", mode, sc.Path), managedCount}
	default:
		return targetStatusResult{status.String(), fmt.Sprintf("[%s] %s (%d local)", mode, sc.Path, localCount), 0}
	}
}

func getSymlinkStatusDetail(target config.TargetConfig, source, mode string) targetStatusResult {
	sc := target.SkillsConfig()
	status := sync.CheckStatus(sc.Path, source)
	detail := fmt.Sprintf("[%s] %s", mode, sc.Path)

	switch status {
	case sync.StatusConflict:
		link, _ := os.Readlink(sc.Path)
		detail = fmt.Sprintf("[%s] %s -> %s", mode, sc.Path, link)
	case sync.StatusMerged:
		// Configured as symlink but actually using merge - needs resync
		detail = fmt.Sprintf("[%s->needs sync] %s", mode, sc.Path)
	}

	return targetStatusResult{status.String(), detail, -1}
}

func printAuditStatus(ac config.AuditConfig) {
	ui.Header("Audit")

	policy := audit.ResolvePolicy(audit.PolicyInputs{
		ConfigProfile:   ac.Profile,
		ConfigThreshold: ac.BlockThreshold,
		ConfigDedupe:    ac.DedupeMode,
		ConfigAnalyzers: ac.EnabledAnalyzers,
	})

	ui.Info("Profile:    %s", colorizeProfile(string(policy.Profile)))
	ui.Info("Block:      severity >= %s", ui.Colorize(ui.SeverityColor(policy.Threshold), strings.ToUpper(policy.Threshold)))
	ui.Info("Dedupe:     %s", colorizeDedupe(string(policy.DedupeMode)))
	ui.Info("Analyzers:  %s", colorizeAnalyzers(policy.EnabledAnalyzers))
}

func checkSkillVersion(cfg *config.Config) {
	ui.Header("Version")

	// CLI version
	ui.Success("CLI: %s", version)

	// Skill version
	localVersion := versioncheck.ReadLocalSkillVersion(cfg.Source)

	if localVersion == "" {
		ui.Warning("Skill: not found or missing version")
		ui.Info("  Run: skillshare upgrade --skill")
		return
	}

	// Fetch remote version (with short timeout)
	remoteVersion := versioncheck.FetchRemoteSkillVersion()
	if remoteVersion == "" {
		// Network error - just show local version
		ui.Info("Skill: %s", localVersion)
		return
	}

	// Compare local vs remote
	if localVersion != remoteVersion {
		ui.Warning("Skill: %s (update available: %s)", localVersion, remoteVersion)
		ui.Info("  Run: skillshare upgrade --skill && skillshare sync")
	} else {
		ui.Success("Skill: %s (up to date)", localVersion)
	}
}

func printStatusHelp() {
	fmt.Println(`Usage: skillshare status [options]

Show status of source, skills, agents, and all targets.

Options:
  --json            Output results as JSON
  --project, -p     Use project-level config
  --global, -g      Use global config
  --help, -h        Show this help

Examples:
  skillshare status              Show current state (skills + agents)
  skillshare status --json       Output as JSON
  skillshare status -p           Show project status`)
}
