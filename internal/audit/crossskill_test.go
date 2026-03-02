package audit

import (
	"testing"
)

// helper to build a Result with given findings and tier profile.
func makeResult(name string, tp TierProfile, findings ...Finding) *Result {
	r := &Result{
		SkillName:   name,
		Findings:    findings,
		TierProfile: tp,
	}
	r.updateRisk()
	return r
}

func tierWith(tiers ...CommandTier) TierProfile {
	var p TierProfile
	for _, t := range tiers {
		p.Add(t)
	}
	return p
}

func credFinding() Finding {
	return Finding{Severity: SeverityHigh, Pattern: "credential-access", Message: "reads creds", File: "SKILL.md", Line: 1}
}

func dataflowTaintFinding() Finding {
	return Finding{Severity: SeverityHigh, Pattern: "dataflow-taint", Message: "taint flow", File: "SKILL.md", Line: 2}
}

func highFinding() Finding {
	return Finding{Severity: SeverityHigh, Pattern: "shell-execution", Message: "shell exec", File: "SKILL.md", Line: 3}
}

func lowFinding() Finding {
	return Finding{Severity: SeverityLow, Pattern: "dangling-link", Message: "broken link", File: "SKILL.md", Line: 4}
}

func TestExtractCapability(t *testing.T) {
	tests := []struct {
		name     string
		result   *Result
		wantCred bool
		wantNet  bool
		wantPriv bool
		wantHigh bool
	}{
		{
			name:     "credential finding sets HasCredReads",
			result:   makeResult("a", TierProfile{}, credFinding()),
			wantCred: true,
			wantHigh: true,
		},
		{
			name:     "dataflow-taint sets HasCredReads",
			result:   makeResult("a", TierProfile{}, dataflowTaintFinding()),
			wantCred: true,
			wantHigh: true,
		},
		{
			name:    "tier profile sets capabilities",
			result:  makeResult("a", tierWith(TierNetwork, TierPrivilege)),
			wantNet: true, wantPriv: true,
		},
		{
			name:     "mixed findings and tiers",
			result:   makeResult("a", tierWith(TierNetwork), credFinding()),
			wantCred: true, wantNet: true, wantHigh: true,
		},
		{
			name:   "low-only findings do not set HasHighPlus",
			result: makeResult("a", TierProfile{}, lowFinding()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cap := extractCapability(tt.result)
			if cap.HasCredReads != tt.wantCred {
				t.Errorf("HasCredReads = %v, want %v", cap.HasCredReads, tt.wantCred)
			}
			if cap.HasNetSend != tt.wantNet {
				t.Errorf("HasNetSend = %v, want %v", cap.HasNetSend, tt.wantNet)
			}
			if cap.HasPrivilege != tt.wantPriv {
				t.Errorf("HasPrivilege = %v, want %v", cap.HasPrivilege, tt.wantPriv)
			}
			if cap.HasHighPlus != tt.wantHigh {
				t.Errorf("HasHighPlus = %v, want %v", cap.HasHighPlus, tt.wantHigh)
			}
		})
	}
}

