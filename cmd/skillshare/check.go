package main

import (
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
	Status      string `json:"status"` // "up_to_date", "update_available", "local", "error"
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
}

// skillWithMeta holds a regular skill plus its parsed metadata for grouping.
type skillWithMeta struct {
	name string
	path string
	meta *install.MetadataEntry
}

// collectCheckItems reads metadata and partitions items for parallel checking.
// Returns: tracked repo inputs, URL-grouped skills, local skill results (no network needed).
// projectRoot is the base of relative local sources; "" in global mode.
func collectCheckItems(sourceDir, projectRoot string, repos []string, skills []string) (
	[]check.RepoCheckInput,
	map[string][]skillWithMeta,
	[]checkSkillResult,
) {
	var repoInputs []check.RepoCheckInput
	for _, repo := range repos {
		repoInputs = append(repoInputs, check.RepoCheckInput{
			Name:     repo,
			RepoPath: filepath.Join(sourceDir, repo),
		})
	}

	// Load centralized metadata store once for all skills.
	store := install.LoadMetadataOrNew(sourceDir)

	urlGroups := make(map[string][]skillWithMeta)
	var localResults []checkSkillResult

	for _, skill := range skills {
		skillPath := filepath.Join(sourceDir, skill)
		entry := store.GetByPath(skill)

		if entry == nil || entry.RepoURL == "" {
			result := checkSkillResult{Name: skill}
			result.Status, result.Message = check.LocalSourceStatus(entry, projectRoot)
			if entry != nil {
				result.Source = entry.Source
				result.Version = entry.Version
				if !entry.InstalledAt.IsZero() {
					result.InstalledAt = entry.InstalledAt.Format("2006-01-02")
				}
			}
			localResults = append(localResults, result)
			continue
		}

		groupKey := urlBranchKey(entry.RepoURL, entry.Branch)
		urlGroups[groupKey] = append(urlGroups[groupKey], skillWithMeta{
			name: skill,
			path: skillPath,
			meta: entry,
		})
	}

	return repoInputs, urlGroups, localResults
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
			renderAgentCheck(projectCfg.EffectiveAgentsSource(cwd), opts.groups, opts.json)
			logCheckOp(cfgPath, 0, 0, 0, 0, scope, start, nil)
			return nil
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
		renderAgentCheck(agentsDir, opts.groups, opts.json)
		logCheckOp(cfgPath, 0, 0, 0, 0, scope, start, nil)
		return nil
	}

	// No names and no groups → check all (existing behavior)
	if len(opts.names) == 0 && len(opts.groups) == 0 {
		cmdErr := runCheck(cfg.EffectiveSkillsSource(), "", opts.json, targetNamesFromConfig(cfg.Targets))
		logCheckOp(cfgPath, 0, 0, 0, 0, scope, start, cmdErr)
		return cmdErr
	}

	// Filtered check: resolve targets then check only those
	cmdErr := runCheckFiltered(cfg.EffectiveSkillsSource(), "", opts)
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

func runCheck(sourceDir, projectRoot string, jsonOutput bool, extraTargetNames []string) error {
	start := time.Now()

	var scanSpinner *ui.Spinner
	if !jsonOutput {
		scanSpinner = ui.StartSpinner("Scanning skills...")
	}

	repos, err := install.GetTrackedRepos(sourceDir)
	if err != nil {
		repos = nil
	}
	missingRepos, err := install.GetMissingTrackedRepos(sourceDir)
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
	repoInputs, urlGroups, localResults := collectCheckItems(sourceDir, projectRoot, repos, skills)

	if scanSpinner != nil {
		scanSpinner.Stop()
	}

	// Build unique URL list
	var urlInputs []check.URLCheckInput
	var urlOrder []string
	for key := range urlGroups {
		url, branch := splitURLBranch(key)
		urlInputs = append(urlInputs, check.URLCheckInput{RepoURL: url, Branch: branch})
		urlOrder = append(urlOrder, key)
	}

	// Count total items for meaningful progress (skill count, not URL count)
	totalSkills := 0
	for _, group := range urlGroups {
		totalSkills += len(group)
	}
	total := len(repoInputs) + totalSkills

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
	urlOutputs := check.ParallelCheckURLs(urlInputs, nil)

	// Increment by skill count per completed URL group
	if progressBar != nil {
		progressBar.Add(totalSkills)
	}

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

	// Broadcast URL results to grouped skills (with per-skill tree hash comparison)
	urlHashMap := make(map[string]check.URLCheckOutput)
	for _, out := range urlOutputs {
		urlHashMap[urlBranchKey(out.RepoURL, out.Branch)] = out
	}

	skillResults := resolveSkillStatuses(urlGroups, urlHashMap, urlOrder)
	skillResults = append(localResults, skillResults...)

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
		out, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	// Display results + summary, with unknown target names in skill-level
	// targets fields among the warnings
	warnings := unknownSkillTargetWarnings(sourceDir, extraTargetNames)
	renderCheckResults(repoResults, skillResults, false, warnings, start)

	return nil
}

