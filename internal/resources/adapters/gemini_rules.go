package adapters

import (
	"path/filepath"
	"sort"
	"strings"
)

// CompileGeminiRules compiles managed gemini rules into target-native files.
func CompileGeminiRules(records []RuleRecord, projectRoot string) ([]CompiledFile, []string, error) {
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
		if strings.TrimSpace(record.Tool) != "" && strings.TrimSpace(record.Tool) != "gemini" {
			continue
		}
		rel := normalizeRulePath(record)
		toolRelative := trimToolPrefix(rel, "gemini")
		if isInstructionRule(toolRelative, record.Name, "GEMINI.md") {
			if instruction != nil {
				warnings = append(warnings, "multiple gemini instruction rules found; using the first one")
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
			Path:    filepath.Join(projectRoot, "GEMINI.md"),
			Content: instruction.Content,
			Format:  "markdown",
		})
	}

	for _, rule := range otherRules {
		rel := trimToolPrefix(normalizeRulePath(rule), "gemini")
		files = append(files, CompiledFile{
			Path:    filepath.Join(ruleOutputBaseDir(projectRoot, ".gemini"), filepath.FromSlash(rel)),
			Content: rule.Content,
			Format:  "markdown",
		})
	}

	return files, warnings, nil
}
