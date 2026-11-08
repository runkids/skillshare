package audit

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"skillshare/internal/utils"
)

const (
	ruleWhitespacePadding = "whitespace-padding"
	paddingBlankMedium    = 20
	paddingBlankHigh      = 40
	paddingHorizontal     = 80
	paddingDensityBytes   = 4096
	paddingDensityPercent = 95
	paddingRepeatedLines  = 100
)

type paddingLine struct {
	text                       string
	sourceLine, separatorBytes int
}

// paddingSeparator recognizes the same visual line units for both analysis and
// work accounting. CRLF is one separator, even though it contains two runes.
func paddingSeparator(source string, i int) int {
	r, size := utf8.DecodeRuneInString(source[i:])
	if !strings.ContainsRune("\r\n\v\f\u0085\u2028\u2029", r) {
		return 0
	}
	if r == '\r' && i+1 < len(source) && source[i+1] == '\n' {
		return 2
	}
	return size
}

// VisualLineCount counts the visual lines analyzed by whitespace padding,
// including an empty final line. It allocates no per-line storage, so callers
// can enforce a line budget before constructing the scanner's line inventory.
func VisualLineCount(source string) int {
	count := 1
	for i := 0; i < len(source); {
		if size := paddingSeparator(source, i); size > 0 {
			count++
			i += size
		} else {
			_, size := utf8.DecodeRuneInString(source[i:])
			i += size
		}
	}
	return count
}

// paddingLines recognizes visual separators without changing LF-based report
// locations. CRLF is one separator; Unicode separators keep their original line.
func paddingLines(source string) []paddingLine {
	var lines []paddingLine
	start, sourceLine := 0, 1
	for i := 0; i < len(source); {
		size := paddingSeparator(source, i)
		if size == 0 {
			_, size = utf8.DecodeRuneInString(source[i:])
			i += size
			continue
		}
		lines = append(lines, paddingLine{source[start:i], sourceLine, size})
		if source[i] == '\n' || size == 2 && source[i] == '\r' {
			sourceLine++
		}
		i += size
		start = i
	}
	if start < len(source) {
		lines = append(lines, paddingLine{source[start:], sourceLine, 0})
	}
	return lines
}

// paddingExemptions uses parsed Markdown, so an incidental pipe in prose cannot
// exempt a line. Normalize separators only for parsing; findings use paddingLine.
func paddingExemptions(lines []paddingLine, precomputed ...[]markdownCodeBlock) []bool {
	excluded := make([]bool, len(lines))
	offsets := make([]int, len(lines))
	var normalized strings.Builder
	for i, line := range lines {
		offsets[i] = normalized.Len()
		normalized.WriteString(line.text)
		normalized.WriteByte('\n')
	}
	content := []byte(normalized.String())
	var blocks []markdownCodeBlock
	if len(precomputed) > 0 {
		blocks = precomputed[0]
	} else {
		blocks = markdownCodeBlocks(content)
	}
	for _, block := range blocks {
		if block.inHTML {
			continue
		}
		for i := max(0, block.start-1); i <= block.end && i < len(lines); i++ {
			excluded[i] = true
		}
	}
	doc := goldmark.New(goldmark.WithExtensions(extension.Table)).Parser().Parse(text.NewReader(content))
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if block, ok := n.(*ast.CodeBlock); ok && block.Lines().Len() > 0 {
			start := block.Lines().At(0).Start
			end := block.Lines().At(block.Lines().Len() - 1).Stop
			first := sort.Search(len(offsets), func(i int) bool { return offsets[i] > start }) - 1
			for i := max(0, first); i < len(lines) && offsets[i] < end; i++ {
				excluded[i] = true
			}
			return ast.WalkSkipChildren, nil
		}
		if cell, ok := n.(*east.TableCell); ok {
			for i := 0; i < cell.Lines().Len(); i++ {
				offset := cell.Lines().At(i).Start
				line := sort.Search(len(offsets), func(i int) bool { return offsets[i] > offset }) - 1
				if line >= 0 {
					excluded[line] = true
					if _, header := cell.Parent().(*east.TableHeader); header && line+1 < len(lines) {
						excluded[line+1] = true
					}
				}
			}
		}
		return ast.WalkContinue, nil
	})
	return excluded
}