// renderCheckResults lists the repos and skills that need attention, then
// closes with what was found. up_to_date items are counted, not listed;
// local skills get their own rows only when showDetails is true (filtered
// check).
func renderCheckResults(repoResults []checkRepoResult, skillResults []checkSkillResult, showDetails bool, warnings []string, start time.Time) {
	type checkRow struct{ mark, name, value string }
	var rows []checkRow
	upToDateRepos, upToDateSkills, updatableRepos, updatableSkills, local, stale, failed := 0, 0, 0, 0, 0, 0, 0

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

// resolveSkillStatuses determines the status of each skill using tree hash
// comparison when available, falling back to commit hash comparison.
//
// Fast path: if all skills in a URL group have Version == RemoteHash,
// they are all up_to_date without any additional network call.
//
// Slow path: when HEAD moved, skills with TreeHash are compared via
// blobless fetch + ls-tree. Skills without TreeHash fall back to
// commit-level comparison (existing behavior).
func resolveSkillStatuses(
	urlGroups map[string][]skillWithMeta,
	urlHashMap map[string]check.URLCheckOutput,
	urlOrder []string,
) []checkSkillResult {
	var results []checkSkillResult

	for _, url := range urlOrder {
		out := urlHashMap[url]
		group := urlGroups[url]

		// Pre-fill base result fields for each skill
		type pending struct {
			result checkSkillResult
			meta   *install.MetadataEntry
		}
		items := make([]pending, len(group))
		for i, sw := range group {
			r := checkSkillResult{
				Name:    sw.name,
				Source:  sw.meta.Source,
				Version: sw.meta.Version,
			}
			if !sw.meta.InstalledAt.IsZero() {
				r.InstalledAt = sw.meta.InstalledAt.Format("2006-01-02")
			}
			items[i] = pending{result: r, meta: sw.meta}
		}

		// Error from ls-remote → all error
		if out.Err != nil {
			for i := range items {
				items[i].result.Status = "error"
			}
			for _, it := range items {
				results = append(results, it.result)
			}
			continue
		}

		// Fast path: commit hash matches → all up_to_date
		allMatch := true
		for _, it := range items {
			if it.meta.Version != out.RemoteHash {
				allMatch = false
				break
			}
		}
		if allMatch {
			for i := range items {
				items[i].result.Status = "up_to_date"
			}
			for _, it := range items {
				results = append(results, it.result)
			}
			continue
		}

		// Slow path: HEAD moved — collect subdirs that have TreeHash
		var subdirs []string
		for _, it := range items {
			if it.meta.TreeHash != "" && it.meta.Subdir != "" {
				subdirs = append(subdirs, it.meta.Subdir)
			}
		}

		// Fetch remote tree hashes once per URL+branch (nil on error → fallback)
		var remoteTreeHashes map[string]string
		if len(subdirs) > 0 {
			repoURL, branch := splitURLBranch(url)
			remoteTreeHashes = check.FetchRemoteTreeHashesForRef(repoURL, branch)
		}

		for i, it := range items {
			// Already matched by commit hash → up_to_date
			if it.meta.Version == out.RemoteHash {
				items[i].result.Status = "up_to_date"
				continue
			}

			// Try tree hash comparison
			if it.meta.TreeHash != "" && it.meta.Subdir != "" && remoteTreeHashes != nil {
				// ls-tree paths never have leading "/", but meta.Subdir may
				normalizedSubdir := strings.TrimPrefix(it.meta.Subdir, "/")
				if remoteHash, ok := remoteTreeHashes[normalizedSubdir]; ok {
					if it.meta.TreeHash == remoteHash {
						items[i].result.Status = "up_to_date"
					} else {
						items[i].result.Status = "update_available"
					}
				} else {
					// Subdir not found remotely — skill deleted upstream
					items[i].result.Status = "stale"
				}
				continue
			}

			// Fallback: commit hash differs, no tree hash → update_available
			items[i].result.Status = "update_available"
		}

		for _, it := range items {
			results = append(results, it.result)
		}
	}

	return results
}

// runCheckFiltered checks only the specified targets (resolved from names/groups).
// Note: unlike runCheck, this intentionally skips warnUnknownSkillTargets because
// filtered checks only verify update status for explicitly named skills/groups.
func runCheckFiltered(sourceDir, projectRoot string, opts *checkOptions) error {
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
			groupMatches, groupErr := resolveGroupUpdatable(name, sourceDir)
			if groupErr != nil {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: %v", name, groupErr))
				continue
			}
			if len(groupMatches) == 0 {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: no updatable skills in group", name))
				continue
			}
			ui.Info("'%s' is a group — expanding to %d updatable skill(s)", name, len(groupMatches))
			for _, m := range groupMatches {
				if !seen[m.name] {
					seen[m.name] = true
					targets = append(targets, m)
				}
			}
			continue
		}

		match, err := resolveByBasename(sourceDir, name)
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
		groupMatches, err := resolveGroupUpdatable(group, sourceDir)
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

	repoInputs, urlGroups, localResults := collectCheckItems(sourceDir, projectRoot, repoNames, skillNames)

	var urlInputs []check.URLCheckInput
	var urlOrder []string
	for key := range urlGroups {
		url, branch := splitURLBranch(key)
		urlInputs = append(urlInputs, check.URLCheckInput{RepoURL: url, Branch: branch})
		urlOrder = append(urlOrder, key)
	}

	totalSkills := 0
	for _, group := range urlGroups {
		totalSkills += len(group)
	}
	total := len(repoInputs) + totalSkills
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
	urlOutputs := check.ParallelCheckURLs(urlInputs, nil)

	if progressBar != nil {
		progressBar.Add(totalSkills)
		progressBar.Stop()
	}
	if spinner != nil {
		spinner.Stop()
	}

	repoResults := toRepoResults(repoOutputs)

	urlHashMap := make(map[string]check.URLCheckOutput)
	for _, out := range urlOutputs {
		urlHashMap[urlBranchKey(out.RepoURL, out.Branch)] = out
	}

	skillResults := resolveSkillStatuses(urlGroups, urlHashMap, urlOrder)
	skillResults = append(localResults, skillResults...)

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
		out, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	// Single target: one row, like update
	if isSingle {
		r := singleCheckStatus(repoResults, skillResults)
		ui.Row(r.mark, targets[0].name, r.text+ui.Took(time.Since(start)), ui.RowWidth(targets[0].name))
		if r.next != "" {
			ui.Next(r.next, r.why)
		}
		return nil
	}

	// Display results + summary (filtered: show local skills individually)
	renderCheckResults(repoResults, skillResults, true, nil, start)

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
func unknownSkillTargetWarnings(sourceDir string, extraTargetNames []string) []string {
	sp := ui.StartSpinner("Validating skill targets...")
	defer sp.Stop()
	discovered, err := ssync.DiscoverSourceSkills(sourceDir)
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
func renderAgentCheck(agentsDir string, groups []string, jsonMode bool) {
	agentResults := check.CheckAgents(agentsDir)

	if len(groups) > 0 {
		filtered, err := filterAgentResultsByGroups(agentResults, groups, agentsDir)
		if err != nil {
			if jsonMode {
				writeJSONError(err) //nolint:errcheck
				return
			}
			ui.Error("%v", err)
			return
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

	if jsonMode {
		out, _ := json.MarshalIndent(agentResults, "", "  ")
		fmt.Println(string(out))
		return
	}
	if len(agentResults) == 0 {
		ui.Done(ui.MarkNone, "No agents found", 0)
		return
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
}

func printCheckHelp() {
	fmt.Println(`Usage: skillshare check [agents] [name...] [options]
       skillshare check --group <group> [options]

Check for available updates to tracked repositories and installed skills.

For tracked repos: fetches from origin and checks if behind
For regular skills: compares installed version with remote HEAD

If no names or groups are specified, all items are checked.
If a positional name matches a group directory, it is automatically expanded.

Arguments:
  name...                Skill name(s) or tracked repo name(s) (optional)

Options:
  --all              Check both skills and agents
  --group, -G <name> Check all updatable skills in a group (repeatable)
  --project, -p      Check project-level skills (.skillshare/)
  --global, -g       Check global skills (~/.config/skillshare)
  --json             Output results as JSON
  --help, -h         Show this help

Examples:
  skillshare check                     # Check all items
  skillshare check my-skill            # Check a single skill
  skillshare check a b c               # Check multiple skills
  skillshare check --group frontend    # Check all skills in frontend/
  skillshare check x -G backend        # Mix names and groups
  skillshare check --json              # Output as JSON (for CI)
  skillshare check -p                  # Check project skills
  skillshare check agents              # Check all agents
  skillshare check agents -G demo      # Check agents in demo/
  skillshare check --all               # Check skills + agents`)
}
