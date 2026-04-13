package rules

// Record is a managed rule loaded from the filesystem.
type Record struct {
	ID           string
	Path         string
	Tool         string
	RelativePath string
	Name         string
	Content      []byte
	Targets      []string
	SourceType   string
	Disabled     bool
}

// Save is the input payload for persisting a managed rule.
type Save struct {
	ID         string
	Content    []byte
	Targets    []string
	SourceType string
	Disabled   bool
}
