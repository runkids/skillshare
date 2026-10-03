package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/search"
	"skillshare/internal/ui"
)

func cmdSearch(args []string) error {
	// Parse mode flags (--project/-p, --global/-g) first
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	// Auto-detect: if mode is auto and project config exists, use project mode
	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	const defaultHubURL = "https://raw.githubusercontent.com/runkids/skillshare-hub/main/skillshare-hub.json"

	var query string
	var jsonOutput bool
	var listOnly bool
	var hubInput string // raw --hub value
	var hubBare bool    // --hub with no value
	var limit int = 20
	var limitSet bool

	// Parse remaining arguments
	i := 0
	for i < len(rest) {
		arg := rest[i]
		key, val, hasEq := strings.Cut(arg, "=")
		switch {
		case key == "--json":
			jsonOutput = true
		case key == "--list" || key == "-l":
			listOnly = true
		case key == "--hub":
			if hasEq {
				hubInput = strings.TrimSpace(val)
			} else if i+1 < len(rest) && !strings.HasPrefix(rest[i+1], "-") {
				i++
				hubInput = strings.TrimSpace(rest[i])
			} else {
				hubBare = true
			}
		case key == "--limit" || key == "-n":
			limitSet = true
			if hasEq {
				n, err := strconv.Atoi(strings.TrimSpace(val))
				if err != nil || n < 1 {
					return fmt.Errorf("--limit must be a positive number")
				}
				limit = n
			} else if i+1 >= len(rest) {
				return fmt.Errorf("--limit requires a value")
			} else {
				i++
				n, err := strconv.Atoi(rest[i])
				if err != nil || n < 1 {
					return fmt.Errorf("--limit must be a positive number")
				}
				limit = n
			}
		case key == "--help" || key == "-h":
			printSearchHelp()
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			if query != "" {
				// Append to query for multi-word search
				query += " " + arg
			} else {
				query = arg
			}
		}
		i++
	}

	// Resolve --hub value to a URL
	var indexURL string
	if hubInput != "" || hubBare {
		resolved, err := resolveHubURL(hubInput, hubBare, mode, cwd, defaultHubURL)
		if err != nil {
			return err
		}
		indexURL = resolved
	}

	// Hub search returns all results by default (limit=0 means no limit)
	if indexURL != "" && !limitSet {
		limit = 0
	}

	// JSON mode: silent search, output JSON
	if jsonOutput {
		return searchJSON(query, limit, indexURL)
	}

	// Interactive mode
	return searchInteractive(query, limit, listOnly, indexURL, mode, cwd)
}

func searchJSON(query string, limit int, indexURL string) error {
	var results []search.SearchResult
	var err error
	if indexURL != "" {
		results, err = search.SearchFromIndexURL(query, limit, indexURL)
	} else {
		results, err = search.Search(query, limit)
	}
	if err != nil {
		// Return error as JSON
		errJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Println(string(errJSON))
		return nil
	}

	output, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(output))
	return nil
}

func searchInteractive(query string, limit int, listOnly bool, indexURL string, mode runMode, cwd string) error {
	// No query provided: prompt for one
	isHub := indexURL != ""
	if query == "" {
		input, shouldExit := promptSearchQuery(isHub)
		if shouldExit {
			return nil
		}
		query = input
	}

	// List-only mode: single search and exit
	if listOnly {
		_, err := doSearch(query, limit, true, indexURL, mode, cwd)
		return err
	}

	// Interactive loop mode
	currentQuery := query
	for {
		searchAgain, err := doSearch(currentQuery, limit, false, indexURL, mode, cwd)
		if err != nil {
			return err
		}

		// If user selected "Search again" or no results found
		if searchAgain {
			fmt.Println()
			nextQuery, shouldExit := promptNextSearch()
			if shouldExit {
				return nil
			}
			if nextQuery != "" {
				currentQuery = nextQuery
			}
			fmt.Println()
			continue
		}

		// User installed something or cancelled - exit
		return nil
	}
}

