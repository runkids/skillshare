package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/check"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
)

// checkRepoResult holds the check result for a tracked repo
type checkRepoResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "up_to_date", "behind", "dirty", "error"
	Behind  int    `json:"behind"`
	Branch  string `json:"branch,omitempty"`
	Message string `json:"message,omitempty"`
}

// checkSkillResult holds the check result for a regular skill
type checkSkillResult struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Version     string `json:"version"`
	Status      string `json:"status"` // "up_to_date", "update_available", "stale", "modified", "local", "error"
	InstalledAt string `json:"installed_at,omitempty"`
	Message     string `json:"message,omitempty"`
}

// checkOutput is the JSON output structure
type checkOutput struct {
	TrackedRepos []checkRepoResult  `json:"tracked_repos"`
	Skills       []checkSkillResult `json:"skills"`
}

// checkOptions holds parsed arguments for check command
type checkOptions struct {
	names  []string // positional (0+ = all)
	groups []string // --group/-G
	json   bool
	diff   bool // --diff: show each skill's changes since install

	parseOpts install.ParseOptions // how --diff parses recorded sources
}

// collectCheckItems reads metadata and partitions items for parallel checking.
// Returns: tracked repo inputs and the planned skill check, in which skills
// without a remote are already resolved (no network needed).
// projectRoot is the base of relative local sources; "" in global mode.
// follow resolves linked skill directories before they are hashed.
func collectCheckItems(sourceDir, projectRoot string, repos []string, skills []string, follow *sourcewalk.Follow) ([]check.RepoCheckInput, *check.Resolution) {
	var repoInputs []check.RepoCheckInput
	for _, repo := range repos {
		repoInputs = append(repoInputs, check.RepoCheckInput{
			Name:     repo,
			RepoPath: filepath.Join(sourceDir, repo),
		})
	}

	// Load centralized metadata store once for all skills.
	store := install.LoadMetadataOrNew(sourceDir)

	checkSkills := make([]check.Skill, len(skills))
	for i, skill := range skills {
		checkSkills[i] = check.Skill{Name: skill, Entry: store.GetByPath(skill), Dir: filepath.Join(sourceDir, skill), Follow: follow}
	}

	return repoInputs, check.Plan(checkSkills, projectRoot)
}

// resolveCheckSkills asks the remotes of a planned skill check in parallel,
// reporting the skill count of each finished remote to progressBar (may be nil).
func resolveCheckSkills(plan *check.Resolution, progressBar *ui.ProgressBar) []checkSkillResult {
	// Without a context the check runs to completion, so the error is always nil.
	resolved, _ := plan.Run(context.Background(), check.Options{
		Parallel: true,
		OnRemoteDone: func(skills int) {
			if progressBar != nil {
				progressBar.Add(skills)
			}
		},
	})
	var results []checkSkillResult
	for _, r := range resolved {
		results = append(results, checkSkillResult{
			Name:        r.Name,
			Source:      r.Source,
			Version:     r.Version,
			Status:      r.Status,
			InstalledAt: r.InstalledAt,
			Message:     r.Message,
		})
	}
	return results
}

// parseCheckArgs parses command line arguments for the check command.
// Returns (opts, showHelp, error).
func parseCheckArgs(args []string) (*checkOptions, bool, error) {
	opts := &checkOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.json = true
		case arg == "--diff":
			opts.diff = true
		case arg == "--group" || arg == "-G":
			i++
			if i >= len(args) {
				return nil, false, fmt.Errorf("--group requires a value")
			}
			opts.groups = append(opts.groups, args[i])
		case arg == "--help" || arg == "-h":
			return nil, true, nil
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown option: %s", arg)
		default:
			opts.names = append(opts.names, arg)
		}
	}
	if opts.diff && opts.json {
		return nil, false, fmt.Errorf("--diff cannot be combined with --json")
	}
	if opts.diff && len(opts.names) == 0 && len(opts.groups) == 0 {
		return nil, false, fmt.Errorf("--diff needs a skill name or --group")
	}

	return opts, false, nil
}

// urlBranchSep separates URL and branch in composite grouping keys.
// Tab is used because it cannot appear in URLs (unlike "@" which appears in SSH URLs).
const urlBranchSep = "\t"

