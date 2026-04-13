package adapters

import (
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// CompileClaudeRules compiles managed claude rules into target-native files.
func CompileClaudeRules(records []RuleRecord, projectRoot string) ([]CompiledFile, []string, error) {
	sorted := append([]RuleRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		return normalizeRulePath(sorted[i]) < normalizeRulePath(sorted[j])
	})

	var (
		files        []CompiledFile
		warnings     []string
		instruction  *RuleRecord
		otherRules   []RuleRecord
	)

	for _, record := range sorted {
		if strings.TrimSpace(record.Tool) != "" && strings.TrimSpace(record.Tool) != "claude" {
			continue
		}
		rel := normalizeRulePath(record)
		toolRelative := trimToolPrefix(rel, "claude")
		if isInstructionRule(toolRelative, record.Name, "CLAUDE.md") {
			if instruction != nil {
				warnings = append(warnings, "multiple claude instruction rules found; using the first one")
				continue
			}
			copy := record
			instruction = &copy
			continue
		}
		otherRules = append(otherRules, record)
	}

	if instruction != nil {
		files = append(files, CompiledFile{
			Path:    filepath.Join(projectRoot, "CLAUDE.md"),
			Content: instruction.Content,
			Format:  "markdown",
		})
	}

	for _, rule := range otherRules {
		rel := trimToolPrefix(normalizeRulePath(rule), "claude")
		files = append(files, CompiledFile{
			Path:    filepath.Join(ruleOutputBaseDir(projectRoot, ".claude"), filepath.FromSlash(rel)),
			Content: rule.Content,
			Format:  "markdown",
		})
	}

	return files, warnings, nil
}

func normalizeRulePath(record RuleRecord) string {
	rel := filepath.ToSlash(strings.TrimSpace(record.RelativePath))
	if rel == "" {
		rel = filepath.ToSlash(strings.TrimSpace(record.ID))
	}
	if rel == "" {
		rel = strings.TrimSpace(record.Name)
	}
	if rel == "" {
		return ""
	}
	rel = path.Clean(rel)
	if rel == "." {
		return ""
	}
	return rel
}

func trimToolPrefix(rel, tool string) string {
	if rel == "" {
		return ""
	}
	prefix := tool + "/"
	if strings.HasPrefix(rel, prefix) {
		rel = strings.TrimPrefix(rel, prefix)
	}
	return strings.TrimPrefix(rel, "/")
}

func isInstructionRule(rel, name, instructionName string) bool {
	trimmed := strings.Trim(strings.TrimSpace(rel), "/")
	if trimmed != "" {
		if strings.Contains(trimmed, "/") {
			return false
		}
		return strings.EqualFold(trimmed, instructionName)
	}
	if strings.Contains(strings.TrimSpace(name), "/") {
		return false
	}
	if strings.EqualFold(path.Base(strings.TrimSpace(name)), instructionName) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(name), instructionName)
}

func ruleOutputBaseDir(root, configDirName string) string {
	cleaned := filepath.Clean(strings.TrimSpace(root))
	if strings.EqualFold(filepath.Base(cleaned), configDirName) {
		return filepath.Join(cleaned, "rules")
	}
	return filepath.Join(cleaned, configDirName, "rules")
}
