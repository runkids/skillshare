package main

import (
	"os"
	"path/filepath"
	"sort"

	"skillshare/internal/config"
	"skillshare/internal/utils"
)

type detectedDir struct {
	name       string
	path       string
	skillCount int
	hasSkills  bool
	exists     bool     // the skills directory exists; false when only the tool is installed
	via        []string // tools that share this directory (codex → universal)
}

const sharedSkillsDirectoryDescription = "Shared skills directory. Use this if your CLI supports ~/.agents/skills, such as Codex."

// sliceHasName returns true if any element's name matches.
func sliceHasName[T any](items []T, name string, getName func(T) string) bool {
	for _, item := range items {
		if getName(item) == name {
			return true
		}
	}
	return false
}

func isSharedUniversalSkillsAlias(name string, target config.TargetConfig, defaultTargets map[string]config.TargetConfig) bool {
	if name == "universal" {
		return false
	}
	universal, ok := defaultTargets["universal"]
	if !ok {
		return false
	}
	return filepath.Clean(target.SkillsConfig().Path) == filepath.Clean(universal.SkillsConfig().Path)
}

// detectCLIDirectories finds installed AI tools. It only reads the disk.
// Tools with skills come first (most skills first), then the rest by name.
func detectCLIDirectories() []detectedDir {
	defaultTargets := config.DefaultTargets()
	var detected []detectedDir
	var aliases []string

	for name, target := range defaultTargets {
		if isSharedUniversalSkillsAlias(name, target, defaultTargets) {
			// The skills path can't identify the tool (it belongs to universal),
			// so fall back to its install dir and credit universal instead.
			if dir := config.DetectDir(name); dir != "" {
				if _, err := os.Stat(dir); err == nil {
					aliases = append(aliases, name)
				}
			}
			continue
		}

		path := target.SkillsConfig().Path
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			count := countSkillDirs(path)
			detected = append(detected, detectedDir{name: name, path: path, skillCount: count, hasSkills: count > 0, exists: true})
		} else if _, err := os.Stat(filepath.Dir(path)); err == nil {
			// The tool is installed but has no skills directory yet; sync creates it.
			detected = append(detected, detectedDir{name: name, path: path})
		}
	}

	// Auto-include universal when any CLI is detected. The universal path
	// (~/.agents/skills/) is the cross-tool shared directory; it may not exist
	// until skillshare creates it.
	if len(detected) > 0 || len(aliases) > 0 {
		if !sliceHasName(detected, "universal", func(d detectedDir) string { return d.name }) {
			if target, ok := defaultTargets["universal"]; ok {
				detected = append(detected, detectedDir{name: "universal", path: target.SkillsConfig().Path})
			}
		}
	}
	sort.Strings(aliases)
	for i := range detected {
		if detected[i].name == "universal" {
			detected[i].via = aliases
		}
	}

	sort.SliceStable(detected, func(i, j int) bool {
		if detected[i].skillCount != detected[j].skillCount {
			return detected[i].skillCount > detected[j].skillCount
		}
		return detected[i].name < detected[j].name
	})
	return detected
}

// toolCount is how many AI tools were found; tools that share the universal
// directory count individually, universal itself does not.
func toolCount(detected []detectedDir) int {
	n := 0
	for _, d := range detected {
		if d.name == "universal" {
			n += len(d.via)
			continue
		}
		n++
	}
	return n
}

func countSkillDirs(path string) int {
	entries, _ := os.ReadDir(path)
	count := 0
	for _, e := range entries {
		if isSkillEntry(path, e) {
			count++
		}
	}
	return count
}

// isSkillEntry reports whether a directory entry is a skill folder (a real
// directory or a link to one), skipping hidden entries.
func isSkillEntry(dir string, e os.DirEntry) bool {
	if utils.IsHidden(e.Name()) {
		return false
	}
	if e.IsDir() {
		return true
	}
	info, err := os.Stat(filepath.Join(dir, e.Name()))
	return err == nil && info.IsDir()
}

// importedSkill is one skill folder init copies into the source.
type importedSkill struct {
	name string
	from string // full path of the skill folder
	tool string
}

// collectImports lists the skill folders of the given tools. When two tools
// have a skill with the same name, the first tool wins and the other copy
// is returned in skipped.
func collectImports(tools []detectedDir) (imports, skipped []importedSkill) {
	seen := map[string]bool{}
	for _, d := range tools {
		entries, err := os.ReadDir(d.path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !isSkillEntry(d.path, e) {
				continue
			}
			s := importedSkill{name: e.Name(), from: filepath.Join(d.path, e.Name()), tool: d.name}
			if seen[s.name] {
				skipped = append(skipped, s)
				continue
			}
			seen[s.name] = true
			imports = append(imports, s)
		}
	}
	return imports, skipped
}
