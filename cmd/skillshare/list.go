package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	gosync "sync"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"

	"golang.org/x/term"
)

// listOptions holds parsed options for the list command.
type listOptions struct {
	Verbose    bool
	ShowHelp   bool
	JSON       bool
	NoTUI      bool
	Pattern    string // positional search pattern (case-insensitive)
	TypeFilter string // --type: "tracked", "local", "github"
	SortBy     string // --sort: "name" (default), "newest", "oldest"
}

// validTypeFilters lists accepted values for --type.
var validTypeFilters = map[string]bool{
	"tracked": true,
	"local":   true,
	"github":  true,
}

// validSortOptions lists accepted values for --sort.
var validSortOptions = map[string]bool{
	"name":   true,
	"newest": true,
	"oldest": true,
}

// parseListArgs parses list command arguments into listOptions.
func parseListArgs(args []string) (listOptions, error) {
	var opts listOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--verbose" || arg == "-v":
			opts.Verbose = true
		case arg == "--json" || arg == "-j":
			opts.JSON = true
		case arg == "--no-tui":
			opts.NoTUI = true
		case arg == "--help" || arg == "-h":
			opts.ShowHelp = true
			return opts, nil
		case arg == "--type" || arg == "-t":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--type requires a value (tracked, local, github)")
			}
			v := strings.ToLower(args[i])
			if !validTypeFilters[v] {
				return opts, fmt.Errorf("invalid type %q: must be tracked, local, or github", args[i])
			}
			opts.TypeFilter = v
		case strings.HasPrefix(arg, "--type="):
			v := strings.ToLower(strings.TrimPrefix(arg, "--type="))
			if !validTypeFilters[v] {
				return opts, fmt.Errorf("invalid type %q: must be tracked, local, or github", v)
			}
			opts.TypeFilter = v
		case strings.HasPrefix(arg, "-t="):
			v := strings.ToLower(strings.TrimPrefix(arg, "-t="))
			if !validTypeFilters[v] {
				return opts, fmt.Errorf("invalid type %q: must be tracked, local, or github", v)
			}
			opts.TypeFilter = v
		case arg == "--sort" || arg == "-s":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--sort requires a value (name, newest, oldest)")
			}
			v := strings.ToLower(args[i])
			if !validSortOptions[v] {
				return opts, fmt.Errorf("invalid sort %q: must be name, newest, or oldest", args[i])
			}
			opts.SortBy = v
		case strings.HasPrefix(arg, "--sort="):
			v := strings.ToLower(strings.TrimPrefix(arg, "--sort="))
			if !validSortOptions[v] {
				return opts, fmt.Errorf("invalid sort %q: must be name, newest, or oldest", v)
			}
			opts.SortBy = v
		case strings.HasPrefix(arg, "-s="):
			v := strings.ToLower(strings.TrimPrefix(arg, "-s="))
			if !validSortOptions[v] {
				return opts, fmt.Errorf("invalid sort %q: must be name, newest, or oldest", v)
			}
			opts.SortBy = v
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown option: %s", arg)
		default:
			// Positional argument → search pattern (first one wins)
			if opts.Pattern == "" {
				opts.Pattern = arg
			} else {
				return opts, fmt.Errorf("unexpected argument: %s", arg)
			}
		}
	}
	return opts, nil
}

// filterSkillEntries filters skills by pattern and type.
// Pattern matches case-insensitively against Name, RelPath, and Source.
func filterSkillEntries(skills []skillEntry, pattern, typeFilter string) []skillEntry {
	if pattern == "" && typeFilter == "" {
		return skills
	}

	pat := strings.ToLower(pattern)
	var result []skillEntry
	for _, s := range skills {
		// Type filter
		if typeFilter != "" {
			switch typeFilter {
			case "tracked":
				if s.RepoName == "" {
					continue
				}
			case "local":
				if s.Source != "" {
					continue
				}
			case "github":
				if s.Source == "" || s.RepoName != "" {
					continue
				}
			}
		}

		// Pattern filter
		if pat != "" {
			nameLower := strings.ToLower(s.Name)
			relPathLower := strings.ToLower(s.RelPath)
			sourceLower := strings.ToLower(s.Source)
			if !strings.Contains(nameLower, pat) &&
				!strings.Contains(relPathLower, pat) &&
				!strings.Contains(sourceLower, pat) {
				continue
			}
		}

		result = append(result, s)
	}
	return result
}

