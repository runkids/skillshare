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
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
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
	Status     listStatusFilter
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

// validStatusFilters maps accepted --status values to their typed filter.
var validStatusFilters = map[string]listStatusFilter{
	"all":      statusFilterAll,
	"enabled":  statusFilterEnabled,
	"disabled": statusFilterDisabled,
}

// parseStatusFilter validates a --status value and normalizes it.
func parseStatusFilter(raw string) (listStatusFilter, error) {
	v := strings.ToLower(raw)
	status, ok := validStatusFilters[v]
	if !ok {
		return statusFilterAll, fmt.Errorf("invalid status %q: must be all, enabled, or disabled", raw)
	}
	return status, nil
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
		case arg == "--status":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--status requires a value (all, enabled, disabled)")
			}
			v, err := parseStatusFilter(args[i])
			if err != nil {
				return opts, err
			}
			opts.Status = v
		case strings.HasPrefix(arg, "--status="):
			v, err := parseStatusFilter(strings.TrimPrefix(arg, "--status="))
			if err != nil {
				return opts, err
			}
			opts.Status = v
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

// filterSkillEntries filters skills by pattern, type, and status.
// Pattern matches case-insensitively against Name, RelPath, and Source.
// Status uses statusFilterAll for no filter; filters combine with AND.
func filterSkillEntries(skills []skillEntry, pattern, typeFilter string, status listStatusFilter) []skillEntry {
	if pattern == "" && typeFilter == "" && status == statusFilterAll {
		return skills
	}

	pat := strings.ToLower(pattern)
	var result []skillEntry
	for _, s := range skills {
		// Status filter
		if status != statusFilterAll && s.Disabled != (status == statusFilterDisabled) {
			continue
		}

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

// statusNote returns a " (status: enabled)" suffix for footers and messages,
// or "" when no status filter is active.
func statusNote(status listStatusFilter) string {
	if status == statusFilterAll {
		return ""
	}
	return fmt.Sprintf(" (status: %s)", status.String())
}

// noMatchMessage builds the "no results" message for the active filter set.
func noMatchMessage(resourceLabel string, opts listOptions) string {
	switch {
	case opts.Pattern != "" && opts.TypeFilter != "":
		return fmt.Sprintf("No %s matching %q (type: %s)%s", resourceLabel, opts.Pattern, opts.TypeFilter, statusNote(opts.Status))
	case opts.Pattern != "":
		return fmt.Sprintf("No %s matching %q%s", resourceLabel, opts.Pattern, statusNote(opts.Status))
	case opts.TypeFilter != "":
		return fmt.Sprintf("No %s matching type %q%s", resourceLabel, opts.TypeFilter, statusNote(opts.Status))
	default:
		return fmt.Sprintf("No %s matching status %q", resourceLabel, opts.Status.String())
	}
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
	default: // "name" or empty — group by top-level bucket, then by RelPath
		sort.SliceStable(skills, func(i, j int) bool {
			ti := skillTopGroup(skills[i])
			tj := skillTopGroup(skills[j])
			if ti != tj {
				// Empty topGroup (standalone) sorts last so flat locals
				// don't split named groups apart in the list TUI.
				if ti == "" {
					return false
				}
				if tj == "" {
					return true
				}
				return ti < tj
			}
			return skills[i].RelPath < skills[j].RelPath
		})
	}
}

// buildSkillEntries builds skill entries from discovered skills.
// Metadata is read from the centralized .metadata.json store.
func buildSkillEntries(discovered []sync.DiscoveredSkill) []skillEntry {
	skills := make([]skillEntry, len(discovered))

	store := install.NewMetadataStore()
	if len(discovered) > 0 {
		sourceDir := strings.TrimSuffix(discovered[0].SourcePath, discovered[0].RelPath)
		sourceDir = strings.TrimRight(sourceDir, `/\`)
		store = install.LoadMetadataOrNew(sourceDir)
	}

	// Pre-fill non-I/O fields + metadata from store
	for i, d := range discovered {
		skills[i] = skillEntry{
			Name:     d.FlatName,
			Kind:     "skill",
			IsNested: d.IsInRepo || utils.HasNestedSeparator(d.FlatName),
			RelPath:  d.RelPath,
			Disabled: d.Disabled,
		}
		if d.IsInRepo {
			skills[i].RepoName = d.RepoRelPath
		}

		// Enrich from centralized metadata store
		if entry := store.GetByPath(d.RelPath); entry != nil {
			skills[i].Source = entry.Source
			skills[i].Type = entry.Type
			if !entry.InstalledAt.IsZero() {
				skills[i].InstalledAt = entry.InstalledAt.Format("2006-01-02")
			}
			skills[i].Branch = entry.Branch
		}
	}

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
// and builds skillEntry items with Kind="agent", enriched from the centralized
// metadata store.
func discoverAndBuildAgentEntries(agentsSource string) []skillEntry {
	if agentsSource == "" {
		return nil
	}
	discovered, err := resource.AgentKind{}.Discover(agentsSource)
	if err != nil {
		return nil
	}

	store := install.LoadMetadataOrNew(agentsSource)

	entries := make([]skillEntry, len(discovered))
	for i, d := range discovered {
		entries[i] = skillEntry{
			Name:     d.Name,
			Kind:     "agent",
			RelPath:  d.RelPath,
			IsNested: d.IsNested,
			Disabled: d.Disabled,
		}
		// For tracked agents, set RepoName to the tracked repo root
		// (e.g. "_vijaythecoder-awesome-claude-agents") so all agents under
		// the same repo share one visual group — regardless of how deeply
		// they're nested (agents/core, agents/specialized/python, etc.).
		if d.IsInRepo && d.RepoRelPath != "" {
			entries[i].RepoName = d.RepoRelPath
		}
		key := strings.TrimSuffix(d.RelPath, ".md")
		if entry := store.GetByPath(key); entry != nil {
			entries[i].Source = entry.Source
			entries[i].Type = entry.Type
			if !entry.InstalledAt.IsZero() {
				entries[i].InstalledAt = entry.InstalledAt.Format("2006-01-02")
			}
		} else if d.RepoRelPath != "" {
			repoPath := filepath.Join(agentsSource, filepath.FromSlash(d.RepoRelPath))
			if repoURL, err := git.GetRemoteURL(repoPath); err == nil {
				entries[i].Source = repoURL
				entries[i].Type = "git"
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

// listRows orders skills by group and returns each one's row label,
// indented under its group, plus the group headings ("frontend/") keyed by
// the index of their first skill.
func listRows(skills []skillEntry) (ordered []skillEntry, labels []string, headings map[int]string) {
	headings = map[int]string{}
	if !hasGroups(skills) {
		for _, s := range skills {
			labels = append(labels, s.Name)
		}
		return skills, labels, headings
	}
	dirs, groups := groupSkillEntries(skills)
	for _, dir := range dirs {
		if dir != "" {
			headings[len(labels)] = dir + "/"
		}
		for _, s := range groups[dir] {
			label := displayName(s, dir)
			if dir != "" {
				label = "  " + label
			}
			labels = append(labels, label)
			ordered = append(ordered, s)
		}
	}
	return ordered, labels, headings
}

// displaySkillsVerbose lists each skill with its source, type and install
// date on the rows below it.
func displaySkillsVerbose(skills []skillEntry) {
	skills, labels, headings := listRows(skills)
	for i, s := range skills {
		if h, ok := headings[i]; ok {
			fmt.Println("  " + ui.DimText(h))
		}
		fmt.Println("  " + theme.Primary().Bold(true).Render(labels[i]))
		indent := strings.Repeat(" ", len(labels[i])-len(strings.TrimLeft(labels[i], " "))+2)
		var rows [][2]string
		if s.Disabled {
			rows = append(rows, [2]string{"Status", "disabled"})
		}
		if s.RepoName != "" {
			rows = append(rows, [2]string{"Repo", s.RepoName})
		}
		if s.Source != "" {
			rows = append(rows, [2]string{"Source", s.Source}, [2]string{"Type", s.Type}, [2]string{"Installed", s.InstalledAt})
		} else if s.RepoName == "" {
			rows = append(rows, [2]string{"Source", "local"})
		}
		for _, r := range rows {
			ui.Row(ui.MarkNone, indent+r[0], ui.DimText(r[1]), len(indent)+9)
		}
	}
}

// displaySkillsCompact lists one skill per row with where it came from.
func displaySkillsCompact(skills []skillEntry) {
	skills, labels, headings := listRows(skills)
	width := ui.RowWidth(labels...)
	for i, s := range skills {
		if h, ok := headings[i]; ok {
			fmt.Println("  " + ui.DimText(h))
		}
		ui.Row(ui.MarkNone, labels[i], ui.DimText(getSkillSuffix(s)), width)
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
		suffix += " · disabled"
	}
	return suffix
}

// displayTrackedRepos displays the tracked repositories section.
// Git status checks run in parallel (bounded by maxDirtyWorkers).
func displayTrackedRepos(trackedRepos []string, discovered []sync.DiscoveredSkill, sourcePath string, follow *sourcewalk.FollowSet) {
	ui.Section("Tracked repos")

	// Parallel git status checks
	const maxDirtyWorkers = 8
	type repoStatus struct {
		dirty bool
		err   error
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
			dirty, err := git.IsDirty(repoPath)
			results[idx] = repoStatus{dirty: dirty, err: err}
		}(i, repoName)
	}
	wg.Wait()

	width := ui.RowWidth(trackedRepos...)
	for i, repoName := range trackedRepos {
		skills := ui.DimText(" · " + plural(countRepoSkills(repoName, discovered), "skill"))
		if follow != nil {
			if entry, ok := follow.InFollowed(repoName); ok && entry.State == sourcewalk.Followed {
				skills += ui.DimText(" → " + utils.FoldHomePath(entry.ResolvedTarget))
			}
		}
		if err := results[i].err; err != nil {
			ui.Row(ui.MarkWarn, repoName, "git status unknown"+skills, width)
			ui.Note((&gitStatusError{err: err}).Error())
		} else if results[i].dirty {
			ui.Row(ui.MarkWarn, repoName, "has changes"+skills, width)
		} else {
			ui.Row(ui.MarkOK, repoName, "up to date"+skills, width)
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

	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	// Extract kind filter (e.g. "skillshare list agents" or "--all").
	kind, rest := parseKindArgWithAll(rest)

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

	follow := globalSkillFollowSet(cfg)
	// TTY + not JSON + TUI enabled → launch TUI with async loading (no blank screen)
	if !opts.JSON && shouldLaunchTUI(opts.NoTUI, cfg) {
		loadFn := func() listLoadResult {
			// Always load both skills and agents — tab UI filters the view.
			var allEntries []skillEntry
			discovered, _, discErr := sync.DiscoverSourceSkillsWithOptions(cfg.EffectiveSkillsSource(), sync.DiscoveryOptions{Follow: follow, IncludeIgnored: true})
			if discErr != nil {
				return listLoadResult{err: fmt.Errorf("cannot discover skills: %w", discErr)}
			}
			allEntries = append(allEntries, buildSkillEntries(discovered)...)
			allEntries = append(allEntries, discoverAndBuildAgentEntries(cfg.EffectiveAgentsSource())...)
			total := len(allEntries)
			// Status is NOT filtered here — the TUI needs both enabled and
			// disabled entries loaded so the `s` key can restore them.
			allEntries = filterSkillEntries(allEntries, opts.Pattern, opts.TypeFilter, statusFilterAll)
			// Always sort: buildGroupedItems relies on contiguous top-group
			// blocks, which the default sort guarantees (tracked → named
			// locals → standalone).
			sortSkillEntries(allEntries, opts.SortBy)
			return listLoadResult{skills: toSkillItems(allEntries), totalCount: total}
		}
		action, skillName, skillKind, err := runListTUI(loadFn, "global", cfg.EffectiveSkillsSource(), cfg.EffectiveAgentsSource(), cfg.Targets, kind, opts.Status)
		if err != nil {
			return err
		}
		switch action {
		case "empty":
			resourceLabel := "skills"
			if kind == kindAgents {
				resourceLabel = "agents"
			}
			printListEmpty(resourceLabel, kind, false)
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
		discoveredSkills, _, discErr = sync.DiscoverSourceSkillsWithOptions(cfg.EffectiveSkillsSource(), sync.DiscoveryOptions{Follow: follow, IncludeIgnored: true})
		if discErr != nil {
			if sp != nil {
				sp.Fail("Discovery failed")
			}
			return fmt.Errorf("cannot discover skills: %w", discErr)
		}
		trackedRepos = extractTrackedReposWithFollow(cfg.EffectiveSkillsSource(), follow)
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
		sp.Stop()
	}
	totalCount := len(allEntries)
	// Apply filter and sort
	allEntries = filterSkillEntries(allEntries, opts.Pattern, opts.TypeFilter, opts.Status)
	// Always sort so the TUI and plain-text renderers see contiguous
	// top-level groups (tracked → named locals → standalone).
	sortSkillEntries(allEntries, opts.SortBy)

	// JSON output
	if opts.JSON {
		return displaySkillsJSON(allEntries)
	}

	printSkillList(skillList{
		entries: allEntries, total: totalCount, trackedRepos: trackedRepos,
		discovered: discoveredSkills, skillsSource: cfg.EffectiveSkillsSource(), follow: follow,
		label: resourceLabel, kind: kind, opts: opts,
	})
	return nil
}

// skillList is what the plain list output shows.
type skillList struct {
	follow       *sourcewalk.FollowSet
	entries      []skillEntry
	total        int
	trackedRepos []string
	discovered   []sync.DiscoveredSkill
	skillsSource string
	label        string // "skills", "agents" or "resources"
	kind         resourceKindFilter
	opts         listOptions
	project      bool
}

// printSkillList prints the plain (--no-tui or non-TTY) list for global and
// project mode.
func printSkillList(l skillList) {
	hasFilter := l.opts.Pattern != "" || l.opts.TypeFilter != "" || l.opts.Status != statusFilterAll
	if len(l.entries) == 0 && len(l.trackedRepos) == 0 && !hasFilter {
		printListEmpty(l.label, l.kind, l.project)
		return
	}
	if hasFilter && len(l.entries) == 0 {
		ui.Done(ui.MarkNone, noMatchMessage(l.label, l.opts), 0)
		return
	}

	if len(l.entries) > 0 {
		title := "Skills"
		if l.kind == kindAgents {
			title = "Agents"
		} else if l.kind == kindAll {
			title = "Skills and agents"
		}
		title = theme.Primary().Bold(true).Render(title)
		if l.project {
			title += ui.DimText(" · project")
		}
		fmt.Println(title)
		if l.opts.Verbose {
			displaySkillsVerbose(l.entries)
		} else {
			displaySkillsCompact(l.entries)
		}
	}

	// Hide tracked repos section when filter/pattern is active
	if len(l.trackedRepos) > 0 && !hasFilter {
		displayTrackedRepos(l.trackedRepos, l.discovered, l.skillsSource, l.follow)
	}

	fmt.Println()
	if hasFilter {
		summary := fmt.Sprintf("%d of %d %s", len(l.entries), l.total, l.label)
		if l.opts.Pattern != "" {
			summary += fmt.Sprintf(" matching %q", l.opts.Pattern)
		}
		ui.Done(ui.MarkNone, summary+statusNote(l.opts.Status), 0)
		return
	}
	tracked, remote := 0, 0
	for _, e := range l.entries {
		if e.RepoName != "" {
			tracked++
		} else if e.Source != "" {
			remote++
		}
	}
	var parts []string
	for _, c := range []struct {
		n    int
		word string
	}{{tracked, "tracked"}, {remote, "remote"}, {len(l.entries) - tracked - remote, "local"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.word))
		}
	}
	summary := plural(len(l.entries), strings.TrimSuffix(l.label, "s"))
	if len(parts) > 1 {
		summary += ui.DimText(" · " + strings.Join(parts, ", "))
	}
	ui.Done(ui.MarkNone, summary, 0)
	if !l.opts.Verbose && len(l.entries) > 0 {
		ui.Note("Add -v for sources and install dates")
	}
}

// printListEmpty says nothing is installed and how to install a skill.
func printListEmpty(label string, kind resourceKindFilter, project bool) {
	ui.Done(ui.MarkNone, "No "+label+" installed", 0)
	if kind.IncludesSkills() {
		cmd := "skillshare install <source>"
		if project {
			cmd += " -p"
		}
		ui.Next(cmd, "install a skill")
	}
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
	printHelp("skillshare list [agents] [pattern] [options]", "List all installed skills in the source directory.\nAn optional pattern filters skills by name, path, or source (case-insensitive).\nThe default view (--status all) includes entries marked disabled.",
		helpGroup{title: "Options", rows: []helpRow{
			{"--all", "List both skills and agents"},
			{"-v, --verbose", "Show detailed information (source, type, install date)"},
			{"-j, --json", "Output as JSON (useful for CI/scripts)"},
			{"--no-tui", "Disable interactive TUI, use plain text output"},
			{"-t, --type <type>", "Filter by type: tracked, local, github"},
			{"--status <status>", "Filter by status: all (default), enabled, disabled"},
			{"-s, --sort <order>", "Sort order: name (default), newest, oldest"},
			{"-p, --project", "Use project-level config in current directory"},
			{"-g, --global", "Use global config (~/.config/skillshare)"},
		}},
		helpExamples(
			helpRow{"skillshare list", ""},
			helpRow{"skillshare list react", ""},
			helpRow{"skillshare list --type local", ""},
			helpRow{"skillshare list --status disabled", ""},
			helpRow{"skillshare list --status enabled --json", ""},
			helpRow{"skillshare list react --type github --sort newest", ""},
			helpRow{"skillshare list --json | jq '.[].name'", ""},
			helpRow{"skillshare list --verbose", ""},
			helpRow{"skillshare list agents", "List agents only"},
			helpRow{"skillshare list --all", "List skills + agents"},
		),
	)
}
