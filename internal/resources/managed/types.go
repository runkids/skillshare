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
	AllTargets  []TargetSyncSpec
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

// CollectPreviewResult reports which discovered items would be collected.
type CollectPreviewResult struct {
	Pulled  []string
	Skipped []string
}

// CollectResult reports which discovered items were created or overwritten.
type CollectResult struct {
	Created     []string
	Overwritten []string
	Skipped     []string
}
