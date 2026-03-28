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

	// TUI-only fields (unexported, excluded from JSON)
	relPath     string
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
			summary.OnDemand.Chars += s.BodyChars
		}
	}
	summary.AlwaysLoaded.Tokens = estimateTokens(summary.AlwaysLoaded.Chars)
	summary.OnDemand.Tokens = estimateTokens(summary.OnDemand.Chars)
	summary.Total.Chars = summary.AlwaysLoaded.Chars + summary.OnDemand.Chars
	summary.Total.Tokens = estimateTokens(summary.Total.Chars)
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
	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}
	applyModeLabel(mode)
	if mode == modeProject {
		return cmdAnalyzeProject(cwd, opts)
	}
	return runAnalyze(opts)
}

func printAnalyzeHelp() {
	fmt.Println(`Usage: skillshare analyze [target] [options]

Analyze context window usage for each target's skills.

Shows two layers of context cost:
  - Always loaded: frontmatter name + description (loaded every request)
  - On-demand: skill body (loaded only when triggered)

Arguments:
  target              Show details for a single target (optional)

Options:
  --verbose, -v       Show top 10 largest descriptions per target
  --project, -p       Analyze project-level skills (.skillshare/)
  --global, -g        Analyze global skills (~/.config/skillshare)
  --json              Output results as JSON
  --filter <text>     Filter skills by name/path (case-insensitive substring)
  --no-tui            Disable interactive TUI
  --help, -h          Show this help

Examples:
  skillshare analyze               # Summary table for all targets
  skillshare analyze --verbose     # Top 10 descriptions per target
  skillshare analyze claude        # Details for claude target
  skillshare analyze --json        # JSON output
  skillshare analyze --filter api  # Show only skills matching "api"
  skillshare analyze -p            # Project mode`)
}

// runAnalyze runs the analyze command in global mode.
func runAnalyze(opts *analyzeOptions) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if opts.targetName == "" && !opts.json && shouldLaunchTUI(opts.noTUI, cfg) {
		loadFn := func() analyzeLoadResult {
			discovered, err := ssync.DiscoverSourceSkillsForAnalyze(cfg.Source)
			if err != nil {
				return analyzeLoadResult{err: err}
			}
			entries, err := buildAnalyzeEntries(discovered, cfg.Targets, cfg.Mode, "")
			if err != nil {
				return analyzeLoadResult{err: err}
			}
			return analyzeLoadResult{targets: entries}
		}
		return runAnalyzeTUI(loadFn, "global", opts.filter)
	}
	return runAnalyzeCore(cfg.Source, cfg.Targets, cfg.Mode, opts)
}

const charsPerToken = 4

func estimateTokens(chars int) int { return chars / charsPerToken }

func runAnalyzeCore(sourcePath string, targets map[string]config.TargetConfig, defaultMode string, opts *analyzeOptions) error {
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
			sp.Success("No skills found")
		}
		if opts.json {
			return writeJSON(&analyzeOutput{})
		}
		return nil
	}

	if sp != nil {
		sp.Success(fmt.Sprintf("Analyzed %d skill(s)", len(discovered)))
	}

	entries, err := buildAnalyzeEntries(discovered, targets, defaultMode, opts.targetName)
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

	if opts.verbose {
		printAnalyzeVerbose(entries)
	} else {
		printAnalyzeTable(entries)
	}
	return nil
}

