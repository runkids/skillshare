package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lithammer/fuzzysearch/fuzzy"

	"skillshare/internal/install"
	"skillshare/internal/ui"
)

// largeRepoThreshold is the skill count above which the directory-based
// selection UI is shown instead of a flat MultiSelect.
const largeRepoThreshold = 50

// directoryGroup groups discovered skills by their parent directory.
type directoryGroup struct {
	dir    string
	skills []install.SkillInfo
}

// groupSkillsByDirectory groups skills by their first directory segment
// after stripping the given prefix. For example, with prefix "data" and
// path "data/subdir-a/skill", the group key is "subdir-a".
// Root-level skills (no directory segment after prefix) are grouped under "(root)".
// Groups are sorted alphabetically by directory name, with "(root)" first.
func groupSkillsByDirectory(skills []install.SkillInfo, prefix string) []directoryGroup {
	groupMap := make(map[string][]install.SkillInfo)
	for _, s := range skills {
		rel := s.Path
		if prefix != "" {
			rel = strings.TrimPrefix(rel, prefix+"/")
		}
		dir := strings.SplitN(rel, "/", 2)[0]
		// If no slash remained (single segment) or root path, it's a root-level skill
		if dir == rel || dir == "." || s.Path == "." {
			dir = "(root)"
		}
		groupMap[dir] = append(groupMap[dir], s)
	}

	groups := make([]directoryGroup, 0, len(groupMap))
	for dir, dirSkills := range groupMap {
		groups = append(groups, directoryGroup{dir: dir, skills: dirSkills})
	}

	sort.Slice(groups, func(i, j int) bool {
		// "(root)" always first
		if groups[i].dir == "(root)" {
			return true
		}
		if groups[j].dir == "(root)" {
			return false
		}
		return groups[i].dir < groups[j].dir
	})

	return groups
}

func promptSkillSelection(skills []install.SkillInfo) ([]install.SkillInfo, error) {
	// Check for orchestrator structure (root + children)
	var rootSkill *install.SkillInfo
	var childSkills []install.SkillInfo
	for i := range skills {
		if skills[i].Path == "." {
			rootSkill = &skills[i]
		} else {
			childSkills = append(childSkills, skills[i])
		}
	}

	// If orchestrator structure detected, use two-stage selection
	if rootSkill != nil && len(childSkills) > 0 {
		return promptOrchestratorSelection(*rootSkill, childSkills)
	}

	// Large repo: directory-based selection with search
	if len(skills) >= largeRepoThreshold {
		return promptLargeRepoSelection(skills, "")
	}

	// Otherwise, use standard multi-select
	return promptMultiSelect(skills)
}

func promptOrchestratorSelection(rootSkill install.SkillInfo, childSkills []install.SkillInfo) ([]install.SkillInfo, error) {
	// Stage 1: Choose install mode
	options := []string{
		fmt.Sprintf("Install entire pack  \033[90m%s + %d children\033[0m", rootSkill.Name, len(childSkills)),
		"Select individual skills",
	}

	var modeIdx int
	prompt := &survey.Select{
		Message:  "Install mode:",
		Options:  options,
		PageSize: 5,
	}

	err := survey.AskOne(prompt, &modeIdx, survey.WithIcons(func(icons *survey.IconSet) {
		icons.SelectFocus.Text = "▸"
		icons.SelectFocus.Format = "yellow"
	}))
	if err != nil {
		return nil, nil
	}

	// If "entire pack" selected, return all skills
	if modeIdx == 0 {
		allSkills := make([]install.SkillInfo, 0, len(childSkills)+1)
		allSkills = append(allSkills, rootSkill)
		allSkills = append(allSkills, childSkills...)
		return allSkills, nil
	}

	// Stage 2: Select individual skills (children only, no root)
	return promptMultiSelect(childSkills)
}

// promptLargeRepoSelection presents a TUI directory picker for large repos.
// The TUI supports multi-level navigation with backspace to go back.
// prefix is unused (kept for caller compatibility); the TUI manages its own prefix stack.
func promptLargeRepoSelection(skills []install.SkillInfo, _ string) ([]install.SkillInfo, error) {
	for {
		selected, installAll, err := runDirPickerTUI(skills)
		if err != nil {
			return nil, err
		}
		if selected == nil {
			return nil, nil // user cancelled from TUI
		}
		if installAll {
			return selected, nil
		}
		// Leaf directory — let user pick individual skills
		result, err := promptMultiSelect(selected)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
		// User cancelled from MultiSelect — loop back to TUI
	}
}