// doSearch performs a search and returns (searchAgain, error)
func doSearch(query string, limit int, listOnly bool, indexURL string, mode runMode, cwd string) (bool, error) {
	where := "GitHub"
	if indexURL != "" {
		where = "the index"
	}
	spinnerMsg := fmt.Sprintf("Searching %s for %q...", where, query)
	if query == "" {
		spinnerMsg = fmt.Sprintf("Browsing popular skills in %s...", where)
	}
	spinner := ui.StartSpinner(spinnerMsg)

	var results []search.SearchResult
	var err error
	if indexURL != "" {
		results, err = search.SearchFromIndexURL(query, limit, indexURL)
	} else {
		results, err = search.Search(query, limit)
	}
	spinner.Stop()
	if err != nil {
		// GitHub-specific errors only apply when not using index
		if indexURL == "" {
			// Handle authentication required error
			if _, ok := err.(*search.AuthRequiredError); ok {
				ui.Warning("GitHub code search needs you to log in")
				ui.Next("gh auth login", "log in with the GitHub CLI", "export GITHUB_TOKEN=<token>", "or set a token")
				return false, nil
			}

			// Handle rate limit error with helpful message
			if rateLimitErr, ok := err.(*search.RateLimitError); ok {
				ui.Warning("GitHub API rate limit reached")
				if rateLimitErr.Remaining == "0" {
					ui.Note(fmt.Sprintf("Limit: %s requests/hour", rateLimitErr.Limit))
				}
				ui.Next("export GITHUB_TOKEN=<token>", "a token raises the limit")
				return false, nil
			}
		}
		return false, err
	}

	// No results
	if len(results) == 0 {
		if query == "" {
			ui.Done(ui.MarkNone, "No skills found", 0)
		} else {
			ui.Done(ui.MarkNone, fmt.Sprintf("No skills found for %q", query), 0)
		}
		return true, nil // Allow search again
	}

	isHub := indexURL != ""

	// List-only mode: show results and exit
	if listOnly {
		printSearchResults(results, isHub)
		fmt.Println()
		ui.Done(ui.MarkNone, "Found "+plural(len(results), "skill"), 0)
		ui.Next("skillshare install <source>", "install one")
		return false, nil
	}

	// Interactive mode: show selector
	return promptInstallFromSearch(results, query, isHub, mode, cwd)
}

func promptSearchQuery(isHub bool) (string, bool) {
	msg := "Enter search keyword: "
	if isHub {
		msg = "Enter search keyword (empty to browse all): "
	}
	fmt.Print(msg)

	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", true // EOF / Ctrl+D
	}

	input = strings.TrimSpace(input)
	if input == "" && !isHub {
		return "", true // Empty = quit (GitHub mode only)
	}

	return input, false
}

func promptNextSearch() (string, bool) {
	fmt.Print("Search again (or press Enter to quit): ")

	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", true // EOF / Ctrl+D
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return "", true // Empty = quit
	}

	return input, false
}

func printSearchResults(results []search.SearchResult, isHub bool) {
	names := make([]string, len(results))
	sources := make([]string, len(results))
	for i, r := range results {
		names[i] = truncate(r.Name, 24)
		// Truncate source if too long
		sources[i] = shortenPath(r.Source)
		if len(sources[i]) > 50 {
			sources[i] = "..." + sources[i][len(sources[i])-47:]
		}
	}
	width := ui.RowWidth(names...)
	sourceWidth := 0
	for _, src := range sources {
		sourceWidth = max(sourceWidth, len(src))
	}

	for i, r := range results {
		value := ui.DimText(sources[i])
		if isHub {
			if ui.IsTTY() {
				value += formatRiskBadge(r.RiskLabel)
			} else {
				value += formatRiskBadgePlain(r.RiskLabel)
			}
		} else {
			value += strings.Repeat(" ", sourceWidth-len(sources[i])) + "  " + ui.Colorize(ui.Yellow, "★ "+search.FormatStars(r.Stars))
		}
		ui.Row(ui.MarkNone, names[i], value, width)
		// Show description if available
		if r.Description != "" {
			ui.Note(strings.Repeat(" ", width+2) + truncate(r.Description, 70))
		}
		// Show tags if available
		if len(r.Tags) > 0 {
			tags := make([]string, len(r.Tags))
			for j, tag := range r.Tags {
				tags[j] = "#" + tag
			}
			ui.Note(strings.Repeat(" ", width+2) + strings.Join(tags, " "))
		}
	}
}

func promptInstallFromSearch(results []search.SearchResult, query string, isHub bool, mode runMode, cwd string) (bool, error) {
	res, err := runSearchSelectTUI(results, query, isHub)
	if err != nil {
		return false, err
	}
	if res.searchAgain {
		return true, nil
	}
	if len(res.selected) == 0 {
		return false, nil
	}
	return false, batchInstallFromSearch(res.selected, mode, cwd)
}

// batchInstallFromSearch installs multiple search results sequentially.
// Single selection uses verbose per-skill output; multi-selection uses progress display.
func batchInstallFromSearch(selected []search.SearchResult, mode runMode, cwd string) error {
	if len(selected) == 1 {
		// Single skill: spinner, warnings and one result row
		if mode == modeProject {
			return installFromSearchResultProject(selected[0], cwd)
		}
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		return installFromSearchResult(selected[0], cfg)
	}
	// Multiple skills: progress display + summary
	return batchInstallFromSearchWithProgress(selected, mode, cwd)
}

