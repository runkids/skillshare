package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// splitGlobal* caches the split of global rules into content vs link rules.
// Reset via ResetGlobalCache().
var (
	splitGlobalOnce         sync.Once
	splitGlobalContentRules []rule
	splitGlobalLinkRules    []rule
)

// globalSplitMarkdownLinkRules returns the cached split of global rules.
func globalSplitMarkdownLinkRules() (contentRules []rule, linkRules []rule) {
	splitGlobalOnce.Do(func() {
		rules, err := Rules()
		if err != nil {
			return
		}
		splitGlobalContentRules, splitGlobalLinkRules = splitMarkdownLinkRules(rules)
	})
	return splitGlobalContentRules, splitGlobalLinkRules
}

var (
	mdAutoLinkRe       = regexp.MustCompile(`<((?:https?://)[^>\s]+)>`)
	mdRefLinkRe        = regexp.MustCompile(`\[([^\]]+)\]\[([^\]]*)\]`)
	mdHTMLAnchorRe     = regexp.MustCompile(`(?i)<a\b[^>]*\bhref\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a>`)
	mdHTMLTagStrip     = regexp.MustCompile(`(?s)<[^>]*>`)
	mdTutorialMarkerRe = regexp.MustCompile(`(?i)(for\s+example|e\.g\.|example:|examples:|sample:|original:|attacker:|safe:|unsafe:|vulnerable:|pattern:|ruleid:|ok:|sink:|message:)`)
)

type markdownLink struct {
	label   string
	target  string
	line    int
	snippet string
}

func splitMarkdownLinkRules(activeRules []rule) (contentRules []rule, markdownLinkRules []rule) {
	if activeRules == nil {
		return nil, nil
	}
	for _, r := range activeRules {
		if isMarkdownLinkRulePattern(r.Pattern) {
			markdownLinkRules = append(markdownLinkRules, r)
			continue
		}
		contentRules = append(contentRules, r)
	}
	return contentRules, markdownLinkRules
}

func isMarkdownLinkRulePattern(pattern string) bool {
	return pattern == "external-link"
}

func checkMarkdownLinkRules(files []mdFileInfo, mdLinks map[string][]markdownLink, markdownLinkRules []rule) []Finding {
	if len(markdownLinkRules) == 0 {
		return nil
	}

	var findings []Finding
	for _, f := range files {
		var links []markdownLink
		if mdLinks != nil {
			links = mdLinks[f.relPath]
		} else {
			links = extractMarkdownLinks(f.data)
		}
		for _, link := range links {
			canonical := fmt.Sprintf("[%s](%s)", link.label, link.target)
			canonicalLower := ""
			canonicalLowerReady := false
			for _, r := range markdownLinkRules {
				if !rulePrefilterAllows(r, canonical, &canonicalLower, &canonicalLowerReady) {
					continue
				}
				if !r.Regex.MatchString(canonical) {
					continue
				}
				if r.Exclude != nil && r.Exclude.MatchString(canonical) {
					continue
				}
				findings = append(findings, Finding{
					Severity:   r.Severity,
					Pattern:    r.Pattern,
					Message:    r.Message,
					File:       f.relPath,
					Line:       link.line,
					Snippet:    link.snippet,
					RuleID:     r.ID,
					Analyzer:   AnalyzerStatic,
					Category:   categoryForPattern(r.Pattern),
					Confidence: r.Confidence,
				})
			}
		}
	}
	return findings
}

// checkDanglingLinks scans collected .md file data for local relative links
// whose targets do not exist on disk. Returns LOW-severity findings.
func checkDanglingLinks(files []mdFileInfo, mdLinks map[string][]markdownLink) []Finding {
	var findings []Finding
	for _, f := range files {
		var links []markdownLink
		if mdLinks != nil {
			links = mdLinks[f.relPath]
		} else {
			links = extractMarkdownLinks(f.data)
		}
		for _, link := range links {
			target := link.target
			if isExternalOrAnchor(target) {
				continue
			}
			cleaned := stripFragment(target)
			if cleaned == "" {
				continue
			}
			abs := filepath.Join(f.absDir, cleaned)
			if _, err := os.Stat(abs); err != nil {
				findings = append(findings, Finding{
					Severity:   SeverityLow,
					Pattern:    "dangling-link",
					Message:    fmt.Sprintf("broken local link: %q not found", target),
					File:       f.relPath,
					Line:       link.line,
					Snippet:    link.snippet,
					RuleID:     "dangling-link",
					Analyzer:   AnalyzerStructure,
					Category:   CategoryStructure,
					Confidence: 1.0,
				})
			}
		}
	}
	return findings
}

