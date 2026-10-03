package main

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"strings"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
)

type analyzeOptions struct {
	targetName string
	verbose    bool
	json       bool
	noTUI      bool
	filter     string
}

type analyzeSkillEntry struct {
	Name              string            `json:"name"`
	DescriptionChars  int               `json:"description_chars"`
	DescriptionTokens int               `json:"description_tokens"`
	BodyChars         int               `json:"body_chars"`
	BodyTokens        int               `json:"body_tokens"`
	LintIssues        []ssync.LintIssue `json:"lint_issues,omitempty"`
	Local             bool              `json:"local,omitempty"`    // lives in the target folder, not the source
	Disabled          bool              `json:"disabled,omitempty"` // .skillignore'd but still exposed by a symlink-mode target

	// TUI-only fields (unexported, excluded from JSON)
	relPath     string
	path        string // the skill's folder, in the source or a target
	isTracked   bool
	targetNames []string
	description string
}

type analyzeCharTokens struct {
	Chars           int `json:"chars"`
	EstimatedTokens int `json:"estimated_tokens"`
}

type analyzeTargetEntry struct {
	Name         string              `json:"name"`
	SkillCount   int                 `json:"skill_count"`
	AlwaysLoaded analyzeCharTokens   `json:"always_loaded"`
	OnDemandMax  analyzeCharTokens   `json:"on_demand_max"`
	Skills       []analyzeSkillEntry `json:"skills"`
}

type analyzeOutput struct {
	Targets []analyzeTargetEntry `json:"targets"`
}

type analyzeLoadResult struct {
	targets []analyzeTargetEntry
	err     error
}

type analyzeFilteredOutput struct {
	Filter          string               `json:"filter"`
	MatchedCount    int                  `json:"matched_count"`
	TotalCount      int                  `json:"total_count"`
	FilteredSummary analyzeFilterSummary `json:"filtered_summary"`
	Skills          []analyzeSkillEntry  `json:"skills"`
}

type analyzeFilterSummary struct {
	AlwaysLoaded analyzeFilterTokens `json:"always_loaded"`
	OnDemand     analyzeFilterTokens `json:"on_demand"`
	Total        analyzeFilterTokens `json:"total"`
}

type analyzeFilterTokens struct {
	Chars  int `json:"chars"`
	Tokens int `json:"tokens"`
}

func skillMatchesFilter(e analyzeSkillEntry, lowerFilter string) bool {
	searchField := e.relPath
	if searchField == "" {
		searchField = e.Name
	}
	return strings.Contains(strings.ToLower(searchField), lowerFilter)
}

func filterAnalyzeSkills(skills []analyzeSkillEntry, filter string) (matched []analyzeSkillEntry, summary analyzeFilterSummary) {
	lower := strings.ToLower(filter)
	for _, s := range skills {
		if skillMatchesFilter(s, lower) {
			matched = append(matched, s)
			summary.AlwaysLoaded.Chars += s.DescriptionChars
			summary.AlwaysLoaded.Tokens += s.DescriptionTokens
			summary.OnDemand.Chars += s.BodyChars
			summary.OnDemand.Tokens += s.BodyTokens
		}
	}
	summary.Total.Chars = summary.AlwaysLoaded.Chars + summary.OnDemand.Chars
	summary.Total.Tokens = summary.AlwaysLoaded.Tokens + summary.OnDemand.Tokens
	return
}

func parseAnalyzeArgs(args []string) (*analyzeOptions, bool, error) {
	opts := &analyzeOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--verbose" || arg == "-v":
			opts.verbose = true
		case arg == "--json":
			opts.json = true
		case arg == "--no-tui":
			opts.noTUI = true
		case arg == "--filter":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--filter requires a value")
			}
			i++
			opts.filter = args[i]
		case strings.HasPrefix(arg, "--filter="):
			opts.filter = strings.TrimPrefix(arg, "--filter=")
			if opts.filter == "" {
				return nil, false, fmt.Errorf("--filter requires a non-empty value")
			}
		case arg == "--help" || arg == "-h":
			return nil, true, nil
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown option: %s", arg)
		default:
			if opts.targetName != "" {
				return nil, false, fmt.Errorf("unexpected argument: %s (only one target name allowed)", arg)
			}
			opts.targetName = arg
		}
	}
	if opts.targetName != "" {
		opts.verbose = true
	}
	return opts, false, nil
}

func cmdAnalyze(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}
	opts, showHelp, err := parseAnalyzeArgs(rest)
	if err != nil {
		return err
	}
	if showHelp {
		printAnalyzeHelp()
		return nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}
	mode = resolveAutoMode(mode, cwd)
	applyModeLabel(mode)
	if mode == modeProject {
		return cmdAnalyzeProject(cwd, opts)
	}
	return runAnalyze(opts)
}

