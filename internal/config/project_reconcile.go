package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/install"
)

// ReconcileProjectSkills scans the project source directory recursively for
// remotely-installed skills (those with install metadata or tracked repos)
// and ensures they are present in the MetadataStore.
// It also updates .skillshare/.gitignore for each tracked skill.
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

	result, err := reconcileSkillsWalk(sourcePath, store, onFound)
	if err != nil {
		return fmt.Errorf("failed to scan project skills: %w", err)
	}

	if pruneStaleEntries(store, result.live) {
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

	if projectCfg != nil && reconcileProjectConfigSkills(projectCfg, store, result.live) {
		if err := projectCfg.Save(projectRoot); err != nil {
			return fmt.Errorf("failed to save project config after reconcile: %w", err)
		}
	}

	return nil
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
		if err := install.UpdateGitIgnoreBatch(filepath.Join(projectRoot, ".skillshare"), gitignoreEntries); err != nil {
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
