package sync

// DiscoverSourceSkillsLiteWithOptions skips frontmatter parsing (Targets is nil)
// and also returns the tracked repositories met during the same walk.
func DiscoverSourceSkillsLiteWithOptions(sourcePath string, opts DiscoveryOptions) ([]DiscoveredSkill, []string, error) {
	skills, repos, _, err := discoverSourceSkillsInternal(sourcePath, discoverOptions{
		follow: opts.Follow, collectTracked: true, collectIgnored: opts.CollectIgnored,
		collectContext: opts.CollectContext, includeIgnored: opts.IncludeIgnored,
	})
	return skills, repos, err
}

// DiscoverSourceSkillsAllWithOptions also returns skills ignored by .skillignore,
// with Disabled=true, for views that list disabled skills.
func DiscoverSourceSkillsAllWithOptions(sourcePath string, opts DiscoveryOptions) ([]DiscoveredSkill, error) {
	opts.IncludeIgnored = true
	skills, _, err := DiscoverSourceSkillsWithOptions(sourcePath, opts)
	return skills, err
}

// DiscoverSourceSkillsForAnalyzeWithOptions keeps disabled skills, which a
// symlink-mode target still loads, and computes context usage in the same walk.
func DiscoverSourceSkillsForAnalyzeWithOptions(sourcePath string, opts DiscoveryOptions) ([]DiscoveredSkill, error) {
	opts.CollectContext = true
	return DiscoverSourceSkillsAllWithOptions(sourcePath, opts)
}
