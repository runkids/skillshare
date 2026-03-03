package audit

// tierAnalyzer generates findings from tier combination risk analysis.
// It runs at skill scope using the accumulated TierProfile.
type tierAnalyzer struct{}

func (a *tierAnalyzer) ID() string           { return AnalyzerTier }
func (a *tierAnalyzer) Scope() AnalyzerScope { return ScopeSkill }

func (a *tierAnalyzer) Analyze(ctx *AnalyzeContext) ([]Finding, error) {
	return TierCombinationFindings(ctx.TierProfile), nil
}
