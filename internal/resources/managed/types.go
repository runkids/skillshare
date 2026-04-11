package managed

import "skillshare/internal/config"

// ResourceSet selects which managed resource kinds are synced.
type ResourceSet struct {
	Rules bool
	Hooks bool
}

// TargetSyncSpec describes one target participating in a managed sync run.
type TargetSyncSpec struct {
	Name   string
	Target config.TargetConfig
}

// SyncRequest configures one managed sync run.
type SyncRequest struct {
	ProjectRoot string
	DryRun      bool
	Resources   ResourceSet
	Targets     []TargetSyncSpec
}

// SyncResult reports one target/resource sync outcome.
type SyncResult struct {
	Target   string
	Resource string
	Updated  []string
	Skipped  []string
	Pruned   []string
	Err      error
}