// urlBranchKey creates a composite grouping key from a URL and optional branch.
func urlBranchKey(url, branch string) string {
	if branch != "" {
		return url + urlBranchSep + branch
	}
	return url
}

// splitURLBranch splits a composite key back into URL and branch.
func splitURLBranch(key string) (url, branch string) {
	if idx := strings.Index(key, urlBranchSep); idx >= 0 {
		return key[:idx], key[idx+1:]
	}
	return key, ""
}

func cmdCheck(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args, "--group", "-G")
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	// Extract kind filter (e.g. "skillshare check agents" or "--all").
	kind, rest := parseKindArgWithAll(rest)

	scope := "global"
	if mode == modeProject {
		scope = "project"
	}

	opts, showHelp, parseErr := parseCheckArgs(rest)
	if showHelp {
		printCheckHelp()
		return nil
	}
	if parseErr != nil {
		return parseErr
	}

	if opts.diff && kind == kindAgents {
		return fmt.Errorf("--diff compares skills only")
	}

	cfgPath := config.ConfigPath()
	if mode == modeProject {
		cfgPath = config.ProjectConfigPath(cwd)
		if kind == kindAgents {
			projectCfg, loadErr := config.LoadProject(cwd)
			if loadErr != nil {
				err := fmt.Errorf("failed to load project config: %w", loadErr)
				logCheckOp(cfgPath, 0, 0, 0, 0, scope, start, err)
				return err
			}
			cmdErr := renderAgentCheck(projectCfg.EffectiveAgentsSource(cwd), opts.groups, opts.json)
			logCheckOp(cfgPath, 0, 0, 0, 0, scope, start, cmdErr)
			return cmdErr
		}
		cmdErr := cmdCheckProject(cwd, opts)
		logCheckOp(cfgPath, 0, 0, 0, 0, scope, start, cmdErr)
		return cmdErr
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Agent-only check: scan agents source directory and skip repo checks.
	if kind == kindAgents {
		agentsDir := cfg.EffectiveAgentsSource()
		cmdErr := renderAgentCheck(agentsDir, opts.groups, opts.json)
		logCheckOp(cfgPath, 0, 0, 0, 0, scope, start, cmdErr)
		return cmdErr
	}

	// No names and no groups → check all (existing behavior)
	if len(opts.names) == 0 && len(opts.groups) == 0 {
		cmdErr := runCheck(cfg.EffectiveSkillsSource(), "", opts.json, targetNamesFromConfig(cfg.Targets), cfg.SkillsWalk())
		logCheckOp(cfgPath, 0, 0, 0, 0, scope, start, cmdErr)
		return cmdErr
	}

	// Filtered check: resolve targets then check only those
	opts.parseOpts = parseOptsFromConfig(cfg)
	cmdErr := runCheckFiltered(cfg.EffectiveSkillsSource(), "", opts, cfg.SkillsWalk())
	logCheckOp(cfgPath, 0, 0, 0, 0, scope, start, cmdErr)
	return cmdErr
}

