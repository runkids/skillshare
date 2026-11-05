package audit

// structureAnalyzer wraps dangling link detection for markdown files.
// Runs at skill scope after all markdown files have been collected.
type structureAnalyzer struct{}

func (a *structureAnalyzer) ID() string           { return AnalyzerStructure }
func (a *structureAnalyzer) Scope() AnalyzerScope { return ScopeSkill }

func (a *structureAnalyzer) Analyze(ctx *AnalyzeContext) ([]Finding, error) {
	if ctx.DisabledIDs["dangling-link"] && ctx.DisabledIDs["reference-escapes-root"] && ctx.DisabledIDs["reference-check-incomplete"] {
		return nil, nil
	}
	root := ctx.ReferenceRoot
	if root == "" {
		root = ctx.SkillPath
	}
	findings := checkBundledReferences(root, ctx.MDFiles, ctx.MDLinks, ctx.SkillPath)
	kept := findings[:0]
	for _, f := range findings {
		if !ctx.DisabledIDs[f.RuleID] {
			kept = append(kept, f)
		}
	}
	return kept, nil
}