// promptSearchSelect lets the user search skills by name with fuzzy matching.
func promptSearchSelect(skills []install.SkillInfo) ([]install.SkillInfo, error) {
	skillNames := make([]string, len(skills))
	skillByName := make(map[string]install.SkillInfo, len(skills))
	for i, s := range skills {
		skillNames[i] = s.Name
		skillByName[s.Name] = s
	}

	for {
		var query string
		prompt := &survey.Input{
			Message: "Search skills (partial name match):",
		}
		if err := survey.AskOne(prompt, &query); err != nil {
			return nil, nil // user cancelled
		}
		query = strings.TrimSpace(query)
		if query == "" {
			ui.Warning("Please enter a search term")
			continue
		}

		ranks := fuzzy.RankFindNormalizedFold(query, skillNames)
		sort.Sort(ranks)

		if len(ranks) == 0 {
			ui.Warning("No skills matching %q — try a different search term", query)
			continue
		}
		if len(ranks) > largeRepoThreshold {
			ui.Warning("Too many matches (%d) — try a narrower search term", len(ranks))
			continue
		}

		matched := make([]install.SkillInfo, len(ranks))
		for i, r := range ranks {
			matched[i] = skillByName[r.Target]
		}
		ui.Info("Found %d matching skill(s)", len(matched))
		return promptMultiSelect(matched)
	}
}

func promptMultiSelect(skills []install.SkillInfo) ([]install.SkillInfo, error) {
	return runSkillSelectTUI(skills)
}

// selectSkills routes to the appropriate skill selection method:
// --skill filter, --all/--yes auto-select, or interactive prompt.
// Callers are expected to apply --exclude filtering before calling this function.
func selectSkills(skills []install.SkillInfo, opts install.InstallOptions) ([]install.SkillInfo, error) {
	switch {
	case opts.HasSkillFilter():
		matched, notFound := filterSkillsByName(skills, opts.Skills)
		if len(notFound) > 0 {
			return nil, fmt.Errorf("skills not found: %s\nAvailable: %s",
				strings.Join(notFound, ", "), skillNames(skills))
		}
		return matched, nil
	case opts.ShouldInstallAll():
		return skills, nil
	default:
		return promptSkillSelection(skills)
	}
}

// applyExclude removes skills whose names appear in the exclude list.
func applyExclude(skills []install.SkillInfo, exclude []string) []install.SkillInfo {
	excludeSet := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		excludeSet[name] = true
	}
	var excluded []string
	filtered := make([]install.SkillInfo, 0, len(skills))
	for _, s := range skills {
		if excludeSet[s.Name] {
			excluded = append(excluded, s.Name)
			continue
		}
		filtered = append(filtered, s)
	}
	if len(excluded) > 0 {
		ui.Info("Excluded %d skill(s): %s", len(excluded), strings.Join(excluded, ", "))
	}
	return filtered
}

// filterSkillsByName matches requested names against discovered skills.
// It tries exact match first, then falls back to fuzzy matching.
func filterSkillsByName(skills []install.SkillInfo, names []string) (matched []install.SkillInfo, notFound []string) {
	skillNames := make([]string, len(skills))
	for i, s := range skills {
		skillNames[i] = s.Name
	}
	skillByName := make(map[string]install.SkillInfo, len(skills))
	for _, s := range skills {
		skillByName[s.Name] = s
	}

	for _, name := range names {
		// Try exact match first
		if s, ok := skillByName[name]; ok {
			matched = append(matched, s)
			continue
		}

		// Fall back to fuzzy match
		ranks := fuzzy.RankFindNormalizedFold(name, skillNames)
		sort.Sort(ranks)
		if len(ranks) == 1 {
			matched = append(matched, skillByName[ranks[0].Target])
		} else if len(ranks) > 1 {
			suggestions := make([]string, len(ranks))
			for i, r := range ranks {
				suggestions[i] = r.Target
			}
			notFound = append(notFound, fmt.Sprintf("%s (did you mean: %s?)", name, strings.Join(suggestions, ", ")))
		} else {
			notFound = append(notFound, name)
		}
	}
	return
}

// skillNames returns a comma-separated list of skill names for error messages.
func skillNames(skills []install.SkillInfo) string {
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}

// printSkillListCompact prints a list of skills with compression for large lists.
// ≤20 skills: print each with SkillBoxCompact. >20: first 10 + "... and N more".
func printSkillListCompact(skills []install.SkillInfo) {
	const threshold = 20
	const showCount = 10

	if len(skills) <= threshold {
		for _, skill := range skills {
			ui.SkillBoxCompact(skill.Name, skill.Path)
		}
		return
	}

	for i := 0; i < showCount; i++ {
		ui.SkillBoxCompact(skills[i].Name, skills[i].Path)
	}
	ui.Info("... and %d more skill(s)", len(skills)-showCount)
}
