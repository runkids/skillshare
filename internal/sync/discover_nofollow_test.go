package sync

import (
	"fmt"

	"skillshare/internal/config"
	"skillshare/internal/skillignore"
)

// Follow-less shorthands for tests of the plain (no .skillfollow) behavior.
// They live in a test file so production callers must pass the operation's
// FollowSet through the *WithOptions entry points.

func DiscoverSourceSkills(sourcePath string) ([]DiscoveredSkill, error) {
	skills, _, err := DiscoverSourceSkillsWithOptions(sourcePath, DiscoveryOptions{})
	return skills, err
}

func DiscoverSourceSkillsLite(sourcePath string) ([]DiscoveredSkill, []string, error) {
	return DiscoverSourceSkillsLiteWithOptions(sourcePath, DiscoveryOptions{})
}

func DiscoverSourceSkillsWithStats(sourcePath string) ([]DiscoveredSkill, *skillignore.IgnoreStats, error) {
	return DiscoverSourceSkillsWithOptions(sourcePath, DiscoveryOptions{CollectIgnored: true})
}

func DiscoverSourceSkillsAll(sourcePath string) ([]DiscoveredSkill, error) {
	return DiscoverSourceSkillsAllWithOptions(sourcePath, DiscoveryOptions{})
}

func DiscoverSourceSkillsForAnalyze(sourcePath string) ([]DiscoveredSkill, error) {
	return DiscoverSourceSkillsForAnalyzeWithOptions(sourcePath, DiscoveryOptions{})
}

func SyncTargetMerge(name string, target config.TargetConfig, sourcePath string, dryRun, force bool, projectRoot string) (*MergeResult, error) {
	skills, err := DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}
	return SyncTargetMergeWithSkills(name, target, skills, sourcePath, dryRun, force, projectRoot)
}

func PruneOrphanLinks(targetPath, sourcePath string, include, exclude []string, targetName, targetNaming string, dryRun, force bool) (*PruneResult, error) {
	skills, err := DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills for pruning: %w", err)
	}
	return PruneOrphanLinksWithSkills(PruneOptions{
		TargetPath:   targetPath,
		SourcePath:   sourcePath,
		Skills:       skills,
		Include:      include,
		Exclude:      exclude,
		TargetNaming: targetNaming,
		TargetName:   targetName,
		DryRun:       dryRun,
		Force:        force,
	})
}
