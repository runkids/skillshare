package config

import (
	"fmt"
	"os"

	"skillshare/internal/install"
)

// ReconcileGlobalSkills scans the global source directory for remotely-installed
// skills (those with install metadata or tracked repos) and ensures they are
// present in the MetadataStore.
func ReconcileGlobalSkills(cfg *Config, store *install.MetadataStore) error {
	return reconcileGlobalSkills(cfg, store, true)
}

// AdoptMovedGlobalSkills is ReconcileGlobalSkills without the prune: a record
// follows its moved copy, but a record whose skill is gone stays. sync runs it,
// since sync must never drop audit acceptances.
func AdoptMovedGlobalSkills(cfg *Config, store *install.MetadataStore) error {
	return reconcileGlobalSkills(cfg, store, false)
}

func reconcileGlobalSkills(cfg *Config, store *install.MetadataStore, prune bool) error {
	sourcePath := cfg.EffectiveSkillsSource()
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return nil
	}
	if !prune && !anyRecordGone(sourcePath, store) {
		return nil
	}

	result, err := reconcileSkillsWalk(sourcePath, cfg.SkillsWalk(), store, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to scan global skills: %w", err)
	}

	if prune && !result.incomplete && pruneStaleEntries(store, result.live) {
		result.changed = true
	}

	if result.changed {
		if err := store.Save(sourcePath); err != nil {
			return err
		}
	}

	return nil
}
