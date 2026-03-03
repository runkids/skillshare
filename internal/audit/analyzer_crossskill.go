package audit

// crossSkillAnalyzer wraps bundle-level cross-skill risk analysis.
// Runs once after all skills are scanned.
type crossSkillAnalyzer struct{}

func (a *crossSkillAnalyzer) ID() string           { return AnalyzerCrossSkill }
func (a *crossSkillAnalyzer) Scope() AnalyzerScope { return ScopeBundle }

func (a *crossSkillAnalyzer) Analyze(ctx *AnalyzeContext) ([]Finding, error) {
	xr := CrossSkillAnalysis(ctx.Results)
	if xr == nil {
		return nil, nil
	}
	return xr.Findings, nil
}
