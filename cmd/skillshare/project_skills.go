package main

import (
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/ui"
)

func reconcileProjectRemoteSkills(runtime *projectRuntime) error {
	// Installs write .metadata.json directly, so the store loaded before the
	// install is stale. Reconciling with it prunes config entries for skills it
	// never saw (#280) — always reload from disk first.
	if fresh, err := install.LoadMetadata(runtime.sourcePath); err == nil {
		runtime.skillsStore = fresh
	}
	follow := skillFollowSet(runtime.sourcePath, runtime.targets, runtime.root)
	return config.ReconcileProjectSkillsWithOptions(runtime.root, runtime.config, runtime.skillsStore, runtime.sourcePath, config.ReconcileOptions{Follow: follow})
}

// trackProjectLock returns a function for defer: it moves the lockfile pins of
// the skills this command changed.
func trackProjectLock(runtime *projectRuntime) func() {
	done := config.TrackProjectLock(runtime.root, runtime.config, runtime.sourcePath)
	return func() {
		if err := done(); err != nil {
			ui.Warning("Could not update %s: %v", install.LockFileName, err)
		}
	}
}
