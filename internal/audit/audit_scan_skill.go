package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// mdFileInfo holds data collected during the walk for structural checks.
type mdFileInfo struct {
	relPath string
	data    []byte
	absDir  string // absolute directory containing this file
}

// ScanSkill scans all scannable files in a skill directory using global rules.
func ScanSkill(skillPath string) (*Result, error) {
	return ScanSkillWithFollow(skillPath, nil)
}

// ScanSkillWithFollow resolves a followed source root while retaining logical
// result paths. A nil policy keeps the ordinary ScanSkill behavior.
func ScanSkillWithFollow(skillPath string, follow *sourcewalk.Follow) (*Result, error) {
	disabled := disabledIDsGlobal()
	return scanSkillImpl(skillPath, nil, disabled, nil, follow)
}

// ScanFile scans a single file using global rules.
func ScanFile(filePath string) (*Result, error) {
	return scanFileImpl(filePath, nil, disabledIDsGlobal())
}

// ScanFileForProject scans a single file using project-mode rules.
func ScanFileForProject(filePath, projectRoot string) (*Result, error) {
	rules, err := RulesWithProject(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("load project rules: %w", err)
	}
	return scanFileImpl(filePath, rules, disabledIDsForProject(projectRoot))
}

// ScanSkillForProject scans a skill using project-mode rules
// (builtin + global user + project user overrides).
func ScanSkillForProject(skillPath, projectRoot string, follow ...*sourcewalk.Follow) (*Result, error) {
	rules, err := RulesWithProject(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("load project rules: %w", err)
	}
	disabled := disabledIDsForProject(projectRoot)
	return scanSkillImpl(skillPath, rules, disabled, nil, follow...)
}

// ScanSkillWithRules scans all scannable files using the given rules.
// If activeRules is nil, the default global rules are used.
// Structural checks (e.g. dangling-link) always run; to disable them
// use ScanSkill / ScanSkillForProject which honour audit-rules.yaml.
func ScanSkillWithRules(skillPath string, activeRules []rule) (*Result, error) {
	return scanSkillImpl(skillPath, activeRules, nil, nil)
}

// ScanSkillFiltered scans a skill using the given registry to control which
// analyzers run. Pass a registry from DefaultRegistry().ForPolicy(policy).
func ScanSkillFiltered(skillPath string, registry *Registry, follow ...*sourcewalk.Follow) (*Result, error) {
	disabled := disabledIDsGlobal()
	return scanSkillImpl(skillPath, nil, disabled, registry, follow...)
}

// ScanSkillFilteredForProject is like ScanSkillFiltered but uses project-mode rules.
func ScanSkillFilteredForProject(skillPath, projectRoot string, registry *Registry, follow ...*sourcewalk.Follow) (*Result, error) {
	rules, err := RulesWithProject(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("load project rules: %w", err)
	}
	disabled := disabledIDsForProject(projectRoot)
	return scanSkillImpl(skillPath, rules, disabled, registry, follow...)
}

