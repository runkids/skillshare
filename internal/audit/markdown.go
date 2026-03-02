package audit

import (
	"fmt"
	"sort"
	"strings"
)

// MarkdownOptions carries summary statistics for the Markdown report header.
type MarkdownOptions struct {
	Scanned    int
	Passed     int
	Warning    int
	Failed     int
	Critical   int
	High       int
	Medium     int
	Low        int
	Info       int
	ScanErrors int
	RiskScore  int
	RiskLabel  string
	Threshold  string
	Mode       string

	AvgAnalyzability float64
}

// ToMarkdown converts audit results into a Markdown report suitable for
// GitHub Issues, PRs, or documentation.
func ToMarkdown(results []*Result, opts MarkdownOptions) string {
	var b strings.Builder

	writeHeader(&b, opts)
	writeSummaryTable(&b, opts)

	// Partition results into failed, warning, and clean.
	var failed, warned, clean []*Result
	for _, r := range results {
		switch {
		case r.IsBlocked:
			failed = append(failed, r)
		case len(r.Findings) > 0:
			warned = append(warned, r)
		default:
			clean = append(clean, r)
		}
	}

	// Sort each group alphabetically by skill name.
	sortResults := func(rs []*Result) {
		sort.Slice(rs, func(i, j int) bool {
			return rs[i].SkillName < rs[j].SkillName
		})
	}
	sortResults(failed)
	sortResults(warned)
	sortResults(clean)

	// Findings section: failed first, then warned.
	hasFindings := len(failed)+len(warned) > 0
	if hasFindings {
		b.WriteString("\n## Findings\n")
		for _, r := range failed {
			writeSkillSection(&b, r, true)
		}
		for _, r := range warned {
			writeSkillSection(&b, r, false)
		}
	}

	// Clean skills section.
	if len(clean) > 0 {
		fmt.Fprintf(&b, "\n## Clean Skills (%d)\n\n", len(clean))
		names := make([]string, len(clean))
		for i, r := range clean {
			names[i] = "`" + r.SkillName + "`"
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteString("\n")
	}

	return b.String()
}

func writeHeader(b *strings.Builder, opts MarkdownOptions) {
	b.WriteString("# Skillshare Audit Report\n\n")
	fmt.Fprintf(b, "- **Scanned**: %d skill(s)\n", opts.Scanned)
	fmt.Fprintf(b, "- **Mode**: %s\n", opts.Mode)
	fmt.Fprintf(b, "- **Threshold**: %s\n", opts.Threshold)
}

func writeSummaryTable(b *strings.Builder, opts MarkdownOptions) {
	b.WriteString("\n## Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|--------|-------|\n")
	fmt.Fprintf(b, "| Passed | %d |\n", opts.Passed)
	fmt.Fprintf(b, "| Warning | %d |\n", opts.Warning)
	fmt.Fprintf(b, "| Failed | %d |\n", opts.Failed)
	fmt.Fprintf(b, "| Severity | C:%d H:%d M:%d L:%d I:%d |\n",
		opts.Critical, opts.High, opts.Medium, opts.Low, opts.Info)
	fmt.Fprintf(b, "| Risk | %s (%d/100) |\n",
		strings.ToUpper(opts.RiskLabel), opts.RiskScore)
	fmt.Fprintf(b, "| Analyzability | %.0f%% avg |\n",
		opts.AvgAnalyzability*100)
	if opts.ScanErrors > 0 {
		fmt.Fprintf(b, "| Scan Errors | %d |\n", opts.ScanErrors)
	}
}

// findingLocation formats a file:line location string for a finding.
func findingLocation(f Finding) string {
	if f.File == "" {
		return ""
	}
	if f.Line > 0 {
		return fmt.Sprintf("%s:%d", f.File, f.Line)
	}
	return f.File
}

func writeSkillSection(b *strings.Builder, r *Result, blocked bool) {
	symbol := "!"
	if blocked {
		symbol = "✗"
	}
	fmt.Fprintf(b, "\n### %s %s\n\n", symbol, r.SkillName)

	// Block decision line.
	maxSev := r.MaxSeverity()
	if maxSev == "" {
		maxSev = "NONE"
	}
	threshold := r.Threshold
	if threshold == "" {
		threshold = DefaultThreshold()
	}
	if blocked {
		fmt.Fprintf(b, "> **BLOCKED** — max severity %s >= threshold %s\n\n", maxSev, threshold)
	} else {
		fmt.Fprintf(b, "> **ALLOWED** — max severity %s < threshold %s\n\n", maxSev, threshold)
	}

	// Findings table — also track whether any snippets exist.
	b.WriteString("| # | Severity | Pattern | Message | Location |\n")
	b.WriteString("|---|----------|---------|---------|----------|\n")
	hasSnippets := false
	for i, f := range r.Findings {
		if f.Snippet != "" {
			hasSnippets = true
		}
		fmt.Fprintf(b, "| %d | %s | %s | %s | %s |\n",
			i+1,
			escapeMarkdownTable(f.Severity),
			escapeMarkdownTable(f.Pattern),
			escapeMarkdownTable(f.Message),
			escapeMarkdownTable(findingLocation(f)),
		)
	}

	// Snippets in collapsible details.
	if hasSnippets {
		b.WriteString("\n<details>\n<summary>Snippets</summary>\n\n")
		for _, f := range r.Findings {
			if f.Snippet == "" {
				continue
			}
			fmt.Fprintf(b, "**%s** — `%s`\n\n", findingLocation(f), f.Pattern)
			// Use 4-space indented code block to avoid backtick conflicts.
			b.WriteString("    " + escapeSnippetLine(f.Snippet) + "\n\n")
		}
		b.WriteString("</details>\n")
	}

	// Metadata bullets.
	fmt.Fprintf(b, "\n- Risk: %s (%d/100)\n",
		strings.ToUpper(r.RiskLabel), r.RiskScore)
	fmt.Fprintf(b, "- Analyzability: %.0f%%\n", r.Analyzability*100)
	if !r.TierProfile.IsEmpty() {
		fmt.Fprintf(b, "- Commands: %s\n", r.TierProfile.String())
	}

	b.WriteString("\n---\n")
}

// escapeSnippetLine replaces newlines so that indented code blocks stay single-line.
func escapeSnippetLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// escapeMarkdownTable replaces characters that would break a Markdown table cell.
func escapeMarkdownTable(s string) string {
	return strings.ReplaceAll(escapeSnippetLine(s), "|", "\\|")
}
