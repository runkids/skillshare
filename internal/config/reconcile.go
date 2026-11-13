package config

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/install"
	"skillshare/internal/utils"
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

	if prune && !result.incomplete {
		// Repository declarations survive without a checkout, including when
		// discovery skips an occupied path. Legacy member records do not.
		for _, key := range store.List() {
			entry := store.Get(key)
			if entry != nil && entry.Tracked && entry.Source != "" &&
				utils.IsTrackedRepoDir(filepath.Base(filepath.FromSlash(install.KeyToRelPath(key, entry)))) {
				result.live[key] = true
			}
		}
		if pruneStaleEntries(store, result.live) {
			result.changed = true
		}
	}

	if result.changed {
		if err := store.Save(sourcePath); err != nil {
			return err
		}
	}

	return nil
}
