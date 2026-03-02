package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"skillshare/internal/utils"
)

// ErrBlocked is a sentinel error indicating that an operation was blocked
// by the security audit. Use errors.Is(err, audit.ErrBlocked) to check.
var ErrBlocked = errors.New("blocked by security audit")

const (
	maxScanFileSize        = 1_000_000 // 1MB
	maxScanDepth           = 6
	analyzabilityThreshold = 0.70
)

// mdFileInfo holds data collected during the walk for structural checks.
type mdFileInfo struct {
	relPath string
	data    []byte
	absDir  string // absolute directory containing this file
}

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
	Snippet  string `json:"snippet"` // trimmed matched line (no truncation)
}

// Result holds all findings for a single skill.
type Result struct {
	SkillName      string      `json:"skillName"`
	Findings       []Finding   `json:"findings"`
	RiskScore      int         `json:"riskScore"`
	RiskLabel      string      `json:"riskLabel"` // "clean", "low", "medium", "high", "critical"
	Threshold      string      `json:"threshold,omitempty"`
	IsBlocked      bool        `json:"isBlocked,omitempty"`
	ScanTarget     string      `json:"scanTarget,omitempty"`
	TotalBytes     int64       `json:"totalBytes"`
	AuditableBytes int64       `json:"auditableBytes"`
	Analyzability  float64     `json:"analyzability"` // AuditableBytes / TotalBytes (1.0 when TotalBytes == 0)
	TierProfile    TierProfile `json:"tierProfile"`
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

// riskLabelRanks maps risk labels to numeric ranks (lower = more severe).
var riskLabelRanks = map[string]int{
	"critical": 0,
	"high":     1,
	"medium":   2,
	"low":      3,
	"clean":    4,
}

// riskLabelRank returns the numeric rank for a risk label (lower = more severe).
func riskLabelRank(label string) int {
	if r, ok := riskLabelRanks[label]; ok {
		return r
	}
	return 999
}

// riskFloorFromSeverity returns the minimum risk label implied by a severity.
func riskFloorFromSeverity(severity string) string {
	switch severity {
	case SeverityCritical:
		return "critical"
	case SeverityHigh:
		return "high"
	case SeverityMedium:
		return "medium"
	case SeverityLow:
		return "low"
	default:
		return "clean"
	}
}

// RiskLabelFromScoreAndMaxSeverity computes the risk label as the higher of
// the score-based label and the severity floor. This ensures a single HIGH
// finding is never reported as "low" risk.
func RiskLabelFromScoreAndMaxSeverity(score int, maxSeverity string) string {
	scoreLabel := RiskLabelFromScore(score)
	floor := riskFloorFromSeverity(maxSeverity)
	if riskLabelRank(floor) < riskLabelRank(scoreLabel) {
		return floor
	}
	return scoreLabel
}

// ScanSkill scans all scannable files in a skill directory using global rules.
func ScanSkill(skillPath string) (*Result, error) {
	disabled := disabledIDsGlobal()
	return scanSkillImpl(skillPath, nil, disabled)
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
	disabled := disabledIDsForProject(projectRoot)
	return scanSkillImpl(skillPath, rules, disabled)
}

// ScanSkillWithRules scans all scannable files using the given rules.
// If activeRules is nil, the default global rules are used.
// Structural checks (e.g. dangling-link) always run; to disable them
// use ScanSkill / ScanSkillForProject which honour audit-rules.yaml.
func ScanSkillWithRules(skillPath string, activeRules []rule) (*Result, error) {
	return scanSkillImpl(skillPath, activeRules, nil)
}

func scanSkillImpl(skillPath string, activeRules []rule, disabled map[string]bool) (*Result, error) {
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

	resolvedRules := activeRules
	if resolvedRules == nil {
		rules, err := Rules()
		if err == nil {
			resolvedRules = rules
		}
	}
	mdContentRules, mdLinkRules := splitMarkdownLinkRules(resolvedRules)

	var mdFiles []mdFileInfo
	// fileCache collects file contents read during walk so that
	// checkContentIntegrity can reuse them instead of re-reading from disk.
	fileCache := make(map[string][]byte)

	var totalBytes, auditableBytes int64
	var skillTierProfile TierProfile

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
		// Files exceeding maxScanFileSize are excluded from totalBytes so that
		// analyzability reflects the ratio among files the scanner considers,
		// not raw on-disk size. Oversized files are a separate concern.
		if fi.Size() > maxScanFileSize {
			return nil
		}

		// Exclude skillshare's own metadata from total bytes — it's not
		// part of skill content and would skew the analyzability ratio.
		if fi.Name() == ".skillshare-meta.json" {
			return nil
		}

		totalBytes += fi.Size()

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

		auditableBytes += int64(len(data))

		// Cache content for content-integrity check reuse.
		fileCache[filepath.ToSlash(relPath)] = data

		isMarkdown := strings.EqualFold(filepath.Ext(fi.Name()), ".md")
		if isMarkdown {
			mdFiles = append(mdFiles, mdFileInfo{
				relPath: relPath,
				data:    data,
				absDir:  filepath.Dir(path),
			})
		}

		rulesForFile := resolvedRules
		if isMarkdown {
			rulesForFile = mdContentRules
		}
		var findings []Finding
		if isMarkdown {
			findings = ScanMarkdownContentWithRules(data, relPath, rulesForFile)
		} else {
			findings = ScanContentWithRules(data, relPath, rulesForFile)
		}
		result.Findings = append(result.Findings, findings...)

		// Tier detection: classify commands in parallel with pattern scan.
		if isMarkdown {
			skillTierProfile.Merge(DetectCommandTiersInMarkdown(data))
		} else {
			skillTierProfile.Merge(DetectCommandTiers(data))
		}

		// Dataflow taint tracking: detect cross-line source→sink chains.
		if !disabled[patternDataflowTaint] {
			var dfFindings []Finding
			if isMarkdown {
				dfFindings = ScanMarkdownDataflow(data, relPath)
			} else if isShellFile(fi.Name()) {
				dfFindings = ScanShellDataflow(data, relPath)
			}
			result.Findings = append(result.Findings,
				DeduplicateDataflow(dfFindings, findings)...)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error scanning skill: %w", err)
	}

	// Markdown link rules are parsed structurally to support full Markdown syntax
	// (title suffix, autolink, reference style, multiline).
	result.Findings = append(result.Findings, checkMarkdownLinkRules(mdFiles, mdLinkRules)...)

	// Structural check: scan collected .md files for dangling local links.
	if !disabled["dangling-link"] {
		result.Findings = append(result.Findings, checkDanglingLinks(mdFiles)...)
	}

	// Content integrity check against pinned hashes.
	if !disabled["content-integrity"] {
		result.Findings = append(result.Findings, checkContentIntegrity(skillPath, fileCache)...)
	}

	// Aggregate tier profile and generate combination findings.
	result.TierProfile = skillTierProfile
	result.Findings = append(result.Findings, TierCombinationFindings(skillTierProfile)...)

	result.TotalBytes = totalBytes
	result.AuditableBytes = auditableBytes
	if totalBytes > 0 {
		result.Analyzability = float64(auditableBytes) / float64(totalBytes)
	} else {
		result.Analyzability = 1.0
	}

	if totalBytes > 0 && result.Analyzability < analyzabilityThreshold {
		result.Findings = append(result.Findings, Finding{
			Severity: SeverityInfo,
			Pattern:  "low-analyzability",
			Message:  fmt.Sprintf("only %.0f%% of skill content is auditable (%.0f%% is binary/non-scannable)", result.Analyzability*100, (1-result.Analyzability)*100),
			File:     ".",
			Line:     0,
		})
	}

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

	fileSize := info.Size()
	result := &Result{
		SkillName:  filepath.Base(filePath),
		ScanTarget: filePath,
		TotalBytes: fileSize,
	}

	// Keep parity with directory scan boundaries.
	if fileSize > maxScanFileSize || !isScannable(info.Name()) {
		// Non-scannable or oversized: nothing is auditable.
		if fileSize > 0 {
			result.Analyzability = 0.0
		} else {
			result.Analyzability = 1.0
		}
		result.updateRisk()
		return result, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if isBinaryContent(data) {
		// Binary content: counted in TotalBytes but not auditable.
		result.Analyzability = 0.0
		result.updateRisk()
		return result, nil
	}

	result.AuditableBytes = int64(len(data))
	result.Analyzability = 1.0

	resolvedRules := activeRules
	if resolvedRules == nil {
		rules, err := Rules()
		if err == nil {
			resolvedRules = rules
		}
	}

	isMarkdown := strings.EqualFold(filepath.Ext(info.Name()), ".md")
	if isMarkdown {
		mdContentRules, mdLinkRules := splitMarkdownLinkRules(resolvedRules)
		result.Findings = ScanMarkdownContentWithRules(data, filepath.Base(filePath), mdContentRules)
		result.Findings = append(result.Findings, checkMarkdownLinkRules([]mdFileInfo{
			{
				relPath: filepath.Base(filePath),
				data:    data,
				absDir:  filepath.Dir(filePath),
			},
		}, mdLinkRules)...)
		result.TierProfile = DetectCommandTiersInMarkdown(data)
	} else {
		result.Findings = ScanContentWithRules(data, filepath.Base(filePath), resolvedRules)
		result.TierProfile = DetectCommandTiers(data)
	}

	// Dataflow taint tracking for single-file scan.
	var dfFindings []Finding
	if isShellFile(info.Name()) {
		dfFindings = ScanShellDataflow(data, filepath.Base(filePath))
	} else if isMarkdown {
		dfFindings = ScanMarkdownDataflow(data, filepath.Base(filePath))
	}
	result.Findings = append(result.Findings,
		DeduplicateDataflow(dfFindings, result.Findings)...)

	result.Findings = append(result.Findings, TierCombinationFindings(result.TierProfile)...)
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
	text := string(content)
	lineNum := 0
	for start := 0; start <= len(text); {
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
					Line:     lineNum,
					Snippet:  strings.TrimSpace(line),
				})
			}
		}
	}

	return findings
}

