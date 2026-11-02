package audit

import (
	"errors"
	"strings"
)

// ErrBlocked is a sentinel error indicating that an operation was blocked
// by the security audit. Use errors.Is(err, audit.ErrBlocked) to check.
var ErrBlocked = errors.New("blocked by security audit")

const (
	maxScanFileSize        = 1_000_000 // 1MB
	maxScanDepth           = 6
	analyzabilityThreshold = 0.70
)

// Analyzer IDs — each maps to one analysis source.
const (
	AnalyzerStatic     = "static"
	AnalyzerDataflow   = "dataflow"
	AnalyzerTier       = "tier"
	AnalyzerIntegrity  = "integrity"
	AnalyzerStructure  = "structure"
	AnalyzerCrossSkill = "cross-skill"
	AnalyzerMetadata   = "metadata"
)

// Category classifies what a finding is about.
const (
	CategoryInjection    = "injection"
	CategoryExfiltration = "exfiltration"
	CategoryCredential   = "credential"
	CategoryObfuscation  = "obfuscation"
	CategoryPrivilege    = "privilege"
	CategoryIntegrity    = "integrity"
	CategoryStructure    = "structure"
	CategoryRisk         = "risk"
	CategoryTrust        = "trust"
)

// categoryForPattern maps a rule pattern name to a broad category.
func categoryForPattern(pattern string) string {
	switch {
	case strings.Contains(pattern, "injection") || strings.Contains(pattern, "unsafe-deserialization") || strings.Contains(pattern, "hidden-comment") || pattern == "prompt-leakage" || pattern == "anti-refusal":
		return CategoryInjection
	case strings.Contains(pattern, "exfiltration") || strings.Contains(pattern, "ssrf-") || strings.Contains(pattern, "fetch-with-pipe") || pattern == "agent-snooping":
		return CategoryExfiltration
	case strings.Contains(pattern, "credential") || strings.Contains(pattern, "hardcoded-secret"):
		return CategoryCredential
	case strings.Contains(pattern, "obfuscation") || strings.Contains(pattern, "escape-obfuscation") ||
		strings.Contains(pattern, "invisible-payload") || strings.Contains(pattern, "hidden-unicode") ||
		strings.Contains(pattern, "data-uri"):
		return CategoryObfuscation
	case strings.Contains(pattern, "destructive") || strings.Contains(pattern, "shell-execution") ||
		strings.Contains(pattern, "dynamic-code-exec") || strings.Contains(pattern, "shell-chain"):
		return CategoryPrivilege
	case strings.Contains(pattern, "content-"):
		return CategoryIntegrity
	case strings.Contains(pattern, "dangling-link"):
		return CategoryStructure
	case strings.Contains(pattern, "tier-") || strings.Contains(pattern, "low-analyzability"):
		return CategoryRisk
	default:
		return CategoryRisk
	}
}

// Finding represents a single security issue detected in a skill.
type Finding struct {
	Severity string `json:"severity"` // "CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO"
	Pattern  string `json:"pattern"`  // rule name (e.g. "prompt-injection")
	Message  string `json:"message"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Snippet  string `json:"snippet"` // trimmed matched line (no truncation)

	// Phase 2 fields — analyzer traceability and deduplication.
	RuleID       string  `json:"ruleId,omitempty"`       // stable rule identifier
	Analyzer     string  `json:"analyzer,omitempty"`     // static|dataflow|tier|integrity|structure|cross-skill
	Category     string  `json:"category,omitempty"`     // injection|exfiltration|credential|obfuscation|...
	Confidence   float64 `json:"confidence,omitempty"`   // 0~1
	Fingerprint  string  `json:"fingerprint,omitempty"`  // sha256 stable hash for deduplication
	Acknowledged bool    `json:"acknowledged,omitempty"` // previously accepted via --force; excluded from blocking
}
