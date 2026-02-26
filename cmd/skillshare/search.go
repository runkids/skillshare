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
	appversion "skillshare/internal/version"
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
	if mode == modeAuto && projectConfigExists(cwd) {
		mode = modeProject
	} else if mode == modeAuto {
		mode = modeGlobal
	}

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
	// Show progress on stderr (so JSON output stays clean on stdout)
	if query == "" {
		fmt.Fprintf(os.Stderr, "Browsing popular skills...\n")
	} else {
		fmt.Fprintf(os.Stderr, "Searching for '%s'...\n", query)
	}

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

	fmt.Fprintf(os.Stderr, "Found %d result(s)\n", len(results))

	output, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(output))
	return nil
}

func searchInteractive(query string, limit int, listOnly bool, indexURL string, mode runMode, cwd string) error {
	// Show logo
	ui.Logo(appversion.Version)

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
	if query == "" {
		ui.StepStart("Browsing", "popular skills")
	} else {
		ui.StepStart("Searching", query)
	}

	var spinnerMsg string
	if indexURL != "" {
		spinnerMsg = "Querying index..."
	} else {
		spinnerMsg = "Querying GitHub..."
	}
	spinner := ui.StartTreeSpinner(spinnerMsg, false)

	var results []search.SearchResult
	var err error
	if indexURL != "" {
		results, err = search.SearchFromIndexURL(query, limit, indexURL)
	} else {
		results, err = search.Search(query, limit)
	}
	if err != nil {
		spinner.Fail("Search failed")

		// GitHub-specific errors only apply when not using index
		if indexURL == "" {
			// Handle authentication required error
			if _, ok := err.(*search.AuthRequiredError); ok {
				fmt.Println()
				ui.Warning("GitHub Code Search API requires authentication")
				fmt.Println()
				ui.Info("Option 1: Login with GitHub CLI (recommended)")
				fmt.Printf("  %sgh auth login%s\n", ui.Gray, ui.Reset)
				fmt.Println()
				ui.Info("Option 2: Set GITHUB_TOKEN environment variable")
				fmt.Printf("  %sexport GITHUB_TOKEN=ghp_your_token_here%s\n", ui.Gray, ui.Reset)
				return false, nil
			}

			// Handle rate limit error with helpful message
			if rateLimitErr, ok := err.(*search.RateLimitError); ok {
				fmt.Println()
				ui.Warning("GitHub API rate limit exceeded")
				if rateLimitErr.Remaining == "0" {
					ui.Info("Limit: %s requests/hour", rateLimitErr.Limit)
				}
				fmt.Println()
				ui.Info("To increase rate limit, set GITHUB_TOKEN:")
				fmt.Printf("  %sexport GITHUB_TOKEN=ghp_your_token_here%s\n", ui.Gray, ui.Reset)
				return false, nil
			}
		}
		return false, err
	}

	// No results
	if len(results) == 0 {
		spinner.Success("No results")
		fmt.Println()
		if query == "" {
			ui.Info("No skills found")
		} else {
			ui.Info("No skills found for '%s'", query)
		}
		return true, nil // Allow search again
	}

	spinner.Success(fmt.Sprintf("Found %d skill(s)", len(results)))

	isHub := indexURL != ""

	// List-only mode: show results and exit
	if listOnly {
		fmt.Println()
		printSearchResults(results, isHub)
		return false, nil
	}

	// Interactive mode: show selector
	fmt.Println()
	return promptInstallFromSearch(results, isHub, mode, cwd)
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
	// Header
	if ui.IsTTY() {
		if isHub {
			fmt.Printf("  %s#   %-24s %-40s%s\n",
				ui.Gray, "Name", "Source", ui.Reset)
			fmt.Printf("  %s─── ──────────────────────── ────────────────────────────────────────%s\n",
				ui.Gray, ui.Reset)
		} else {
			fmt.Printf("  %s#   %-24s %-40s %s%s\n",
				ui.Gray, "Name", "Source", "Stars", ui.Reset)
			fmt.Printf("  %s─── ──────────────────────── ──────────────────────────────────────── ─────%s\n",
				ui.Gray, ui.Reset)
		}
	}

	for i, r := range results {
		num := fmt.Sprintf("%d.", i+1)

		// Truncate source if too long
		source := r.Source
		if len(source) > 40 {
			source = "..." + source[len(source)-37:]
		}

		if ui.IsTTY() {
			if isHub {
				riskBadge := formatRiskBadge(r.RiskLabel)
				fmt.Printf("  %s%-3s%s %-24s %s%s%s%s\n",
					ui.Cyan, num, ui.Reset,
					truncate(r.Name, 24),
					ui.Gray, source, ui.Reset, riskBadge)
			} else {
				stars := search.FormatStars(r.Stars)
				fmt.Printf("  %s%-3s%s %-24s %s%-40s%s %s★ %s%s\n",
					ui.Yellow, num, ui.Reset,
					truncate(r.Name, 24),
					ui.Gray, source, ui.Reset,
					ui.Yellow, stars, ui.Reset)
			}

			// Show description if available
			if r.Description != "" {
				desc := truncate(r.Description, 70)
				fmt.Printf("      %s%s%s\n", ui.Gray, desc, ui.Reset)
			}
			// Show tags if available
			if len(r.Tags) > 0 {
				fmt.Printf("      %s", ui.Gray)
				for j, tag := range r.Tags {
					if j > 0 {
						fmt.Print(" ")
					}
					fmt.Printf("#%s", tag)
				}
				fmt.Printf("%s\n", ui.Reset)
			}
		} else {
			// Non-TTY output
			if isHub {
				riskBadge := formatRiskBadgePlain(r.RiskLabel)
				fmt.Printf("  %-3s %-24s %s%s\n",
					num, truncate(r.Name, 24), source, riskBadge)
			} else {
				stars := search.FormatStars(r.Stars)
				fmt.Printf("  %-3s %-24s %-40s ★ %s\n",
					num, truncate(r.Name, 24), source, stars)
			}
			if r.Description != "" {
				fmt.Printf("      %s\n", truncate(r.Description, 70))
			}
			if len(r.Tags) > 0 {
				tags := make([]string, len(r.Tags))
				for j, tag := range r.Tags {
					tags[j] = "#" + tag
				}
				fmt.Printf("      %s\n", strings.Join(tags, " "))
			}
		}
	}
}

