package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type hookJSONAction struct {
	Type          string `json:"type"`
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	Command       string `json:"command,omitempty"`
	URL           string `json:"url,omitempty"`
	Prompt        string `json:"prompt,omitempty"`
	Timeout       any    `json:"timeout,omitempty"`
	StatusMessage string `json:"statusMessage,omitempty"`
}

type hookJSONEntry struct {
	Matcher    string           `json:"matcher,omitempty"`
	Sequential *bool            `json:"sequential,omitempty"`
	Hooks      []hookJSONAction `json:"hooks"`
}

func normalizeHookRecord(record HookRecord) (HookRecord, string, error) {
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
	if rel == "" {
		return HookRecord{}, fmt.Sprintf("skipping hook %q: missing relative path", record.ID), nil
	}
	if strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
		return HookRecord{}, "", fmt.Errorf("invalid managed hook path %q", rel)
	}

	tool := strings.ToLower(strings.TrimSpace(record.Tool))
	if tool == "" {
		if parts := strings.SplitN(rel, "/", 2); len(parts) > 1 && strings.TrimSpace(parts[0]) != "" {
			tool = strings.ToLower(strings.TrimSpace(parts[0]))
		}
	}
	if tool == "" {
		return HookRecord{}, fmt.Sprintf("skipping hook %q: missing tool", record.ID), nil
	}

	if !strings.HasPrefix(rel, tool+"/") {
		rel = path.Join(tool, strings.TrimPrefix(rel, "/"))
	}

	event := strings.TrimSpace(record.Event)
	if event == "" {
		return HookRecord{}, fmt.Sprintf("skipping hook %q: missing event", record.ID), nil
	}
	matcher := strings.TrimSpace(record.Matcher)
	if tool == "codex" && (event == "UserPromptSubmit" || event == "Stop") {
		matcher = ""
	}
	if matcher == "" && !hookAllowsEmptyMatcher(tool, event) {
		return HookRecord{}, fmt.Sprintf("skipping hook %q: missing matcher", record.ID), nil
	}

	record.Tool = tool
	record.RelativePath = rel
	record.Event = event
	record.Matcher = matcher
	return record, "", nil
}

func buildHookDocument(records []HookRecord, allowHandler func(HookHandler) (hookJSONAction, string, bool)) (map[string][]hookJSONEntry, []string, error) {
	if len(records) == 0 {
		return nil, nil, nil
	}

	sorted := append([]HookRecord(nil), records...)
	sortHookRecords(sorted)

	document := make(map[string][]hookJSONEntry)
	var warnings []string

	for _, record := range sorted {
		normalized, warn, err := normalizeHookRecord(record)
		if err != nil {
			return nil, nil, err
		}
		if warn != "" {
			warnings = append(warnings, warn)
			continue
		}

		actions := make([]hookJSONAction, 0, len(normalized.Handlers))
		for _, handler := range normalized.Handlers {
			action, actionWarn, ok := allowHandler(handler)
			if actionWarn != "" {
				warnings = append(warnings, actionWarn)
			}
			if !ok {
				continue
			}
			actions = append(actions, action)
		}
		if len(actions) == 0 {
			warnings = append(warnings, fmt.Sprintf("skipping hook %q: no supported handlers", normalized.ID))
			continue
		}

		document[normalized.Event] = append(document[normalized.Event], hookJSONEntry{
			Matcher:    normalized.Matcher,
			Sequential: normalized.Sequential,
			Hooks:      actions,
		})
	}

	return document, warnings, nil
}

func sortHookRecords(records []HookRecord) {
	if len(records) < 2 {
		return
	}
	sort.Slice(records, func(i, j int) bool {
		return normalizedHookSortKey(records[i]) < normalizedHookSortKey(records[j])
	})
}

func normalizedHookSortKey(record HookRecord) string {
	rel := strings.TrimSpace(record.RelativePath)
	if rel == "" {
		rel = strings.TrimSpace(record.ID)
	}
	rel = filepath.ToSlash(rel)
	if rel == "" {
		rel = strings.TrimSpace(record.Tool) + "/" + strings.TrimSpace(record.Event) + "/" + strings.TrimSpace(record.Matcher)
	}
	return path.Clean(rel)
}

func encodeHookDocument(document map[string][]hookJSONEntry) ([]byte, error) {
	return json.Marshal(map[string]any{"hooks": document})
}

func hookAllowsEmptyMatcher(tool, event string) bool {
	normalizedTool := strings.ToLower(strings.TrimSpace(tool))
	normalizedEvent := strings.TrimSpace(event)
	if normalizedTool == "codex" && (normalizedEvent == "UserPromptSubmit" || normalizedEvent == "Stop") {
		return true
	}
	return normalizedTool == "gemini"
}

func mergeJSONConfig(raw string, updates map[string]any) (string, error) {
	doc := map[string]any{}
	if strings.TrimSpace(raw) != "" {
		decoded, err := stripJSONComments(raw)
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal(decoded, &doc); err != nil {
			return "", err
		}
	}
	for key, value := range updates {
		doc[key] = value
	}
	buf, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func stripJSONComments(raw string) ([]byte, error) {
	raw = strings.TrimPrefix(raw, "\uFEFF")
	if strings.TrimSpace(raw) == "" {
		return []byte(raw), nil
	}

	var out bytes.Buffer
	out.Grow(len(raw))

	inString := false
	escaped := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				out.WriteByte(ch)
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(raw) && raw[i+1] == '/' {
				inBlockComment = false
				i++
				continue
			}
			if ch == '\n' {
				out.WriteByte(ch)
			}
			continue
		}
		if inString {
			out.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}
		if ch == '/' && i+1 < len(raw) {
			switch raw[i+1] {
			case '/':
				inLineComment = true
				i++
				continue
			case '*':
				inBlockComment = true
				i++
				continue
			}
		}

		out.WriteByte(ch)
	}

	if inBlockComment {
		return nil, fmt.Errorf("unterminated JSON block comment")
	}

	return out.Bytes(), nil
}
