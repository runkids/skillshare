package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/projectdir"
	"skillshare/internal/sourcewalk"
)

// ReconcileProjectSkills scans the project source directory recursively for
// remotely-installed skills (those with install metadata or tracked repos)
// and ensures they are present in the MetadataStore.
// It also updates the project directory's .gitignore for each tracked skill.
func ReconcileProjectSkills(projectRoot string, projectCfg *ProjectConfig, store *install.MetadataStore, sourcePath string) error {
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return nil
	}

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

	if !result.incomplete && pruneStaleEntries(store, result.live) {
		result.changed = true
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

	if result.incomplete && projectCfg != nil {
		for _, sk := range projectCfg.Skills {
			result.live[sk.FullName()] = true
		}
	}
	if projectCfg != nil && reconcileProjectConfigSkills(projectCfg, store, result.live) {
		if err := projectCfg.Save(projectRoot); err != nil {
			return fmt.Errorf("failed to save project config after reconcile: %w", err)
		}
	}

	if projectCfg != nil {
		if err := WriteProjectLock(projectRoot, projectCfg, store, sourcePath); err != nil {
			return fmt.Errorf("failed to write %s: %w", install.LockFileName, err)
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
// Returns true if ProjectConfig.Skills was modified.
func reconcileProjectConfigSkills(cfg *ProjectConfig, store *install.MetadataStore, live map[string]bool) bool {
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

	// Remove config entries whose skills no longer exist on disk.
	if len(live) > 0 || len(cfg.Skills) > 0 {
		kept := cfg.Skills[:0]
		for _, s := range cfg.Skills {
			if live[s.FullName()] {
				kept = append(kept, s)
			} else if !storeHasSource(store, s.FullName()) {
				changed = true
			} else {
				kept = append(kept, s)
			}
		}
		cfg.Skills = kept
	}

	return changed
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
