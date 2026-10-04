package install

import "skillshare/internal/sourcewalk"

// Follow-less shorthands for tests of the plain (no .skillfollow) behavior.
// They live in a test file so production callers must pass the operation's
// FollowSet through the *WithOptions entry points.

func GetUpdatableSkills(sourceDir string) ([]string, error) {
	return GetUpdatableSkillsWithOptions(sourceDir, sourcewalk.Options{})
}

func GetTrackedRepos(sourceDir string) ([]string, error) {
	return GetTrackedReposWithOptions(sourceDir, sourcewalk.Options{})
}

func GetMissingTrackedRepos(sourceDir string) ([]TrackedRepoMeta, error) {
	return GetMissingTrackedReposWithOptions(sourceDir, sourcewalk.Options{})
}
