package sync

import (
	"cmp"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
)

// LeakedSkills returns the skills scanner's runtime loads from writer's folder
// although sync leaves them out of scanner's own folder: source skills its
// filters skip, and skills the user keeps in writer's folder. It needs a
// runtime that loads one skill per name (config.LoadsOneSkillPerName): which
// copy wins is not fixed (OpenCode loads them concurrently), but a source skill
// synced to both folders has the same content either way. known is false when
// that cannot be told (the runtime's rule for same-named skills is unknown, a
// filter is invalid, or nothing was discovered); every skill in writer's folder
// may then reach scanner.
func LeakedSkills(scanner, writer string, targets map[string]config.TargetConfig, defaultMode string, discovered []DiscoveredSkill) (leaked []string, known bool) {
	scannerAgent := scanner
	if a := targets[scanner].Agent; a != "" {
		scannerAgent = a
	}
	if !config.LoadsOneSkillPerName(scannerAgent) || discovered == nil {
		return nil, false
	}
	// A symlink-mode folder is the source itself, .skillignore'd skills that
	// discovery leaves out included; only a scanner that links it too sees them all.
	if isSymlinkMode(targets[writer], defaultMode) && !isSymlinkMode(targets[scanner], defaultMode) {
		return nil, false
	}
	own, err := syncedSkills(scanner, targets[scanner], defaultMode, discovered)
	if err != nil {
		return nil, false
	}
	theirs, err := syncedSkills(writer, targets[writer], defaultMode, discovered)
	if err != nil {
		return nil, false
	}

	synced := make(map[string]bool, len(own))
	ownNames := make(map[string]bool, len(own))
	for _, s := range own {
		synced[s.Skill.FlatName] = true
		ownNames[s.TargetName] = true
	}
	managed := make(map[string]bool, len(theirs))
	for _, s := range theirs {
		managed[s.TargetName] = true
		if !synced[s.Skill.FlatName] {
			leaked = append(leaked, s.Skill.FlatName)
		}
	}
	// Merge and copy mode keep what the user put in writer's folder.
	w, sc := targets[writer], targets[scanner]
	writerDir := config.ExpandPath(w.SkillsConfig().Path)
	ownDir := config.ExpandPath(sc.SkillsConfig().Path)
	entries, _ := os.ReadDir(writerDir)
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || managed[name] || ownNames[name] {
			continue
		}
		// Only a folder with SKILL.md is a skill the runtime loads.
		if !isFile(filepath.Join(writerDir, name, "SKILL.md")) || isFile(filepath.Join(ownDir, name, "SKILL.md")) {
			continue
		}
		leaked = append(leaked, name)
	}
	sort.Strings(leaked)
	return leaked, true
}

// syncedSkills returns the source skills sync puts in a target's folder: the
// whole source in symlink mode, otherwise what its filters and target_naming
// let through.
func syncedSkills(name string, target config.TargetConfig, defaultMode string, discovered []DiscoveredSkill) ([]ResolvedTargetSkill, error) {
	if isSymlinkMode(target, defaultMode) {
		skills := make([]ResolvedTargetSkill, len(discovered))
		for i, d := range discovered {
			skills[i] = ResolvedTargetSkill{Skill: d, TargetName: d.FlatName}
		}
		return skills, nil
	}
	res, err := ResolveTargetSkillsForTarget(name, target.SkillsConfig(), discovered)
	if err != nil {
		return nil, err
	}
	return res.Skills, nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isSymlinkMode(target config.TargetConfig, defaultMode string) bool {
	return cmp.Or(target.SkillsConfig().Mode, defaultMode) == "symlink"
}

// HarmlessOverlap returns a check for config.DetectPathOverlap: scanner reading
// writer's folder is harmless when it loads nothing sync leaves out of its own.
func HarmlessOverlap(targets map[string]config.TargetConfig, defaultMode string, discovered []DiscoveredSkill) func(scanner, writer string) bool {
	return func(scanner, writer string) bool {
		leaked, known := LeakedSkills(scanner, writer, targets, defaultMode, discovered)
		return known && len(leaked) == 0
	}
}
