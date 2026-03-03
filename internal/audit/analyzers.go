package audit

// AnalyzerScope defines when an analyzer runs in the pipeline.
type AnalyzerScope string

const (
	// ScopeFile analyzers run once per scannable file during the Walk phase.
	ScopeFile AnalyzerScope = "file"
	// ScopeSkill analyzers run once per skill after all files are walked.
	ScopeSkill AnalyzerScope = "skill"
	// ScopeBundle analyzers run once after all skills are scanned.
	ScopeBundle AnalyzerScope = "bundle"
)

// Analyzer is the pluggable analysis unit for the audit pipeline.
// Each analyzer encapsulates one detection strategy (regex, dataflow, etc.)
// and declares its scope so the runner knows when to invoke it.
type Analyzer interface {
	// ID returns the stable identifier (matches AnalyzerStatic, etc.).
	ID() string
	// Scope returns when this analyzer runs: file, skill, or bundle.
	Scope() AnalyzerScope
	// Analyze runs the analysis and returns findings.
	// The context carries all inputs relevant to the declared scope.
	Analyze(ctx *AnalyzeContext) ([]Finding, error)
}

// AnalyzeContext carries all data needed by analyzers at different scopes.
// Only fields relevant to the analyzer's scope will be populated.
type AnalyzeContext struct {
	// --- File-scope fields (populated per-file during Walk) ---
	FileName   string // relative path within the skill
	Content    []byte
	IsMarkdown bool
	IsShell    bool

	// --- Skill-scope fields (populated after Walk completes) ---
	SkillPath   string
	MDFiles     []mdFileInfo
	FileCache   map[string][]byte
	TierProfile TierProfile

	// --- Bundle-scope fields (populated after all skills scanned) ---
	Results []*Result

	// --- Shared across all scopes ---
	Rules          []rule
	MDContentRules []rule
	MDLinkRules    []rule
	DisabledIDs    map[string]bool
}