func printAnalyzeHelp() {
	printHelp("skillshare analyze [target] [options]", "Analyze context window usage for each target's skills.\n\nShows two layers of context cost:\n  - Always loaded: frontmatter name + description (loaded every request)\n  - On-demand: skill body (loaded only when triggered)",
		helpGroup{title: "Arguments", rows: []helpRow{
			{"target", "Show details for a single target (optional)"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"-v, --verbose", "Show top 10 largest descriptions per target"},
			{"-p, --project", "Analyze project-level skills (.skillshare/)"},
			{"-g, --global", "Analyze global skills (~/.config/skillshare)"},
			{"--json", "Output results as JSON"},
			{"--filter <text>", "Filter skills by name/path (case-insensitive substring)"},
			{"--no-tui", "Disable interactive TUI"},
		}},
		helpExamples(
			helpRow{"skillshare analyze", "Summary table for all targets"},
			helpRow{"skillshare analyze --verbose", "Top 10 descriptions per target"},
			helpRow{"skillshare analyze claude", "Details for claude target"},
			helpRow{"skillshare analyze --json", "JSON output"},
			helpRow{"skillshare analyze --filter api", "Show only skills matching \"api\""},
			helpRow{"skillshare analyze -p", "Project mode"},
		),
	)
}

// runAnalyze runs the analyze command in global mode.
func runAnalyze(opts *analyzeOptions) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if opts.targetName == "" && !opts.json && shouldLaunchTUI(opts.noTUI, cfg) {
		loadFn := func() analyzeLoadResult {
			discovered, err := ssync.DiscoverSourceSkillsForAnalyze(cfg.EffectiveSkillsSource())
			if err != nil {
				return analyzeLoadResult{err: err}
			}
			entries, err := buildAnalyzeEntries(discovered, cfg.Targets, cfg.Mode, cfg.EffectiveSkillsSource(), "")
			if err != nil {
				return analyzeLoadResult{err: err}
			}
			return analyzeLoadResult{targets: entries}
		}
		return runAnalyzeTUI(loadFn, "global", opts.filter)
	}
	return runAnalyzeCore(cfg.EffectiveSkillsSource(), cfg.Targets, cfg.Mode, cfg.ContextBudget, opts)
}

func runAnalyzeCore(sourcePath string, targets map[string]config.TargetConfig, defaultMode string, budget config.ContextBudgetConfig, opts *analyzeOptions) error {
	var sp *ui.Spinner
	if !opts.json {
		sp = ui.StartSpinner("Analyzing skills...")
	}

	discovered, err := ssync.DiscoverSourceSkillsForAnalyze(sourcePath)
	if err != nil {
		if sp != nil {
			sp.Fail("Analysis failed")
		}
		if opts.json {
			return writeJSONError(err)
		}
		return err
	}

	if len(discovered) == 0 {
		if sp != nil {
			sp.Stop()
		}
		if opts.json {
			return writeJSON(&analyzeOutput{})
		}
		ui.Done(ui.MarkNone, "No skills to analyze", 0)
		return nil
	}

	if sp != nil {
		sp.Stop()
	}

	entries, err := buildAnalyzeEntries(discovered, targets, defaultMode, sourcePath, opts.targetName)
	if err != nil {
		if opts.json {
			return writeJSONError(err)
		}
		return err
	}

	if opts.json {
		if opts.filter != "" && len(entries) > 0 {
			entry := entries[0]
			matched, summary := filterAnalyzeSkills(entry.Skills, opts.filter)
			return writeJSON(&analyzeFilteredOutput{
				Filter:          opts.filter,
				MatchedCount:    len(matched),
				TotalCount:      len(entry.Skills),
				FilteredSummary: summary,
				Skills:          matched,
			})
		} else if opts.filter != "" {
			return writeJSON(&analyzeFilteredOutput{Filter: opts.filter})
		}
		return writeJSON(&analyzeOutput{Targets: entries})
	}

	if opts.filter != "" {
		for i, entry := range entries {
			matched, summary := filterAnalyzeSkills(entry.Skills, opts.filter)
			entries[i].Skills = matched
			entries[i].SkillCount = len(matched)
			entries[i].AlwaysLoaded = analyzeCharTokens{Chars: summary.AlwaysLoaded.Chars, EstimatedTokens: summary.AlwaysLoaded.Tokens}
			entries[i].OnDemandMax = analyzeCharTokens{Chars: summary.OnDemand.Chars, EstimatedTokens: summary.OnDemand.Tokens}
		}
	}

	if len(entries) == 0 {
		if len(targets) == 0 {
			ui.Done(ui.MarkNone, "No targets configured", 0)
			ui.Next("skillshare target add <name> <path>", "add one")
		} else {
			ui.Done(ui.MarkNone, "No skills reach any target", 0)
		}
		return nil
	}
	printAnalyze(entries, opts.verbose)

	if violations := checkBudget(entries, budget); len(violations) > 0 {
		fmt.Println()
		printBudgetWarning(violations, false)
	}

	return nil
}