func logCheckOp(cfgPath string, repos, skills, updatesAvailable, errors int, scope string, start time.Time, cmdErr error) {
	e := oplog.NewEntry("check", statusFromErr(cmdErr), time.Since(start))
	e.Args = map[string]any{
		"repos_checked":     repos,
		"skills_checked":    skills,
		"updates_available": updatesAvailable,
		"errors":            errors,
		"scope":             scope,
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func runCheck(sourceDir, projectRoot string, jsonOutput bool, extraTargetNames []string, walk sourcewalk.Options) error {
	start := time.Now()

	var scanSpinner *ui.Spinner
	if !jsonOutput {
		scanSpinner = ui.StartSpinner("Scanning skills...")
	}

	repos, err := install.GetTrackedRepos(sourceDir, walk)
	if err != nil {
		repos = nil
	}
	missingRepos, err := install.GetMissingTrackedRepos(sourceDir, walk)
	if err != nil {
		missingRepos = nil
	}

	skills, err := install.GetUpdatableSkills(sourceDir)
	if err != nil {
		skills = nil
	}

	if len(repos) == 0 && len(missingRepos) == 0 && len(skills) == 0 {
		if scanSpinner != nil {
			scanSpinner.Stop()
		}
		if jsonOutput {
			out, _ := json.MarshalIndent(checkOutput{
				TrackedRepos: []checkRepoResult{},
				Skills:       []checkSkillResult{},
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		}
		ui.Done(ui.MarkNone, "No tracked repositories or updatable skills found", 0)
		ui.Next("skillshare install <repo> --track", "add a tracked repository")
		return nil
	}

	// Collect & group
	repoInputs, plan := collectCheckItems(sourceDir, projectRoot, repos, skills, walk.Follow)

	if scanSpinner != nil {
		scanSpinner.Stop()
	}

	// Count total items for meaningful progress (skill count, not URL count)
	total := len(repoInputs) + plan.RemoteSkills()

	// Parallel check with progress bar
	var progressBar *ui.ProgressBar
	if !jsonOutput && total > 0 {
		progressBar = ui.StartProgress("Checking for updates", total)
	}

	repoOnDone := func() {
		if progressBar != nil {
			progressBar.Increment()
		}
	}

	repoOutputs := check.ParallelCheckRepos(repoInputs, repoOnDone)
	skillResults := resolveCheckSkills(plan, progressBar)

	if progressBar != nil {
		progressBar.Stop()
	}

	// Convert repo outputs
	repoResults := toRepoResults(repoOutputs)
	for _, repo := range missingRepos {
		repoResults = append(repoResults, checkRepoResult{
			Name:    repo.Name,
			Status:  "missing",
			Message: missingTrackedRepoMessage(repo.Name),
		})
	}

	// JSON output
	if jsonOutput {
		output := checkOutput{
			TrackedRepos: repoResults,
			Skills:       skillResults,
		}
		if output.TrackedRepos == nil {
			output.TrackedRepos = []checkRepoResult{}
		}
		if output.Skills == nil {
			output.Skills = []checkSkillResult{}
		}
		return writeJSONResult(output, checkResultsError(repoResults, skillResults))
	}

	// Display results + summary, with unknown target names in skill-level
	// targets fields among the warnings
	warnings := unknownSkillTargetWarnings(sourceDir, extraTargetNames, walk)
	renderCheckResults(repoResults, skillResults, false, warnings, start)

	return checkResultsError(repoResults, skillResults)
}

// renderCheckResults lists the repos and skills that need attention, then
// closes with what was found. up_to_date items are counted, not listed;
// local skills get their own rows only when showDetails is true (filtered
// check).
func renderCheckResults(repoResults []checkRepoResult, skillResults []checkSkillResult, showDetails bool, warnings []string, start time.Time) {
	type checkRow struct{ mark, name, value string }
	var rows []checkRow
	upToDateRepos, upToDateSkills, updatableRepos, updatableSkills, local, stale, modified, failed := 0, 0, 0, 0, 0, 0, 0, 0

	for _, r := range repoResults {
		switch r.Status {
		case "up_to_date":
			upToDateRepos++
		case "behind":
			updatableRepos++
			rows = append(rows, checkRow{ui.MarkWarn, r.Name, plural(r.Behind, "commit") + " behind"})
		case "dirty":
			rows = append(rows, checkRow{ui.MarkWarn, r.Name, "uncommitted changes"})
		case "error":
			failed++
			rows = append(rows, checkRow{ui.MarkFail, r.Name, r.Message})
		case "missing":
			rows = append(rows, checkRow{ui.MarkWarn, r.Name, r.Message})
		}
	}
	for _, s := range skillResults {
		switch s.Status {
		case "up_to_date":
			upToDateSkills++
		case "local":
			local++
			if showDetails {
				rows = append(rows, checkRow{ui.MarkNone, s.Name, "local source"})
			}
		case "update_available":
			updatableSkills++
			rows = append(rows, checkRow{ui.MarkWarn, s.Name, updateAvailableText(s.Source)})
		case "stale":
			stale++
			rows = append(rows, checkRow{ui.MarkWarn, s.Name, "stale — no longer in the upstream repository"})
		case "modified":
			modified++
			rows = append(rows, checkRow{ui.MarkWarn, s.Name, s.Message})
		case "error":
			failed++
			rows = append(rows, checkRow{ui.MarkFail, s.Name, skillErrorMessage(s)})
		}
	}

	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.name
	}
	width := ui.RowWidth(names...)
	for _, r := range rows {
		ui.Row(r.mark, r.name, r.value, width)
	}
	for _, w := range warnings {
		ui.Warning("Skill targets: %s", w)
	}
	if len(rows)+len(warnings) > 0 {
		fmt.Println()
	}

	upToDate := upToDateRepos + upToDateSkills
	var mark, text string
	switch {
	case updatableRepos+updatableSkills > 0:
		mark, text = ui.MarkWarn, "Updates available for "+repoSkillCounts(updatableRepos, updatableSkills)
		if upToDate > 0 {
			text += fmt.Sprintf(", %d up to date", upToDate)
		}
	case upToDate > 0:
		mark, text = ui.MarkOK, repoSkillCounts(upToDateRepos, upToDateSkills)+" up to date"
	case modified > 0:
		mark, text = ui.MarkWarn, plural(modified, "skill")+" modified locally"
	case local > 0:
		mark, text = ui.MarkNone, plural(local, "local skill")+" skipped"
		local = 0
	default:
		mark, text = ui.MarkNone, "Nothing checked"
	}
	if local > 0 {
		text += fmt.Sprintf(", %d local skipped", local)
	}
	if stale > 0 {
		mark, text = ui.MarkWarn, fmt.Sprintf("%s, %d stale", text, stale)
	}
	if modified > 0 && updatableRepos+updatableSkills+upToDate > 0 {
		mark, text = ui.MarkWarn, fmt.Sprintf("%s, %d modified locally", text, modified)
	}
	if failed > 0 {
		mark, text = ui.MarkWarn, fmt.Sprintf("%s, %d failed to check", text, failed)
	}
	ui.Done(mark, text, time.Since(start))

	var next []string
	if updatableRepos+updatableSkills > 0 {
		next = append(next, "skillshare update --all", "pull the updates")
	}
	if stale > 0 {
		next = append(next, "skillshare update --all --prune", "remove stale skills")
	}
	if modified > 0 {
		next = append(next, "skillshare check <name> --diff", "see the local changes",
			"skillshare update <name> --force", "discard local changes and reinstall")
	}
	if len(next) > 0 {
		ui.Next(next...)
	}
}

// repoSkillCounts names how many repos and skills, leaving out zeros.
func repoSkillCounts(repos, skills int) string {
	var parts []string
	if repos > 0 {
		parts = append(parts, plural(repos, "repo"))
	}
	if skills > 0 {
		parts = append(parts, plural(skills, "skill"))
	}
	return strings.Join(parts, ", ")
}

// updateAvailableText says an update is available, with its source in dim.
func updateAvailableText(source string) string {
	if source == "" {
		return "update available"
	}
	return "update available " + ui.DimText("· "+formatSourceShort(sourceLabel(source)))
}

func toRepoResults(outputs []check.RepoCheckOutput) []checkRepoResult {
	results := make([]checkRepoResult, len(outputs))
	for i, o := range outputs {
		results[i] = checkRepoResult{
			Name:    o.Name,
			Status:  o.Status,
			Behind:  o.Behind,
			Branch:  o.Branch,
			Message: o.Message,
		}
	}
	return results
}

// runCheckFiltered checks only the specified targets (resolved from names/groups).
// Note: unlike runCheck, this intentionally skips warnUnknownSkillTargets because
// filtered checks only verify update status for explicitly named skills/groups.
func runCheckFiltered(sourceDir, projectRoot string, opts *checkOptions, walks ...sourcewalk.Options) error {
	start := time.Now()

	// --- Resolve targets ---
	var resolveSpinner *ui.Spinner
	if !opts.json {
		resolveSpinner = ui.StartSpinner("Resolving skills...")
	}

	checkStore, _ := install.LoadMetadata(sourceDir)
	var targets []updateTarget
	seen := map[string]bool{}
	var resolveWarnings []string

	for _, name := range opts.names {
		// Check group directory first (same logic as update)
		if isGroupDir(name, sourceDir, checkStore) {
			groupMatches, groupErr := resolveGroupUpdatable(name, sourceDir, walks...)
			if groupErr != nil {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: %v", name, groupErr))
				continue
			}
			if len(groupMatches) == 0 {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: no updatable skills in group", name))
				continue
			}
			ui.Note(fmt.Sprintf("'%s' is a group — expanding to %s", name, plural(len(groupMatches), "updatable skill")))
			for _, m := range groupMatches {
				if !seen[m.name] {
					seen[m.name] = true
					targets = append(targets, m)
				}
			}
			continue
		}

		match, err := resolveByBasename(sourceDir, name, walks...)
		if err != nil {
			resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if !seen[match.name] {
			seen[match.name] = true
			targets = append(targets, match)
		}
	}

	for _, group := range opts.groups {
		groupMatches, err := resolveGroupUpdatable(group, sourceDir, walks...)
		if err != nil {
			resolveWarnings = append(resolveWarnings, fmt.Sprintf("--group %s: %v", group, err))
			continue
		}
		if len(groupMatches) == 0 {
			resolveWarnings = append(resolveWarnings, fmt.Sprintf("--group %s: no updatable skills in group", group))
			continue
		}
		for _, m := range groupMatches {
			if !seen[m.name] {
				seen[m.name] = true
				targets = append(targets, m)
			}
		}
	}

	if resolveSpinner != nil {
		resolveSpinner.Stop()
	}

	for _, w := range resolveWarnings {
		ui.Warning("%s", w)
	}

	if len(targets) == 0 {
		if opts.json {
			out, _ := json.MarshalIndent(checkOutput{
				TrackedRepos: []checkRepoResult{},
				Skills:       []checkSkillResult{},
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		}
		if len(resolveWarnings) > 0 {
			return fmt.Errorf("no valid skills to check")
		}
		return fmt.Errorf("no skills found")
	}

	var follow *sourcewalk.Follow
	if len(walks) > 0 {
		follow = walks[0].Follow
	}
	if opts.diff {
		return runCheckDiff(sourceDir, targets, opts.parseOpts, follow)
	}

	// --- Partition targets for parallel check ---
	var repoNames []string
	var skillNames []string
	for _, t := range targets {
		if t.isRepo {
			repoNames = append(repoNames, t.name)
		} else {
			skillNames = append(skillNames, t.name)
		}
	}

	repoInputs, plan := collectCheckItems(sourceDir, projectRoot, repoNames, skillNames, follow)

	total := len(repoInputs) + plan.RemoteSkills()
	isSingle := len(targets) == 1

	// Single target: spinner; multiple targets: progress bar
	var progressBar *ui.ProgressBar
	var spinner *ui.Spinner
	if !opts.json && total > 0 {
		if isSingle {
			spinner = ui.StartSpinner("Checking...")
		} else {
			progressBar = ui.StartProgress("Checking targets", total)
		}
	}

	repoOnDone := func() {
		if progressBar != nil {
			progressBar.Increment()
		}
	}

	repoOutputs := check.ParallelCheckRepos(repoInputs, repoOnDone)
	skillResults := resolveCheckSkills(plan, progressBar)

	if progressBar != nil {
		progressBar.Stop()
	}
	if spinner != nil {
		spinner.Stop()
	}

	repoResults := toRepoResults(repoOutputs)

	if opts.json {
		output := checkOutput{
			TrackedRepos: repoResults,
			Skills:       skillResults,
		}
		if output.TrackedRepos == nil {
			output.TrackedRepos = []checkRepoResult{}
		}
		if output.Skills == nil {
			output.Skills = []checkSkillResult{}
		}
		return writeJSONResult(output, checkResultsError(repoResults, skillResults))
	}

	// Single target: one row, like update
	if isSingle {
		r := singleCheckStatus(repoResults, skillResults)
		ui.Row(r.mark, targets[0].name, r.text+ui.Took(time.Since(start)), ui.RowWidth(targets[0].name))
		if r.next != "" {
			ui.Next(r.next, r.why)
		}
		return checkResultsError(repoResults, skillResults)
	}

	// Display results + summary (filtered: show local skills individually)
	renderCheckResults(repoResults, skillResults, true, nil, start)

	return checkResultsError(repoResults, skillResults)
}

func checkResultsError(repos []checkRepoResult, skills []checkSkillResult) error {
	failed := 0
	for _, r := range repos {
		if r.Status == "error" {
			failed++
		}
	}
	for _, s := range skills {
		if s.Status == "error" {
			failed++
		}
	}
	return checkFailureError(failed)
}

func checkFailureError(failed int) error {
	if failed > 0 {
		return fmt.Errorf("%s failed to check", plural(failed, "item"))
	}
	return nil
}

// skillErrorMessage returns the reason for an "error" skill result, falling
// back to the remote-probe failure that most errors come from.
func skillErrorMessage(s checkSkillResult) string {
	if s.Message != "" {
		return s.Message
	}
	return "cannot reach remote"
}

// singleCheckResult is the row for a single-target check and the command
// that acts on it, if any.
type singleCheckResult struct {
	mark, text string
	next, why  string
}

// singleCheckStatus derives the row for a single-target check.
func singleCheckStatus(repos []checkRepoResult, skills []checkSkillResult) singleCheckResult {
	// Repo results
	for _, r := range repos {
		switch r.Status {
		case "up_to_date":
			return singleCheckResult{mark: ui.MarkOK, text: "up to date"}
		case "behind":
			return singleCheckResult{ui.MarkWarn, plural(r.Behind, "commit") + " behind", "skillshare update " + r.Name, "pull them"}
		case "dirty":
			return singleCheckResult{mark: ui.MarkWarn, text: "uncommitted changes"}
		case "error":
			return singleCheckResult{mark: ui.MarkFail, text: r.Message}
		default:
			return singleCheckResult{mark: ui.MarkWarn, text: r.Message}
		}
	}
	// Skill results
	for _, s := range skills {
		switch s.Status {
		case "up_to_date":
			return singleCheckResult{mark: ui.MarkOK, text: "up to date"}
		case "update_available":
			return singleCheckResult{ui.MarkWarn, updateAvailableText(s.Source), "skillshare update " + s.Name, "pull the update"}
		case "stale":
			return singleCheckResult{ui.MarkWarn, "stale — no longer in the upstream repository", "skillshare update --all --prune", "remove stale skills"}
		case "modified":
			return singleCheckResult{ui.MarkWarn, s.Message, "skillshare check " + s.Name + " --diff", "see the changes; update --force discards them"}
		case "local":
			return singleCheckResult{mark: ui.MarkNone, text: "local source, nothing to compare"}
		case "error":
			return singleCheckResult{mark: ui.MarkFail, text: skillErrorMessage(s)}
		default:
			return singleCheckResult{mark: ui.MarkNone, text: s.Status}
		}
	}
	return singleCheckResult{mark: ui.MarkOK, text: "up to date"}
}

// unknownSkillTargetWarnings names skill-level targets that match no
// configured or known target.
func unknownSkillTargetWarnings(sourceDir string, extraTargetNames []string, walk sourcewalk.Options) []string {
	sp := ui.StartSpinner("Validating skill targets...")
	defer sp.Stop()
	discovered, err := ssync.DiscoverSourceSkills(sourceDir, walk)
	if err != nil {
		return nil
	}
	return findUnknownSkillTargets(discovered, extraTargetNames)
}

// formatSourceShort returns a shortened source for display
func formatSourceShort(source string) string {
	// Remove common prefixes for shorter display
	source = strings.TrimPrefix(source, "https://")
	source = strings.TrimPrefix(source, "http://")
	source = strings.TrimSuffix(source, ".git")
	return source
}

// renderAgentCheck runs CheckAgents and displays results (text or JSON).
// If groups is non-empty, only agents in those group subdirectories are shown.
func renderAgentCheck(agentsDir string, groups []string, jsonMode bool) error {
	agentResults := check.CheckAgents(agentsDir)

	if len(groups) > 0 {
		filtered, err := filterAgentResultsByGroups(agentResults, groups, agentsDir)
		if err != nil {
			if jsonMode {
				return writeJSONError(err)
			}
			return err
		}
		agentResults = filtered
	}

	trackedIndices := make([]int, 0, len(agentResults))
	tracked := make([]check.AgentCheckResult, 0, len(agentResults))
	for i := range agentResults {
		if agentResults[i].Source != "" {
			trackedIndices = append(trackedIndices, i)
			tracked = append(tracked, agentResults[i])
		}
	}
	if len(tracked) > 0 {
		if jsonMode {
			check.EnrichAgentResultsWithRemote(tracked, nil)
		} else {
			sp := ui.StartSpinner(fmt.Sprintf("Checking %s...", plural(len(tracked), "tracked agent")))
			check.EnrichAgentResultsWithRemote(tracked, nil)
			sp.Stop()
		}
		for i, idx := range trackedIndices {
			agentResults[idx] = tracked[i]
		}
	}

	failed := 0
	for _, r := range agentResults {
		if r.Status == "error" {
			failed++
		}
	}
	if jsonMode {
		return writeJSONResult(agentResults, checkFailureError(failed))
	}
	if len(agentResults) == 0 {
		ui.Done(ui.MarkNone, "No agents found", 0)
		return nil
	}
	names := make([]string, len(agentResults))
	for i, r := range agentResults {
		names[i] = r.Name
	}
	width := ui.RowWidth(names...)
	upToDate, updates, attention := 0, 0, 0
	for _, r := range agentResults {
		switch r.Status {
		case "up_to_date":
			upToDate++
			ui.Row(ui.MarkOK, r.Name, "up to date", width)
		case "update_available":
			updates++
			ui.Row(ui.MarkWarn, r.Name, updateAvailableText(r.Source), width)
		case "drifted", "dirty":
			attention++
			ui.Row(ui.MarkWarn, r.Name, r.Message, width)
		case "local":
			ui.Row(ui.MarkNone, r.Name, "local agent", width)
		case "error":
			attention++
			ui.Row(ui.MarkFail, r.Name, r.Message, width)
		}
	}
	fmt.Println()
	switch {
	case updates > 0:
		ui.Done(ui.MarkWarn, "Updates available for "+plural(updates, "agent"), 0)
		ui.Next("skillshare update agents --all", "pull the updates")
	case attention > 0:
		ui.Done(ui.MarkWarn, plural(attention, "agent")+" need attention", 0)
	case upToDate > 0:
		ui.Done(ui.MarkOK, plural(upToDate, "agent")+" up to date", 0)
	default:
		ui.Done(ui.MarkNone, "No tracked agents to compare", 0)
	}
	return checkFailureError(failed)
}

func printCheckHelp() {
	printHelp("skillshare check [agents] [name...] [options]\n       skillshare check --group <group> [options]", "Check for available updates to tracked repositories and installed skills.\n\nFor tracked repos: fetches from origin and checks if behind\nFor regular skills: compares installed version with remote HEAD\n\nIf no names or groups are specified, all items are checked.\nIf a positional name matches a group directory, it is automatically expanded.",
		helpGroup{title: "Arguments", rows: []helpRow{
			{"name...", "Skill name(s) or tracked repo name(s) (optional)"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"--all", "Check both skills and agents"},
			{"-G, --group <name>", "Check all updatable skills in a group (repeatable)"},
			{"-p, --project", "Check project-level skills (.skillshare/)"},
			{"-g, --global", "Check global skills (~/.config/skillshare)"},
			{"--json", "Output results as JSON"},
			{"--diff", "Show what changed in named skills since install"},
		}},
		helpExamples(
			helpRow{"skillshare check", "Check all items"},
			helpRow{"skillshare check my-skill", "Check a single skill"},
			helpRow{"skillshare check a b c", "Check multiple skills"},
			helpRow{"skillshare check --group frontend", "Check all skills in frontend/"},
			helpRow{"skillshare check x -G backend", "Mix names and groups"},
			helpRow{"skillshare check --json", "Output as JSON (for CI)"},
			helpRow{"skillshare check my-skill --diff", "Show local edits to a skill"},
			helpRow{"skillshare check -p", "Check project skills"},
			helpRow{"skillshare check agents", "Check all agents"},
			helpRow{"skillshare check agents -G demo", "Check agents in demo/"},
			helpRow{"skillshare check --all", "Check skills + agents"},
		),
	)
}
