package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"

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

	var query string
	var jsonOutput bool
	var listOnly bool
	var indexURL string
	var limit int = 20

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
		case key == "--index-url":
			if hasEq {
				indexURL = strings.TrimSpace(val)
			} else if i+1 >= len(rest) {
				return fmt.Errorf("--index-url requires a value")
			} else {
				i++
				indexURL = strings.TrimSpace(rest[i])
			}
		case key == "--limit" || key == "-n":
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
	if query == "" {
		input, shouldExit := promptSearchQuery()
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

	// List-only mode: show results and exit
	if listOnly {
		fmt.Println()
		printSearchResults(results)
		return false, nil
	}

	// Interactive mode: show selector
	fmt.Println()
	return promptInstallFromSearch(results, mode, cwd)
}

func promptSearchQuery() (string, bool) {
	var input string
	prompt := &survey.Input{
		Message: "Enter search keyword:",
	}

	err := survey.AskOne(prompt, &input)
	if err != nil {
		return "", true // Ctrl+C
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return "", true // Empty = quit
	}

	return input, false
}

func promptNextSearch() (string, bool) {
	// Use survey Input for next search
	var input string
	prompt := &survey.Input{
		Message: "Search again (or press Enter to quit):",
	}

	err := survey.AskOne(prompt, &input)
	if err != nil {
		return "", true // Ctrl+C
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return "", true // Empty = quit
	}

	return input, false
}

func printSearchResults(results []search.SearchResult) {
	// Header
	if ui.IsTTY() {
		fmt.Printf("  %s#   %-24s %-40s %s%s\n",
			ui.Gray, "Name", "Source", "Stars", ui.Reset)
		fmt.Printf("  %s─── ──────────────────────── ──────────────────────────────────────── ─────%s\n",
			ui.Gray, ui.Reset)
	}

	for i, r := range results {
		num := fmt.Sprintf("%d.", i+1)

		// Truncate source if too long
		source := r.Source
		if len(source) > 40 {
			source = "..." + source[len(source)-37:]
		}

		// Format stars
		stars := search.FormatStars(r.Stars)

		if ui.IsTTY() {
			fmt.Printf("  %s%-3s%s %-24s %s%-40s%s %s★ %s%s\n",
				ui.Yellow, num, ui.Reset,
				truncate(r.Name, 24),
				ui.Gray, source, ui.Reset,
				ui.Yellow, stars, ui.Reset)

			// Show description if available
			if r.Description != "" {
				desc := truncate(r.Description, 70)
				fmt.Printf("      %s%s%s\n", ui.Gray, desc, ui.Reset)
			}
		} else {
			// Non-TTY output
			fmt.Printf("  %-3s %-24s %-40s ★ %s\n",
				num,
				truncate(r.Name, 24),
				source,
				stars)
			if r.Description != "" {
				fmt.Printf("      %s\n", truncate(r.Description, 70))
			}
		}
	}
}

func promptInstallFromSearch(results []search.SearchResult, mode runMode, cwd string) (bool, error) {
	// Build options list with name and full source
	// First option is "Search again"
	options := make([]string, len(results)+1)
	options[0] = fmt.Sprintf("\033[36m⟲ Search again...\033[0m")

	for i, r := range results {
		stars := search.FormatStars(r.Stars)
		options[i+1] = fmt.Sprintf("%-20s ★ %-5s \033[90m%s\033[0m",
			r.Name, stars, r.Source)
	}

	// Use survey Select for better UX
	var selectedIdx int
	prompt := &survey.Select{
		Message:  "Select skill to install (↑↓ navigate, enter select, type to filter):",
		Options:  options,
		PageSize: 12,
	}

	err := survey.AskOne(prompt, &selectedIdx, survey.WithIcons(func(icons *survey.IconSet) {
		icons.SelectFocus.Text = "▸"
		icons.SelectFocus.Format = "yellow"
	}))
	if err != nil {
		return false, nil // User cancelled (Ctrl+C) - exit
	}

	// "Search again" selected
	if selectedIdx == 0 {
		return true, nil // Signal to search again
	}

	selected := results[selectedIdx-1]

	// Install the selected skill
	fmt.Println()
	if mode == modeProject {
		return false, installFromSearchResultProject(selected, cwd)
	}

	cfg, err := config.Load()
	if err != nil {
		return false, fmt.Errorf("failed to load config: %w", err)
	}
	return false, installFromSearchResult(selected, cfg)
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

	installResult, err := install.Install(source, destPath, install.InstallOptions{})
	if err != nil {
		spinner.Fail("Failed to install")
		logSummary.FailedSkills = []string{result.Name}
		return err
	}

	spinner.Success(fmt.Sprintf("Installed: %s", result.Name))
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

	installResult, err := install.Install(source, destPath, install.InstallOptions{})
	if err != nil {
		spinner.Fail("Failed to install")
		logSummary.FailedSkills = []string{result.Name}
		return err
	}

	spinner.Success(fmt.Sprintf("Installed: %s", result.Name))
	logSummary.SkillCount = 1
	logSummary.InstalledSkills = []string{result.Name}

	// Show warnings
	for _, warning := range installResult.Warnings {
		ui.Warning("%s", warning)
	}

	// Next steps
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

func printSearchHelp() {
	fmt.Println(`Usage: skillshare search [query] [options]

Search GitHub for skills containing SKILL.md files.
When no query is provided, browses popular skills.

Options:
  --project, -p      Install to project-level config (.skillshare/)
  --global, -g       Install to global config (~/.config/skillshare)
  --index-url URL    Search from a private index (local path or HTTP URL)
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

  # Private index search
  skillshare search --index-url /path/to/index.json
  skillshare search react --index-url https://internal.corp/skills/index.json`)
}
