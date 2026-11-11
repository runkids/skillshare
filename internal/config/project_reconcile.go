package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/projectdir"
	"skillshare/internal/sourcewalk"
)

// ReconcileProjectSkills scans the project source directory recursively for
// remotely-installed skills (those with install metadata or tracked repos)
// and ensures they are present in the MetadataStore.
// It also updates the project directory's .gitignore for each tracked skill
// and for the skills and agents metadata files.
func ReconcileProjectSkills(projectRoot string, projectCfg *ProjectConfig, store *install.MetadataStore, sourcePath string) error {
	return ReconcileProjectInstall(projectRoot, projectCfg, nil, store, sourcePath)
}

// ReconcileProjectInstall is ReconcileProjectSkills for a command that just
// installed skills from a source the user named. before is the store as it was
// when the command started. A config entry whose skill the command installed
// from another source, branch or mode takes the new record, so teammates
// install what this machine has (#858). Every other entry keeps what the
// config declares: a teammate may have changed it since this copy was installed.
func ReconcileProjectInstall(projectRoot string, projectCfg *ProjectConfig, before, store *install.MetadataStore, sourcePath string) error {
	return reconcileProjectFull(projectRoot, projectCfg, before, nil, store, sourcePath)
}

// ReconcileProjectMove is ReconcileProjectSkills after a command moved skills:
// moved maps each old source-relative path to its new one, so the config
// entry of the old path goes.
func ReconcileProjectMove(projectRoot string, projectCfg *ProjectConfig, store *install.MetadataStore, sourcePath string, moved map[string]string) error {
	return reconcileProjectFull(projectRoot, projectCfg, nil, moved, store, sourcePath)
}

func reconcileProjectFull(projectRoot string, projectCfg *ProjectConfig, before *install.MetadataStore, moved map[string]string, store *install.MetadataStore, sourcePath string) error {
	if err := reconcileProjectSkills(projectRoot, projectCfg, before, moved, store, sourcePath, true); err != nil {
		return err
	}
	agentsSource := filepath.Join(projectdir.Resolve(projectRoot), "agents")
	if projectCfg != nil {
		agentsSource = projectCfg.EffectiveAgentsSource(projectRoot)
	}
	return ignoreProjectMetadata(projectRoot, sourcePath, agentsSource)
}

// skillOrigin is what a config entry declares and an install record holds.
type skillOrigin struct {
	source  string
	tracked bool
	branch  string
}

// followInstalledSources points config entries at the records that changed
// since before. It reports whether the config changed.
func followInstalledSources(cfg *ProjectConfig, before, store *install.MetadataStore) bool {
	changed := false
	for i := range cfg.Skills {
		skill := &cfg.Skills[i]
		now := store.GetExact(skill.FullName())
		if skill.EffectiveKind() != "skill" || now == nil || now.Source == "" || now.Kind == "agent" {
			continue
		}
		installed := skillOrigin{now.Source, now.Tracked, now.Branch}
		if was := before.GetExact(skill.FullName()); was != nil && installed == (skillOrigin{was.Source, was.Tracked, was.Branch}) {
			continue
		}
		if installed == (skillOrigin{skill.Source, skill.Tracked, skill.Branch}) {
			continue
		}
		skill.Source, skill.Tracked, skill.Branch = installed.source, installed.tracked, installed.branch
		changed = true
	}
	return changed
}

// ForgetProjectSkills drops the config entries and lock pins of uninstalled
// skills, so a later `install -p`, here or on a teammate's machine, does not
// bring them back. A name is a skill, or a group folder with every skill
// below it.
func ForgetProjectSkills(projectRoot string, cfg *ProjectConfig, removed []string) error {
	gone := func(name string) bool {
		for _, r := range removed {
			if name == r || strings.HasPrefix(name, r+"/") {
				return true
			}
		}
		return false
	}
	declared := len(cfg.Skills)
	cfg.Skills = slices.DeleteFunc(cfg.Skills, func(s SkillEntry) bool {
		return s.EffectiveKind() == "skill" && gone(s.FullName())
	})
	if len(cfg.Skills) != declared {
		if err := cfg.Save(projectRoot); err != nil {
			return fmt.Errorf("failed to save project config: %w", err)
		}
	}
	dir := projectdir.Resolve(projectRoot)
	lock, err := install.LoadLock(dir)
	if err != nil {
		return err
	}
	maps.DeleteFunc(lock.Skills, func(name string, _ install.LockEntry) bool { return gone(name) })
	return lock.Save(dir)
}

