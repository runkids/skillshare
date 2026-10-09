package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	gosync "sync"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/resource"
	"skillshare/internal/skillignore"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
	versioncheck "skillshare/internal/version"
)

// statusJSONOutput is the JSON representation for status --json output.
type statusJSONOutput struct {
	Source       statusJSONSource   `json:"source"`
	SkillCount   int                `json:"skill_count"`
	TrackedRepos []statusJSONRepo   `json:"tracked_repos"`
	Targets      []statusJSONTarget `json:"targets"`
	Agents       *statusJSONAgents  `json:"agents,omitempty"`
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
	Status     string `json:"status,omitempty"`
	Message    string `json:"message,omitempty"`
}

type statusJSONTarget struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Mode        string   `json:"mode"`
	Status      string   `json:"status"`
	SyncedCount int      `json:"synced_count"`
	Include     []string `json:"include"`
	Exclude     []string `json:"exclude"`
	// SkillsEnabled is false for a target with skills switched off.
	SkillsEnabled bool `json:"skills_enabled"`
	// Warning says why sync will reject the target, with the fix.
	Warning string `json:"warning,omitempty"`
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

	mode = resolveAutoMode(mode, cwd)

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

	walk := cfg.SkillsWalk()
	if !jsonOutput {
		sp := ui.StartSpinner("Discovering skills...")
		discovered, stats, discoverErr := sync.DiscoverSourceSkillsWithStats(cfg.EffectiveSkillsSource(), walk)
		if discoverErr != nil {
			discovered = nil
		}
		trackedRepos := extractTrackedRepos(cfg.EffectiveSkillsSource(), walk)
		sp.Stop()
		printSkippedSourceLinkWarnings(walk, false)

		printSourceStatus(cfg.EffectiveSkillsSource(), cfg.EffectiveAgentsSource(), utils.FoldHomePath, len(discovered), countSourceAgents(cfg.EffectiveAgentsSource()), stats)
		printTrackedReposStatus(cfg.EffectiveSkillsSource(), discovered, trackedRepos)
		if err := printTargetsStatus(cfg, discovered); err != nil {
			return err
		}

		if len(cfg.Extras) > 0 {
			printExtrasStatus(cfg.Extras, func(extra config.ExtraConfig) string {
				return config.ResolveExtrasSourceDir(extra, cfg.EffectiveExtrasSource(), cfg.EffectiveSkillsSource())
			})
		}

		printAuditStatus(cfg.Audit)
		checkSkillVersion(cfg)
		return nil
	}

	// JSON mode
	output := statusJSONOutput{
		Version: version,
	}

	discovered, stats, _ := sync.DiscoverSourceSkillsWithStats(cfg.EffectiveSkillsSource(), walk)
	trackedRepos := extractTrackedRepos(cfg.EffectiveSkillsSource(), walk)

	printSkippedSourceLinkWarnings(walk, true)
	output.Source = statusJSONSource{
		Path:        cfg.EffectiveSkillsSource(),
		Exists:      dirExists(cfg.EffectiveSkillsSource()),
		Skillignore: buildSkillignoreJSON(stats),
	}
	output.SkillCount = len(discovered)
	output.TrackedRepos = buildTrackedRepoJSON(cfg.EffectiveSkillsSource(), trackedRepos, discovered, walk)

	for name, target := range cfg.Targets {
		sc := target.SkillsConfig()
		tMode := getTargetMode(sc.Mode, cfg.Mode)
		res := getTargetStatusDetail(target, cfg.EffectiveSkillsSource(), tMode)
		output.Targets = append(output.Targets, statusJSONTarget{
			Name:        name,
			Path:        sc.Path,
			Mode:        tMode,
			Status:      res.statusStr,
			SyncedCount: res.syncedCount,
			Include:     sc.Include,
			Exclude:     sc.Exclude,

			SkillsEnabled: sc.IsEnabled(),
			Warning:       namingWarning(target, tMode),
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

	return writeJSON(&output)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// countSourceAgents counts the agent files at the top of agentsSource, or
// returns -1 when the folder does not exist.
func countSourceAgents(agentsSource string) int {
	entries, err := os.ReadDir(agentsSource)
	if err != nil {
		return -1
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			count++
		}
	}
	return count
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

// extractTrackedRepos walks sourcePath for `_`-prefixed directories that are
// git repositories. Using a directory walk (instead of deriving from discovered
// skills) ensures repos with zero discoverable skills — e.g. those whose only
// SKILL.md sits at the repo root — still appear in status output.
func extractTrackedRepos(sourcePath string, walk sourcewalk.Options) []string {
	repos, err := install.GetTrackedRepos(sourcePath, walk)
	if err != nil {
		return nil
	}
	sort.Strings(repos)
	return repos
}

// buildTrackedRepoJSON builds statusJSONRepo entries with parallel git.IsDirty checks.
func buildTrackedRepoJSON(sourcePath string, trackedRepos []string, discovered []sync.DiscoveredSkill, walks ...sourcewalk.Options) []statusJSONRepo {
	results := make([]statusJSONRepo, len(trackedRepos))

	missingRepos, _ := install.GetMissingTrackedRepos(sourcePath, walks...)

	// Count skills per repo (single pass). RepoRelPath covers skills at the
	// repo root and nested ones, and repos placed with --into (org/_repo).
	repoSkillCount := make(map[string]int, len(trackedRepos))
	for _, d := range discovered {
		if d.IsInRepo {
			repoSkillCount[d.RepoRelPath]++
		}
	}

	// Parallel git.IsDirty checks
	var wg gosync.WaitGroup
	for i, repoName := range trackedRepos {
		wg.Add(1)
		go func(idx int, name string) {
			defer wg.Done()
			repoPath := filepath.Join(sourcePath, name)
			dirty, err := git.IsDirty(repoPath)
			results[idx] = statusJSONRepo{
				Name:       name,
				SkillCount: repoSkillCount[name],
				Dirty:      dirty,
			}
			if err != nil {
				results[idx].Status = "unknown"
				results[idx].Message = (&gitStatusError{err: err}).Error()
			}
		}(i, repoName)
	}
	wg.Wait()

	for _, repo := range missingRepos {
		results = append(results, statusJSONRepo{
			Name:    repo.Name,
			Status:  "missing",
			Message: missingTrackedRepoMessage(repo.Name),
		})
	}
	return results
}

// targetStatusResult bundles status detail with synced count to avoid
// duplicate CheckStatusMerge/Copy calls.
type targetStatusResult struct {
	statusStr   string
	syncedCount int // -1 for symlink mode (no drift check)
	localCount  int
}

func printTargetsStatus(cfg *config.Config, discovered []sync.DiscoveredSkill) error {
	builtinAgents := config.DefaultAgentTargets()
	agentsSource := cfg.EffectiveAgentsSource()
	agentsExist := dirExists(agentsSource)
	var agents []resource.DiscoveredResource
	if agentsExist {
		agents, _ = resource.AgentKind{}.Discover(agentsSource)
	}

	var rows []statusTarget
	var warnings []string
	notSynced := 0
	for _, name := range slices.Sorted(maps.Keys(cfg.Targets)) {
		target := cfg.Targets[name]
		sc := target.SkillsConfig()
		mode := getTargetMode(sc.Mode, cfg.Mode)
		res := getTargetStatusDetail(target, cfg.EffectiveSkillsSource(), mode)
		if err := target.NamingModeConfigError(mode); err != nil {
			warnings = append(warnings, name+": "+err.Error())
		}

		// A target with skills off expects nothing, so it has no drift.
		expected := 0
		if sc.IsEnabled() && (mode == "merge" || mode == "copy") {
			var err error
			expected, err = sync.ExpectedSkillCount(name, sc, discovered)
			if err != nil {
				return fmt.Errorf("target %s has invalid include/exclude config: %w", name, err)
			}
			notSynced = max(notSynced, expected-res.syncedCount)
		} else if sc.IsEnabled() && (len(sc.Include) > 0 || len(sc.Exclude) > 0) {
			warnings = append(warnings, name+": include/exclude ignored in symlink mode")
		}

		row := statusTarget{name: name, path: utils.FoldHomePath(sc.Path), skills: skillsCell(res, sc.Path, mode, expected)}
		if sc.IsEnabled() {
			row.mode = mode
		}
		if agentsExist {
			if agentPath := resolveAgentTargetPath(target, builtinAgents, name); agentPath != "" {
				ac := target.AgentsConfig()
				expected, err := expectedAgentsForTarget(ac, name, agents)
				if err != nil {
					return err
				}
				preserved := 0
				linked := countLinkedAgents(ac, agentPath, expected, &preserved)
				row.agents = agentsCell(linked, len(expected), preserved)
			}
		}
		rows = append(rows, row)
	}
	printStatusTargets(rows, agentsExist, warnings, notSynced)
	return nil
}

// namingWarning is the naming and mode error of a target that syncs skills, as text.
func namingWarning(target config.TargetConfig, mode string) string {
	if err := target.NamingModeConfigError(mode); err != nil {
		return err.Error()
	}
	return ""
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
	if !target.SkillsConfig().IsEnabled() {
		return targetStatusResult{statusStr: "skills off"}
	}
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
	status, linkedCount, localCount := sync.CheckStatusMerge(target.SkillsConfig().Path, source)

	switch status {
	case sync.StatusMerged, sync.StatusLinked:
		return targetStatusResult{status.String(), linkedCount, localCount}
	default:
		return targetStatusResult{status.String(), 0, localCount}
	}
}

func getCopyStatusDetail(target config.TargetConfig, mode string) targetStatusResult {
	status, managedCount, localCount := sync.CheckStatusCopy(target.SkillsConfig().Path)

	switch status {
	case sync.StatusCopied, sync.StatusLinked:
		return targetStatusResult{status.String(), managedCount, localCount}
	default:
		return targetStatusResult{status.String(), 0, localCount}
	}
}

func getSymlinkStatusDetail(target config.TargetConfig, source, mode string) targetStatusResult {
	return targetStatusResult{sync.CheckStatus(target.SkillsConfig().Path, source).String(), -1, 0}
}

// checkSkillVersion prints the CLI and built-in skill versions, and how to
// update the skill when it is missing or behind.
func checkSkillVersion(cfg *config.Config) {
	localVersion := versioncheck.ReadLocalSkillVersion(cfg.EffectiveSkillsSource())
	cli := "CLI " + version

	if localVersion == "" {
		fmt.Println(statusLine("Version", cli+theme.Dim().Render(" · ")+"skill not installed"))
		ui.Warning("Install the skillshare skill %s %s", theme.Dim().Render("— run"), theme.Accent().Render("skillshare upgrade --skill"))
		return
	}

	skill := cli + theme.Dim().Render(" · ") + "skill " + localVersion
	switch remoteVersion := versioncheck.CachedRemoteSkillVersion(); {
	case remoteVersion == "":
		// Offline: show the local version only.
		fmt.Println(statusLine("Version", skill))
	case versioncheck.SkillOutdated(localVersion, remoteVersion):
		fmt.Println(statusLine("Version", skill))
		ui.Warning("Skill %s is available %s %s", remoteVersion, theme.Dim().Render("— run"), theme.Accent().Render("skillshare upgrade --skill && skillshare sync"))
	default:
		fmt.Println(statusLine("Version", skill+" "+theme.Dim().Render("(up to date)")))
	}
}

func printStatusHelp() {
	printHelp("skillshare status [options]", "Show status of source, skills, agents, and all targets.",
		helpGroup{title: "Options", rows: []helpRow{
			{"--json", "Output results as JSON"},
			{"-p, --project", "Use project-level config"},
			{"-g, --global", "Use global config"},
		}},
		helpExamples(
			helpRow{"skillshare status", "Show current state (skills + agents)"},
			helpRow{"skillshare status --json", "Output as JSON"},
			helpRow{"skillshare status -p", "Show project status"},
		),
	)
}
