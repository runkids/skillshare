package adapters

// CompiledFile is one target-native file generated from managed resources.
type CompiledFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Format  string `json:"format"`
}

// RuleRecord is an adapter-friendly view over one managed rule record.
type RuleRecord struct {
	ID           string
	Tool         string
	RelativePath string
	Name         string
	Content      string
}

// HookRecord is an adapter-friendly view over one managed hook record.
type HookRecord struct {
	ID           string
	Tool         string
	RelativePath string
	Event        string
	Matcher      string
	Sequential   *bool
	Handlers     []HookHandler
}

// HookHandler is one action within a managed hook record.
type HookHandler struct {
	Type           string
	Name           string
	Description    string
	Command        string
	URL            string
	Prompt         string
	Timeout        string
	TimeoutSeconds *int
	StatusMessage  string
}