func buildAnalyzeEntries(
	discovered []ssync.DiscoveredSkill,
	targets map[string]config.TargetConfig,
	defaultMode string,
	filterTarget string,
) ([]analyzeTargetEntry, error) {
	var entries []analyzeTargetEntry

	for name, target := range targets {
		if filterTarget != "" && name != filterTarget {
			continue
		}

		sc := target.SkillsConfig()
		tMode := getTargetMode(sc.Mode, defaultMode)

		var filtered []ssync.DiscoveredSkill
		if tMode == "symlink" {
			filtered = discovered
		} else {
			var err error
			filtered, err = ssync.FilterSkills(discovered, sc.Include, sc.Exclude)
			if err != nil {
				return nil, fmt.Errorf("target %s: %w", name, err)
			}
			filtered = ssync.FilterSkillsByTarget(filtered, name)
		}

		if len(filtered) == 0 {
			continue
		}

		skills := make([]analyzeSkillEntry, 0, len(filtered))
		var totalDescChars, totalBodyChars int
		for _, s := range filtered {
			totalDescChars += s.DescChars
			totalBodyChars += s.BodyChars
			skills = append(skills, analyzeSkillEntry{
				Name:              s.FlatName,
				DescriptionChars:  s.DescChars,
				DescriptionTokens: estimateTokens(s.DescChars),
				BodyChars:         s.BodyChars,
				BodyTokens:        estimateTokens(s.BodyChars),
				LintIssues:        s.LintIssues,
				relPath:           s.RelPath,
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
				EstimatedTokens: estimateTokens(totalDescChars),
			},
			OnDemandMax: analyzeCharTokens{
				Chars:           totalBodyChars,
				EstimatedTokens: estimateTokens(totalBodyChars),
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

func formatTokensStr(chars int) string {
	tokens := estimateTokens(chars)
	if tokens < 1000 {
		return fmt.Sprintf("~%d", tokens)
	}
	s := fmt.Sprintf("%d", tokens)
	var b strings.Builder
	b.WriteByte('~')
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

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

func colorTargetNames(entries []analyzeTargetEntry) string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = ui.Yellow + e.Name + ui.Reset
	}
	return strings.Join(names, ", ")
}

func printAnalyzeHeader(entries []analyzeTargetEntry) {
	if allTargetsIdentical(entries) {
		e := entries[0]
		ui.Info("%d skills across %d targets (%s)", e.SkillCount, len(entries), colorTargetNames(entries))
	}
}

func printAnalyzeEntry(e analyzeTargetEntry, showTopN bool) {
	fmt.Printf("  Always loaded:  %s tokens\n", formatTokensStr(e.AlwaysLoaded.Chars))
	fmt.Printf("  On-demand max:  %s tokens\n", formatTokensStr(e.OnDemandMax.Chars))
	if !showTopN {
		fmt.Println()
		return
	}
	fmt.Println()
	fmt.Println("  Largest descriptions:")
	limit := min(analyzeTopN, len(e.Skills))
	for _, s := range e.Skills[:limit] {
		fmt.Printf("  %-32s %s tokens\n",
			truncateName(s.Name, analyzeNameMaxLen),
			formatTokensStr(s.DescriptionChars),
		)
	}
	if remaining := len(e.Skills) - limit; remaining > 0 {
		fmt.Printf("  ... %d more\n", remaining)
	}
	fmt.Println()
}

func printAnalyzeTable(entries []analyzeTargetEntry) {
	ui.Header(ui.WithModeLabel("Context Analysis"))

	if allTargetsIdentical(entries) {
		printAnalyzeHeader(entries)
		printAnalyzeEntry(entries[0], false)
	} else {
		for _, e := range entries {
			ui.Info("%s%s%s (%d skills)", ui.Yellow, e.Name, ui.Reset, e.SkillCount)
			printAnalyzeEntry(e, false)
		}
	}

	fmt.Printf("%sUse --verbose to see top %d largest skill descriptions.%s\n", ui.Dim, analyzeTopN, ui.Reset)
}

func printAnalyzeVerbose(entries []analyzeTargetEntry) {
	ui.Header(ui.WithModeLabel("Context Analysis"))

	if allTargetsIdentical(entries) {
		printAnalyzeHeader(entries)
		printAnalyzeEntry(entries[0], true)
	} else {
		for _, e := range entries {
			ui.Info("%s%s%s (%d skills)", ui.Yellow, e.Name, ui.Reset, e.SkillCount)
			printAnalyzeEntry(e, true)
		}
	}
}
