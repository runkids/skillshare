package audit

// structureAnalyzer inspects bundled declarations and dangling Markdown links.
// Runs at skill scope after file collection.
type structureAnalyzer struct{}

func (a *structureAnalyzer) ID() string           { return AnalyzerStructure }
func (a *structureAnalyzer) Scope() AnalyzerScope { return ScopeSkill }

func (a *structureAnalyzer) Analyze(ctx *AnalyzeContext) ([]Finding, error) {
	findings, err := (&bundledAnalyzer{}).Analyze(ctx)
	if ctx.DisabledIDs["dangling-link"] && ctx.DisabledIDs["reference-escapes-root"] && ctx.DisabledIDs["reference-check-incomplete"] {
		return findings, err
	}
	root := ctx.ReferenceRoot
	if root == "" {
		root = ctx.SkillPath
	}
	for _, f := range checkBundledReferences(root, ctx.MDFiles, ctx.MDLinks, ctx.SkillPath) {
		if !ctx.DisabledIDs[f.RuleID] {
			findings = append(findings, f)
		}
	}
	return findings, err
}
