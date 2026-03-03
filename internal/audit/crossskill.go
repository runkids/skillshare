package audit

import (
	"fmt"
	"sort"
	"strings"
)

// skillCapability summarises the security-relevant capabilities of a single skill,
// derived entirely from its existing Result (TierProfile + Findings).
type skillCapability struct {
	Name           string
	HasCredReads   bool // finding.Pattern ∈ {"credential-access", "dataflow-taint"}
	HasNetSend     bool // TierProfile contains TierNetwork
	HasPrivilege   bool // TierProfile contains TierPrivilege
	HasStealth     bool // TierProfile contains TierStealth
	HasInterpreter bool // TierProfile contains TierInterpreter
	HasHighPlus    bool // any finding with severity >= HIGH
}

// credReadPatterns are the finding patterns that indicate credential reading.
var credReadPatterns = map[string]bool{
	"credential-access":  true,
	patternDataflowTaint: true,
}

// extractCapability derives a skillCapability from a scan Result.
func extractCapability(r *Result) skillCapability {
	cap := skillCapability{
		Name:           r.SkillName,
		HasNetSend:     r.TierProfile.HasTier(TierNetwork),
		HasPrivilege:   r.TierProfile.HasTier(TierPrivilege),
		HasStealth:     r.TierProfile.HasTier(TierStealth),
		HasInterpreter: r.TierProfile.HasTier(TierInterpreter),
		HasHighPlus:    r.HasHigh(),
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
// It uses a set-based O(N) approach: one pass to build capability sets, then
// each rule checks whether both sides are non-empty to produce a single summary
// finding per rule.
//
// Returns a synthetic Result (SkillName="_cross-skill") with findings, or nil
// if no cross-skill issues are detected.
func CrossSkillAnalysis(results []*Result) *Result {
	if len(results) < 2 {
		return nil
	}

	// Single pass: extract capabilities and classify into sets.
	var (
		credReadersOnly []string // HasCredReads && !HasNetSend
		netNoCred       []string // HasNetSend && !HasCredReads
		privilegeOnly   []string // HasPrivilege && !HasNetSend
		netNoPrivilege  []string // HasNetSend && !HasPrivilege
		stealthSkills   []string // HasStealth
		highPlusSkills  []string // HasHighPlus
		interpreterOnly []string // HasInterpreter && !HasNetSend
	)

	for _, r := range results {
		cap := extractCapability(r)

		if cap.HasCredReads && !cap.HasNetSend {
			credReadersOnly = append(credReadersOnly, cap.Name)
		}
		if cap.HasNetSend && !cap.HasCredReads {
			netNoCred = append(netNoCred, cap.Name)
		}
		if cap.HasPrivilege && !cap.HasNetSend {
			privilegeOnly = append(privilegeOnly, cap.Name)
		}
		if cap.HasNetSend && !cap.HasPrivilege {
			netNoPrivilege = append(netNoPrivilege, cap.Name)
		}
		if cap.HasStealth {
			stealthSkills = append(stealthSkills, cap.Name)
		}
		if cap.HasHighPlus {
			highPlusSkills = append(highPlusSkills, cap.Name)
		}
		if cap.HasInterpreter && !cap.HasNetSend {
			interpreterOnly = append(interpreterOnly, cap.Name)
		}
	}

	var findings []Finding

	// Rule 1: Source + Sink — credential readers × network senders.
	if len(credReadersOnly) > 0 && len(netNoCred) > 0 {
		findings = append(findings, syntheticFinding(SeverityHigh, "cross-skill-exfiltration",
			fmt.Sprintf("cross-skill exfiltration vector: %s (reads credentials) × %s (has network access)",
				formatNameList(credReadersOnly), formatNameList(netNoCred))))
	}

	// Rule 2: Privilege + Network — privilege commands × network access.
	if len(privilegeOnly) > 0 && len(netNoPrivilege) > 0 {
		findings = append(findings, syntheticFinding(SeverityMedium, "cross-skill-privilege-network",
			fmt.Sprintf("cross-skill privilege escalation: %s (has privilege commands) × %s (has network access)",
				formatNameList(privilegeOnly), formatNameList(netNoPrivilege))))
	}

	// Rule 3: Stealth + High-Risk — stealth skills alongside high-risk skills.
	// Skip if the only overlap is a single skill appearing in both sets (self-pair).
	if len(stealthSkills) > 0 && len(highPlusSkills) > 0 {
		if !isSingleSelfPair(stealthSkills, highPlusSkills) {
			findings = append(findings, syntheticFinding(SeverityHigh, "cross-skill-stealth",
				fmt.Sprintf("stealth skill %s installed alongside high-risk skill %s — evasion risk",
					formatNameList(stealthSkills), formatNameList(highPlusSkills))))
		}
	}

	// Rule 4: Credential readers × Interpreter — interpreter can process stolen credentials.
	if len(credReadersOnly) > 0 && len(interpreterOnly) > 0 {
		findings = append(findings, syntheticFinding(SeverityMedium, "cross-skill-cred-interpreter",
			fmt.Sprintf("cross-skill credential processing: %s (reads credentials) × %s (has interpreter) — interpreter can process stolen data",
				formatNameList(credReadersOnly), formatNameList(interpreterOnly))))
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

// isSingleSelfPair returns true when both slices contain exactly one element
// and it's the same name — i.e. the only "pair" is a skill with itself.
func isSingleSelfPair(a, b []string) bool {
	return len(a) == 1 && len(b) == 1 && a[0] == b[0]
}

const maxNameListSize = 5

// formatNameList formats a list of skill names for cross-skill messages.
// Shows up to 5 names; if more, appends "and N more".
func formatNameList(names []string) string {
	sorted := make([]string, len(names))
	copy(sorted, names)
	sort.Strings(sorted)

	if len(sorted) <= maxNameListSize {
		return strings.Join(sorted, ", ")
	}
	return strings.Join(sorted[:maxNameListSize], ", ") +
		fmt.Sprintf(" and %d more", len(sorted)-maxNameListSize)
}

// syntheticFinding creates a Finding not tied to a specific file (File=".", Line=0).
// Used by cross-skill analysis and tier combination checks.
func syntheticFinding(severity, pattern, message string) Finding {
	return Finding{
		Severity: severity,
		Pattern:  pattern,
		Message:  message,
		File:     ".",
		Line:     0,
	}
}
