package inspect

type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

type RuleItem struct {
	Name          string   `json:"name"`
	ID            string   `json:"id"`
	SourceTool    string   `json:"sourceTool"`
	Scope         Scope    `json:"scope"`
	Path          string   `json:"path"`
	Exists        bool     `json:"exists"`
	Collectible   bool     `json:"collectible"`
	CollectReason string   `json:"collectReason,omitempty"`
	Content       string   `json:"content"`
	Size          int64    `json:"size"`
	ScopedPaths   []string `json:"scopedPaths,omitempty"`
	IsScoped      bool     `json:"isScoped"`
}

type HookItem struct {
	SourceTool     string `json:"sourceTool"`
	Scope          Scope  `json:"scope"`
	Event          string `json:"event"`
	Matcher        string `json:"matcher,omitempty"`
	GroupID        string `json:"groupId"`
	Collectible    bool   `json:"collectible"`
	CollectReason  string `json:"collectReason,omitempty"`
	Sequential     *bool  `json:"sequential,omitempty"`
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	Command        string `json:"command"`
	URL            string `json:"url,omitempty"`
	Prompt         string `json:"prompt,omitempty"`
	Timeout        string `json:"timeout,omitempty"`
	TimeoutSeconds *int   `json:"timeoutSec,omitempty"`
	StatusMessage  string `json:"statusMessage,omitempty"`
	EntryIndex     int    `json:"-"`
	ActionIndex    int    `json:"-"`
	ActionType     string `json:"actionType"`
	Path           string `json:"path"`
}