func TestCrossSkillAnalysis(t *testing.T) {
	tests := []struct {
		name         string
		results      []*Result
		wantNil      bool
		wantPatterns []string // expected patterns in findings
	}{
		{
			name:    "nil on empty input",
			results: nil,
			wantNil: true,
		},
		{
			name:    "nil on single skill",
			results: []*Result{makeResult("a", tierWith(TierNetwork), credFinding())},
			wantNil: true,
		},
		{
			name: "source + sink basic pair",
			results: []*Result{
				makeResult("reader", TierProfile{}, credFinding()),                     // cred read, no network
				makeResult("sender", tierWith(TierNetwork)),                             // network, no cred read
			},
			wantPatterns: []string{"cross-skill-exfiltration"},
		},
		{
			name: "source + sink: both have both caps — no finding",
			results: []*Result{
				makeResult("a", tierWith(TierNetwork), credFinding()), // has both
				makeResult("b", tierWith(TierNetwork), credFinding()), // has both
			},
			wantNil: true,
		},
		{
			name: "source + sink: 3-skill chain triggers all complementary pairs",
			results: []*Result{
				makeResult("reader1", TierProfile{}, credFinding()),
				makeResult("sender1", tierWith(TierNetwork)),
				makeResult("reader2", TierProfile{}, dataflowTaintFinding()),
			},
			wantPatterns: []string{
				"cross-skill-exfiltration", // reader1 + sender1
				"cross-skill-exfiltration", // reader2 + sender1
			},
		},
		{
			name: "privilege + network basic pair",
			results: []*Result{
				makeResult("admin", tierWith(TierPrivilege)),
				makeResult("fetcher", tierWith(TierNetwork)),
			},
			wantPatterns: []string{"cross-skill-privilege-network"},
		},
		{
			name: "privilege + network: same skill has both — no finding",
			results: []*Result{
				makeResult("a", tierWith(TierPrivilege, TierNetwork)),
				makeResult("b", TierProfile{}),
			},
			wantNil: true,
		},
		{
			name: "stealth + high-risk basic pair",
			results: []*Result{
				makeResult("sneaky", tierWith(TierStealth)),
				makeResult("dangerous", TierProfile{}, highFinding()),
			},
			wantPatterns: []string{"cross-skill-stealth"},
		},
		{
			name: "stealth alone — no high-risk partner — no finding",
			results: []*Result{
				makeResult("sneaky", tierWith(TierStealth)),
				makeResult("clean", TierProfile{}, lowFinding()),
			},
			wantNil: true,
		},
		{
			name: "stealth skill with high findings paired with another high-risk skill — one finding",
			results: []*Result{
				makeResult("sneaky", tierWith(TierStealth), highFinding()),
				makeResult("dangerous", TierProfile{}, highFinding()),
			},
			wantPatterns: []string{
				"cross-skill-stealth", // sneaky (stealth) + dangerous (high); reverse doesn't fire since dangerous has no stealth
			},
		},
		{
			name: "all clean skills — no findings",
			results: []*Result{
				makeResult("a", TierProfile{}),
				makeResult("b", TierProfile{}, lowFinding()),
				makeResult("c", tierWith(TierReadOnly)),
			},
			wantNil: true,
		},
		{
			name: "combined rules: cred+net and stealth+high",
			results: []*Result{
				makeResult("reader", TierProfile{}, credFinding()),         // cred read, high finding
				makeResult("sender", tierWith(TierNetwork)),                // network
				makeResult("sneaky", tierWith(TierStealth)),                // stealth
			},
			wantPatterns: []string{
				"cross-skill-exfiltration", // reader + sender
				"cross-skill-stealth",      // sneaky + reader (reader has high finding)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CrossSkillAnalysis(tt.results)
			if tt.wantNil {
				if result != nil {
					t.Fatalf("expected nil, got %d findings: %v", len(result.Findings), result.Findings)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.SkillName != "_cross-skill" {
				t.Errorf("SkillName = %q, want _cross-skill", result.SkillName)
			}

			// Collect actual patterns.
			var gotPatterns []string
			for _, f := range result.Findings {
				gotPatterns = append(gotPatterns, f.Pattern)
				if f.File != "." {
					t.Errorf("finding File = %q, want \".\"", f.File)
				}
				if f.Line != 0 {
					t.Errorf("finding Line = %d, want 0", f.Line)
				}
			}

			// Check each wanted pattern appears at least once.
			patternCounts := make(map[string]int)
			for _, p := range gotPatterns {
				patternCounts[p]++
			}
			wantCounts := make(map[string]int)
			for _, p := range tt.wantPatterns {
				wantCounts[p]++
			}
			for p, wc := range wantCounts {
				if gc := patternCounts[p]; gc < wc {
					t.Errorf("pattern %q: got %d, want >= %d", p, gc, wc)
				}
			}
		})
	}
}

func TestCrossSkillAnalysis_RiskScore(t *testing.T) {
	results := []*Result{
		makeResult("reader", TierProfile{}, credFinding()),
		makeResult("sender", tierWith(TierNetwork)),
	}
	xr := CrossSkillAnalysis(results)
	if xr == nil {
		t.Fatal("expected non-nil result")
	}
	if xr.RiskScore <= 0 {
		t.Errorf("RiskScore = %d, want > 0", xr.RiskScore)
	}
	if xr.RiskLabel == "clean" {
		t.Error("RiskLabel = clean, want non-clean")
	}
}