// sortSkillEntries sorts skills by the given criteria.
func sortSkillEntries(skills []skillEntry, sortBy string) {
	switch sortBy {
	case "newest":
		sort.SliceStable(skills, func(i, j int) bool {
			a, b := skills[i].InstalledAt, skills[j].InstalledAt
			if a == "" && b == "" {
				return false
			}
			if a == "" {
				return false // empty dates go last
			}
			if b == "" {
				return true
			}
			return a > b // descending
		})
	case "oldest":
		sort.SliceStable(skills, func(i, j int) bool {
			a, b := skills[i].InstalledAt, skills[j].InstalledAt
			if a == "" && b == "" {
				return false
			}
			if a == "" {
				return false // empty dates go last
			}
			if b == "" {
				return true
			}
			return a < b // ascending
		})
	default: // "name" or empty
		sort.SliceStable(skills, func(i, j int) bool {
			return skills[i].Name < skills[j].Name
		})
	}
}

// buildSkillEntries builds skill entries from discovered skills.
// ReadMeta calls are parallelized with a bounded worker pool.
func buildSkillEntries(discovered []sync.DiscoveredSkill) []skillEntry {
	skills := make([]skillEntry, len(discovered))

	// Pre-fill non-I/O fields
	for i, d := range discovered {
		skills[i] = skillEntry{
			Name:     d.FlatName,
			IsNested: d.IsInRepo || utils.HasNestedSeparator(d.FlatName),
			RelPath:  d.RelPath,
		}
		if d.IsInRepo {
			parts := strings.SplitN(d.RelPath, "/", 2)
			if len(parts) > 0 {
				skills[i].RepoName = parts[0]
			}
		}
	}

	// Parallel ReadMeta with bounded concurrency
	const metaWorkers = 64
	sem := make(chan struct{}, metaWorkers)
	var wg gosync.WaitGroup

	for i, d := range discovered {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, sourcePath string) {
			defer wg.Done()
			defer func() { <-sem }()
			if meta, err := install.ReadMeta(sourcePath); err == nil && meta != nil {
				skills[idx].Source = meta.Source
				skills[idx].Type = meta.Type
				skills[idx].InstalledAt = meta.InstalledAt.Format("2006-01-02")
			}
		}(i, d.SourcePath)
	}
	wg.Wait()

	return skills
}

// extractGroupDir returns the parent directory from a RelPath.
// "frontend/react-helper" → "frontend", "my-skill" → "", "_team/frontend/ui" → "_team/frontend"
func extractGroupDir(relPath string) string {
	i := strings.LastIndex(relPath, "/")
	if i < 0 {
		return ""
	}
	return relPath[:i]
}

// groupSkillEntries groups skill entries by their parent directory.
// Returns ordered group keys and a map of group→entries.
// Top-level skills (no parent dir) are grouped under "".
func groupSkillEntries(skills []skillEntry) ([]string, map[string][]skillEntry) {
	groups := make(map[string][]skillEntry)
	for _, s := range skills {
		dir := extractGroupDir(s.RelPath)
		groups[dir] = append(groups[dir], s)
	}

	// Collect sorted directory keys (non-empty first, then top-level "")
	var dirs []string
	for k := range groups {
		if k != "" {
			dirs = append(dirs, k)
		}
	}
	sort.Strings(dirs)
	// Append top-level group last
	if _, ok := groups[""]; ok {
		dirs = append(dirs, "")
	}

	return dirs, groups
}

// displayName returns the base skill name for display within a group.
// When grouped under a directory, show just the base name; otherwise show full name.
func displayName(s skillEntry, groupDir string) string {
	if groupDir == "" {
		return s.Name
	}
	// Use the last segment of RelPath as the display name
	base := s.RelPath
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return base
}

