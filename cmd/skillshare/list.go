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
	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/resource"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
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
			Kind:     "skill",
			IsNested: d.IsInRepo || utils.HasNestedSeparator(d.FlatName),
			RelPath:  d.RelPath,
			Disabled: d.Disabled,
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
				skills[idx].Branch = meta.Branch
			}
		}(i, d.SourcePath)
	}
	wg.Wait()

	// Fallback: for tracked-repo skills with no branch in metadata, read from git.
	// Cache per-repo to avoid repeated subprocess calls for skills in the same repo.
	repoBranchCache := make(map[string]string)
	for i, d := range discovered {
		if skills[i].Branch == "" && skills[i].RepoName != "" {
			if cached, ok := repoBranchCache[skills[i].RepoName]; ok {
				skills[i].Branch = cached
				continue
			}
			sourceDir := strings.TrimSuffix(d.SourcePath, d.RelPath)
			repoPath := filepath.Join(sourceDir, skills[i].RepoName)
			if branch, err := git.GetCurrentBranch(repoPath); err == nil {
				repoBranchCache[skills[i].RepoName] = branch
				skills[i].Branch = branch
			}
		}
	}

	return skills
}

// discoverAndBuildAgentEntries discovers agents from the given source directory
// and builds skillEntry items with Kind="agent". Reads sidecar metadata for
// installed agents (<name>.skillshare-meta.json).
func discoverAndBuildAgentEntries(agentsSource string) []skillEntry {
	if agentsSource == "" {
		return nil
	}
	discovered, err := resource.AgentKind{}.Discover(agentsSource)
	if err != nil {
		return nil
	}

	entries := make([]skillEntry, len(discovered))
	for i, d := range discovered {
		entries[i] = skillEntry{
			Name:     d.Name,
			Kind:     "agent",
			RelPath:  d.RelPath,
			IsNested: d.IsNested,
			Disabled: d.Disabled,
		}
		// Read sidecar metadata: <basename>.skillshare-meta.json (alongside the .md file)
		baseName := strings.TrimSuffix(filepath.Base(d.RelPath), ".md")
		metaPath := filepath.Join(filepath.Dir(d.SourcePath), baseName+".skillshare-meta.json")
		if data, readErr := os.ReadFile(metaPath); readErr == nil {
			var meta install.SkillMeta
			if jsonErr := json.Unmarshal(data, &meta); jsonErr == nil {
				entries[i].Source = meta.Source
				entries[i].Type = meta.Type
				if !meta.InstalledAt.IsZero() {
					entries[i].InstalledAt = meta.InstalledAt.Format("2006-01-02")
				}
			}
		}
	}
	return entries
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
			fmt.Printf("  %s%s/%s\n", ui.Dim, dir, ui.Reset)
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
	if s.Disabled {
		fmt.Printf("%s%sStatus:%s      %sdisabled%s\n", indent, ui.Dim, ui.Reset, ui.Dim, ui.Reset)
	}
	if s.RepoName != "" {
		fmt.Printf("%s%sTracked repo:%s %s\n", indent, ui.Dim, ui.Reset, s.RepoName)
	}
	if s.Source != "" {
		fmt.Printf("%s%sSource:%s      %s\n", indent, ui.Dim, ui.Reset, s.Source)
		fmt.Printf("%s%sType:%s        %s\n", indent, ui.Dim, ui.Reset, s.Type)
		fmt.Printf("%s%sInstalled:%s   %s\n", indent, ui.Dim, ui.Reset, s.InstalledAt)
	} else {
		fmt.Printf("%s%sSource:%s      (local - no metadata)\n", indent, ui.Dim, ui.Reset)
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
			format := fmt.Sprintf("  %s→%s %%-%ds  %s%%s%s\n", ui.Cyan, ui.Reset, maxNameLen, ui.Dim, ui.Reset)
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
			fmt.Printf("  %s%s/%s\n", ui.Dim, dir, ui.Reset)
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
				format := fmt.Sprintf("    %s→%s %%-%ds  %s%%s%s\n", ui.Cyan, ui.Reset, maxNameLen, ui.Dim, ui.Reset)
				fmt.Printf(format, name, suffix)
			} else {
				format := fmt.Sprintf("  %s→%s %%-%ds  %s%%s%s\n", ui.Cyan, ui.Reset, maxNameLen, ui.Dim, ui.Reset)
				fmt.Printf(format, name, suffix)
			}
		}
	}
}

