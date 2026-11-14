package audit

import (
	"path/filepath"
	"regexp"
	"strings"

	"skillshare/internal/utils"
)

// ScanContent scans raw content for security issues and returns findings.
// filename is used for reporting (e.g. "SKILL.md").
func ScanContent(content []byte, filename string) []Finding {
	return ScanContentWithRules(content, filename, nil)
}

// ScanContentWithRules scans content using the given rules.
// If rules is nil, the default global rules are used.
func ScanContentWithRules(content []byte, filename string, activeRules []rule) []Finding {
	findings, _ := scanContentWithRules(content, filename, activeRules, 0)
	return findings
}

// ScanContentWithRulesBounded retains whole-content padding and line context
// while capping findings during both padding analysis and regex matching. The
// returned bool is true as soon as maxFindings is reached, or rules cannot load;
// callers must treat that as incomplete. Input byte/line limits belong to the
// caller. The legacy scanner remains unlimited and preserves its behavior.
func ScanContentWithRulesBounded(content []byte, filename string, activeRules []rule, maxFindings int) ([]Finding, bool) {
	if maxFindings <= 0 {
		return nil, true
	}
	return scanContentWithRules(content, filename, activeRules, maxFindings)
}

func scanContentWithRules(content []byte, filename string, activeRules []rule, maxFindings int) ([]Finding, bool) {
	if activeRules == nil {
		var err error
		activeRules, err = Rules()
		if err != nil {
			return nil, true
		}
	}

	activeRules = rulesForFile(activeRules, filename)
	findings, exhausted := scanWhitespacePaddingBounded(content, filename, activeRules, maxFindings)
	if exhausted {
		return findings, true
	}
	// A BOM at the very start is the file's encoding mark, not a hidden character.
	text := utils.TrimBOM(string(content))
	// Admitted scripts need not have an extension. A first-line shebang is
	// code evidence; extensionless notes without one retain prose severity.
	scriptContext := strings.HasPrefix(text, "#!")
	var inert []bool
	var blocks []markdownCodeBlock
	if strings.EqualFold(filepath.Ext(filename), ".md") {
		for _, r := range activeRules {
			if r.usesMetadataContext() {
				blocks = markdownCodeBlocks([]byte(text))
				break
			}
		}
	}
	blockIndex := 0
	lineNum := 0
	for start := 0; start <= len(text); {
		for blockIndex < len(blocks) && lineNum >= blocks[blockIndex].end {
			blockIndex++
		}
		codeContext := scriptContext || (blockIndex < len(blocks) && lineNum >= blocks[blockIndex].start && metadataCodeFence(blocks[blockIndex].language))
		lineNum++
		end := strings.IndexByte(text[start:], '\n')
		var line string
		if end == -1 {
			line = text[start:]
			start = len(text) + 1
		} else {
			line = text[start : start+end]
			start = start + end + 1
		}
		lineLower := ""
		lineLowerReady := false
		for _, r := range activeRules {
			if !rulePrefilterAllows(r, line, &lineLower, &lineLowerReady) {
				continue
			}
			if r.matchesLine(line) {
				sev, sevContext := r.inertSeverity(r.findingSeverity(line, filename, codeContext), &inert, filename, text, lineNum)
				findings = append(findings, Finding{
					Severity:        sev,
					SeverityContext: sevContext,
					Pattern:         r.Pattern,
					Message:         r.Message,
					File:            filename,
					Line:            lineNum,
					Snippet:         strings.TrimSpace(line),
					RuleID:          r.ID,
					Analyzer:        AnalyzerStatic,
					Category:        categoryForPattern(r.Pattern),
					Confidence:      r.Confidence,
				})
				if maxFindings > 0 && len(findings) >= maxFindings {
					return findings, true
				}
			}
		}
	}

	return findings, false
}

// ScanMarkdownContentWithRules scans markdown content, suppresses selected
// tutorial patterns, and downgrades SDK-style system parameters in code fences.
func ScanMarkdownContentWithRules(content []byte, filename string, activeRules []rule) []Finding {
	if activeRules == nil {
		var err error
		activeRules, err = Rules()
		if err != nil {
			return nil
		}
	}

	findings, _ := scanFileUnified(content, filename, true, activeRules, nil, true, false, false)
	return findings
}

// Match parameter values, not prose after a role label. Other injection rules
// still scan the entire line and retain their configured severity.
var mdSystemValueRe = regexp.MustCompile("^(?:[\"'`\\[]|[A-Za-z_][A-Za-z0-9_]*(?:\\.[A-Za-z_][A-Za-z0-9_]*)*\\(|[|>][+-]?$|[A-Za-z_][A-Za-z0-9_]*,[ \\t]*(?://.*)?$)")

func markdownFindingSeverity(r rule, line string, inCodeFence bool, nextLine string) string {
	if r.Pattern != "prompt-injection" || r.Severity != SeverityCritical || r.severityOverridden {
		return r.Severity
	}
	if !inCodeFence || r.ID != "prompt-injection-1" {
		return r.Severity
	}
	if isMarkdownSystemParameter(line, nextLine) {
		return SeverityHigh
	}
	return r.Severity
}

func isMarkdownSystemParameter(line, nextLine string) bool {
	key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
	if !ok || (key != "system" && key != "System") {
		return false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		// A split argument must continue with code, not a bare role directive.
		value = strings.TrimSpace(nextLine)
		return strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") || strings.HasPrefix(value, "[")
	}
	return mdSystemValueRe.MatchString(value)
}

var tutorialSuppressedPatterns = map[string]bool{
	"dynamic-code-exec":    true,
	"shell-execution":      true,
	"destructive-commands": true,
	"suspicious-fetch":     true,

	"insecure-http":      true,
	"escape-obfuscation": true,
	"hidden-unicode":     true,
	"fetch-with-pipe":    true,
	"untrusted-install":  true,
}

func shouldSuppressTutorialExample(pattern, line string, inCodeFence, tutorialPath bool) bool {
	if !tutorialSuppressedPatterns[pattern] {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if inCodeFence || tutorialPath {
		return true
	}
	return mdTutorialMarkerRe.MatchString(trimmed)
}

func isLikelyTutorialPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	parts := strings.Split(lower, "/")
	for _, p := range parts {
		switch p {
		case "reference", "references", "resource", "resources", "template", "templates", "example", "examples":
			return true
		}
	}
	return false
}