// hasGroups returns true if any skill has a parent directory (i.e., non-flat).
func hasGroups(skills []skillEntry) bool {
	for _, s := range skills {
		if extractGroupDir(s.RelPath) != "" {
			return true
		}
	}
	return false
}

// displaySkillsVerbose displays skills in verbose mode, grouped by directory
func displaySkillsVerbose(skills []skillEntry) {
	if !hasGroups(skills) {
		// Flat display — no grouping needed
		for _, s := range skills {
			fmt.Printf("  %s%s%s\n", ui.Cyan, s.Name, ui.Reset)
			printVerboseDetails(s, "    ")
		}
		return
	}

	dirs, groups := groupSkillEntries(skills)
	for i, dir := range dirs {
		if dir != "" {
			if i > 0 {
				fmt.Println()
			}
			fmt.Printf("  %s%s/%s\n", ui.Gray, dir, ui.Reset)
		} else if i > 0 {
			fmt.Println()
		}

		for _, s := range groups[dir] {
			name := displayName(s, dir)
			indent := "    "
			detailIndent := "      "
			if dir == "" {
				indent = "  "
				detailIndent = "    "
			}
			fmt.Printf("%s%s%s%s\n", indent, ui.Cyan, name, ui.Reset)
			printVerboseDetails(s, detailIndent)
		}
	}
}

func printVerboseDetails(s skillEntry, indent string) {
	if s.RepoName != "" {
		fmt.Printf("%s%sTracked repo:%s %s\n", indent, ui.Gray, ui.Reset, s.RepoName)
	}
	if s.Source != "" {
		fmt.Printf("%s%sSource:%s      %s\n", indent, ui.Gray, ui.Reset, s.Source)
		fmt.Printf("%s%sType:%s        %s\n", indent, ui.Gray, ui.Reset, s.Type)
		fmt.Printf("%s%sInstalled:%s   %s\n", indent, ui.Gray, ui.Reset, s.InstalledAt)
	} else {
		fmt.Printf("%s%sSource:%s      (local - no metadata)\n", indent, ui.Gray, ui.Reset)
	}
	fmt.Println()
}

// displaySkillsCompact displays skills in compact mode, grouped by directory
func displaySkillsCompact(skills []skillEntry) {
	if !hasGroups(skills) {
		// Flat display — identical to previous behavior
		maxNameLen := 0
		for _, s := range skills {
			if len(s.Name) > maxNameLen {
				maxNameLen = len(s.Name)
			}
		}
		for _, s := range skills {
			suffix := getSkillSuffix(s)
			format := fmt.Sprintf("  %s→%s %%-%ds  %s%%s%s\n", ui.Cyan, ui.Reset, maxNameLen, ui.Gray, ui.Reset)
			fmt.Printf(format, s.Name, suffix)
		}
		return
	}

	dirs, groups := groupSkillEntries(skills)
	for i, dir := range dirs {
		if dir != "" {
			if i > 0 {
				fmt.Println()
			}
			fmt.Printf("  %s%s/%s\n", ui.Gray, dir, ui.Reset)
		} else if i > 0 {
			fmt.Println()
		}

		// Calculate max name length within this group
		maxNameLen := 0
		for _, s := range groups[dir] {
			name := displayName(s, dir)
			if len(name) > maxNameLen {
				maxNameLen = len(name)
			}
		}

		for _, s := range groups[dir] {
			name := displayName(s, dir)
			suffix := getSkillSuffix(s)
			if dir != "" {
				format := fmt.Sprintf("    %s→%s %%-%ds  %s%%s%s\n", ui.Cyan, ui.Reset, maxNameLen, ui.Gray, ui.Reset)
				fmt.Printf(format, name, suffix)
			} else {
				format := fmt.Sprintf("  %s→%s %%-%ds  %s%%s%s\n", ui.Cyan, ui.Reset, maxNameLen, ui.Gray, ui.Reset)
				fmt.Printf(format, name, suffix)
			}
		}
	}
}