// ignoreProjectMetadata keeps each source's .metadata.json out of Git. It
// records this machine's installs (hashes, timestamps) for folders that are
// themselves ignored; teammates share config.yaml and skills.lock.json instead.
func ignoreProjectMetadata(projectRoot string, sourcePaths ...string) error {
	for _, sourcePath := range sourcePaths {
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			continue
		}
		gitignoreDir, prefix := ProjectGitignoreTarget(projectRoot, sourcePath)
		if gitignoreDir == "" {
			continue
		}
		if err := install.UpdateGitIgnoreFiles(gitignoreDir, []string{prefix + "/" + install.MetadataFileName}); err != nil {
			return fmt.Errorf("failed to update .gitignore: %w", err)
		}
	}
	return nil
}

// AdoptMovedProjectSkills follows hand-moved declared skills without pruning
// records or reconciling unrelated project entries.
func AdoptMovedProjectSkills(projectRoot string, projectCfg *ProjectConfig, store *install.MetadataStore, sourcePath string) error {
	return reconcileProjectSkills(projectRoot, projectCfg, nil, nil, store, sourcePath, false)
}

// reconcileProjectSkills with a non-nil before also runs followInstalledSources.
// movedByCommand holds the old paths of skills the calling command moved.
func reconcileProjectSkills(projectRoot string, projectCfg *ProjectConfig, before *install.MetadataStore, movedByCommand map[string]string, store *install.MetadataStore, sourcePath string, prune bool) error {
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return nil
	}
	if !prune && !anyRecordGone(sourcePath, store) {
		return nil
	}
	// First, so the walk below compares records with what the config now declares.
	configChanged := before != nil && projectCfg != nil && followInstalledSources(projectCfg, before, store)

	gitignoreDir, prefix := ProjectGitignoreTarget(projectRoot, sourcePath)

	var gitignoreEntries []string
	onFound := func(fullPath string) {
		if gitignoreDir != "" {
			gitignoreEntries = append(gitignoreEntries, prefix+"/"+fullPath)
		}
	}

	var walk sourcewalk.Options
	if projectCfg != nil {
		targets, _ := ResolveValidProjectTargets(projectRoot, projectCfg)
		walk = SkillsWalk(projectCfg.FollowSourceLinks, sourcePath, targets)
	}
	// The manifest, not the metadata, says what each skill installs; a record
	// it no longer declares that way must not follow a copy.
	declared := map[string]install.SkillEntryDTO{}
	if projectCfg != nil {
		for _, skill := range ProjectSkillEntries(projectCfg.Skills) {
			declared[skill.FullName()] = skill
		}
	}
	canMove := func(key string) bool {
		entry := store.Get(key)
		skill, ok := declared[filepath.ToSlash(install.KeyToRelPath(key, entry))]
		return ok && entry.MatchesDeclaration(skill)
	}
	result, err := reconcileSkillsWalk(sourcePath, walk, store, onFound, canMove)
	if err != nil {
		return fmt.Errorf("failed to scan project skills: %w", err)
	}
	if !prune {
		// A stale record can trigger the walk without any skill having moved.
		if len(result.moved) == 0 {
			if result.changed {
				return store.Save(sourcePath)
			}
			return nil
		}
		gitignoreEntries = nil
		for _, dest := range result.moved {
			onFound(dest)
		}
	}

	// Config entries go only for skills that were installed here and are now
	// gone or moved. An entry with no record is one this machine has not
	// installed yet (a fresh clone), and stays for `install -p`.
	gone := map[string]bool{}
	for old := range movedByCommand {
		gone[old] = true
	}
	for old := range result.moved {
		gone[old] = true
	}
	if prune && !result.incomplete {
		for _, key := range store.List() {
			if !result.live[key] {
				gone[filepath.ToSlash(install.KeyToRelPath(key, store.Get(key)))] = true
			}
		}
		if pruneStaleEntries(store, result.live) {
			result.changed = true
		}
	}

	// A moved skill's old ignore rule would keep ignoring whatever lands there.
	if gitignoreDir != "" && len(result.moved) > 0 {
		var stale []string
		for old := range result.moved {
			stale = append(stale, prefix+"/"+old)
		}
		if _, err := install.RemoveFromGitIgnoreBatch(gitignoreDir, stale); err != nil {
			return fmt.Errorf("failed to update .gitignore: %w", err)
		}
	}

	if len(gitignoreEntries) > 0 {
		if err := install.UpdateGitIgnoreBatch(gitignoreDir, gitignoreEntries); err != nil {
			return fmt.Errorf("failed to update .gitignore: %w", err)
		}
	}

	if result.changed {
		if err := store.Save(sourcePath); err != nil {
			return err
		}
	}

	if projectCfg != nil {
		if prune {
			configChanged = reconcileProjectConfigSkills(projectCfg, store, gone) || configChanged
		} else {
			kept := projectCfg.Skills[:0]
			for _, skill := range projectCfg.Skills {
				if dest, ok := result.moved[skill.FullName()]; ok {
					configChanged = true
					// Keep an existing destination declaration, as full reconcile does.
					if _, exists := declared[dest]; exists {
						continue
					}
					skill.Name = skillBaseName(dest)
					skill.Group = store.Get(dest).Group
				}
				kept = append(kept, skill)
			}
			projectCfg.Skills = kept
		}
	}
	if configChanged {
		if err := projectCfg.Save(projectRoot); err != nil {
			return fmt.Errorf("failed to save project config after reconcile: %w", err)
		}
	}

	if projectCfg != nil {
		// The pin must follow first, or WriteProjectLock pins the local commit.
		if len(result.moved) > 0 {
			if err := install.MoveLockPins(projectdir.Resolve(projectRoot), result.moved); err != nil {
				return fmt.Errorf("failed to write %s: %w", install.LockFileName, err)
			}
		}
		if prune {
			if err := WriteProjectLock(projectRoot, projectCfg, store, sourcePath); err != nil {
				return fmt.Errorf("failed to write %s: %w", install.LockFileName, err)
			}
		}
	}

	return nil
}