// scanWhitespacePadding is part of the registered static analyzer. The compiled
// rule carries the same global/project toggles and overrides as regex rules.
func scanWhitespacePadding(content []byte, filename string, rules []rule, blocks ...[]markdownCodeBlock) []Finding {
	findings, _ := scanWhitespacePaddingBounded(content, filename, rules, 0, blocks...)
	return findings
}

func scanWhitespacePaddingBounded(content []byte, filename string, rules []rule, maxFindings int, blocks ...[]markdownCodeBlock) ([]Finding, bool) {
	var configured *rule
	for i := range rules {
		if rules[i].ID == ruleWhitespacePadding {
			configured = &rules[i]
			break
		}
	}
	if configured == nil {
		return nil, false
	}
	lines := paddingLines(utils.TrimBOM(string(content)))
	findings, whitespace, exhausted := paddingLineFindingsBounded(lines, nil, filename, configured, maxFindings)
	if !strings.EqualFold(filepath.Ext(filename), ".md") {
		return findings, exhausted
	}
	// With no possible Markdown exemption, the first pass is already final.
	if !paddingMayHaveExemptions(lines) {
		return findings, exhausted
	}
	// Exemptions can change density but cannot create horizontal/blank/repeated
	// runs. Most files have too little whitespace to meet density even after
	// removing exempt text, so avoid parsing their Markdown again.
	if len(findings) == 0 && whitespace*100 <= paddingDensityBytes*paddingDensityPercent {
		return nil, false
	}
	// Non-LF visual separators are normalized for the exemption parser; the
	// ordinary Markdown scan does not use that normalization.
	if strings.ContainsAny(string(content), "\r\v\f\u0085\u2028\u2029") {
		blocks = nil
	}
	findings, _, exhausted = paddingLineFindingsBounded(lines, paddingExemptions(lines, blocks...), filename, configured, maxFindings)
	return findings, exhausted
}

func paddingLineFindingsBounded(lines []paddingLine, excluded []bool, filename string, configured *rule, maxFindings int) ([]Finding, int, bool) {
	var findings []Finding
	exhausted := false
	occurrences := make(map[string]int)
	emit := func(index int, severity, detail, acceptanceContext string) {
		if exhausted {
			return
		}
		occurrences[acceptanceContext]++
		acceptanceContext = fmt.Sprintf("%s:occurrence:%d", acceptanceContext, occurrences[acceptanceContext])
		if configured.severityOverridden {
			severity = configured.Severity
		}
		findings = append(findings, Finding{Severity: severity, Pattern: ruleWhitespacePadding,
			Message: fmt.Sprintf("%s: %s", configured.Message, detail), File: filename,
			Line: lines[index].sourceLine, Snippet: strings.TrimSpace(lines[index].text), acceptanceContext: acceptanceContext,
			RuleID: ruleWhitespacePadding, Analyzer: AnalyzerStatic, Category: CategoryObfuscation, Confidence: configured.Confidence})
		exhausted = maxFindings > 0 && len(findings) >= maxFindings
	}
	blankStart, blankCount, repeatStart, repeatCount := 0, 0, 0, 0
	previous := ""
	horizontalStart, horizontalCount := 0, 0
	var horizontalText strings.Builder
	flushHorizontal := func() {
		if horizontalCount == 0 {
			return
		}
		context := paddingAcceptanceContext("leading-horizontal", horizontalCount, paddingTextDigest(horizontalText.String()))
		emit(horizontalStart, SeverityMedium, fmt.Sprintf("%d consecutive lines with at least %d leading horizontal whitespace characters", horizontalCount, paddingHorizontal), context)
		horizontalCount = 0
		horizontalText.Reset()
	}
	flushBlank := func(payloadIndex int) {
		if blankCount >= paddingBlankMedium {
			severity := SeverityMedium
			if blankCount >= paddingBlankHigh {
				severity = SeverityHigh
			}
			context := paddingAcceptanceContext("blank-run", blankCount, paddingPayloadDigest(lines, excluded, payloadIndex))
			emit(blankStart, severity, fmt.Sprintf("%d consecutive blank lines", blankCount), context)
		}
		blankCount = 0
	}
	eligible, whitespace, first := 0, 0, -1
	for i, line := range lines {
		if exhausted {
			return findings, whitespace, true
		}
		if len(excluded) > 0 && excluded[i] {
			flushHorizontal()
			// Code/table text can be hidden by padding outside the block.
			// Whitespace inside the block remains exempt from accounting.
			if strings.TrimSpace(line.text) != "" {
				flushBlank(i)
			}
			blankCount = 0
			repeatCount = 0
			previous = ""
			continue
		}
		if first < 0 {
			first = i
		}
		eligible += len(line.text) + line.separatorBytes
		whitespace += line.separatorBytes
		horizontal := 0
		leading := true
		for _, r := range line.text {
			if unicode.IsSpace(r) {
				whitespace += utf8.RuneLen(r)
				if leading {
					horizontal++
				}
			} else {
				leading = false
			}
		}
		if !leading && horizontal >= paddingHorizontal {
			if horizontalCount == 0 {
				horizontalStart = i
			}
			horizontalCount++
			horizontalText.WriteString(line.text)
			horizontalText.WriteByte('\n')
		} else {
			flushHorizontal()
		}
		trimmed := strings.TrimSpace(line.text)
		if trimmed == "" {
			if blankCount == 0 {
				blankStart = i
			}
			blankCount++
			repeatCount = 0
			previous = ""
			continue
		}
		flushBlank(i)
		if line.text == previous {
			repeatCount++
		} else {
			if repeatCount >= paddingRepeatedLines {
				context := paddingAcceptanceContext("repeated-lines", repeatCount, paddingTextDigest(previous+"\n"+line.text))
				emit(repeatStart, SeverityMedium, fmt.Sprintf("%d identical lines before text", repeatCount), context)
			}
			previous = line.text
			repeatStart = i
			repeatCount = 1
		}
	}
	flushHorizontal()
	if exhausted {
		return findings, whitespace, true
	}
	if eligible > paddingDensityBytes && whitespace*100 >= eligible*paddingDensityPercent {
		context := paddingAcceptanceContext("density", eligible, paddingEligibleDigest(lines, excluded))
		emit(first, SeverityMedium, fmt.Sprintf("at least %d%% whitespace in %d eligible bytes", paddingDensityPercent, eligible), context)
	}
	return findings, whitespace, exhausted
}

