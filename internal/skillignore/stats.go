package skillignore

// IgnoreStats holds information about .skillignore files and their effect.
type IgnoreStats struct {
	RootFile      string   // root .skillignore path ("" = not exist)
	RepoFiles     []string // repo-level .skillignore paths
	Patterns      []string // all active patterns (no blanks/comments)
	IgnoredSkills []string // excluded skill relPaths
}

// Active reports whether any .skillignore file is in effect.
func (s *IgnoreStats) Active() bool {
	return s.RootFile != "" || len(s.RepoFiles) > 0
}

// PatternCount returns the total number of active patterns.
func (s *IgnoreStats) PatternCount() int {
	return len(s.Patterns)
}

// IgnoredCount returns the number of ignored skills.
func (s *IgnoreStats) IgnoredCount() int {
	return len(s.IgnoredSkills)
}