// WriteProjectLock pins remote skills that have no pin yet and drops pins for
// skills that left the config. It never moves an existing pin: a copy that is
// merely behind (a teammate updated, this machine has not run `install -p`)
// must not drag the lockfile back. Moving a pin is TrackProjectLock's job.
// ponytail: skills only; agents get pinned once someone needs it.
func WriteProjectLock(projectRoot string, cfg *ProjectConfig, store *install.MetadataStore, sourcePath string) error {
	return writeProjectLock(projectRoot, cfg, store, sourcePath, nil)
}

// TrackProjectLock snapshots the commit of every remote skill and returns a
// function to call once the command is done. Skills whose commit changed in
// between were moved by this command (update, forced reinstall), so their pins
// follow; every other pin stays as it is.
func TrackProjectLock(projectRoot string, cfg *ProjectConfig, sourcePath string) func() error {
	before := projectSkillCommits(cfg, install.LoadMetadataOrNew(sourcePath), sourcePath)
	return func() error {
		return writeProjectLock(projectRoot, cfg, install.LoadMetadataOrNew(sourcePath), sourcePath, before)
	}
}

func projectSkillCommits(cfg *ProjectConfig, store *install.MetadataStore, sourcePath string) map[string]string {
	commits := make(map[string]string, len(cfg.Skills))
	for _, s := range cfg.Skills {
		name := s.FullName()
		commits[name] = install.InstalledCommit(filepath.Join(sourcePath, filepath.FromSlash(name)), store.GetByPath(name))
	}
	return commits
}