func installFromSearchResultProject(result search.SearchResult, cwd string) (err error) {
	start := time.Now()
	logSummary := installLogSummary{
		Source: result.Source,
		Mode:   "project",
	}
	defer func() {
		logInstallOp(config.ProjectConfigPath(cwd), []string{result.Source}, start, err, logSummary)
	}()

	if err := ensureProjectConfig(cwd); err != nil {
		return err
	}

	runtime, err := loadProjectRuntime(cwd)
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	source, err := install.ParseSourceWithOptions(result.Source, parseOptsFromProjectConfig(runtime.config))
	if err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}

	destPath := filepath.Join(runtime.sourcePath, result.Name)

	// Check if already exists
	if _, err := os.Stat(destPath); err == nil {
		ui.StepSkip("Install", result.Name+" "+ui.DimText("already installed"))
		ui.Next("skillshare install "+result.Source+" -p --force", "replace it")
		return nil
	}

	// Install
	spinner := ui.StartSpinner(fmt.Sprintf("Installing %s...", result.Source))

	opts := install.InstallOptions{}
	if result.Skill != "" {
		opts.Skills = []string{result.Skill}
	}
	if ui.IsTTY() {
		opts.OnProgress = func(line string) {
			if text := parseGitProgressLine(line); text != "" {
				spinner.Update(text)
			}
		}
	}

	installResult, err := install.Install(source, destPath, opts)
	spinner.Stop()
	if err != nil {
		logSummary.FailedSkills = []string{result.Name}
		ui.StepFail("Install", result.Name)
		return err
	}

	logSummary.SkillCount = 1
	logSummary.InstalledSkills = []string{result.Name}

	for _, warning := range installResult.Warnings {
		ui.Warning("%s", warning)
	}

	// Update .gitignore for the installed skill (skip for external sources)
	gitDir, gitPrefix := config.ProjectGitignoreTarget(runtime.root, runtime.sourcePath)
	if gitDir != "" {
		if err := install.UpdateGitIgnore(gitDir, gitPrefix+"/"+result.Name); err != nil {
			ui.Warning("Failed to update .gitignore: %v", err)
		}
	}

	// Reconcile project config with installed skills
	if err := reconcileProjectRemoteSkills(runtime); err != nil {
		return err
	}

	printInstallRow(result.Name, destPath, false, start)
	ui.Next("skillshare sync -p", "link it into your targets")

	return nil
}

func installFromSearchResult(result search.SearchResult, cfg *config.Config) (err error) {
	start := time.Now()
	logSummary := installLogSummary{
		Source: result.Source,
		Mode:   "global",
	}
	defer func() {
		logInstallOp(config.ConfigPath(), []string{result.Source}, start, err, logSummary)
	}()

	// Parse source
	source, err := install.ParseSourceWithOptions(result.Source, parseOptsFromConfig(cfg))
	if err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}

	// Determine destination
	destPath := filepath.Join(cfg.EffectiveSkillsSource(), result.Name)

	// Check if already exists
	if _, err := os.Stat(destPath); err == nil {
		ui.StepSkip("Install", result.Name+" "+ui.DimText("already installed"))
		ui.Next("skillshare install "+result.Source+" --force", "replace it")
		return nil
	}

	// Install
	spinner := ui.StartSpinner(fmt.Sprintf("Installing %s...", result.Source))

	opts := install.InstallOptions{}
	if result.Skill != "" {
		opts.Skills = []string{result.Skill}
	}
	if ui.IsTTY() {
		opts.OnProgress = func(line string) {
			if text := parseGitProgressLine(line); text != "" {
				spinner.Update(text)
			}
		}
	}

	installResult, err := install.Install(source, destPath, opts)
	spinner.Stop()
	if err != nil {
		logSummary.FailedSkills = []string{result.Name}
		ui.StepFail("Install", result.Name)
		return err
	}

	logSummary.SkillCount = 1
	logSummary.InstalledSkills = []string{result.Name}

	// Reconcile global config with installed skills
	store, _ := install.LoadMetadataWithMigration(cfg.EffectiveSkillsSource(), "")
	if store == nil {
		store = install.NewMetadataStore()
	}
	if rErr := config.ReconcileGlobalSkills(cfg, store); rErr != nil {
		ui.Warning("Failed to reconcile global skills config: %v", rErr)
	}

	// Show warnings
	for _, warning := range installResult.Warnings {
		ui.Warning("%s", warning)
	}

	printInstallRow(result.Name, destPath, false, start)
	ui.Next("skillshare sync", "link it into your targets")

	return nil
}

