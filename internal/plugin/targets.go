package plugin

// TargetDefinition is the shared capability contract for CLI, TUI and dashboard.
// An empty operation list means discovery-only, not an installed/usable target.
type TargetDefinition struct {
	Target     string   `json:"target"`
	Label      string   `json:"label"`
	Project    bool     `json:"project"`
	Operations []string `json:"operations"`
	Reason     string   `json:"reason,omitempty"`
	// ReasonKey names Reason for the dashboard to translate; Reason is the fallback.
	ReasonKey string `json:"reasonKey,omitempty"`
}

func TargetDefinitions() []TargetDefinition {
	all := []string{"add", "import", "sync", "check", "update", "remove", "enable", "disable"}
	definitions := []TargetDefinition{
		{Target: "claude", Label: "Claude Code", Project: true, Operations: all},
		{Target: "codex", Label: "Codex", Operations: []string{"add", "import", "sync", "check", "remove", "enable", "disable"}},
		{Target: "cursor", Label: "Cursor", Operations: []string{"add", "sync", "check", "update", "remove", "enable", "disable"}},
		{Target: "antigravity", Label: "Antigravity Desktop", Project: true, Operations: []string{"add", "sync", "check", "update", "remove", "enable", "disable"}},
		{Target: "pi", Label: "Pi", Project: true, Operations: all},
		{Target: "opencode", Label: "OpenCode", Project: true, Operations: all},
		{Target: "antigravity-cli", Label: "Antigravity CLI", Operations: []string{"add", "import", "sync", "check", "remove", "enable", "disable"}, Reason: "Updates require review in Antigravity CLI to preserve native enablement.", ReasonKey: "plugins.reason.antigravity-cli"},
		{Target: "copilot", Label: "GitHub Copilot CLI", Operations: all},
		{Target: "grok", Label: "Grok Build", Operations: []string{"import", "sync", "check", "remove", "enable", "disable"}, Reason: "Install or update in Grok to complete native trust, then import.", ReasonKey: "plugins.reason.grok"},
		{Target: "kimi", Label: "Kimi Code", Operations: []string{}, Reason: automationProblem("kimi"), ReasonKey: "plugins.problem.kimi"},
		{Target: "hermes", Label: "Hermes", Operations: []string{}, Reason: automationProblem("hermes"), ReasonKey: "plugins.problem.hermes"},
		{Target: "devin", Label: "Devin", Operations: []string{}, Reason: automationProblem("devin"), ReasonKey: "plugins.problem.devin"},
	}
	return definitions
}