func buildAnalyzeEntries(
	discovered []ssync.DiscoveredSkill,
	targets map[string]config.TargetConfig,
	defaultMode string,
	sourcePath string,
	filterTarget string,
) ([]analyzeTargetEntry, error) {
	var entries []analyzeTargetEntry

	for name, target := range targets {
		if filterTarget != "" && name != filterTarget {
			continue
		}

		filtered, err := ssync.TargetSkills(name, target, getTargetMode("", defaultMode), sourcePath, discovered)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", name, err)
		}

		if len(filtered) == 0 {
			continue
		}

		skills := make([]analyzeSkillEntry, 0, len(filtered))
		var totalDescChars, totalBodyChars, totalDescTokens, totalBodyTokens int
		for _, s := range filtered {
			totalDescChars += s.DescChars
			totalBodyChars += s.BodyChars
			totalDescTokens += s.DescTokens
			totalBodyTokens += s.BodyTokens
			skills = append(skills, analyzeSkillEntry{
				Name:              s.FlatName,
				DescriptionChars:  s.DescChars,
				DescriptionTokens: s.DescTokens,
				BodyChars:         s.BodyChars,
				BodyTokens:        s.BodyTokens,
				LintIssues:        s.LintIssues,
				Local:             s.Local,
				Disabled:          s.Disabled,
				relPath:           s.RelPath,
				path:              s.SourcePath,
				isTracked:         s.IsInRepo,
				targetNames:       s.Targets,
				description:       s.Description,
			})
		}

		slices.SortFunc(skills, func(a, b analyzeSkillEntry) int {
			return cmp.Compare(b.DescriptionChars, a.DescriptionChars)
		})

		entries = append(entries, analyzeTargetEntry{
			Name:       name,
			SkillCount: len(skills),
			AlwaysLoaded: analyzeCharTokens{
				Chars:           totalDescChars,
				EstimatedTokens: totalDescTokens,
			},
			OnDemandMax: analyzeCharTokens{
				Chars:           totalBodyChars,
				EstimatedTokens: totalBodyTokens,
			},
			Skills: skills,
		})
	}

	if filterTarget != "" && len(entries) == 0 {
		return nil, fmt.Errorf("target %q not configured", filterTarget)
	}

	slices.SortFunc(entries, func(a, b analyzeTargetEntry) int {
		return cmp.Compare(b.AlwaysLoaded.Chars, a.AlwaysLoaded.Chars)
	})

	return entries, nil
}

func formatTokensStr(tokens int) string { return "~" + formatTokenComma(tokens) }

const analyzeTopN = 10
const analyzeNameMaxLen = 30

func truncateName(name string, maxLen int) string {
	runes := []rune(name)
	if len(runes) <= maxLen {
		return name
	}
	return string(runes[:maxLen-1]) + "…"
}

func allTargetsIdentical(entries []analyzeTargetEntry) bool {
	if len(entries) <= 1 {
		return false
	}
	first := entries[0]
	for _, e := range entries[1:] {
		if e.SkillCount != first.SkillCount ||
			e.AlwaysLoaded.Chars != first.AlwaysLoaded.Chars ||
			e.OnDemandMax.Chars != first.OnDemandMax.Chars {
			return false
		}
	}
	return true
}

// printAnalyzeSection prints one target, or several with identical
// numbers, as a section: token rows and, when showTopN is set, the
// largest descriptions.
func printAnalyzeSection(names []string, e analyzeTargetEntry, showTopN, first bool) {
	if !first {
		fmt.Println()
	}
	title := ui.Bold + strings.Join(names, ", ") + ui.Reset + ui.DimText(" · "+plural(e.SkillCount, "skill"))
	if ui.ModeLabel == "project" {
		title += ui.DimText(" · project")
	}
	fmt.Println(title)
	width := ui.RowWidth("Always loaded", "On-demand max")
	ui.Row(ui.MarkNone, "Always loaded", formatTokensStr(e.AlwaysLoaded.EstimatedTokens)+" tokens", width)
	ui.Row(ui.MarkNone, "On-demand max", formatTokensStr(e.OnDemandMax.EstimatedTokens)+" tokens", width)
	if !showTopN || len(e.Skills) == 0 {
		return
	}
	limit := min(analyzeTopN, len(e.Skills))
	labels := make([]string, limit)
	for i, s := range e.Skills[:limit] {
		labels[i] = "  " + truncateName(s.Name, analyzeNameMaxLen)
	}
	fmt.Println()
	fmt.Println("  " + ui.DimText("Largest descriptions"))
	nameWidth := ui.RowWidth(labels...)
	for i, s := range e.Skills[:limit] {
		ui.Row(ui.MarkNone, labels[i], formatTokensStr(s.DescriptionTokens)+" tokens", nameWidth)
	}
	if remaining := len(e.Skills) - limit; remaining > 0 {
		ui.Note("  … " + plural(remaining, "more skill"))
	}
}

// printAnalyze prints every target, folding targets with identical numbers
// into one section.
func printAnalyze(entries []analyzeTargetEntry, verbose bool) {
	if allTargetsIdentical(entries) {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name
		}
		printAnalyzeSection(names, entries[0], verbose, true)
	} else {
		for i, e := range entries {
			printAnalyzeSection([]string{e.Name}, e, verbose, i == 0)
		}
	}
	if !verbose {
		fmt.Println()
		ui.Note(fmt.Sprintf("Add -v for the %d largest descriptions", analyzeTopN))
	}
}