// getSkillSuffix returns the display suffix for a skill
func getSkillSuffix(s skillEntry) string {
	if s.RepoName != "" {
		return fmt.Sprintf("tracked: %s", s.RepoName)
	}
	if s.Source != "" {
		return abbreviateSource(s.Source)
	}
	return "local"
}

// displayTrackedRepos displays the tracked repositories section.
// Git status checks run in parallel (bounded by maxDirtyWorkers).
func displayTrackedRepos(trackedRepos []string, discovered []sync.DiscoveredSkill, sourcePath string) {
	fmt.Println()
	ui.Header("Tracked repositories")

	// Parallel git status checks
	const maxDirtyWorkers = 8
	type repoStatus struct {
		dirty bool
	}
	results := make([]repoStatus, len(trackedRepos))
	sem := make(chan struct{}, maxDirtyWorkers)
	var wg gosync.WaitGroup

	for i, repoName := range trackedRepos {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, name string) {
			defer wg.Done()
			defer func() { <-sem }()
			repoPath := filepath.Join(sourcePath, name)
			dirty, _ := isRepoDirty(repoPath)
			results[idx] = repoStatus{dirty: dirty}
		}(i, repoName)
	}
	wg.Wait()

	for i, repoName := range trackedRepos {
		skillCount := countRepoSkills(repoName, discovered)
		if results[i].dirty {
			ui.ListItem("warning", repoName, fmt.Sprintf("%d skills, has changes", skillCount))
		} else {
			ui.ListItem("success", repoName, fmt.Sprintf("%d skills, up-to-date", skillCount))
		}
	}
}

// countRepoSkills counts skills in a tracked repo
func countRepoSkills(repoName string, discovered []sync.DiscoveredSkill) int {
	count := 0
	for _, d := range discovered {
		if d.IsInRepo && strings.HasPrefix(d.RelPath, repoName+"/") {
			count++
		}
	}
	return count
}

