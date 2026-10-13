package sync

import (
	"os"
	"path/filepath"

	"skillshare/internal/resource"
	"skillshare/internal/utils"
)

// SkillSyncedToTarget reports whether sync has placed the skill at relPath in
// the skills directory targetPath under mode. resolution is the target's
// ResolveTargetSkillsForTarget result, so a caller asking about many skills
// resolves the target once; it may be nil in symlink mode, which links the
// whole directory and so ignores filters and naming.
func SkillSyncedToTarget(resolution *TargetSkillResolution, mode, targetPath, sourcePath, relPath string) bool {
	if mode == "symlink" {
		return CheckStatus(targetPath, sourcePath) == StatusLinked
	}
	if resolution == nil {
		return false
	}
	manifest, err := ReadManifest(targetPath)
	if err != nil {
		return false
	}
	taken := resolution.ValidTargetNames()
	for _, resolved := range resolution.Skills {
		if resolved.Skill.RelPath != relPath {
			continue
		}
		// A managed entry still under an earlier naming's name counts as synced.
		if legacy, err := findLegacyTargetEntry(mode, targetPath, resolved, taken, manifest); err == nil && legacy.name != "" {
			return true
		}
		entry := filepath.Join(targetPath, resolved.TargetName)
		info, err := os.Lstat(entry)
		return err == nil && isManagedTargetEntry(mode, entry, info, resolved, manifest, namedTarget{name: resolved.TargetName})
	}
	return false
}

// AgentSyncedToTarget reports whether sync has placed agent in targetDir.
// mode is the target's agents mode (empty is merge); extension is its
// agents extension, whose outputs are tracked in the manifest.
func AgentSyncedToTarget(mode, extension, targetDir, sourceDir string, agent resource.DiscoveredResource) bool {
	one := []resource.DiscoveredResource{agent}
	if extension != "" {
		synced, _ := ExtensionOutputStatus(targetDir, one)
		return synced[0]
	}
	switch EffectiveAgentMode(mode) {
	case "symlink":
		return CheckStatus(targetDir, sourceDir) == StatusLinked
	case "copy":
		return SyncedAgentCopies(targetDir, one) == 1
	default:
		link := filepath.Join(targetDir, agent.FlatName)
		return utils.IsSymlinkOrJunction(link) && isSymlinkToSource(link, agent.AbsPath)
	}
}
