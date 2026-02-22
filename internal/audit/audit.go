package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"skillshare/internal/utils"
)

const (
	maxScanFileSize = 1_000_000 // 1MB
	maxScanDepth    = 6
)

var riskWeights = map[string]int{
	SeverityCritical: 25,
	SeverityHigh:     15,
	SeverityMedium:   8,
	SeverityLow:      3,
	SeverityInfo:     1,
}

// Finding represents a single security issue detected in a skill.
type Finding struct {
	Severity string `json:"severity"` // "CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO"
	Pattern  string `json:"pattern"`  // rule name (e.g. "prompt-injection")
	Message  string `json:"message"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Snippet  string `json:"snippet"` // max 80 chars of the matched line
}

// Result holds all findings for a single skill.
type Result struct {
	SkillName  string    `json:"skillName"`
	Findings   []Finding `json:"findings"`
	RiskScore  int       `json:"riskScore"`
	RiskLabel  string    `json:"riskLabel"` // "clean", "low", "medium", "high", "critical"
	Threshold  string    `json:"threshold,omitempty"`
	IsBlocked  bool      `json:"isBlocked,omitempty"`
	ScanTarget string    `json:"scanTarget,omitempty"`
}

func (r *Result) updateRisk() {
	r.RiskScore = CalculateRiskScore(r.Findings)
	r.RiskLabel = RiskLabelFromScoreAndMaxSeverity(r.RiskScore, r.MaxSeverity())
}

// HasCritical returns true if any finding is CRITICAL severity.
func (r *Result) HasCritical() bool {
	return r.HasSeverityAtOrAbove(SeverityCritical)
}

// HasHigh returns true if any finding is HIGH or above.
func (r *Result) HasHigh() bool {
	return r.HasSeverityAtOrAbove(SeverityHigh)
}

// HasSeverityAtOrAbove returns true if any finding severity is at or above threshold.
func (r *Result) HasSeverityAtOrAbove(threshold string) bool {
	normalized, err := NormalizeThreshold(threshold)
	if err != nil {
		normalized = DefaultThreshold()
	}
	cutoff := SeverityRank(normalized)
	for _, f := range r.Findings {
		if SeverityRank(f.Severity) <= cutoff {
			return true
		}
	}
	return false
}

// MaxSeverity returns the highest severity found, or "" if no findings.
func (r *Result) MaxSeverity() string {
	max := ""
	maxRank := 999
	for _, f := range r.Findings {
		rank := SeverityRank(f.Severity)
		if rank < maxRank {
			max = f.Severity
			maxRank = rank
		}
	}
	return max
}

// CountBySeverity returns the count of findings at CRITICAL/HIGH/MEDIUM severities.
func (r *Result) CountBySeverity() (critical, high, medium int) {
	critical, high, medium, _, _ = r.CountBySeverityAll()
	return
}

// CountBySeverityAll returns the count of findings at each severity level.
func (r *Result) CountBySeverityAll() (critical, high, medium, low, info int) {
	for _, f := range r.Findings {
		switch f.Severity {
		case SeverityCritical:
			critical++
		case SeverityHigh:
			high++
		case SeverityMedium:
			medium++
		case SeverityLow:
			low++
		case SeverityInfo:
			info++
		}
	}
	return
}

// CalculateRiskScore converts findings into a normalized 0-100 risk score.
func CalculateRiskScore(findings []Finding) int {
	score := 0
	for _, f := range findings {
		score += riskWeights[f.Severity]
	}
	if score > 100 {
		return 100
	}
	return score
}

// RiskLabelFromScore maps risk score into one of: clean/low/medium/high/critical.
func RiskLabelFromScore(score int) string {
	switch {
	case score <= 0:
		return "clean"
	case score <= 25:
		return "low"
	case score <= 50:
		return "medium"
	case score <= 75:
		return "high"
	default:
		return "critical"
	}
}

// riskFloorLabelFromSeverity maps the highest finding severity to a minimum
// risk label so severity is not downplayed by weighted scoring.
func riskFloorLabelFromSeverity(maxSeverity string) string {
	switch maxSeverity {
	case SeverityCritical:
		return "critical"
	case SeverityHigh:
		return "high"
	case SeverityMedium:
		return "medium"
	case SeverityLow, SeverityInfo:
		return "low"
	default:
		return "clean"
	}
}

// riskLabelRank returns an ordinal for comparing risk labels.
func riskLabelRank(label string) int {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "clean":
		return 0
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	case "critical":
		return 4
	default:
		return -1
	}
}

// maxRiskLabel returns the higher-priority risk label.
func maxRiskLabel(a, b string) string {
	if riskLabelRank(b) > riskLabelRank(a) {
		return b
	}
	return a
}

// RiskLabelFromScoreAndMaxSeverity returns the effective risk label by combining
// weighted score and max finding severity, ensuring severity is never
// under-represented.
func RiskLabelFromScoreAndMaxSeverity(score int, maxSeverity string) string {
	return maxRiskLabel(RiskLabelFromScore(score), riskFloorLabelFromSeverity(maxSeverity))
}

// ScanSkill scans all scannable files in a skill directory using global rules.
func ScanSkill(skillPath string) (*Result, error) {
	return ScanSkillWithRules(skillPath, nil)
}

// ScanFile scans a single file using global rules.
func ScanFile(filePath string) (*Result, error) {
	return ScanFileWithRules(filePath, nil)
}

// ScanFileForProject scans a single file using project-mode rules.
func ScanFileForProject(filePath, projectRoot string) (*Result, error) {
	rules, err := RulesWithProject(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("load project rules: %w", err)
	}
	return ScanFileWithRules(filePath, rules)
}

// ScanSkillForProject scans a skill using project-mode rules
// (builtin + global user + project user overrides).
func ScanSkillForProject(skillPath, projectRoot string) (*Result, error) {
	rules, err := RulesWithProject(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("load project rules: %w", err)
	}
	return ScanSkillWithRules(skillPath, rules)
}

// ScanSkillWithRules scans all scannable files using the given rules.
// If activeRules is nil, the default global rules are used.
func ScanSkillWithRules(skillPath string, activeRules []rule) (*Result, error) {
	info, err := os.Stat(skillPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access skill path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", skillPath)
	}

	result := &Result{
		SkillName:  filepath.Base(skillPath),
		ScanTarget: skillPath,
	}

	err = filepath.Walk(skillPath, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		relPath, relErr := filepath.Rel(skillPath, path)
		if relErr != nil {
			return nil
		}
		depth := relDepth(relPath)

		if fi.IsDir() {
			if path != skillPath && utils.IsHidden(fi.Name()) {
				return filepath.SkipDir
			}
			if depth > maxScanDepth {
				return filepath.SkipDir
			}
			return nil
		}

		if depth > maxScanDepth {
			return nil
		}
		if fi.Size() > maxScanFileSize {
			return nil
		}

		if !isScannable(fi.Name()) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if isBinaryContent(data) {
			return nil
		}

		findings := ScanContentWithRules(data, relPath, activeRules)
		result.Findings = append(result.Findings, findings...)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error scanning skill: %w", err)
	}

	result.Findings = append(result.Findings, scanDanglingLinks(skillPath)...)

	result.updateRisk()
	return result, nil
}

// ScanFileWithRules scans a single file using the given rules.
// If activeRules is nil, the default global rules are used.
func ScanFileWithRules(filePath string, activeRules []rule) (*Result, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot access file path: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("not a file: %s", filePath)
	}

	result := &Result{
		SkillName:  filepath.Base(filePath),
		ScanTarget: filePath,
	}

	// Keep parity with directory scan boundaries.
	if info.Size() > maxScanFileSize || !isScannable(info.Name()) {
		result.updateRisk()
		return result, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if isBinaryContent(data) {
		result.updateRisk()
		return result, nil
	}

	result.Findings = ScanContentWithRules(data, filepath.Base(filePath), activeRules)
	result.updateRisk()
	return result, nil
}

// ScanContent scans raw content for security issues and returns findings.
// filename is used for reporting (e.g. "SKILL.md").
func ScanContent(content []byte, filename string) []Finding {
	return ScanContentWithRules(content, filename, nil)
}

// ScanContentWithRules scans content using the given rules.
// If rules is nil, the default global rules are used.
func ScanContentWithRules(content []byte, filename string, activeRules []rule) []Finding {
	if activeRules == nil {
		var err error
		activeRules, err = Rules()
		if err != nil {
			return nil
		}
	}

	var findings []Finding
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		for _, r := range activeRules {
			if r.Regex.MatchString(line) {
				if r.Exclude != nil && r.Exclude.MatchString(line) {
					continue
				}
				findings = append(findings, Finding{
					Severity: r.Severity,
					Pattern:  r.Pattern,
					Message:  r.Message,
					File:     filename,
					Line:     lineNum + 1, // 1-indexed
					Snippet:  truncate(strings.TrimSpace(line), 80),
				})
			}
		}
	}

	return findings
}

// isScannable returns true if the file should be scanned.
func isScannable(name string) bool {
	// Skip skillshare's own metadata files
	if name == ".skillshare-meta.json" {
		return false
	}

	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".txt", ".yaml", ".yml", ".json", ".toml",
		".sh", ".bash", ".zsh", ".fish",
		".py", ".js", ".ts", ".rb", ".go", ".rs":
		return true
	}
	// Also scan files without extension (e.g. Makefile, Dockerfile)
	if ext == "" {
		return true
	}
	return false
}

func relDepth(rel string) int {
	if rel == "." {
		return 0
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	return len(parts) - 1
}

func isBinaryContent(content []byte) bool {
	checkLen := len(content)
	if checkLen > 512 {
		checkLen = 512
	}
	for i := 0; i < checkLen; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

// truncate shortens s to maxLen characters, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// markdownLink captures a discovered markdown link with source line and label.
type markdownLink struct {
	Target string
	Line   int
	Label  string
}

var mdPathTokenRe = regexp.MustCompile(`(?i)[A-Za-z0-9_./-]+\.md`)

// extractMarkdownInlineLinks extracts inline links plus plain/code .md mentions
// used by dangling-link and external-repository checks.
func extractMarkdownInlineLinks(src []byte) []markdownLink {
	parser := goldmark.New().Parser()
	doc := parser.Parse(text.NewReader(src))

	var links []markdownLink
	seen := map[string]struct{}{}
	appendUnique := func(target string, line int, label string) {
		key := fmt.Sprintf("%d|%s|%s", line, target, strings.ToLower(strings.TrimSpace(label)))
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		links = append(links, markdownLink{Target: target, Line: line, Label: strings.TrimSpace(label)})
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		linkNode, ok := n.(*ast.Link)
		if ok {
			target := strings.TrimSpace(string(linkNode.Destination))
			if target != "" {
				line := lineFromOffset(src, linkStartOffset(linkNode, src))
				appendUnique(target, line, extractLinkLabel(linkNode, src))
			}
			return ast.WalkContinue, nil
		}

		var raw string
		var offset int
		switch inline := n.(type) {
		case *ast.Text:
			if _, inCodeSpan := inline.Parent().(*ast.CodeSpan); inCodeSpan {
				return ast.WalkContinue, nil
			}
			raw = strings.TrimSpace(string(inline.Text(src)))
			offset = inline.Segment.Start
		case *ast.CodeSpan:
			raw = strings.TrimSpace(string(inline.Text(src)))
			offset = nodeStartOffset(inline, src)
		default:
			return ast.WalkContinue, nil
		}

		if raw == "" {
			return ast.WalkContinue, nil
		}

		line := lineFromOffset(src, offset)
		for _, token := range mdPathTokenRe.FindAllString(raw, -1) {
			target := strings.TrimSpace(token)
			if target == "" {
				continue
			}
			appendUnique(target, line, "")
		}
		return ast.WalkContinue, nil
	})

	return links
}

// extractLinkLabel returns normalized visible label text for a markdown link.
func extractLinkLabel(linkNode *ast.Link, src []byte) string {
	var parts []string
	_ = ast.Walk(linkNode, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Text:
			text := strings.TrimSpace(string(node.Text(src)))
			if text != "" {
				parts = append(parts, text)
			}
		case *ast.CodeSpan:
			text := strings.TrimSpace(string(node.Text(src)))
			if text != "" {
				parts = append(parts, text)
			}
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(strings.Join(parts, " "))
}

// linkStartOffset returns the first text offset inside a link for line mapping.
func linkStartOffset(linkNode *ast.Link, src []byte) int {
	offset := -1
	_ = ast.Walk(linkNode, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if textNode, ok := n.(*ast.Text); ok {
			offset = textNode.Segment.Start
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	if offset >= 0 {
		return offset
	}
	return 0
}

// nodeStartOffset returns the first text offset for an arbitrary AST node.
func nodeStartOffset(node ast.Node, src []byte) int {
	offset := -1
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if textNode, ok := n.(*ast.Text); ok {
			offset = textNode.Segment.Start
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	if offset >= 0 {
		return offset
	}
	return 0
}

// lineFromOffset maps a byte offset in src to a 1-based line number.
func lineFromOffset(src []byte, offset int) int {
	if offset <= 0 {
		return 1
	}
	if offset > len(src) {
		offset = len(src)
	}
	line := 1
	for i := 0; i < offset; i++ {
		if src[i] == '\n' {
			line++
		}
	}
	return line
}

// isExternalOrAnchorLink returns true if the link target should be skipped
// (external schemes, protocol-relative URLs, or pure anchors).
func isExternalOrAnchorLink(target string) bool {
	lower := strings.ToLower(target)
	for _, prefix := range []string{
		"http://", "https://", "mailto:", "tel:", "data:", "ftp://", "//",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	// Pure anchor links such as (#section) — no file to resolve.
	return strings.HasPrefix(target, "#")
}

// isExternalWebLink returns true for web URLs (including protocol-relative).
func isExternalWebLink(target string) bool {
	lower := strings.ToLower(strings.TrimSpace(target))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "//")
}

// isSourceRepositoryLabel matches common "source repository" link labels.
func isSourceRepositoryLabel(label string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(label)), " "))
	return normalized == "source repository" || normalized == "source repo"
}

// stripFragmentAndQuery removes any #fragment or ?query from a link target.
func stripFragmentAndQuery(target string) string {
	if i := strings.IndexByte(target, '#'); i >= 0 {
		target = target[:i]
	}
	if i := strings.IndexByte(target, '?'); i >= 0 {
		target = target[:i]
	}
	return target
}

// scanDanglingLinks walks all .md files in skillPath and returns a Finding for
// every local relative markdown link whose target does not exist on disk.
func scanDanglingLinks(skillPath string) []Finding {
	var findings []Finding

	_ = filepath.Walk(skillPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		relPath, relErr := filepath.Rel(skillPath, path)
		if relErr != nil {
			return nil
		}
		depth := relDepth(relPath)

		if fi.IsDir() {
			if path != skillPath && utils.IsHidden(fi.Name()) {
				return filepath.SkipDir
			}
			if depth > maxScanDepth {
				return filepath.SkipDir
			}
			return nil
		}

		if depth > maxScanDepth {
			return nil
		}
		if fi.Size() > maxScanFileSize {
			return nil
		}
		if strings.ToLower(filepath.Ext(fi.Name())) != ".md" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		fileDir := filepath.Join(skillPath, filepath.Dir(relPath))
		lines := strings.Split(string(data), "\n")
		for _, link := range extractMarkdownInlineLinks(data) {
			target := link.Target
			lineNum := link.Line
			snippet := ""
			if lineNum > 0 && lineNum <= len(lines) {
				snippet = truncate(strings.TrimSpace(lines[lineNum-1]), 80)
			}

			if isExternalWebLink(target) && isSourceRepositoryLabel(link.Label) {
				findings = append(findings, Finding{
					Severity: SeverityHigh,
					Pattern:  "external-source-repository-link",
					Message:  fmt.Sprintf("external source repository link detected: %q", target),
					File:     relPath,
					Line:     lineNum,
					Snippet:  snippet,
				})
				continue
			}

			if isExternalOrAnchorLink(target) {
				continue
			}
			cleaned := stripFragmentAndQuery(target)
			if cleaned == "" {
				continue
			}
			absTarget := filepath.Join(fileDir, cleaned)
			if _, statErr := os.Stat(absTarget); statErr != nil {
				findings = append(findings, Finding{
					Severity: SeverityMedium,
					Pattern:  "dangling-link",
					Message:  fmt.Sprintf("dangling local markdown link: target %q not found", target),
					File:     relPath,
					Line:     lineNum,
					Snippet:  snippet,
				})
			}
		}
		return nil
	})

	return findings
}