// ScanMarkdownContentWithRules scans markdown content and suppresses selected
// non-critical patterns when they appear in educational example context.
func ScanMarkdownContentWithRules(content []byte, filename string, activeRules []rule) []Finding {
	if activeRules == nil {
		var err error
		activeRules, err = Rules()
		if err != nil {
			return nil
		}
	}

	var findings []Finding
	text := string(content)
	inCodeFence := false
	fenceMarker := ""
	tutorialPath := isLikelyTutorialPath(filename)
	lineNum := 0

	for start := 0; start <= len(text); {
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

		if marker, ok := detectFenceMarker(line); ok {
			if !inCodeFence {
				inCodeFence = true
				fenceMarker = marker
			} else if marker == fenceMarker {
				inCodeFence = false
				fenceMarker = ""
			}
			continue
		}

		for _, r := range activeRules {
			if !r.Regex.MatchString(line) {
				continue
			}
			if r.Exclude != nil && r.Exclude.MatchString(line) {
				continue
			}
			if shouldSuppressTutorialExample(r.Pattern, line, inCodeFence, tutorialPath) {
				continue
			}
			findings = append(findings, Finding{
				Severity: r.Severity,
				Pattern:  r.Pattern,
				Message:  r.Message,
				File:     filename,
				Line:     lineNum,
				Snippet:  strings.TrimSpace(line),
			})
		}
	}

	return findings
}

