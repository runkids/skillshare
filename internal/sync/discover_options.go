package sync

// DiscoverSourceSkillsLiteWithOptions is the options-bearing Lite entry point.
// Like Lite, it omits frontmatter and collects tracked repositories.
func DiscoverSourceSkillsLiteWithOptions(sourcePath string, opts DiscoveryOptions) ([]DiscoveredSkill, []string, error) {
	skills, repos, _, err := discoverSourceSkillsInternal(sourcePath, discoverOptions{
		follow: opts.Follow, collectTracked: true, collectIgnored: opts.CollectIgnored,
		collectContext: opts.CollectContext, includeIgnored: opts.IncludeIgnored,
	})
	return skills, repos, err
}

// DiscoverSourceSkillsAllWithOptions includes disabled skills, like All.
func DiscoverSourceSkillsAllWithOptions(sourcePath string, opts DiscoveryOptions) ([]DiscoveredSkill, error) {
	opts.IncludeIgnored = true
	skills, _, err := DiscoverSourceSkillsWithOptions(sourcePath, opts)
	return skills, err
}

// DiscoverSourceSkillsForAnalyzeWithOptions includes disabled skills and context.
func DiscoverSourceSkillsForAnalyzeWithOptions(sourcePath string, opts DiscoveryOptions) ([]DiscoveredSkill, error) {
	opts.CollectContext = true
	return DiscoverSourceSkillsAllWithOptions(sourcePath, opts)
}