func scanSkillImpl(skillPath string, activeRules []rule, disabled map[string]bool, registry *Registry, follow ...*sourcewalk.Follow) (*Result, error) {
	logicalPath := skillPath
	if len(follow) > 0 {
		if resolved, ok := follow[0].Resolve(skillPath); ok {
			skillPath = resolved
		}
	}
	if registry == nil {
		registry = DefaultRegistry()
	}
	info, err := os.Stat(skillPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access skill path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", skillPath)
	}

	result := &Result{
		SkillName:  filepath.Base(logicalPath),
		ScanTarget: logicalPath,
	}

	resolvedRules := activeRules
	var mdContentRules, mdLinkRules []rule
	if resolvedRules == nil {
		rules, err := Rules()
		if err == nil {
			resolvedRules = rules
		}
		// Use cached split for global rules.
		mdContentRules, mdLinkRules = globalSplitMarkdownLinkRules()
	} else {
		mdContentRules, mdLinkRules = splitMarkdownLinkRules(resolvedRules)
	}

	var mdFiles []mdFileInfo
	// fileCache collects file contents read during walk so that
	// checkContentIntegrity can reuse them instead of re-reading from disk.
	fileCache := make(map[string][]byte)
	// allFiles collects every encountered file relPath (including non-scannable)
	// so checkContentIntegrity can skip its own filepath.Walk.
	allFiles := make(map[string]bool)

	var totalBytes, auditableBytes int64
	var skillTierProfile TierProfile

	// Pre-compute per-file analyzer checks to avoid O(n) linear scans per file.
	hasStatic := registry.Has(AnalyzerStatic)
	hasDataflow := registry.Has(AnalyzerDataflow)
	dfEnabled := hasDataflow && !disabled[patternDataflowTaint]

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

		// Exclude skillshare's own metadata — not part of skill content.
		if fi.Name() == ".skillshare-meta.json" {
			return nil
		}

		// Collect ALL non-meta files for integrity check (before size/scannable filters).
		normalizedRel := filepath.ToSlash(relPath)
		allFiles[normalizedRel] = true

		// Files exceeding maxScanFileSize are excluded from totalBytes so that
		// analyzability reflects the ratio among files the scanner considers,
		// not raw on-disk size. Oversized files are a separate concern.
		if fi.Size() > maxScanFileSize {
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
		fileCache[normalizedRel] = data

		isMarkdown := strings.EqualFold(filepath.Ext(fi.Name()), ".md")
		if isMarkdown {
			mdFiles = append(mdFiles, mdFileInfo{
				relPath: relPath,
				data:    data,
				absDir:  filepath.Dir(path),
			})
		}

		// --- Unified file scan: static + tier + dataflow ---
		var rulesForFile []rule
		if isMarkdown {
			rulesForFile = mdContentRules
		} else {
			rulesForFile = resolvedRules
		}
		staticFindings, dfFindings := scanFileUnified(
			data, relPath, isMarkdown,
			rulesForFile, &skillTierProfile,
			hasStatic, dfEnabled, isShellFile(fi.Name()),
		)
		result.Findings = append(result.Findings, staticFindings...)
		result.Findings = append(result.Findings,
			DeduplicateDataflow(dfFindings, staticFindings)...)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error scanning skill: %w", err)
	}

	// Pre-extract markdown links once for reuse by link rules + dangling link checks.
	mdLinks := make(map[string][]markdownLink, len(mdFiles))
	for _, mf := range mdFiles {
		mdLinks[mf.relPath] = extractMarkdownLinks(mf.data)
	}

	// --- Skill-scope analyzers (via registry) ---
	result.TierProfile = skillTierProfile
	skillCtx := &AnalyzeContext{
		SkillPath:      logicalPath, // Central metadata belongs to the logical source.
		MDFiles:        mdFiles,
		FileCache:      fileCache,
		TierProfile:    skillTierProfile,
		MDLinks:        mdLinks,
		AllFiles:       allFiles,
		Rules:          resolvedRules,
		MDContentRules: mdContentRules,
		MDLinkRules:    mdLinkRules,
		DisabledIDs:    disabled,
	}
	for _, a := range registry.SkillAnalyzers() {
		if af, err := a.Analyze(skillCtx); err == nil {
			result.Findings = append(result.Findings, af...)
		}
	}

	result.TotalBytes = totalBytes
	result.AuditableBytes = auditableBytes
	if totalBytes > 0 {
		result.Analyzability = float64(auditableBytes) / float64(totalBytes)
	} else {
		result.Analyzability = 1.0
	}

	if totalBytes > 0 && result.Analyzability < analyzabilityThreshold {
		result.Findings = append(result.Findings, Finding{
			Severity:   SeverityInfo,
			Pattern:    "low-analyzability",
			Message:    fmt.Sprintf("only %.0f%% of skill content is auditable (%.0f%% is binary/non-scannable)", result.Analyzability*100, (1-result.Analyzability)*100),
			File:       ".",
			Line:       0,
			RuleID:     "low-analyzability",
			Analyzer:   AnalyzerStatic,
			Category:   CategoryRisk,
			Confidence: 1.0,
		})
	}

	StampFingerprints(result.Findings)
	result.updateRisk()
	return result, nil
}

// scanFileUnified combines static regex scanning, tier detection, and dataflow
// collection, sharing line traversal or parsed Markdown blocks.
//
// activeRules should be mdContentRules for markdown files and resolvedRules
// for non-markdown files. profile is modified in place.
func scanFileUnified(
	data []byte, relPath string, isMarkdown bool,
	activeRules []rule, profile *TierProfile,
	hasStatic, hasDataflow, isShell bool,
) (staticFindings, dfFindings []Finding) {
	// A BOM at the very start is the file's encoding mark, not a hidden character.
	text := utils.TrimBOM(string(data))

	if isMarkdown {
		return scanFileUnifiedMarkdown(text, relPath, activeRules, profile, hasStatic, hasDataflow)
	}
	return scanFileUnifiedPlain(text, relPath, activeRules, profile, hasStatic, hasDataflow, isShell)
}

// scanFileUnifiedMarkdown shares parsed code blocks across static, tier, and
// dataflow analysis. All original lines still receive static security checks.
func scanFileUnifiedMarkdown(
	text, relPath string,
	activeRules []rule, profile *TierProfile,
	hasStatic, hasDataflow bool,
) (staticFindings, dfFindings []Finding) {
	blocks := markdownCodeBlocks([]byte(text))
	for _, block := range blocks {
		if profile != nil {
			for _, line := range block.lines {
				classifyLineCommands(line, profile)
			}
		}
		if hasDataflow && (block.language == "" || shellLangs[block.language]) {
			dfFindings = append(dfFindings, analyzeShellBlock(block.lines, block.start, relPath)...)
		}
	}
	if !hasStatic {
		return nil, dfFindings
	}

	tutorialPath := isLikelyTutorialPath(relPath)
	blockIndex, lineNum := 0, 0
	for line := range strings.SplitSeq(text, "\n") {
		for blockIndex < len(blocks) && lineNum >= blocks[blockIndex].end {
			blockIndex++
		}
		inCodeFence := blockIndex < len(blocks) && lineNum >= blocks[blockIndex].start
		parameterContext := inCodeFence && !blocks[blockIndex].inHTML
		nextLine := ""
		if inCodeFence {
			block := blocks[blockIndex]
			if index := lineNum - block.start + 1; index < len(block.lines) {
				nextLine = block.lines[index]
			}
		}
		lineNum++
		lineLower := ""
		lineLowerReady := false
		for _, r := range activeRules {
			if !rulePrefilterAllows(r, line, &lineLower, &lineLowerReady) || !r.matchesLine(line) {
				continue
			}
			if shouldSuppressTutorialExample(r.Pattern, line, inCodeFence, tutorialPath) {
				continue
			}
			staticFindings = append(staticFindings, Finding{
				Severity: markdownFindingSeverity(r, line, parameterContext, nextLine),
				Pattern:  r.Pattern, Message: r.Message, File: relPath, Line: lineNum,
				Snippet: strings.TrimSpace(line), RuleID: r.ID, Analyzer: AnalyzerStatic,
				Category: categoryForPattern(r.Pattern), Confidence: 0.95,
			})
		}
	}
	return staticFindings, dfFindings
}

