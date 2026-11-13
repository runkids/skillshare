package config

import (
	"fmt"
	"os"
	"path"
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

	result, err := reconcileSkillsWalk(sourcePath, cfg.SkillsWalk(), store, nil, nil, true)
	if err != nil {
		return fmt.Errorf("failed to scan global skills: %w", err)
	}

	if prune && !result.incomplete {
		// Repository declarations survive without a checkout, including when
		// discovery skips an occupied path. Missing non-prefixed members and
		// members below a recorded repository are not kept by this loop;
		// ambiguous prefixed members without such an ancestor remain.
		for _, key := range store.List() {
			entry := store.Get(key)
			if entry == nil || !entry.Tracked || entry.Source == "" {
				continue
			}
			rel := filepath.ToSlash(install.KeyToRelPath(key, entry))
			if !utils.IsTrackedRepoDir(path.Base(rel)) {
				continue
			}
			member := false
			for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
				owner := store.GetExact(parent)
				if owner != nil && owner.Tracked && owner.Source != "" && utils.IsTrackedRepoDir(path.Base(parent)) &&
					filepath.ToSlash(install.KeyToRelPath(parent, owner)) == parent {
					member = true
					break
				}
			}
			if !member {
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