func promptInstallFromSearch(results []search.SearchResult, isHub bool, mode runMode, cwd string) (bool, error) {
	res, err := runSearchSelectTUI(results, isHub)
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
		// Single skill: verbose output (StepStart + TreeSpinner + warnings)
		fmt.Println()
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

	// Auto-init project if not yet initialized
	if !projectConfigExists(cwd) {
		if err := performProjectInit(cwd, projectInitOptions{}); err != nil {
			return err
		}
	}

	runtime, err := loadProjectRuntime(cwd)
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	source, err := install.ParseSource(result.Source)
	if err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}

	destPath := filepath.Join(runtime.sourcePath, result.Name)

	// Check if already exists
	if _, err := os.Stat(destPath); err == nil {
		ui.Warning("Skill '%s' already exists in project", result.Name)
		ui.Info("Use 'skillshare install %s -p --force' to overwrite", result.Source)
		return nil
	}

	// Install
	ui.StepStart("Installing", result.Source)

	spinner := ui.StartTreeSpinner("Cloning repository...", true)

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
	if err != nil {
		spinner.Fail("Failed to install")
		logSummary.FailedSkills = []string{result.Name}
		elapsed := time.Since(start)
		ui.StepResult("error", fmt.Sprintf("Failed to install %s", result.Name), elapsed)
		return err
	}

	spinner.Success("Cloned")
	logSummary.SkillCount = 1
	logSummary.InstalledSkills = []string{result.Name}

	for _, warning := range installResult.Warnings {
		ui.Warning("%s", warning)
	}

	// Update .gitignore for the installed skill
	if err := install.UpdateGitIgnore(filepath.Join(runtime.root, ".skillshare"), filepath.Join("skills", result.Name)); err != nil {
		ui.Warning("Failed to update .skillshare/.gitignore: %v", err)
	}

	// Reconcile project config with installed skills
	if err := reconcileProjectRemoteSkills(runtime); err != nil {
		return err
	}

	elapsed := time.Since(start)
	ui.StepResult("success", fmt.Sprintf("Installed %s", result.Name), elapsed)

	// Sync hint
	fmt.Println()
	ui.Info("Run 'skillshare sync' to distribute to project targets")

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
	source, err := install.ParseSource(result.Source)
	if err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}

	// Determine destination
	destPath := filepath.Join(cfg.Source, result.Name)

	// Check if already exists
	if _, err := os.Stat(destPath); err == nil {
		ui.Warning("Skill '%s' already exists", result.Name)
		ui.Info("Use 'skillshare install %s --force' to overwrite", result.Source)
		return nil
	}

	// Install
	ui.StepStart("Installing", result.Source)

	spinner := ui.StartTreeSpinner("Cloning repository...", true)

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
	if err != nil {
		spinner.Fail("Failed to install")
		logSummary.FailedSkills = []string{result.Name}
		elapsed := time.Since(start)
		ui.StepResult("error", fmt.Sprintf("Failed to install %s", result.Name), elapsed)
		return err
	}

	spinner.Success("Cloned")
	logSummary.SkillCount = 1
	logSummary.InstalledSkills = []string{result.Name}

	// Reconcile global config with installed skills
	reg, _ := config.LoadRegistry(filepath.Dir(config.ConfigPath()))
	if reg == nil {
		reg = &config.Registry{}
	}
	if rErr := config.ReconcileGlobalSkills(cfg, reg); rErr != nil {
		ui.Warning("Failed to reconcile global skills config: %v", rErr)
	}

	// Show warnings
	for _, warning := range installResult.Warnings {
		ui.Warning("%s", warning)
	}

	elapsed := time.Since(start)
	ui.StepResult("success", fmt.Sprintf("Installed %s", result.Name), elapsed)

	// Sync hint
	fmt.Println()
	ui.Info("Run 'skillshare sync' to distribute to all targets")

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

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
		color = ui.Gray
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
	fmt.Println(`Usage: skillshare search [query] [options]

Search GitHub for skills containing SKILL.md files.
When no query is provided, browses popular skills.

Options:
  --project, -p      Install to project-level config (.skillshare/)
  --global, -g       Install to global config (~/.config/skillshare)
  --hub [URL]        Search from a hub index (default: skillshare-hub; or custom URL/path)
  --json             Output results as JSON
  --list, -l         List results only (no install prompt)
  --limit N, -n      Maximum results (default: 20, max: 100)
  --help, -h         Show this help

Examples:
  skillshare search                   Browse popular skills
  skillshare search pdf
  skillshare search "code review"
  skillshare search commit --limit 10
  skillshare search frontend --json
  skillshare search react --list
  skillshare search pdf -p

  # Hub search (default: skillshare-hub)
  skillshare search --hub                      Browse skillshare-hub
  skillshare search react --hub                Search "react" in skillshare-hub
  skillshare search --hub ./skillshare-hub.json          Custom local index
  skillshare search react --hub https://internal.corp/skills/index.json

  # Hub search with saved hubs
  skillshare hub add https://internal.corp/hub.json --label team
  skillshare search --hub team                       Search using saved hub label
  skillshare hub default team
  skillshare search --hub                            Uses default hub`)
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
		strings.HasPrefix(v, "~")
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
