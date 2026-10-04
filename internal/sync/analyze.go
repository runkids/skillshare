package sync

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/utils"
)

// TargetSkills returns the skills a target actually loads, for analyze and
// context-cost reports.
//
// A symlink-mode target exposes the whole source folder, so every discovered
// skill counts, .skillignore'd ones included (pass a discovery that keeps them,
// such as DiscoverSourceSkillsForAnalyzeWithOptions). Any other mode gets the enabled
// skills its include/exclude and `targets:` filters let through, plus the
// skills that already live in the target folder without coming from the
// source (Local), which the tool loads just the same.
func TargetSkills(name string, target config.TargetConfig, defaultMode, sourcePath string, discovered []DiscoveredSkill) ([]DiscoveredSkill, error) {
	sc := target.SkillsConfig()
	if !sc.IsEnabled() {
		// Nothing is synced, but the tool still loads whatever the folder holds.
		return folderSkills(sc.Path, sourcePath, discovered), nil
	}
	mode := sc.Mode
	if mode == "" {
		mode = defaultMode
	}
	if mode == "symlink" {
		return discovered, nil
	}

	enabled := slices.DeleteFunc(slices.Clone(discovered), func(s DiscoveredSkill) bool { return s.Disabled })
	skills, err := FilterSkills(enabled, sc.Include, sc.Exclude)
	if err != nil {
		return nil, err
	}
	skills = FilterSkillsByTarget(skills, name)
	return append(skills, localTargetSkills(sc.Path, sourcePath)...), nil
}

// localTargetSkills finds skills that sit in targetPath as real folders but are
// not managed by skillshare: merge mode links source skills, so a real folder
// is local; copy mode records its copies in the manifest, so anything missing
// from it is local. Errors mean "nothing to report" — the target folder may
// not exist yet.
func localTargetSkills(targetPath, sourcePath string) []DiscoveredSkill {
	if targetPath == "" || sourcePath == "" {
		return nil
	}
	info, err := os.Stat(targetPath)
	if err != nil || !info.IsDir() || utils.ResolveSymlink(targetPath) == utils.ResolveSymlink(sourcePath) {
		return nil
	}
	manifest, err := ReadManifest(targetPath)
	if err != nil {
		return nil
	}
	// filepath.Walk does not follow symlinks, so merge-mode links never yield a SKILL.md.
	found, _, _, err := discoverSourceSkillsInternal(targetPath, discoverOptions{collectContext: true})
	if err != nil {
		return nil
	}
	var local []DiscoveredSkill
	for _, s := range found {
		top, _, _ := strings.Cut(s.RelPath, "/")
		if _, managed := manifest.Managed[s.RelPath]; managed {
			continue
		}
		if _, managed := manifest.Managed[top]; managed {
			continue
		}
		s.Local = true
		s.Targets = nil // a folder the tool already reads is loaded whatever its path implies
		local = append(local, s)
	}
	return local
}

// folderSkills lists every skill a target folder holds with skills off: links
// into the source that another target sharing the folder keeps, copies left
// from copy mode, and local skills. None of it is synced for this target, but
// the tool loads it all.
func folderSkills(targetPath, sourcePath string, discovered []DiscoveredSkill) []DiscoveredSkill {
	if targetPath == "" || sourcePath == "" {
		return nil
	}
	info, err := os.Stat(targetPath)
	if err != nil || !info.IsDir() {
		return nil
	}
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil
	}
	// The whole folder links to the source (symlink mode kept by another target).
	if utils.ResolveSymlink(targetPath) == utils.ResolveSymlink(absSource) {
		return discovered
	}
	byFlat := make(map[string]DiscoveredSkill, len(discovered))
	for _, s := range discovered {
		byFlat[s.FlatName] = s
	}
	var skills []DiscoveredSkill
	entries, _ := os.ReadDir(targetPath)
	for _, e := range entries {
		path := filepath.Join(targetPath, e.Name())
		if e.Type()&os.ModeSymlink == 0 || !linksIntoSource(path, absSource) {
			continue
		}
		if s, ok := byFlat[e.Name()]; ok {
			skills = append(skills, s)
		}
	}
	// Real folders: copies and local skills alike (filepath.Walk skips the links above).
	managedSet := map[string]string{}
	if m, err := ReadManifest(targetPath); err == nil {
		managedSet = m.Managed
	}
	found, _, _, err := discoverSourceSkillsInternal(targetPath, discoverOptions{collectContext: true})
	if err != nil {
		return skills
	}
	for _, s := range found {
		top, _, _ := strings.Cut(s.RelPath, "/")
		_, managed := managedSet[top]
		s.Local = !managed
		s.Targets = nil
		skills = append(skills, s)
	}
	return skills
}
