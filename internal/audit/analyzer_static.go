package audit

// staticAnalyzer wraps regex-based rule scanning for both content and markdown links.
type staticAnalyzer struct{}

func (a *staticAnalyzer) ID() string           { return AnalyzerStatic }
func (a *staticAnalyzer) Scope() AnalyzerScope { return ScopeFile }

func (a *staticAnalyzer) Analyze(ctx *AnalyzeContext) ([]Finding, error) {
	if ctx.IsMarkdown {
		return ScanMarkdownContentWithRules(ctx.Content, ctx.FileName, ctx.MDContentRules), nil
	}
	return ScanContentWithRules(ctx.Content, ctx.FileName, ctx.Rules), nil
}

// markdownLinkAnalyzer runs markdown link rule checks (skill scope — needs all MD files).
type markdownLinkAnalyzer struct{}

func (a *markdownLinkAnalyzer) ID() string           { return AnalyzerStatic }
func (a *markdownLinkAnalyzer) Scope() AnalyzerScope { return ScopeSkill }

func (a *markdownLinkAnalyzer) Analyze(ctx *AnalyzeContext) ([]Finding, error) {
	if len(ctx.MDFiles) == 0 || len(ctx.MDLinkRules) == 0 {
		return nil, nil
	}
	return checkMarkdownLinkRules(ctx.MDFiles, ctx.MDLinkRules), nil
}
