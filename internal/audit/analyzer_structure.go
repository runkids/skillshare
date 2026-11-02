package audit

// structureAnalyzer inspects bundled declarations and dangling Markdown links.
// Runs at skill scope after file collection.
type structureAnalyzer struct{}

func (a *structureAnalyzer) ID() string           { return AnalyzerStructure }
func (a *structureAnalyzer) Scope() AnalyzerScope { return ScopeSkill }

func (a *structureAnalyzer) Analyze(ctx *AnalyzeContext) ([]Finding, error) {
	findings, err := (&bundledAnalyzer{}).Analyze(ctx)
	if ctx.DisabledIDs["dangling-link"] {
		return findings, err
	}
	return append(findings, checkDanglingLinks(ctx.MDFiles, ctx.MDLinks)...), err
}
