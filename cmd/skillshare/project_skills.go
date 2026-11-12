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
	reloadProjectSkillsStore(runtime)
	return config.ReconcileProjectSkills(runtime.root, runtime.config, runtime.skillsStore, runtime.sourcePath)
}

func reloadProjectSkillsStore(runtime *projectRuntime) {
	if fresh, err := install.LoadMetadata(runtime.sourcePath); err == nil {
		runtime.skillsStore = fresh
	}
}

// reconcileProjectInstalledSkills is reconcileProjectRemoteSkills after an
// install from a source the user named: config entries follow the skills it
// installed from another source. runtime must be loaded before the install.
func reconcileProjectInstalledSkills(runtime *projectRuntime) error {
	before := runtime.skillsStore
	reloadProjectSkillsStore(runtime)
	return config.ReconcileProjectInstall(runtime.root, runtime.config, before, runtime.skillsStore, runtime.sourcePath)
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
