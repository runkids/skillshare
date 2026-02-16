package main

import (
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
)

// parseListArgs parses list command arguments
func parseListArgs(args []string) (verbose bool, showHelp bool, err error) {
	for _, arg := range args {
		switch arg {
		case "--verbose", "-v":
			verbose = true
		case "--help", "-h":
			return false, true, nil
		default:
			if strings.HasPrefix(arg, "-") {
				return false, false, fmt.Errorf("unknown option: %s", arg)
			}
		}
	}
	return verbose, false, nil
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

	verbose, showHelp, err := parseListArgs(rest)
	if showHelp {
		printListHelp()
		return nil
	}
	if err != nil {
		return err
	}

	if mode == modeProject {
		_ = verbose
		return cmdListProject(cwd)
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

	if len(skills) == 0 && len(trackedRepos) == 0 {
		ui.Info("No skills installed")
		ui.Info("Use 'skillshare install <source>' to install a skill")
		return nil
	}

	if len(skills) > 0 {
		ui.Header("Installed skills")
		if verbose {
			displaySkillsVerbose(skills)
		} else {
			displaySkillsCompact(skills)
		}
	}

	if len(trackedRepos) > 0 {
		displayTrackedRepos(trackedRepos, discovered, cfg.Source)
	}

	if !verbose && len(skills) > 0 {
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
	fmt.Println(`Usage: skillshare list [options]

List all installed skills in the source directory.

Options:
  --verbose, -v   Show detailed information (source, type, install date)
  --project, -p   Use project-level config in current directory
  --global, -g    Use global config (~/.config/skillshare)
  --help, -h      Show this help

Examples:
  skillshare list
  skillshare list --verbose`)
}
