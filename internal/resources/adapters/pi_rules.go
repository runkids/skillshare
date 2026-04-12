package adapters

import (
	"path/filepath"
	"sort"
	"strings"
)

// CompilePiRules compiles managed pi rules into target-native files.
func CompilePiRules(records []RuleRecord, projectRoot string) ([]CompiledFile, []string, error) {
	sorted := append([]RuleRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		return normalizeRulePath(sorted[i]) < normalizeRulePath(sorted[j])
	})

	var files []CompiledFile
	for _, record := range sorted {
		if strings.TrimSpace(record.Tool) != "" && strings.TrimSpace(record.Tool) != "pi" {
			continue
		}

		rel := trimToolPrefix(normalizeRulePath(record), "pi")
		switch rel {
		case "AGENTS.md":
			files = append(files, CompiledFile{
				Path:    filepath.Join(projectRoot, "AGENTS.md"),
				Content: record.Content,
				Format:  "markdown",
			})
		case "SYSTEM.md":
			files = append(files, CompiledFile{
				Path:    filepath.Join(piOutputBaseDir(projectRoot), "SYSTEM.md"),
				Content: record.Content,
				Format:  "markdown",
			})
		case "APPEND_SYSTEM.md":
			files = append(files, CompiledFile{
				Path:    filepath.Join(piOutputBaseDir(projectRoot), "APPEND_SYSTEM.md"),
				Content: record.Content,
				Format:  "markdown",
			})
		}
	}

	return files, nil, nil
}

func piOutputBaseDir(root string) string {
	cleaned := filepath.Clean(strings.TrimSpace(root))
	if cleaned == "" || cleaned == "." {
		return cleaned
	}

	base := strings.ToLower(filepath.Base(cleaned))
	parent := strings.ToLower(filepath.Base(filepath.Dir(cleaned)))
	if base == ".pi" || (base == "agent" && parent == ".pi") {
		return cleaned
	}
	return filepath.Join(cleaned, ".pi")
}