// getSkillSuffix returns the display suffix for a skill
func getSkillSuffix(s skillEntry) string {
	var suffix string
	if s.RepoName != "" {
		suffix = fmt.Sprintf("tracked: %s", s.RepoName)
	} else if s.Source != "" {
		suffix = abbreviateSource(s.Source)
	} else {
		suffix = "local"
	}
	if s.Disabled {
		suffix += "  [disabled]"
	}
	return suffix
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
			dirty, _ := git.IsDirty(repoPath)
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

	// Extract kind filter (e.g. "skillshare list agents").
	kind, rest := parseKindArg(rest)

	opts, err := parseListArgs(rest)
	if opts.ShowHelp {
		printListHelp()
		return nil
	}
	if err != nil {
		return err
	}

	if mode == modeProject {
		return cmdListProject(cwd, opts, kind)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// TTY + not JSON + TUI enabled → launch TUI with async loading (no blank screen)
	if !opts.JSON && shouldLaunchTUI(opts.NoTUI, cfg) {
		loadFn := func() listLoadResult {
			// Always load both skills and agents — tab UI filters the view.
			var allEntries []skillEntry
			discovered, discErr := sync.DiscoverSourceSkillsAll(cfg.Source)
			if discErr != nil {
				return listLoadResult{err: fmt.Errorf("cannot discover skills: %w", discErr)}
			}
			allEntries = append(allEntries, buildSkillEntries(discovered)...)
			allEntries = append(allEntries, discoverAndBuildAgentEntries(cfg.EffectiveAgentsSource())...)
			total := len(allEntries)
			allEntries = filterSkillEntries(allEntries, opts.Pattern, opts.TypeFilter)
			if opts.SortBy != "" {
				sortSkillEntries(allEntries, opts.SortBy)
			}
			return listLoadResult{skills: toSkillItems(allEntries), totalCount: total}
		}
		action, skillName, skillKind, err := runListTUI(loadFn, "global", cfg.Source, cfg.EffectiveAgentsSource(), cfg.Targets, kind)
		if err != nil {
			return err
		}
		switch action {
		case "empty":
			resourceLabel := "skills"
			if kind == kindAgents {
				resourceLabel = "agents"
			}
			ui.Info("No %s installed", resourceLabel)
			return nil
		case "audit":
			if skillKind == "agent" {
				return cmdAudit([]string{"agents", "-g", skillName})
			}
			return cmdAudit([]string{"-g", skillName})
		case "update":
			if skillKind == "agent" {
				return cmdUpdate([]string{"agents", "-g", skillName})
			}
			return cmdUpdate([]string{"-g", skillName})
		case "uninstall":
			if skillKind == "agent" {
				return cmdUninstall([]string{"agents", "-g", "--force", skillName})
			}
			return cmdUninstall([]string{"-g", "--force", skillName})
		}
		return nil
	}

	// Non-TUI path (JSON or plain text): synchronous loading with spinner
	resourceLabel := "skills"
	if kind == kindAgents {
		resourceLabel = "agents"
	} else if kind == kindAll {
		resourceLabel = "resources"
	}

	var sp *ui.Spinner
	if !opts.JSON && ui.IsTTY() {
		sp = ui.StartSpinner(fmt.Sprintf("Loading %s...", resourceLabel))
	}

	var allEntries []skillEntry
	var trackedRepos []string
	var discoveredSkills []sync.DiscoveredSkill

	if kind.IncludesSkills() {
		var discErr error
		discoveredSkills, discErr = sync.DiscoverSourceSkillsAll(cfg.Source)
		if discErr != nil {
			if sp != nil {
				sp.Fail("Discovery failed")
			}
			return fmt.Errorf("cannot discover skills: %w", discErr)
		}
		trackedRepos = extractTrackedRepos(discoveredSkills)
		if sp != nil {
			sp.Update(fmt.Sprintf("Reading metadata for %d skills...", len(discoveredSkills)))
		}
		allEntries = append(allEntries, buildSkillEntries(discoveredSkills)...)
	}

	if kind.IncludesAgents() {
		agentEntries := discoverAndBuildAgentEntries(cfg.EffectiveAgentsSource())
		allEntries = append(allEntries, agentEntries...)
	}

	if sp != nil {
		sp.Success(fmt.Sprintf("Loaded %d %s", len(allEntries), resourceLabel))
	}
	totalCount := len(allEntries)
	hasFilter := opts.Pattern != "" || opts.TypeFilter != ""

	// Apply filter and sort
	allEntries = filterSkillEntries(allEntries, opts.Pattern, opts.TypeFilter)
	if opts.SortBy != "" {
		sortSkillEntries(allEntries, opts.SortBy)
	}

	// JSON output
	if opts.JSON {
		return displaySkillsJSON(allEntries)
	}

	// Handle empty results
	if len(allEntries) == 0 && len(trackedRepos) == 0 && !hasFilter {
		ui.Info("No %s installed", resourceLabel)
		if kind.IncludesSkills() {
			ui.Info("Use 'skillshare install <source>' to install a skill")
		}
		return nil
	}

	if hasFilter && len(allEntries) == 0 {
		if opts.Pattern != "" && opts.TypeFilter != "" {
			ui.Info("No %s matching %q (type: %s)", resourceLabel, opts.Pattern, opts.TypeFilter)
		} else if opts.Pattern != "" {
			ui.Info("No %s matching %q", resourceLabel, opts.Pattern)
		} else {
			ui.Info("No %s matching type %q", resourceLabel, opts.TypeFilter)
		}
		return nil
	}

	// Plain text output (--no-tui or non-TTY)
	if len(allEntries) > 0 {
		headerLabel := "Installed skills"
		if kind == kindAgents {
			headerLabel = "Installed agents"
		} else if kind == kindAll {
			headerLabel = "Installed skills & agents"
		}
		ui.Header(headerLabel)
		if opts.Verbose {
			displaySkillsVerbose(allEntries)
		} else {
			displaySkillsCompact(allEntries)
		}
	}

	// Hide tracked repos section when filter/pattern is active
	if len(trackedRepos) > 0 && !hasFilter {
		displayTrackedRepos(trackedRepos, discoveredSkills, cfg.Source)
	}

	// Show match stats when filter is active
	if hasFilter && len(allEntries) > 0 {
		fmt.Println()
		if opts.Pattern != "" {
			ui.Info("%d of %d %s matching %q", len(allEntries), totalCount, resourceLabel, opts.Pattern)
		} else {
			ui.Info("%d of %d %s", len(allEntries), totalCount, resourceLabel)
		}
	} else if !opts.Verbose && len(allEntries) > 0 {
		fmt.Println()
		ui.Info("Use --verbose for more details")
	}

	return nil
}

type skillEntry struct {
	Name        string
	Kind        string // "skill" or "agent"
	Source      string
	Type        string
	InstalledAt string
	IsNested    bool
	RepoName    string
	RelPath     string
	Disabled    bool
	Branch      string
}

// skillJSON is the JSON representation for --json output.
type skillJSON struct {
	Name        string `json:"name"`
	Kind        string `json:"kind,omitempty"` // "skill" or "agent"
	RelPath     string `json:"relPath"`
	Source      string `json:"source,omitempty"`
	Type        string `json:"type,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
	RepoName    string `json:"repoName,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
}

func displaySkillsJSON(skills []skillEntry) error {
	items := make([]skillJSON, len(skills))
	for i, s := range skills {
		items[i] = skillJSON{
			Name:        s.Name,
			Kind:        s.Kind,
			RelPath:     s.RelPath,
			Source:      s.Source,
			Type:        s.Type,
			InstalledAt: s.InstalledAt,
			RepoName:    s.RepoName,
			Disabled:    s.Disabled,
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
	fmt.Println(`Usage: skillshare list [agents|all] [pattern] [options]

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
  skillshare list --verbose
  skillshare list agents                       # List agents only
  skillshare list all                          # List skills + agents`)
}
