package sync

import "skillshare/internal/skillignore"

// DefaultFileIgnorePatterns are file-level artifact ignores for copy-mode sync.
// They do not affect skill discovery; .skillignore handles that separately.
func DefaultFileIgnorePatterns() []string {
	return []string{".DS_Store", ".git/", "__pycache__/"}
}

// EffectiveFileIgnorePatterns returns built-in artifact ignores followed by
// user-configured patterns so later negations can override earlier defaults.
func EffectiveFileIgnorePatterns(patterns []string) []string {
	defaults := DefaultFileIgnorePatterns()
	out := make([]string, 0, len(defaults)+len(patterns))
	out = append(out, defaults...)
	out = append(out, patterns...)
	return out
}

func compileFileIgnore(patterns []string) *skillignore.Matcher {
	if len(patterns) == 0 {
		return &skillignore.Matcher{}
	}
	return skillignore.Compile(patterns)
}

func isFileIgnored(m *skillignore.Matcher, relPath string, isDir bool) bool {
	return m != nil && m.Match(relPath, isDir)
}
