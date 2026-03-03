package audit

// integrityAnalyzer wraps content hash integrity verification.
// Runs at skill scope after all files have been cached.
type integrityAnalyzer struct{}

func (a *integrityAnalyzer) ID() string           { return AnalyzerIntegrity }
func (a *integrityAnalyzer) Scope() AnalyzerScope { return ScopeSkill }

func (a *integrityAnalyzer) Analyze(ctx *AnalyzeContext) ([]Finding, error) {
	if ctx.DisabledIDs["content-integrity"] {
		return nil, nil
	}
	return checkContentIntegrity(ctx.SkillPath, ctx.FileCache), nil
}
