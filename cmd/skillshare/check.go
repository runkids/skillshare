package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/check"
	"skillshare/internal/config"
	"skillshare/internal/install"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
)

// checkRepoResult holds the check result for a tracked repo
type checkRepoResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "up_to_date", "behind", "dirty", "error"
	Behind  int    `json:"behind"`
	Message string `json:"message,omitempty"`
}

// checkSkillResult holds the check result for a regular skill
type checkSkillResult struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Version     string `json:"version"`
	Status      string `json:"status"` // "up_to_date", "update_available", "local", "error"
	InstalledAt string `json:"installed_at,omitempty"`
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
	meta *install.SkillMeta
}

// collectCheckItems reads metadata and partitions items for parallel checking.
// Returns: tracked repo inputs, URL-grouped skills, local skill results (no network needed).
func collectCheckItems(sourceDir string, repos []string, skills []string) (
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

	urlGroups := make(map[string][]skillWithMeta)
	var localResults []checkSkillResult

	for _, skill := range skills {
		skillPath := filepath.Join(sourceDir, skill)
		meta, err := install.ReadMeta(skillPath)

		if err != nil || meta == nil || meta.RepoURL == "" {
			result := checkSkillResult{Name: skill, Status: "local"}
			if meta != nil {
				result.Source = meta.Source
				result.Version = meta.Version
				if !meta.InstalledAt.IsZero() {
					result.InstalledAt = meta.InstalledAt.Format("2006-01-02")
				}
			}
			localResults = append(localResults, result)
			continue
		}

		urlGroups[meta.RepoURL] = append(urlGroups[meta.RepoURL], skillWithMeta{
			name: skill,
			path: skillPath,
			meta: meta,
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

func cmdCheck(args []string) error {
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

	opts, showHelp, parseErr := parseCheckArgs(rest)
	if showHelp {
		printCheckHelp()
		return nil
	}
	if parseErr != nil {
		return parseErr
	}

	if mode == modeProject {
		return cmdCheckProject(cwd, opts)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// No names and no groups → check all (existing behavior)
	if len(opts.names) == 0 && len(opts.groups) == 0 {
		return runCheck(cfg.Source, opts.json)
	}

	// Filtered check: resolve targets then check only those
	return runCheckFiltered(cfg.Source, opts)
}

func runCheck(sourceDir string, jsonOutput bool) error {
	repos, err := install.GetTrackedRepos(sourceDir)
	if err != nil {
		repos = nil
	}

	skills, err := install.GetUpdatableSkills(sourceDir)
	if err != nil {
		skills = nil
	}

	if len(repos) == 0 && len(skills) == 0 {
		if jsonOutput {
			out, _ := json.MarshalIndent(checkOutput{
				TrackedRepos: []checkRepoResult{},
				Skills:       []checkSkillResult{},
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		}
		ui.Header(ui.WithModeLabel("Checking for updates"))
		ui.Info("No tracked repositories or updatable skills found")
		ui.Info("Use 'skillshare install <repo> --track' to add a tracked repository")
		return nil
	}

	// Collect & group
	repoInputs, urlGroups, localResults := collectCheckItems(sourceDir, repos, skills)

	// Build unique URL list
	var urlInputs []check.URLCheckInput
	var urlOrder []string
	for url := range urlGroups {
		urlInputs = append(urlInputs, check.URLCheckInput{RepoURL: url})
		urlOrder = append(urlOrder, url)
	}

	// Count total items for meaningful progress (skill count, not URL count)
	totalSkills := 0
	for _, group := range urlGroups {
		totalSkills += len(group)
	}
	total := len(repoInputs) + totalSkills

	if !jsonOutput {
		ui.Header(ui.WithModeLabel("Checking for updates"))
		ui.StepStart("Source", sourceDir)
		ui.StepContinue("Items", fmt.Sprintf("%d tracked repo(s), %d skill(s)", len(repos), len(skills)))
	}

	// Parallel check with progress bar
	var progressBar *ui.ProgressBar
	if !jsonOutput && total > 0 {
		fmt.Println()
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

	// Broadcast URL results to grouped skills (with per-skill tree hash comparison)
	urlHashMap := make(map[string]check.URLCheckOutput)
	for _, out := range urlOutputs {
		urlHashMap[out.RepoURL] = out
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

	// Display results + summary
	renderCheckResults(repoResults, skillResults, false)

	// Warn about unknown target names in skill-level targets field
	warnUnknownSkillTargets(sourceDir)

	return nil
}

// renderCheckResults displays repos and skills that need attention,
// suppresses up_to_date/local items, and prints a summary line.
// When showDetails is false (full check), update_available items are summarized.
// When showDetails is true (filtered check), each item is listed individually.
func renderCheckResults(repoResults []checkRepoResult, skillResults []checkSkillResult, showDetails bool) {
	// Repos: only show behind/dirty/error
	upToDateRepos := 0
	hasRepoOutput := false
	for _, r := range repoResults {
		switch r.Status {
		case "up_to_date":
			upToDateRepos++
		case "behind":
			if !hasRepoOutput {
				fmt.Println()
				hasRepoOutput = true
			}
			ui.ListItem("info", r.Name, fmt.Sprintf("%d commit(s) behind", r.Behind))
		case "dirty":
			if !hasRepoOutput {
				fmt.Println()
				hasRepoOutput = true
			}
			ui.ListItem("warning", r.Name, "has uncommitted changes")
		case "error":
			if !hasRepoOutput {
				fmt.Println()
				hasRepoOutput = true
			}
			ui.ListItem("error", r.Name, fmt.Sprintf("error: %s", r.Message))
		}
	}

	// Skills: only show update_available/error; suppress up_to_date and local
	upToDateSkills := 0
	localSkills := 0
	hasSkillOutput := false
	for _, s := range skillResults {
		switch s.Status {
		case "up_to_date":
			upToDateSkills++
		case "local":
			localSkills++
			if showDetails {
				if !hasSkillOutput {
					fmt.Println()
					hasSkillOutput = true
				}
				ui.ListItem("info", s.Name, "local source")
			}
		case "update_available":
			if showDetails {
				if !hasSkillOutput {
					fmt.Println()
					hasSkillOutput = true
				}
				detail := "update available"
				if s.Source != "" {
					detail += fmt.Sprintf("  %s", formatSourceShort(s.Source))
				}
				ui.ListItem("info", s.Name, detail)
			}
		case "error":
			if !hasSkillOutput {
				fmt.Println()
				hasSkillOutput = true
			}
			ui.ListItem("warning", s.Name, "cannot reach remote")
		}
	}

	// Summary
	updatableRepos := 0
	for _, r := range repoResults {
		if r.Status == "behind" {
			updatableRepos++
		}
	}
	updatableSkills := 0
	for _, s := range skillResults {
		if s.Status == "update_available" {
			updatableSkills++
		}
	}

	fmt.Println()
	upToDateTotal := upToDateRepos + upToDateSkills
	if upToDateTotal > 0 {
		parts := []string{}
		if upToDateRepos > 0 {
			parts = append(parts, fmt.Sprintf("%d repo(s)", upToDateRepos))
		}
		if upToDateSkills > 0 {
			parts = append(parts, fmt.Sprintf("%d skill(s)", upToDateSkills))
		}
		ui.SuccessMsg("%s up to date", strings.Join(parts, " + "))
	}
	if localSkills > 0 {
		ui.Info("%d local skill(s) skipped", localSkills)
	}
	if updatableRepos+updatableSkills == 0 {
		if upToDateTotal == 0 && localSkills == 0 {
			ui.SuccessMsg("Everything is up to date")
		}
	} else {
		parts := []string{}
		if updatableRepos > 0 {
			parts = append(parts, fmt.Sprintf("%d repo(s)", updatableRepos))
		}
		if updatableSkills > 0 {
			parts = append(parts, fmt.Sprintf("%d skill(s)", updatableSkills))
		}
		ui.Info("%s have updates available", strings.Join(parts, " + "))
		ui.Info("Run 'skillshare update <name>' or 'skillshare update --all'")
	}
}

func toRepoResults(outputs []check.RepoCheckOutput) []checkRepoResult {
	results := make([]checkRepoResult, len(outputs))
	for i, o := range outputs {
		results[i] = checkRepoResult{
			Name:    o.Name,
			Status:  o.Status,
			Behind:  o.Behind,
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
			meta   *install.SkillMeta
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

		// Fetch remote tree hashes once per URL (nil on error → fallback)
		var remoteTreeHashes map[string]string
		if len(subdirs) > 0 {
			remoteTreeHashes = check.FetchRemoteTreeHashes(url)
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
					// Subdir not found remotely (deleted?)
					items[i].result.Status = "update_available"
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
func runCheckFiltered(sourceDir string, opts *checkOptions) error {
	// --- Resolve targets ---
	var targets []updateTarget
	seen := map[string]bool{}
	var resolveWarnings []string

	for _, name := range opts.names {
		// Check group directory first (same logic as update)
		if isGroupDir(name, sourceDir) {
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

	repoInputs, urlGroups, localResults := collectCheckItems(sourceDir, repoNames, skillNames)

	var urlInputs []check.URLCheckInput
	var urlOrder []string
	for url := range urlGroups {
		urlInputs = append(urlInputs, check.URLCheckInput{RepoURL: url})
		urlOrder = append(urlOrder, url)
	}

	totalSkills := 0
	for _, group := range urlGroups {
		totalSkills += len(group)
	}
	total := len(repoInputs) + totalSkills

	var progressBar *ui.ProgressBar
	if !opts.json && total > 0 {
		fmt.Println()
		progressBar = ui.StartProgress("Checking targets", total)
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
	}

	if progressBar != nil {
		progressBar.Stop()
	}

	repoResults := toRepoResults(repoOutputs)

	urlHashMap := make(map[string]check.URLCheckOutput)
	for _, out := range urlOutputs {
		urlHashMap[out.RepoURL] = out
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

	// Display results + summary (filtered: show local skills individually)
	renderCheckResults(repoResults, skillResults, true)

	return nil
}

func warnUnknownSkillTargets(sourceDir string) {
	discovered, err := ssync.DiscoverSourceSkills(sourceDir)
	if err != nil {
		return
	}

	warnings := findUnknownSkillTargets(discovered)
	if len(warnings) > 0 {
		fmt.Println()
		for _, w := range warnings {
			ui.Warning("Skill targets: %s", w)
		}
	}
}

// formatSourceShort returns a shortened source for display
func formatSourceShort(source string) string {
	// Remove common prefixes for shorter display
	source = strings.TrimPrefix(source, "https://")
	source = strings.TrimPrefix(source, "http://")
	source = strings.TrimSuffix(source, ".git")
	return source
}

func printCheckHelp() {
	fmt.Println(`Usage: skillshare check [name...] [options]
       skillshare check --group <group> [options]

Check for available updates to tracked repositories and installed skills.

For tracked repos: fetches from origin and checks if behind
For regular skills: compares installed version with remote HEAD

If no names or groups are specified, all items are checked.
If a positional name matches a group directory, it is automatically expanded.

Arguments:
  name...                Skill name(s) or tracked repo name(s) (optional)

Options:
  --group, -G <name>  Check all updatable skills in a group (repeatable)
  --project, -p       Check project-level skills (.skillshare/)
  --global, -g        Check global skills (~/.config/skillshare)
  --json              Output results as JSON
  --help, -h          Show this help

Examples:
  skillshare check                     # Check all items
  skillshare check my-skill            # Check a single skill
  skillshare check a b c               # Check multiple skills
  skillshare check --group frontend    # Check all skills in frontend/
  skillshare check x -G backend        # Mix names and groups
  skillshare check --json              # Output as JSON (for CI)
  skillshare check -p                  # Check project skills`)
}