// scanFileUnifiedPlain handles non-markdown files: static rules and tier
// detection run on every line in a single loop. Shell files also collect
// lines for dataflow analysis.
func scanFileUnifiedPlain(
	text, relPath string,
	activeRules []rule, profile *TierProfile,
	hasStatic, hasDataflow, isShell bool,
) (staticFindings, dfFindings []Finding) {
	// For shell dataflow, collect all lines to pass to analyzeShellBlock.
	var allLines []string
	collectLines := hasDataflow && isShell

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

		// Always collect for dataflow (including blank lines for correct offsets).
		if collectLines {
			allLines = append(allLines, line)
		}

		// Blank line: no regex will match, no commands to classify.
		if len(line) == 0 {
			continue
		}

		// Tier detection: all lines.
		classifyLineCommands(line, profile)

		// Static regex: all lines.
		if hasStatic {
			lineLower := ""
			lineLowerReady := false
			for _, r := range activeRules {
				if !rulePrefilterAllows(r, line, &lineLower, &lineLowerReady) {
					continue
				}
				if r.matchesLine(line) {
					staticFindings = append(staticFindings, Finding{
						Severity:   r.Severity,
						Pattern:    r.Pattern,
						Message:    r.Message,
						File:       relPath,
						Line:       lineNum,
						Snippet:    strings.TrimSpace(line),
						RuleID:     r.ID,
						Analyzer:   AnalyzerStatic,
						Category:   categoryForPattern(r.Pattern),
						Confidence: 0.95,
					})
				}
			}
		}
	}

	// Shell dataflow: analyze the full file as one block.
	if collectLines && len(allLines) > 0 {
		dfFindings = analyzeShellBlock(allLines, 0, relPath)
	}

	return staticFindings, dfFindings
}

// rulePrefilterAllows applies a conservative literal prefilter before regex.
func rulePrefilterAllows(r rule, line string, lineLower *string, lineLowerReady *bool) bool {
	if r.prefilter == "" {
		return true
	}
	if !r.prefilterFold {
		return strings.Contains(line, r.prefilter)
	}
	if !*lineLowerReady {
		*lineLower = strings.ToLower(line)
		*lineLowerReady = true
	}
	return strings.Contains(*lineLower, r.prefilter)
}

// ScanFileWithRules scans a single file using the given rules.
// If activeRules is nil, the default global rules are used.
// Synthetic tier findings are not filtered by audit-rules.yaml here; use
// ScanFile / ScanFileForProject to honour disabled tier rules.
func ScanFileWithRules(filePath string, activeRules []rule) (*Result, error) {
	return scanFileImpl(filePath, activeRules, nil)
}

func scanFileImpl(filePath string, activeRules []rule, disabled map[string]bool) (*Result, error) {
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
	var dfFindings []Finding
	if isMarkdown {
		contentRules, linkRules := splitMarkdownLinkRules(resolvedRules)
		result.Findings, dfFindings = scanFileUnified(data, filepath.Base(filePath), true,
			contentRules, &result.TierProfile, true, true, false)
		result.Findings = append(result.Findings, checkMarkdownLinkRules([]mdFileInfo{
			{relPath: filepath.Base(filePath), data: data, absDir: filepath.Dir(filePath)},
		}, nil, linkRules)...)
	} else {
		result.Findings = ScanContentWithRules(data, filepath.Base(filePath), resolvedRules)
		result.TierProfile = DetectCommandTiers(data)
		if isShellFile(info.Name()) {
			dfFindings = ScanShellDataflow(data, filepath.Base(filePath))
		}
	}

	result.Findings = append(result.Findings,
		DeduplicateDataflow(dfFindings, result.Findings)...)

	result.Findings = append(result.Findings,
		filterDisabledFindings(TierCombinationFindings(result.TierProfile), disabled)...)
	StampFingerprints(result.Findings)
	result.updateRisk()
	return result, nil
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
		".py", ".js", ".ts", ".rb", ".go", ".rs", ".swift":
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
