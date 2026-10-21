package config

import (
	"cmp"
	"path/filepath"
	"slices"
	"sort"

	"skillshare/internal/utils"
)

// DetectPathOverlap returns the set of target names whose configured skill
// paths overlap with another enabled target — either by sharing the same
// primary path or by being scanned via cross-runtime discovery (also_scans).
//
// Used by `sync` (CLI hint) and the Web UI sync handler to surface a brief
// warning that points users at `doctor` for the full breakdown. A non-nil
// harmless leaves out a scanner reading a writer's folder when it reports true.
func DetectPathOverlap(targets map[string]TargetConfig, isProject bool, harmless func(scanner, writer string) bool) []string {
	if len(targets) < 2 {
		return nil
	}

	primaryByName := make(map[string]string, len(targets))
	for name, target := range targets {
		// A target with skills off neither writes nor counts as a scanner.
		raw := target.SkillsConfig().Path
		if raw == "" || !target.SkillsConfig().IsEnabled() {
			continue
		}
		primaryByName[name] = filepath.Clean(ExpandPath(raw))
	}

	writersByPath := make(map[string][]string, len(primaryByName))
	for name, p := range primaryByName {
		writersByPath[p] = append(writersByPath[p], name)
	}

	involved := make(map[string]struct{})
	for _, names := range writersByPath {
		if len(names) < 2 {
			continue
		}
		for _, n := range names {
			involved[n] = struct{}{}
		}
	}

	for scanner := range primaryByName {
		for _, p := range TargetScanPaths(scanner, targets[scanner], isProject) {
			resolved := filepath.Clean(p)
			if resolved == primaryByName[scanner] {
				// The scanner's own write path — the shared-primary pass above
				// already covers anyone else writing there.
				continue
			}
			writers, ok := writersByPath[resolved]
			if !ok {
				continue
			}
			for _, w := range writers {
				if w == scanner || (harmless != nil && harmless(scanner, w)) {
					continue
				}
				involved[scanner] = struct{}{}
				involved[w] = struct{}{}
			}
		}
	}

	if len(involved) == 0 {
		return nil
	}

	out := make([]string, 0, len(involved))
	for n := range involved {
		out = append(out, n)
	}
	return out
}

// SkillsPathKeptBy returns a target, other than name, those in leaving and those
// with skills off, that writes to the same skills folder as name, or "" when none does. Removing name
// must then leave the folder alone: its links are that target's too.
func SkillsPathKeptBy(targets map[string]TargetConfig, name string, leaving map[string]bool) string {
	self := targets[name]
	raw := self.SkillsConfig().Path
	if raw == "" {
		return ""
	}
	path := filepath.Clean(ExpandPath(raw))
	for other, target := range targets {
		if other == name || leaving[other] || target.SkillsConfig().Path == "" || !target.SkillsConfig().IsEnabled() {
			continue
		}
		if utils.PathsEqual(filepath.Clean(ExpandPath(target.SkillsConfig().Path)), path) {
			return other
		}
	}
	return ""
}

// SkillsFolderConflict is a skills folder that two or more targets with skills
// on sync into with different settings, so each sync undoes the others'
// links. Keep is the target to leave on; Stop the rest.
type SkillsFolderConflict struct {
	Path    string   `json:"path"`
	Targets []string `json:"targets"`
	Keep    string   `json:"keep"`
	Stop    []string `json:"stop"`
}

// SkillsFolderConflicts returns the shared skills folders whose targets'
// settings differ, sorted by path. defaultMode is the global mode a target
// without its own inherits. Targets with identical settings agree on the
// folder's contents and are not a conflict. universal is kept when it is in
// the group, otherwise the alphabetically first target.
func SkillsFolderConflicts(targets map[string]TargetConfig, defaultMode string) []SkillsFolderConflict {
	byFolder := make(map[string][]string)
	for name, tc := range targets {
		if !tc.SkillsConfig().IsEnabled() {
			continue
		}
		if folder := skillsFolder(tc); folder != "" {
			byFolder[folder] = append(byFolder[folder], name)
		}
	}

	var out []SkillsFolderConflict
	for folder, names := range byFolder {
		if len(names) < 2 {
			continue
		}
		sort.Strings(names)
		first := targets[names[0]]
		differ := false
		for _, n := range names[1:] {
			other := targets[n]
			if !sameSkillsSettings(first.SkillsConfig(), other.SkillsConfig(), defaultMode) {
				differ = true
				break
			}
		}
		if !differ {
			continue
		}
		keep := names[0]
		if slices.Contains(names, "universal") {
			keep = "universal"
		}
		var stop []string
		for _, n := range names {
			if n != keep {
				stop = append(stop, n)
			}
		}
		out = append(out, SkillsFolderConflict{Path: folder, Targets: names, Keep: keep, Stop: stop})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func sameSkillsSettings(a, b ResourceTargetConfig, defaultMode string) bool {
	mode := cmp.Or(a.Mode, defaultMode, "merge")
	if mode != cmp.Or(b.Mode, defaultMode, "merge") {
		return false
	}
	// symlink mode links the whole folder to the source, so no per-skill setting applies.
	if mode == "symlink" {
		return true
	}
	return EffectiveTargetNaming(a.TargetNaming) == EffectiveTargetNaming(b.TargetNaming) &&
		sameSet(a.Include, b.Include) && sameSet(a.Exclude, b.Exclude)
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
