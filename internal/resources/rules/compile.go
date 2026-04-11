package rules

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/resources/adapters"
)

type CompiledFile = adapters.CompiledFile

// CompileTarget compiles managed rule records into target-native files.
func CompileTarget(records []Record, target, projectRoot string) ([]CompiledFile, []string, error) {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return nil, nil, fmt.Errorf("target is required")
	}

	var (
		converted []adapters.RuleRecord
		warnings  []string
	)

	for _, record := range records {
		adapterRecord, warn, err := normalizeRecord(record)
		if err != nil {
			return nil, nil, err
		}
		if warn != "" {
			warnings = append(warnings, warn)
			continue
		}
		if adapterRecord.Tool != target {
			continue
		}
		converted = append(converted, adapterRecord)
	}

	sort.Slice(converted, func(i, j int) bool {
		return converted[i].RelativePath < converted[j].RelativePath
	})

	var (
		files           []CompiledFile
		adapterWarnings []string
		err             error
	)

	switch target {
	case "claude":
		files, adapterWarnings, err = adapters.CompileClaudeRules(converted, projectRoot)
	case "codex":
		files, adapterWarnings, err = adapters.CompileCodexRules(converted, projectRoot)
	case "gemini":
		files, adapterWarnings, err = adapters.CompileGeminiRules(converted, projectRoot)
	default:
		return nil, nil, fmt.Errorf("%w %q", ErrUnsupportedTarget, target)
	}
	if err != nil {
		return nil, nil, err
	}

	warnings = append(warnings, adapterWarnings...)
	return files, warnings, nil
}

func normalizeRecord(record Record) (adapters.RuleRecord, string, error) {
	rel := strings.TrimSpace(record.RelativePath)
	if rel == "" {
		rel = strings.TrimSpace(record.ID)
	}
	rel = filepath.ToSlash(rel)
	if rel != "" {
		rel = path.Clean(rel)
	}
	if rel == "." {
		rel = ""
	}
	if strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
		return adapters.RuleRecord{}, "", fmt.Errorf("invalid managed rule path %q", rel)
	}

	tool := strings.ToLower(strings.TrimSpace(record.Tool))
	if tool == "" && rel != "" {
		parts := strings.SplitN(rel, "/", 2)
		if len(parts) > 1 {
			tool = strings.ToLower(parts[0])
		}
	}
	if tool == "" {
		return adapters.RuleRecord{}, fmt.Sprintf("skipping rule %q: missing tool", record.ID), nil
	}

	if rel == "" {
		name := strings.TrimSpace(record.Name)
		if name == "" {
			return adapters.RuleRecord{}, fmt.Sprintf("skipping rule %q: missing relative path", record.ID), nil
		}
		rel = tool + "/" + name
	}
	if !strings.HasPrefix(rel, tool+"/") {
		rel = tool + "/" + strings.TrimPrefix(rel, "/")
	}

	name := strings.TrimSpace(record.Name)
	if name == "" {
		name = path.Base(rel)
	}

	return adapters.RuleRecord{
		ID:           record.ID,
		Tool:         tool,
		RelativePath: rel,
		Name:         name,
		Content:      string(record.Content),
	}, "", nil
}
