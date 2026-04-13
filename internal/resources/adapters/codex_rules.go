package adapters

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// CompileCodexRules compiles managed codex rules into one AGENTS.md file.
func CompileCodexRules(records []RuleRecord, projectRoot string) ([]CompiledFile, []string, error) {
	sorted := append([]RuleRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		return normalizeRulePath(sorted[i]) < normalizeRulePath(sorted[j])
	})

	segments := make([]string, 0, len(sorted))
	for _, record := range sorted {
		if strings.TrimSpace(record.Tool) != "" && strings.TrimSpace(record.Tool) != "codex" {
			continue
		}

		rel := normalizeRulePath(record)
		if rel == "" {
			continue
		}
		if !strings.HasPrefix(rel, "codex/") {
			rel = "codex/" + strings.TrimPrefix(rel, "/")
		}

		segments = append(segments,
			fmt.Sprintf("<!-- skillshare:%s -->\n%s", rel, strings.TrimSpace(record.Content)),
		)
	}

	if len(segments) == 0 {
		return nil, nil, nil
	}

	return []CompiledFile{{
		Path:    filepath.Join(projectRoot, "AGENTS.md"),
		Content: strings.Join(segments, "\n\n"),
		Format:  "markdown",
	}}, nil, nil
}
