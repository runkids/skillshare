package hooks

// Record is one managed matcher-group hook loaded from disk.
type Record struct {
	ID           string
	Path         string
	RelativePath string
	Tool         string
	Event        string
	Matcher      string
	Handlers     []Handler
}

// Save is the payload for persisting one managed matcher-group hook.
type Save struct {
	ID       string
	Tool     string
	Event    string
	Matcher  string
	Handlers []Handler
}

// Handler is one action within a managed matcher-group hook.
type Handler struct {
	Type           string `yaml:"type"`
	Command        string `yaml:"command,omitempty"`
	URL            string `yaml:"url,omitempty"`
	Prompt         string `yaml:"prompt,omitempty"`
	Timeout        string `yaml:"timeout,omitempty"`
	TimeoutSeconds *int   `yaml:"timeoutSec,omitempty"`
	StatusMessage  string `yaml:"statusMessage,omitempty"`
}