func truncate(s string, maxLen int) string { return truncateStr(s, maxLen) }

// formatRiskBadge returns a colored risk badge for TTY output.
// Returns empty string when label is empty (not audited).
func formatRiskBadge(label string) string {
	if label == "" {
		return ""
	}
	var color string
	switch label {
	case "clean":
		color = ui.Green
	case "low":
		color = ui.Cyan
	case "medium":
		color = ui.Yellow
	case "high", "critical":
		color = ui.Red
	default:
		color = ui.Dim
	}
	return fmt.Sprintf(" %s[%s]%s", color, label, ui.Reset)
}

// formatRiskBadgePlain returns a plain risk badge for non-TTY output.
func formatRiskBadgePlain(label string) string {
	if label == "" {
		return ""
	}
	return fmt.Sprintf(" [%s]", label)
}

func printSearchHelp() {
	printHelp("skillshare search [query] [options]", "Search GitHub for skills containing SKILL.md files.\nWhen no query is provided, browses popular skills.",
		helpGroup{title: "Options", rows: []helpRow{
			{"-p, --project", "Install to project-level config (.skillshare/)"},
			{"-g, --global", "Install to global config (~/.config/skillshare)"},
			{"--hub [URL]", "Search from a hub index (default: skillshare-hub; or custom URL/path)"},
			{"--json", "Output results as JSON"},
			{"-l, --list", "List results only (no install prompt)"},
			{"-n, --limit N", "Maximum results (default: 20, max: 100)"},
		}},
		helpExamples(
			helpRow{"skillshare search", "Browse popular skills"},
			helpRow{"skillshare search pdf", ""},
			helpRow{"skillshare search \"code review\"", ""},
			helpRow{"skillshare search commit --limit 10", ""},
			helpRow{"skillshare search frontend --json", ""},
			helpRow{"skillshare search react --list", ""},
			helpRow{"skillshare search pdf -p", ""},
		),
		helpGroup{title: "Hub search (default: skillshare-hub)", examples: true, rows: []helpRow{
			{"skillshare search --hub", "Browse skillshare-hub"},
			{"skillshare search react --hub", "Search \"react\" in skillshare-hub"},
			{"skillshare search --hub ./skillshare-hub.json", "Custom local index"},
			{"skillshare search react --hub https://internal.corp/skills/index.json", ""},
		}},
		helpGroup{title: "Hub search with saved hubs", examples: true, rows: []helpRow{
			{"skillshare hub add https://internal.corp/hub.json --label team", ""},
			{"skillshare search --hub team", "Search using saved hub label"},
			{"skillshare hub default team", ""},
			{"skillshare search --hub", "Uses default hub"},
		}},
	)
}

// resolveHubURL resolves the --hub flag value to a URL.
// - bare=true, input="" → config default → fallback to defaultHubURL
// - input looks like URL/path → passthrough
// - otherwise → label lookup in config
func resolveHubURL(input string, bare bool, mode runMode, cwd, defaultHubURL string) (string, error) {
	if bare && input == "" {
		// --hub with no value: try config default, then community hub
		hubCfg := loadHubConfig(mode, cwd)
		url, err := hubCfg.DefaultHub()
		if err != nil {
			return "", err
		}
		if url != "" {
			return url, nil
		}
		return defaultHubURL, nil
	}

	// Check if input looks like a URL or path
	if looksLikeURLOrPath(input) {
		return input, nil
	}

	// Try label lookup
	hubCfg := loadHubConfig(mode, cwd)
	url, ok := hubCfg.ResolveHub(input)
	if !ok {
		return "", fmt.Errorf("hub %q not found; run 'skillshare hub list' to see saved hubs", input)
	}
	return url, nil
}

// looksLikeURLOrPath returns true if the value appears to be a URL or file path
// rather than a hub label.
func looksLikeURLOrPath(v string) bool {
	return strings.HasPrefix(v, "http://") ||
		strings.HasPrefix(v, "https://") ||
		strings.HasPrefix(v, "file://") ||
		strings.HasPrefix(v, "/") ||
		strings.HasPrefix(v, "./") ||
		strings.HasPrefix(v, "../") ||
		strings.HasPrefix(v, "~") ||
		install.IsSSHURL(v)
}

// loadHubConfig loads the HubConfig from the appropriate config, returning
// an empty HubConfig on error (graceful fallback).
func loadHubConfig(mode runMode, cwd string) config.HubConfig {
	if mode == modeProject {
		pcfg, err := config.LoadProject(cwd)
		if err != nil {
			return config.HubConfig{}
		}
		return pcfg.Hub
	}
	cfg, err := config.Load()
	if err != nil {
		return config.HubConfig{}
	}
	return cfg.Hub
}
