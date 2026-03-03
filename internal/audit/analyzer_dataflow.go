package audit

// dataflowAnalyzer wraps taint tracking for shell scripts and markdown code blocks.
type dataflowAnalyzer struct{}

func (a *dataflowAnalyzer) ID() string           { return AnalyzerDataflow }
func (a *dataflowAnalyzer) Scope() AnalyzerScope { return ScopeFile }

func (a *dataflowAnalyzer) Analyze(ctx *AnalyzeContext) ([]Finding, error) {
	if ctx.DisabledIDs[patternDataflowTaint] {
		return nil, nil
	}
	if ctx.IsShell {
		return ScanShellDataflow(ctx.Content, ctx.FileName), nil
	}
	if ctx.IsMarkdown {
		return ScanMarkdownDataflow(ctx.Content, ctx.FileName), nil
	}
	return nil, nil
}