func extractMarkdownLinks(data []byte) []markdownLink {
	lines := strings.Split(string(data), "\n")
	defs := parseReferenceDefinitions(lines)
	var links []markdownLink
	seen := make(map[string]bool)
	inCodeFence := false
	fenceMarker := ""

	add := func(link markdownLink) {
		if strings.TrimSpace(link.target) == "" {
			return
		}
		key := fmt.Sprintf("%d|%s|%s", link.line, link.label, link.target)
		if seen[key] {
			return
		}
		seen[key] = true
		links = append(links, link)
	}

	for i, line := range lines {
		lineNum := i + 1
		if marker, ok := detectFenceMarker(line); ok {
			if !inCodeFence {
				inCodeFence = true
				fenceMarker = marker
				continue
			}
			if marker == fenceMarker {
				inCodeFence = false
				fenceMarker = ""
				continue
			}
		}
		if inCodeFence {
			continue
		}

		codeSpans := inlineCodeSpans(line)
		for _, link := range extractInlineLinksFromLine(line, lineNum, codeSpans) {
			add(link)
		}
		for _, link := range extractAutoLinksFromLine(line, lineNum, codeSpans) {
			add(link)
		}
		for _, link := range extractReferenceLinksFromLine(line, lineNum, defs, codeSpans) {
			add(link)
		}
		for _, link := range extractHTMLAnchorLinksFromLine(line, lineNum, codeSpans) {
			add(link)
		}
		if i+1 < len(lines) {
			if _, ok := detectFenceMarker(lines[i+1]); ok {
				continue
			}
			if link, ok := extractMultilineLink(lines[i], lines[i+1], lineNum); ok {
				add(link)
			}
		}
	}

	return links
}

func parseReferenceDefinitions(lines []string) map[string]string {
	defs := make(map[string]string)
	inCodeFence := false
	fenceMarker := ""

	for _, line := range lines {
		if marker, ok := detectFenceMarker(line); ok {
			if !inCodeFence {
				inCodeFence = true
				fenceMarker = marker
				continue
			}
			if marker == fenceMarker {
				inCodeFence = false
				fenceMarker = ""
				continue
			}
		}
		if inCodeFence {
			continue
		}

		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "[") {
			continue
		}
		idx := strings.Index(trimmed, "]:")
		if idx <= 1 {
			continue
		}
		label := normalizeReferenceLabel(trimmed[1:idx])
		if label == "" {
			continue
		}
		target := parseMarkdownLinkTarget(strings.TrimSpace(trimmed[idx+2:]))
		if target == "" {
			continue
		}
		defs[label] = target
	}
	return defs
}

func normalizeReferenceLabel(label string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(label)), " "))
}

func extractInlineLinksFromLine(line string, lineNum int, codeSpans []span) []markdownLink {
	var links []markdownLink

	for i := 0; i < len(line); i++ {
		if line[i] != '[' {
			continue
		}
		if indexInSpans(i, codeSpans) {
			continue
		}
		if i > 0 && line[i-1] == '!' && !indexInSpans(i-1, codeSpans) {
			// Image link: ![alt](url)
			continue
		}
		labelEnd := findMatchingBracket(line, i)
		if labelEnd == -1 {
			continue
		}

		j := labelEnd + 1
		if j >= len(line) || line[j] != '(' {
			i = labelEnd
			continue
		}

		inside, linkEnd := readParenthesized(line, j)
		if linkEnd == -1 {
			i = labelEnd
			continue
		}

		label := strings.TrimSpace(line[i+1 : labelEnd])
		target := parseMarkdownLinkTarget(inside)
		if target != "" {
			links = append(links, markdownLink{
				label:   label,
				target:  target,
				line:    lineNum,
				snippet: strings.TrimSpace(line),
			})
		}

		i = linkEnd
	}

	return links
}

func extractAutoLinksFromLine(line string, lineNum int, codeSpans []span) []markdownLink {
	var links []markdownLink
	matches := mdAutoLinkRe.FindAllStringSubmatchIndex(line, -1)
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		start := m[0]
		if indexInSpans(start, codeSpans) {
			continue
		}
		target := strings.TrimSpace(line[m[2]:m[3]])
		links = append(links, markdownLink{
			label:   target,
			target:  target,
			line:    lineNum,
			snippet: strings.TrimSpace(line),
		})
	}
	return links
}

func extractReferenceLinksFromLine(line string, lineNum int, defs map[string]string, codeSpans []span) []markdownLink {
	var links []markdownLink
	matches := mdRefLinkRe.FindAllStringSubmatchIndex(line, -1)
	for _, m := range matches {
		if len(m) < 6 {
			continue
		}
		start := m[0]
		if indexInSpans(start, codeSpans) {
			continue
		}
		if start > 0 && line[start-1] == '!' && !indexInSpans(start-1, codeSpans) {
			// Image reference: ![alt][id]
			continue
		}

		label := strings.TrimSpace(line[m[2]:m[3]])
		ref := strings.TrimSpace(line[m[4]:m[5]])
		if ref == "" {
			ref = label
		}
		target, ok := defs[normalizeReferenceLabel(ref)]
		if !ok {
			continue
		}
		links = append(links, markdownLink{
			label:   label,
			target:  target,
			line:    lineNum,
			snippet: strings.TrimSpace(line),
		})
	}
	return links
}

