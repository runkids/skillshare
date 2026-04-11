package hooks

// CanonicalRelativePath returns the managed hook ID for a tool/event/matcher triplet.
func CanonicalRelativePath(tool, event, matcher string) (string, error) {
	return canonicalRelativePath(tool, event, matcher)
}