func writeProjectLock(projectRoot string, cfg *ProjectConfig, store *install.MetadataStore, sourcePath string, before map[string]string) error {
	dir := projectdir.Resolve(projectRoot)
	old, err := install.LoadLock(dir)
	if err != nil {
		return err
	}
	now := projectSkillCommits(cfg, store, sourcePath)
	lock := &install.Lock{Skills: map[string]install.LockEntry{}}
	for _, s := range cfg.Skills {
		name := s.FullName()
		commit := now[name]
		prev, pinned := old.Skills[name]
		pinned = pinned && prev.Source == s.Source
		moved := before != nil && before[name] != commit
		// Keep the pin unless this command moved the skill. An unknown commit
		// (not installed here, or a source without one) never replaces a pin.
		if pinned && (!moved || commit == "") {
			lock.Skills[name] = prev
			continue
		}
		if commit == "" {
			continue
		}
		pin := install.LockEntry{Source: s.Source, Commit: commit}
		if entry := store.GetByPath(name); entry != nil {
			pin.TreeHash = entry.TreeHash
		}
		lock.Skills[name] = pin
	}
	return lock.Save(dir)
}

// reconcileProjectConfigSkills syncs MetadataStore entries into ProjectConfig.Skills
// so the declarative skill list in config.yaml stays in sync with installed skills.
// gone holds the paths of skills that were removed or moved away.
// Returns true if ProjectConfig.Skills was modified.
func reconcileProjectConfigSkills(cfg *ProjectConfig, store *install.MetadataStore, gone map[string]bool) bool {
	existing := make(map[string]int, len(cfg.Skills))
	for i, s := range cfg.Skills {
		existing[s.FullName()] = i
	}

	changed := false

	// Add entries present in store but missing from config.
	for _, name := range store.List() {
		entry := store.Get(name)
		if entry == nil || entry.Source == "" {
			continue
		}
		if entry.Kind == "agent" {
			continue
		}
		if _, ok := existing[name]; ok {
			continue
		}
		cfg.Skills = append(cfg.Skills, SkillEntry{
			Name:    skillBaseName(name),
			Source:  entry.Source,
			Tracked: entry.Tracked,
			Group:   entry.Group,
			Branch:  entry.Branch,
		})
		changed = true
	}

	// Drop the entries of skills that are gone, unless a record took the path.
	kept := len(cfg.Skills)
	cfg.Skills = slices.DeleteFunc(cfg.Skills, func(s SkillEntry) bool {
		return s.EffectiveKind() == "skill" && gone[s.FullName()] && !storeHasSource(store, s.FullName())
	})

	return changed || len(cfg.Skills) != kept
}

func skillBaseName(fullPath string) string {
	if idx := strings.LastIndex(fullPath, "/"); idx >= 0 {
		return fullPath[idx+1:]
	}
	return fullPath
}

func storeHasSource(store *install.MetadataStore, name string) bool {
	e := store.Get(name)
	return e != nil && e.Source != ""
}

// ReconcileProjectAgents scans the project agents source directory for
// installed agents and ensures they are present in the MetadataStore.
// Also updates .skillshare/.gitignore for each agent.
func ReconcileProjectAgents(projectRoot string, store *install.MetadataStore, agentsSourcePath string) error {
	if _, err := os.Stat(agentsSourcePath); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(agentsSourcePath)
	if err != nil {
		return nil
	}

	changed := false
	var gitignoreEntries []string

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}

		agentName := strings.TrimSuffix(name, ".md")

		existing := store.Get(agentName)
		if existing == nil || existing.Source == "" {
			continue
		}

		if existing.Kind != "agent" {
			existing.Kind = "agent"
			changed = true
		}

		gitignoreEntries = append(gitignoreEntries, filepath.Join("agents", name))
	}

	if len(gitignoreEntries) > 0 {
		if err := install.UpdateGitIgnoreBatch(projectdir.Resolve(projectRoot), gitignoreEntries); err != nil {
			return fmt.Errorf("failed to update .skillshare/.gitignore for agents: %w", err)
		}
	}

	if changed {
		if err := store.Save(agentsSourcePath); err != nil {
			return err
		}
	}

	return nil
}

// ProjectSkillEntries lists a project's config skills as install entries, the
// project counterpart of install.MetadataSkillEntries.
func ProjectSkillEntries(skills []SkillEntry) []install.SkillEntryDTO {
	dtos := make([]install.SkillEntryDTO, 0, len(skills))
	for _, s := range skills {
		dtos = append(dtos, install.SkillEntryDTO{Name: s.Name, Source: s.Source, Tracked: s.Tracked, Group: s.Group, Branch: s.Branch})
	}
	return dtos
}