func extractHTMLAnchorLinksFromLine(line string, lineNum int, codeSpans []span) []markdownLink {
	var links []markdownLink
	matches := mdHTMLAnchorRe.FindAllStringSubmatchIndex(line, -1)
	for _, m := range matches {
		if len(m) < 6 {
			continue
		}
		start := m[0]
		if indexInSpans(start, codeSpans) {
			continue
		}

		target := strings.TrimSpace(line[m[2]:m[3]])
		if target == "" {
			continue
		}
		labelRaw := strings.TrimSpace(line[m[4]:m[5]])
		label := strings.TrimSpace(mdHTMLTagStrip.ReplaceAllString(labelRaw, ""))
		if label == "" {
			label = target
		}

		links = append(links, markdownLink{
			label:   label,
			target:  target,
			line:    lineNum,
			snippet: strings.TrimSpace(line),
		})
	}
	return links
}

func extractMultilineLink(line, nextLine string, lineNum int) (markdownLink, bool) {
	labelLine := strings.TrimSpace(line)
	if !strings.HasPrefix(labelLine, "[") || !strings.HasSuffix(labelLine, "]") {
		return markdownLink{}, false
	}
	label := strings.TrimSpace(labelLine[1 : len(labelLine)-1])
	if label == "" {
		return markdownLink{}, false
	}

	targetLine := strings.TrimSpace(nextLine)
	if !strings.HasPrefix(targetLine, "(") || !strings.HasSuffix(targetLine, ")") {
		return markdownLink{}, false
	}
	target := parseMarkdownLinkTarget(strings.TrimSpace(targetLine[1 : len(targetLine)-1]))
	if target == "" {
		return markdownLink{}, false
	}

	return markdownLink{
		label:   label,
		target:  target,
		line:    lineNum,
		snippet: strings.TrimSpace(line),
	}, true
}

func parseMarkdownLinkTarget(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "<") {
		if end := strings.IndexByte(s, '>'); end > 0 {
			return strings.TrimSpace(s[1:end])
		}
		return ""
	}

	i := 0
	depth := 0
	escaped := false
	for i < len(s) {
		ch := s[i]
		if escaped {
			escaped = false
			i++
			continue
		}
		if ch == '\\' {
			escaped = true
			i++
			continue
		}
		if (ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r') && depth == 0 {
			break
		}
		if ch == '(' {
			depth++
		} else if ch == ')' {
			if depth == 0 {
				break
			}
			depth--
		}
		i++
	}

	target := strings.TrimSpace(s[:i])
	if strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">") && len(target) > 2 {
		target = target[1 : len(target)-1]
	}
	return target
}

func findMatchingBracket(line string, start int) int {
	depth := 0
	escaped := false
	for i := start; i < len(line); i++ {
		ch := line[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '[' {
			depth++
		} else if ch == ']' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func readParenthesized(line string, start int) (string, int) {
	depth := 0
	escaped := false
	for i := start; i < len(line); i++ {
		ch := line[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				return line[start+1 : i], i
			}
		}
	}
	return "", -1
}

type span struct {
	start int
	end   int // exclusive
}

func detectFenceMarker(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "```") {
		return "```", true
	}
	if strings.HasPrefix(trimmed, "~~~") {
		return "~~~", true
	}
	return "", false
}

func inlineCodeSpans(line string) []span {
	var spans []span
	for i := 0; i < len(line); i++ {
		if line[i] != '`' {
			continue
		}

		ticks := 1
		for i+ticks < len(line) && line[i+ticks] == '`' {
			ticks++
		}

		closeStart := findClosingBackticks(line, i+ticks, ticks)
		if closeStart == -1 {
			i += ticks - 1
			continue
		}

		closeEnd := closeStart + ticks
		spans = append(spans, span{start: i, end: closeEnd})
		i = closeEnd - 1
	}
	return spans
}

func findClosingBackticks(line string, from, ticks int) int {
	for i := from; i < len(line); i++ {
		if line[i] != '`' {
			continue
		}
		count := 1
		for i+count < len(line) && line[i+count] == '`' {
			count++
		}
		if count == ticks {
			return i
		}
		i += count - 1
	}
	return -1
}

func indexInSpans(idx int, spans []span) bool {
	for _, s := range spans {
		if idx >= s.start && idx < s.end {
			return true
		}
	}
	return false
}

// isExternalOrAnchor returns true for links that should not be checked on disk.
func isExternalOrAnchor(target string) bool {
	lower := strings.ToLower(target)
	for _, prefix := range []string{
		"http://", "https://", "mailto:", "tel:", "data:", "ftp://", "file:", "//",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	if idx := strings.IndexByte(target, ':'); idx > 0 {
		scheme := strings.ToLower(target[:idx])
		if isURLScheme(scheme) {
			return true
		}
	}
	return strings.HasPrefix(target, "#")
}

func isURLScheme(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if i > 0 && ((r >= '0' && r <= '9') || r == '+' || r == '-' || r == '.') {
			continue
		}
		return false
	}
	return true
}

// stripFragment removes #fragment and ?query from a link target.
func stripFragment(target string) string {
	if i := strings.IndexByte(target, '#'); i >= 0 {
		target = target[:i]
	}
	if i := strings.IndexByte(target, '?'); i >= 0 {
		target = target[:i]
	}
	return target
}
