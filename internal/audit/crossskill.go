package audit

import "fmt"

// skillCapability summarises the security-relevant capabilities of a single skill,
// derived entirely from its existing Result (TierProfile + Findings).
type skillCapability struct {
	Name         string
	HasCredReads bool // finding.Pattern ∈ {"credential-access", "dataflow-taint"}
	HasNetSend   bool // TierProfile contains TierNetwork
	HasPrivilege bool // TierProfile contains TierPrivilege
	HasStealth   bool // TierProfile contains TierStealth
	HasHighPlus  bool // any finding with severity >= HIGH
}

// credReadPatterns are the finding patterns that indicate credential reading.
var credReadPatterns = map[string]bool{
	"credential-access": true,
	"dataflow-taint":    true,
}

// extractCapability derives a skillCapability from a scan Result.
func extractCapability(r *Result) skillCapability {
	cap := skillCapability{
		Name:        r.SkillName,
		HasNetSend:  r.TierProfile.HasTier(TierNetwork),
		HasPrivilege: r.TierProfile.HasTier(TierPrivilege),
		HasStealth:  r.TierProfile.HasTier(TierStealth),
		HasHighPlus: r.HasHigh(),
	}
	for _, f := range r.Findings {
		if credReadPatterns[f.Pattern] {
			cap.HasCredReads = true
			break
		}
	}
	return cap
}

// CrossSkillAnalysis checks for dangerous capability combinations across skills.
// It returns a synthetic Result (SkillName="_cross-skill") with findings, or nil
// if no cross-skill issues are detected. The caller should append the result to
// the results slice *after* summariseAuditResults so summary counts stay correct.
func CrossSkillAnalysis(results []*Result) *Result {
	if len(results) < 2 {
		return nil
	}

	caps := make([]skillCapability, len(results))
	for i, r := range results {
		caps[i] = extractCapability(r)
	}

	var findings []Finding

	for i := range caps {
		for j := i + 1; j < len(caps); j++ {
			findings = append(findings, checkPair(caps[i], caps[j])...)
		}
	}

	if len(findings) == 0 {
		return nil
	}

	r := &Result{
		SkillName:     "_cross-skill",
		Findings:      findings,
		Analyzability: 1.0,
	}
	r.updateRisk()
	return r
}

// checkPair evaluates the three cross-skill rules for a pair of capabilities.
func checkPair(a, b skillCapability) []Finding {
	var findings []Finding

	// Rule 1: Source + Sink — credential reader paired with network sender.
	// Only fires when each skill lacks the other's capability (complementary pair).
	if a.HasCredReads && !a.HasNetSend && b.HasNetSend && !b.HasCredReads {
		findings = append(findings, crossFinding(SeverityHigh, "cross-skill-exfiltration",
			fmt.Sprintf("cross-skill exfiltration vector: %s reads credentials, %s has network access", a.Name, b.Name)))
	}
	if b.HasCredReads && !b.HasNetSend && a.HasNetSend && !a.HasCredReads {
		findings = append(findings, crossFinding(SeverityHigh, "cross-skill-exfiltration",
			fmt.Sprintf("cross-skill exfiltration vector: %s reads credentials, %s has network access", b.Name, a.Name)))
	}

	// Rule 2: Privilege + Network — privilege commands paired with network access.
	if a.HasPrivilege && !a.HasNetSend && b.HasNetSend && !b.HasPrivilege {
		findings = append(findings, crossFinding(SeverityMedium, "cross-skill-privilege-network",
			fmt.Sprintf("cross-skill privilege escalation: %s has privilege commands, %s has network access", a.Name, b.Name)))
	}
	if b.HasPrivilege && !b.HasNetSend && a.HasNetSend && !a.HasPrivilege {
		findings = append(findings, crossFinding(SeverityMedium, "cross-skill-privilege-network",
			fmt.Sprintf("cross-skill privilege escalation: %s has privilege commands, %s has network access", b.Name, a.Name)))
	}

	// Rule 3: Stealth + High-Risk — stealth skill alongside a high-risk skill.
	// Only between different skills (same-skill stealth is caught by tier-stealth).
	if a.HasStealth && b.HasHighPlus {
		findings = append(findings, crossFinding(SeverityHigh, "cross-skill-stealth",
			fmt.Sprintf("stealth skill %s installed alongside high-risk skill %s — evasion risk", a.Name, b.Name)))
	}
	if b.HasStealth && a.HasHighPlus {
		findings = append(findings, crossFinding(SeverityHigh, "cross-skill-stealth",
			fmt.Sprintf("stealth skill %s installed alongside high-risk skill %s — evasion risk", b.Name, a.Name)))
	}

	return findings
}

// crossFinding creates a Finding for cross-skill analysis (File=".", Line=0).
func crossFinding(severity, pattern, message string) Finding {
	return Finding{
		Severity: severity,
		Pattern:  pattern,
		Message:  message,
		File:     ".",
		Line:     0,
	}
}
