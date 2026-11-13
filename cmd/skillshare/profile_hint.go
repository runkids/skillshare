package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"skillshare/internal/profile"
	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
)

type profileConfigView interface {
	ProfileSelector() profile.Selector
}

func profileHint(cfg profileConfigView, flatNames []string) string {
	selector := cfg.ProfileSelector()
	if !selector.Active() {
		return ""
	}
	var missing []string
	for _, name := range flatNames {
		selected, err := selector.Selects(name)
		if err != nil {
			return ""
		}
		if !selected {
			missing = append(missing, shellArg(name))
		}
	}
	if len(missing) == 0 {
		return ""
	}
	command := fmt.Sprintf("skillshare profile add %s %s", shellArg(selector.Name()), strings.Join(missing, " "))
	if runtime.GOOS == "windows" && strings.ContainsAny(command, "$`%!") {
		command = "PowerShell: " + command
	}
	return fmt.Sprintf("Not in active profile %q: %s", selector.Name(), command)
}

// installedProfileHint discovers once and keeps logical source-relative paths:
// a repository or recursive local publication can contain several skills.
func installedProfileHint(cfg profileConfigView, source string, walk sourcewalk.Options, roots []string) string {
	if len(roots) == 0 || !cfg.ProfileSelector().Active() {
		return ""
	}
	skills, err := ssync.DiscoverSourceSkillsAll(source, walk)
	if err != nil || walk.Follow.Incomplete() {
		return ""
	}
	var names []string
	for _, skill := range skills {
		for _, root := range roots {
			rel, err := filepath.Rel(root, filepath.Join(source, filepath.FromSlash(skill.RelPath)))
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
				names = append(names, skill.FlatName)
				break
			}
		}
	}
	slices.Sort(names)
	return profileHint(cfg, slices.Compact(names))
}

func printProfileHint(hint string, jsonMode bool) {
	if hint == "" {
		return
	}
	if jsonMode {
		fmt.Fprintln(os.Stderr, hint)
	} else {
		ui.Note(hint)
	}
}
