package audit

var riskWeights = map[string]int{
	SeverityCritical: 25,
	SeverityHigh:     15,
	SeverityMedium:   8,
	SeverityLow:      3,
	SeverityInfo:     1,
}

// Result holds all findings for a single skill.
type Result struct {
	scannedFiles   int         // Internal coverage evidence; not part of serialized results.
	SkillName      string      `json:"skillName"`
	Kind           string      `json:"kind,omitempty"` // "skill" or "agent" — set by caller
	Findings       []Finding   `json:"findings"`
	RiskScore      int         `json:"riskScore"`
	RiskLabel      string      `json:"riskLabel"` // "clean", "low", "medium", "high", "critical"
	Threshold      string      `json:"threshold,omitempty"`
	IsBlocked      bool        `json:"isBlocked,omitempty"`
	ScanTarget     string      `json:"scanTarget,omitempty"`
	TotalBytes     int64       `json:"totalBytes"`
	AuditableBytes int64       `json:"auditableBytes"`
	Analyzability  float64     `json:"analyzability"` // AuditableBytes / TotalBytes (1.0 when TotalBytes == 0)
	TierProfile    TierProfile `json:"tierProfile"`
}

func (r *Result) updateRisk() {
	r.RiskScore = CalculateRiskScore(r.Findings)
	r.RiskLabel = RiskLabelFromScoreAndMaxSeverity(r.RiskScore, r.MaxSeverity())
}

// HasCritical returns true if any finding is CRITICAL severity.
func (r *Result) HasCritical() bool {
	return r.HasSeverityAtOrAbove(SeverityCritical)
}

// HasHigh returns true if any finding is HIGH or above.
func (r *Result) HasHigh() bool {
	return r.HasSeverityAtOrAbove(SeverityHigh)
}

// HasSeverityAtOrAbove returns true if any finding severity is at or above threshold.
func (r *Result) HasSeverityAtOrAbove(threshold string) bool {
	normalized, err := NormalizeThreshold(threshold)
	if err != nil {
		normalized = DefaultThreshold()
	}
	cutoff := SeverityRank(normalized)
	for _, f := range r.Findings {
		if f.Acknowledged {
			continue
		}
		if SeverityRank(f.Severity) <= cutoff {
			return true
		}
	}
	return false
}

// MaxSeverity returns the highest severity found, or "" if no findings.
func (r *Result) MaxSeverity() string {
	max := ""
	maxRank := 999
	for _, f := range r.Findings {
		rank := SeverityRank(f.Severity)
		if rank < maxRank {
			max = f.Severity
			maxRank = rank
		}
	}
	return max
}

// CountByCategory returns the count of findings per Category.
// Only categories with count > 0 are included.
func (r *Result) CountByCategory() map[string]int {
	m := make(map[string]int)
	for _, f := range r.Findings {
		if f.Category != "" {
			m[f.Category]++
		}
	}
	return m
}

// CountBySeverity returns the count of findings at CRITICAL/HIGH/MEDIUM severities.
func (r *Result) CountBySeverity() (critical, high, medium int) {
	critical, high, medium, _, _ = r.CountBySeverityAll()
	return
}

// CountBySeverityAll returns the count of findings at each severity level.
func (r *Result) CountBySeverityAll() (critical, high, medium, low, info int) {
	for _, f := range r.Findings {
		switch f.Severity {
		case SeverityCritical:
			critical++
		case SeverityHigh:
			high++
		case SeverityMedium:
			medium++
		case SeverityLow:
			low++
		case SeverityInfo:
			info++
		}
	}
	return
}

// CalculateRiskScore converts findings into a normalized 0-100 risk score.
func CalculateRiskScore(findings []Finding) int {
	score := 0
	for _, f := range findings {
		score += riskWeights[f.Severity]
	}
	if score > 100 {
		return 100
	}
	return score
}

// RiskLabelFromScore maps risk score into one of: clean/low/medium/high/critical.
func RiskLabelFromScore(score int) string {
	switch {
	case score <= 0:
		return "clean"
	case score <= 25:
		return "low"
	case score <= 50:
		return "medium"
	case score <= 75:
		return "high"
	default:
		return "critical"
	}
}

// riskLabelRanks maps risk labels to numeric ranks (lower = more severe).
var riskLabelRanks = map[string]int{
	"critical": 0,
	"high":     1,
	"medium":   2,
	"low":      3,
	"clean":    4,
}

// riskLabelRank returns the numeric rank for a risk label (lower = more severe).
func riskLabelRank(label string) int {
	if r, ok := riskLabelRanks[label]; ok {
		return r
	}
	return 999
}

// riskFloorFromSeverity returns the minimum risk label implied by a severity.
func riskFloorFromSeverity(severity string) string {
	switch severity {
	case SeverityCritical:
		return "critical"
	case SeverityHigh:
		return "high"
	case SeverityMedium:
		return "medium"
	case SeverityLow:
		return "low"
	default:
		return "clean"
	}
}

// RiskLabelFromScoreAndMaxSeverity computes the risk label as the higher of
// the score-based label and the severity floor. This ensures a single HIGH
// finding is never reported as "low" risk.
func RiskLabelFromScoreAndMaxSeverity(score int, maxSeverity string) string {
	scoreLabel := RiskLabelFromScore(score)
	floor := riskFloorFromSeverity(maxSeverity)
	if riskLabelRank(floor) < riskLabelRank(scoreLabel) {
		return floor
	}
	return scoreLabel
}
