package adapters

import (
	"path/filepath"
	"sort"
	"strings"

	managedpi "skillshare/internal/resources/managed/pi"
)

// CompilePiRules compiles managed pi rules into target-native files.
func CompilePiRules(records []RuleRecord, projectRoot string) ([]CompiledFile, []string, error) {
	sorted := append([]RuleRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		return normalizeRulePath(sorted[i]) < normalizeRulePath(sorted[j])
	})

	var files []CompiledFile
	var warnings []string
	for _, record := range sorted {
		if strings.TrimSpace(record.Tool) != "" && strings.TrimSpace(record.Tool) != "pi" {
			continue
		}

		id, ok := managedpi.NormalizeManagedRuleID(normalizeRulePath(record))
		if !ok {
			warnings = append(warnings, "unsupported pi rule id: "+normalizeRulePath(record))
			continue
		}

		outputPath, ok := managedpi.CompilePath(projectRoot, id)
		if !ok {
			warnings = append(warnings, "unsupported pi rule id: "+id)
			continue
		}
		files = append(files, CompiledFile{
			Path:    filepath.Clean(outputPath),
			Content: record.Content,
			Format:  "markdown",
		})
	}

	return files, warnings, nil
}
