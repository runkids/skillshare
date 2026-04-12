package hooks

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/resources/adapters"
)

type CompiledFile = adapters.CompiledFile

var ErrUnsupportedTarget = errors.New("unsupported target")

// CompileTarget compiles managed hook records into target-native files.
func CompileTarget(records []Record, targetFamily, targetName, projectRoot, rawConfig string) ([]CompiledFile, []string, error) {
	targetFamily = strings.ToLower(strings.TrimSpace(targetFamily))
	targetName = strings.TrimSpace(targetName)
	if targetFamily == "" {
		return nil, nil, fmt.Errorf("target is required")
	}
	if targetName == "" {
		targetName = targetFamily
	}

	var (
		converted []adapters.HookRecord
		warnings  []string
	)

	for _, record := range records {
		if record.Disabled || !matchesAssignedTarget(record.Targets, targetName) {
			continue
		}
		adapterRecord, warn, err := normalizeRecord(record)
		if err != nil {
			return nil, nil, err
		}
		if warn != "" {
			warnings = append(warnings, warn)
			continue
		}
		if adapterRecord.Tool != targetFamily {
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

	switch targetFamily {
	case "claude":
		files, adapterWarnings, err = adapters.CompileClaudeHooks(converted, projectRoot, rawConfig)
	case "codex":
		files, adapterWarnings, err = adapters.CompileCodexHooks(converted, projectRoot, rawConfig)
	default:
		return nil, nil, fmt.Errorf("%w %q", ErrUnsupportedTarget, targetFamily)
	}
	if len(converted) == 0 && strings.TrimSpace(rawConfig) == "" {
		return nil, warnings, nil
	}
	if err != nil {
		return nil, nil, err
	}

	warnings = append(warnings, adapterWarnings...)
	return files, warnings, nil
}

func matchesAssignedTarget(targets []string, targetName string) bool {
	normalized := normalizeAssignedTargets(targets)
	if len(normalized) == 0 {
		return true
	}
	for _, target := range normalized {
		if target == targetName {
			return true
		}
	}
	return false
}

func normalizeAssignedTargets(targets []string) []string {
	if len(targets) == 0 {
		return nil
	}
	out := make([]string, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		out = append(out, target)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeRecord(record Record) (adapters.HookRecord, string, error) {
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
		return adapters.HookRecord{}, "", fmt.Errorf("invalid managed hook path %q", rel)
	}

	tool := strings.ToLower(strings.TrimSpace(record.Tool))
	if tool == "" && rel != "" {
		if parts := strings.SplitN(rel, "/", 2); len(parts) > 1 && strings.TrimSpace(parts[0]) != "" {
			tool = strings.ToLower(strings.TrimSpace(parts[0]))
		}
	}
	if tool == "" {
		return adapters.HookRecord{}, fmt.Sprintf("skipping hook %q: missing tool", record.ID), nil
	}
	if rel == "" {
		return adapters.HookRecord{}, fmt.Sprintf("skipping hook %q: missing relative path", record.ID), nil
	}
	if !strings.HasPrefix(rel, tool+"/") {
		rel = path.Join(tool, strings.TrimPrefix(rel, "/"))
	}

	event := strings.TrimSpace(record.Event)
	if event == "" {
		return adapters.HookRecord{}, fmt.Sprintf("skipping hook %q: missing event", record.ID), nil
	}
	matcher := strings.TrimSpace(record.Matcher)
	if tool == "codex" && (event == "UserPromptSubmit" || event == "Stop") {
		matcher = ""
	}
	if matcher == "" && tool != "codex" {
		return adapters.HookRecord{}, fmt.Sprintf("skipping hook %q: missing matcher", record.ID), nil
	}
	if len(record.Handlers) == 0 {
		return adapters.HookRecord{}, fmt.Sprintf("skipping hook %q: missing handlers", record.ID), nil
	}

	handlers := make([]adapters.HookHandler, len(record.Handlers))
	for i, handler := range record.Handlers {
		handlers[i] = adapters.HookHandler{
			Type:           strings.TrimSpace(handler.Type),
			Command:        strings.TrimSpace(handler.Command),
			URL:            strings.TrimSpace(handler.URL),
			Prompt:         strings.TrimSpace(handler.Prompt),
			Timeout:        strings.TrimSpace(handler.Timeout),
			TimeoutSeconds: handler.TimeoutSeconds,
			StatusMessage:  strings.TrimSpace(handler.StatusMessage),
		}
	}

	return adapters.HookRecord{
		ID:           strings.TrimSpace(record.ID),
		Tool:         tool,
		RelativePath: rel,
		Event:        event,
		Matcher:      matcher,
		Handlers:     handlers,
	}, "", nil
}