// isScannable returns true if the file should be scanned.
func isScannable(name string) bool {
	// Skip skillshare's own metadata files
	if name == ".skillshare-meta.json" { // install.MetaFileName (cycle prevents import)
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

var (
	mdAutoLinkRe       = regexp.MustCompile(`<((?:https?://)[^>\s]+)>`)
	mdRefLinkRe        = regexp.MustCompile(`\[([^\]]+)\]\[([^\]]*)\]`)
	mdHTMLAnchorRe     = regexp.MustCompile(`(?i)<a\b[^>]*\bhref\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a>`)
	mdHTMLTagStrip     = regexp.MustCompile(`(?s)<[^>]*>`)
	mdTutorialMarkerRe = regexp.MustCompile(`(?i)(for\s+example|e\.g\.|example:|examples:|sample:|original:|attacker:|safe:|unsafe:|vulnerable:|pattern:|ruleid:|ok:|sink:|message:)`)
)

var tutorialSuppressedPatterns = map[string]bool{
	"dynamic-code-exec":    true,
	"shell-execution":      true,
	"destructive-commands": true,
	"suspicious-fetch":     true,
	"system-writes":        true,
	"insecure-http":        true,
	"escape-obfuscation":   true,
	"hidden-unicode":       true,
	"fetch-with-pipe":      true,
}

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
	return pattern == "external-link" || pattern == "source-repository-link"
}

func checkMarkdownLinkRules(files []mdFileInfo, markdownLinkRules []rule) []Finding {
	if len(markdownLinkRules) == 0 {
		return nil
	}

	var findings []Finding
	for _, f := range files {
		links := extractMarkdownLinks(f.data)
		for _, link := range links {
			canonical := fmt.Sprintf("[%s](%s)", link.label, link.target)
			for _, r := range markdownLinkRules {
				if !r.Regex.MatchString(canonical) {
					continue
				}
				if r.Exclude != nil && r.Exclude.MatchString(canonical) {
					continue
				}
				findings = append(findings, Finding{
					Severity: r.Severity,
					Pattern:  r.Pattern,
					Message:  r.Message,
					File:     f.relPath,
					Line:     link.line,
					Snippet:  link.snippet,
				})
			}
		}
	}
	return findings
}

// checkDanglingLinks scans collected .md file data for local relative links
// whose targets do not exist on disk. Returns LOW-severity findings.
func checkDanglingLinks(files []mdFileInfo) []Finding {
	var findings []Finding
	for _, f := range files {
		for _, link := range extractMarkdownLinks(f.data) {
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
					Severity: SeverityLow,
					Pattern:  "dangling-link",
					Message:  fmt.Sprintf("broken local link: %q not found", target),
					File:     f.relPath,
					Line:     link.line,
					Snippet:  link.snippet,
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
		for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
			j++
		}
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

// isExternalOrAnchor returns true for links that should not be checked on disk.
func isExternalOrAnchor(target string) bool {
	lower := strings.ToLower(target)
	for _, prefix := range []string{
		"http://", "https://", "mailto:", "tel:", "data:", "ftp://", "//",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return strings.HasPrefix(target, "#")
}

// checkContentIntegrity compares files on disk against pinned hashes in
// .skillshare-meta.json. Backward-compatible: skips silently when meta or
// file_hashes is absent. cache holds file contents already read during the
// walk phase; files not in cache are read from disk as fallback.
func checkContentIntegrity(skillPath string, cache map[string][]byte) []Finding {
	metaPath := filepath.Join(skillPath, ".skillshare-meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil // no meta → skip
	}

	var raw struct {
		FileHashes map[string]string `json:"file_hashes"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || len(raw.FileHashes) == 0 {
		return nil // no hashes → skip
	}

	var findings []Finding

	// Check pinned files: missing or tampered
	for rel, expected := range raw.FileHashes {
		normalizedRel := filepath.FromSlash(rel)
		// Reject absolute keys in metadata (e.g. "/etc/passwd").
		// file_hashes must always be skill-relative paths.
		if filepath.IsAbs(normalizedRel) {
			continue
		}

		absPath := filepath.Clean(filepath.Join(skillPath, normalizedRel))
		// Containment check: reject keys that escape the skill directory
		if !strings.HasPrefix(absPath, filepath.Clean(skillPath)+string(filepath.Separator)) {
			continue
		}
		info, err := os.Stat(absPath)
		if err != nil {
			findings = append(findings, Finding{
				Severity: SeverityLow,
				Pattern:  "content-missing",
				Message:  fmt.Sprintf("pinned file missing: %s", rel),
				File:     rel,
				Line:     0,
			})
			continue
		}
		if info.IsDir() {
			continue
		}
		if info.Size() > maxScanFileSize {
			findings = append(findings, Finding{
				Severity: SeverityMedium,
				Pattern:  "content-oversize",
				Message:  fmt.Sprintf("pinned file exceeds scan size limit (%d bytes): %s", info.Size(), rel),
				File:     rel,
				Line:     0,
			})
			continue
		}
		// Use cached content when available to avoid re-reading from disk.
		normalized := filepath.ToSlash(rel)
		var actual string
		if cached, ok := cache[normalized]; ok {
			sum := sha256.Sum256(cached)
			actual = hex.EncodeToString(sum[:])
		} else {
			var hashErr error
			actual, hashErr = utils.FileHash(absPath)
			if hashErr != nil {
				continue
			}
		}
		if "sha256:"+actual != expected {
			findings = append(findings, Finding{
				Severity: SeverityMedium,
				Pattern:  "content-tampered",
				Message:  fmt.Sprintf("file hash mismatch: %s", rel),
				File:     rel,
				Line:     0,
			})
		}
	}

	// Check for unexpected files not in the pinned set
	filepath.Walk(skillPath, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if fi.IsDir() {
			if fi.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if fi.Name() == ".skillshare-meta.json" {
			return nil
		}
		rel, relErr := filepath.Rel(skillPath, path)
		if relErr != nil {
			return nil
		}
		normalized := filepath.ToSlash(rel)
		if _, ok := raw.FileHashes[normalized]; !ok {
			findings = append(findings, Finding{
				Severity: SeverityLow,
				Pattern:  "content-unexpected",
				Message:  fmt.Sprintf("file not in pinned hashes: %s", normalized),
				File:     normalized,
				Line:     0,
			})
		}
		return nil
	})

	return findings
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