func paddingAcceptanceContext(form string, count int, payloadDigest string) string {
	return fmt.Sprintf("%s:%d:%s", form, count, payloadDigest)
}

func paddingTextDigest(text string) string {
	digest := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", digest)
}

// paddingPayloadDigest identifies the paragraph or Markdown block following a
// padded blank run. Exempt code and table contents are included in full.
func paddingPayloadDigest(lines []paddingLine, excluded []bool, start int) string {
	h := sha256.New()
	for i := start; i < len(lines); i++ {
		if i > start && (len(excluded) == 0 || !excluded[i]) && strings.TrimSpace(lines[i].text) == "" {
			break
		}
		_, _ = h.Write([]byte(lines[i].text))
		if lines[i].separatorBytes > 0 {
			_, _ = h.Write([]byte{'\n'})
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func paddingEligibleDigest(lines []paddingLine, excluded []bool) string {
	h := sha256.New()
	for i, line := range lines {
		if len(excluded) > 0 && excluded[i] {
			continue
		}
		_, _ = h.Write([]byte(line.text))
		if line.separatorBytes > 0 {
			_, _ = h.Write([]byte{'\n'})
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Conservatively detect syntax that could introduce fenced or indented code,
// block quotes, list-contained code, or tables. False positives cost a parse.
func paddingMayHaveExemptions(lines []paddingLine) bool {
	for _, line := range lines {
		s := line.text
		indent := len(s) - len(strings.TrimLeft(s, " \t"))
		if strings.ContainsAny(s, "|>") || indent >= 4 || strings.Contains(s[:indent], "\t") {
			return true
		}
		t := strings.TrimLeft(s, " \t")
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			return true
		}
		if len(t) >= 2 && strings.ContainsRune("-*+", rune(t[0])) && (t[1] == ' ' || t[1] == '\t') {
			return true
		}
		i := 0
		for i < len(t) && t[i] >= '0' && t[i] <= '9' {
			i++
		}
		if i > 0 && i+1 < len(t) && (t[i] == '.' || t[i] == ')') && (t[i+1] == ' ' || t[i+1] == '\t') {
			return true
		}
	}
	return false
}