func cmdList(args []string) error {
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

	opts, err := parseListArgs(rest)
	if opts.ShowHelp {
		printListHelp()
		return nil
	}
	if err != nil {
		return err
	}

	if mode == modeProject {
		return cmdListProject(cwd, opts)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// TTY + not JSON + not --no-tui → launch TUI with async loading (no blank screen)
	if !opts.JSON && !opts.NoTUI && isTTY {
		loadFn := func() listLoadResult {
			discovered, _, err := sync.DiscoverSourceSkillsLite(cfg.Source)
			if err != nil {
				return listLoadResult{err: fmt.Errorf("cannot discover skills: %w", err)}
			}
			skills := buildSkillEntries(discovered)
			total := len(skills)
			skills = filterSkillEntries(skills, opts.Pattern, opts.TypeFilter)
			if opts.SortBy != "" {
				sortSkillEntries(skills, opts.SortBy)
			}
			return listLoadResult{skills: toSkillItems(skills), totalCount: total}
		}
		action, skillName, err := runListTUI(loadFn, "global", cfg.Source, cfg.Targets)
		if err != nil {
			return err
		}
		switch action {
		case "empty":
			ui.Info("No skills installed")
			ui.Info("Use 'skillshare install <source>' to install a skill")
			return nil
		case "audit":
			return cmdAudit([]string{skillName})
		case "update":
			return cmdUpdate([]string{skillName})
		case "uninstall":
			return cmdUninstall([]string{skillName})
		}
		return nil
	}

	// Non-TUI path (JSON or plain text): synchronous loading with spinner
	var sp *ui.Spinner
	if !opts.JSON && isTTY {
		sp = ui.StartSpinner("Loading skills...")
	}
	discovered, trackedRepos, err := sync.DiscoverSourceSkillsLite(cfg.Source)
	if err != nil {
		if sp != nil {
			sp.Fail("Discovery failed")
		}
		return fmt.Errorf("cannot discover skills: %w", err)
	}

	if sp != nil {
		sp.Update(fmt.Sprintf("Reading metadata for %d skills...", len(discovered)))
	}
	skills := buildSkillEntries(discovered)
	if sp != nil {
		sp.Success(fmt.Sprintf("Loaded %d skills", len(skills)))
	}
	totalCount := len(skills)
	hasFilter := opts.Pattern != "" || opts.TypeFilter != ""

	// Apply filter and sort
	skills = filterSkillEntries(skills, opts.Pattern, opts.TypeFilter)
	if opts.SortBy != "" {
		sortSkillEntries(skills, opts.SortBy)
	}

	// JSON output
	if opts.JSON {
		return displaySkillsJSON(skills)
	}

	// Handle empty results before starting pager
	if len(skills) == 0 && len(trackedRepos) == 0 && !hasFilter {
		ui.Info("No skills installed")
		ui.Info("Use 'skillshare install <source>' to install a skill")
		return nil
	}

	if hasFilter && len(skills) == 0 {
		if opts.Pattern != "" && opts.TypeFilter != "" {
			ui.Info("No skills matching %q (type: %s)", opts.Pattern, opts.TypeFilter)
		} else if opts.Pattern != "" {
			ui.Info("No skills matching %q", opts.Pattern)
		} else {
			ui.Info("No skills matching type %q", opts.TypeFilter)
		}
		return nil
	}

	// Plain text output (--no-tui or non-TTY)
	if len(skills) > 0 {
		ui.Header("Installed skills")
		if opts.Verbose {
			displaySkillsVerbose(skills)
		} else {
			displaySkillsCompact(skills)
		}
	}

	// Hide tracked repos section when filter/pattern is active
	if len(trackedRepos) > 0 && !hasFilter {
		displayTrackedRepos(trackedRepos, discovered, cfg.Source)
	}

	// Show match stats when filter is active
	if hasFilter && len(skills) > 0 {
		fmt.Println()
		if opts.Pattern != "" {
			ui.Info("%d of %d skill(s) matching %q", len(skills), totalCount, opts.Pattern)
		} else {
			ui.Info("%d of %d skill(s)", len(skills), totalCount)
		}
	} else if !opts.Verbose && len(skills) > 0 {
		fmt.Println()
		ui.Info("Use --verbose for more details")
	}

	return nil
}

type skillEntry struct {
	Name        string
	Source      string
	Type        string
	InstalledAt string
	IsNested    bool
	RepoName    string
	RelPath     string
}

// skillJSON is the JSON representation for --json output.
type skillJSON struct {
	Name        string `json:"name"`
	RelPath     string `json:"relPath"`
	Source      string `json:"source,omitempty"`
	Type        string `json:"type,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
	RepoName    string `json:"repoName,omitempty"`
}

func displaySkillsJSON(skills []skillEntry) error {
	items := make([]skillJSON, len(skills))
	for i, s := range skills {
		items[i] = skillJSON{
			Name:        s.Name,
			RelPath:     s.RelPath,
			Source:      s.Source,
			Type:        s.Type,
			InstalledAt: s.InstalledAt,
			RepoName:    s.RepoName,
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}

// abbreviateSource shortens long sources for display
func abbreviateSource(source string) string {
	// Remove https:// prefix
	source = strings.TrimPrefix(source, "https://")
	source = strings.TrimPrefix(source, "http://")

	// Truncate if too long
	if len(source) > 50 {
		return source[:47] + "..."
	}
	return source
}

func printListHelp() {
	fmt.Println(`Usage: skillshare list [pattern] [options]

List all installed skills in the source directory.
An optional pattern filters skills by name, path, or source (case-insensitive).

Options:
  --verbose, -v          Show detailed information (source, type, install date)
  --json, -j             Output as JSON (useful for CI/scripts)
  --no-tui               Disable interactive TUI, use plain text output
  --type, -t <type>      Filter by type: tracked, local, github
  --sort, -s <order>     Sort order: name (default), newest, oldest
  --project, -p          Use project-level config in current directory
  --global, -g           Use global config (~/.config/skillshare)
  --help, -h             Show this help

Examples:
  skillshare list
  skillshare list react
  skillshare list --type local
  skillshare list react --type github --sort newest
  skillshare list --json | jq '.[].name'
  skillshare list --verbose`)
}
