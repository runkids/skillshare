package audit

// structureAnalyzer wraps dangling link detection for markdown files.
// Runs at skill scope after all markdown files have been collected.
type structureAnalyzer struct{}

func (a *structureAnalyzer) ID() string           { return AnalyzerStructure }
func (a *structureAnalyzer) Scope() AnalyzerScope { return ScopeSkill }

func (a *structureAnalyzer) Analyze(ctx *AnalyzeContext) ([]Finding, error) {
	if ctx.DisabledIDs["dangling-link"] {
		return nil, nil
	}
	return checkDanglingLinks(ctx.MDFiles), nil
}
