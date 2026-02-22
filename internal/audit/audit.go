package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"skillshare/internal/utils"
)

const (
	maxScanFileSize = 1_000_000 // 1MB
	maxScanDepth    = 6
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

	var mdFiles []mdFileInfo

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

		if strings.ToLower(filepath.Ext(fi.Name())) == ".md" {
			mdFiles = append(mdFiles, mdFileInfo{
				relPath: relPath,
				data:    data,
				absDir:  filepath.Dir(path),
			})
		}

		findings := ScanContentWithRules(data, relPath, activeRules)
		result.Findings = append(result.Findings, findings...)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error scanning skill: %w", err)
	}

	// Structural check: scan collected .md files for dangling local links.
	if !disabled["dangling-link"] {
		result.Findings = append(result.Findings, checkDanglingLinks(mdFiles)...)
	}

	// Content integrity check against pinned hashes.
	if !disabled["content-integrity"] {
		result.Findings = append(result.Findings, checkContentIntegrity(skillPath)...)
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

// mdLinkRe matches Markdown inline links: [label](target).
var mdLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

// checkDanglingLinks scans collected .md file data for local relative links
// whose targets do not exist on disk. Returns LOW-severity findings.
func checkDanglingLinks(files []mdFileInfo) []Finding {
	var findings []Finding
	for _, f := range files {
		lines := strings.Split(string(f.data), "\n")
		for lineNum, line := range lines {
			for _, m := range mdLinkRe.FindAllStringSubmatch(line, -1) {
				target := m[1]
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
						Line:     lineNum + 1,
						Snippet:  truncate(strings.TrimSpace(line), 80),
					})
				}
			}
		}
	}
	return findings
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
// file_hashes is absent.
func checkContentIntegrity(skillPath string) []Finding {
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
		absPath := filepath.Join(skillPath, filepath.FromSlash(rel))
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
		actual, hashErr := fileHash(absPath)
		if hashErr != nil {
			continue
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

// fileHash returns the hex-encoded SHA-256 digest of a file.
func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
