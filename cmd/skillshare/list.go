package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

// buildSkillEntries builds skill entries from discovered skills
func buildSkillEntries(discovered []sync.DiscoveredSkill) []skillEntry {
	var skills []skillEntry
	for _, d := range discovered {
		entry := skillEntry{
			Name:     d.FlatName,
			IsNested: d.IsInRepo || utils.HasNestedSeparator(d.FlatName),
			RelPath:  d.RelPath,
		}

		if d.IsInRepo {
			parts := strings.SplitN(d.RelPath, "/", 2)
			if len(parts) > 0 {
				entry.RepoName = parts[0]
			}
		}

		if meta, err := install.ReadMeta(d.SourcePath); err == nil && meta != nil {
			entry.Source = meta.Source
			entry.Type = meta.Type
			entry.InstalledAt = meta.InstalledAt.Format("2006-01-02")
		}

		skills = append(skills, entry)
	}
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

// displayTrackedRepos displays the tracked repositories section
func displayTrackedRepos(trackedRepos []string, discovered []sync.DiscoveredSkill, sourcePath string) {
	fmt.Println()
	ui.Header("Tracked repositories")

	for _, repoName := range trackedRepos {
		repoPath := filepath.Join(sourcePath, repoName)
		skillCount := countRepoSkills(repoName, discovered)

		if isDirty, _ := isRepoDirty(repoPath); isDirty {
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

	discovered, err := sync.DiscoverSourceSkills(cfg.Source)
	if err != nil {
		return fmt.Errorf("cannot discover skills: %w", err)
	}

	trackedRepos, _ := install.GetTrackedRepos(cfg.Source)
	skills := buildSkillEntries(discovered)
	totalCount := len(skills)
	hasFilter := opts.Pattern != "" || opts.TypeFilter != ""

	// Apply filter and sort
	skills = filterSkillEntries(skills, opts.Pattern, opts.TypeFilter)
	if opts.SortBy != "" {
		sortSkillEntries(skills, opts.SortBy)
	}

	// JSON output — skip pager and text formatting
	if opts.JSON {
		return displaySkillsJSON(skills)
	}

	// TTY + not --no-tui + has skills → launch interactive TUI
	if !opts.NoTUI && len(skills) > 0 && term.IsTerminal(int(os.Stdout.Fd())) {
		items := toSkillItems(skills)
		return runListTUI(items, totalCount, "global", cfg.Source, cfg.Targets)
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
