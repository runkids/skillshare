package inspect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const maxHookConfigSize = 512 * 1024

type hookLocation struct {
	sourceTool string
	scope      Scope
	path       string
}

func ScanHooks(projectRoot string) ([]HookItem, []string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve home directory: %w", err)
	}

	root := strings.TrimSpace(projectRoot)
	if root != "" {
		root, err = filepath.Abs(root)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve project root: %w", err)
		}
	}

	locations := []hookLocation{
		{sourceTool: "claude", scope: ScopeUser, path: filepath.Join(home, ".claude", "settings.json")},
		{sourceTool: "codex", scope: ScopeUser, path: filepath.Join(home, ".codex", "hooks.json")},
		{sourceTool: "gemini", scope: ScopeUser, path: filepath.Join(home, ".gemini", "settings.json")},
	}
	if root != "" {
		locations = append(locations,
			hookLocation{sourceTool: "claude", scope: ScopeProject, path: filepath.Join(root, ".claude", "settings.json")},
			hookLocation{sourceTool: "codex", scope: ScopeProject, path: filepath.Join(root, ".codex", "hooks.json")},
			hookLocation{sourceTool: "gemini", scope: ScopeProject, path: filepath.Join(root, ".gemini", "settings.json")},
			hookLocation{sourceTool: "claude", scope: ScopeProject, path: filepath.Join(root, ".claude", "settings.local.json")},
		)
	}

	locations = dedupeHookLocations(locations)

	var (
		items    []HookItem
		warnings []string
	)

	for _, loc := range locations {
		locItems, locWarnings := readHookItems(loc.path, loc.sourceTool, loc.scope)
		warnings = append(warnings, locWarnings...)
		items = append(items, locItems...)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		if items[i].Event != items[j].Event {
			return items[i].Event < items[j].Event
		}
		if items[i].Matcher != items[j].Matcher {
			return items[i].Matcher < items[j].Matcher
		}
		if items[i].EntryIndex != items[j].EntryIndex {
			return items[i].EntryIndex < items[j].EntryIndex
		}
		return items[i].ActionIndex < items[j].ActionIndex
	})

	return items, dedupeWarnings(warnings), nil
}

func dedupeHookLocations(locations []hookLocation) []hookLocation {
	deduped := make([]hookLocation, 0, len(locations))
	byPath := make(map[string]int, len(locations))

	for _, loc := range locations {
		path := resolvedComparablePath(loc.path)

		if idx, ok := byPath[path]; ok {
			existing := deduped[idx]
			if existing.scope == ScopeUser && loc.scope == ScopeProject {
				deduped[idx] = loc
			}
			continue
		}

		byPath[path] = len(deduped)
		deduped = append(deduped, loc)
	}

	return deduped
}

func sameResolvedPath(a, b string) bool {
	return resolvedComparablePath(a) == resolvedComparablePath(b)
}

func resolvedComparablePath(path string) string {
	if !filepath.IsAbs(path) {
		if absPath, err := filepath.Abs(path); err == nil {
			path = absPath
		}
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

func readHookItems(path, sourceTool string, scope Scope) ([]HookItem, []string) {
	data, warn, ok := readValidatedRegularFile(path, "hook config", maxHookConfigSize)
	if warn != "" {
		return nil, []string{warn}
	}
	if !ok {
		return nil, nil
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, []string{fmt.Sprintf("%s: invalid JSON: %v", path, err)}
	}

	rawHooks, ok := root["hooks"]
	if !ok {
		return nil, nil
	}
	if isJSONNull(rawHooks) {
		return nil, []string{fmt.Sprintf("%s: invalid hooks block: null", path)}
	}
	if len(rawHooks) == 0 {
		return nil, nil
	}

	var events map[string]json.RawMessage
	if err := json.Unmarshal(rawHooks, &events); err != nil {
		return nil, []string{fmt.Sprintf("%s: invalid hooks block: %v", path, err)}
	}

	var items []HookItem
	var warnings []string
	for _, event := range hookEventsForTool(sourceTool) {
		rawEvent, ok := events[event]
		if !ok {
			continue
		}
		if isJSONNull(rawEvent) {
			warnings = append(warnings, fmt.Sprintf("%s: invalid %s hook list: null", path, event))
			continue
		}
		var entries []json.RawMessage
		if err := json.Unmarshal(rawEvent, &entries); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: invalid %s hook list: %v", path, event, err))
			continue
		}
		for i, rawEntry := range entries {
			normalized, entryWarnings := normalizeHookEntry(path, sourceTool, scope, event, i, rawEntry)
			warnings = append(warnings, entryWarnings...)
			if len(normalized) == 0 {
				continue
			}
			items = append(items, normalized...)
		}
	}

	return items, warnings
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

func hookEventsForTool(sourceTool string) []string {
	switch sourceTool {
	case "claude":
		return []string{
			"SessionStart",
			"UserPromptSubmit",
			"PreToolUse",
			"PermissionRequest",
			"PermissionDenied",
			"PostToolUse",
			"PostToolUseFailure",
			"Notification",
			"SubagentStart",
			"SubagentStop",
			"TaskCreated",
			"TaskCompleted",
			"Stop",
			"StopFailure",
			"TeammateIdle",
			"InstructionsLoaded",
			"ConfigChange",
			"CwdChanged",
			"FileChanged",
			"WorktreeCreate",
			"WorktreeRemove",
			"PreCompact",
			"PostCompact",
			"Elicitation",
			"ElicitationResult",
			"SessionEnd",
		}
	case "codex":
		return []string{
			"SessionStart",
			"PreToolUse",
			"PostToolUse",
			"UserPromptSubmit",
			"Stop",
		}
	case "gemini":
		return []string{
			"BeforeTool",
			"AfterTool",
			"BeforeAgent",
			"AfterAgent",
			"BeforeModel",
			"BeforeToolSelection",
			"AfterModel",
			"SessionStart",
			"SessionEnd",
			"Notification",
			"PreCompress",
		}
	default:
		return nil
	}
}

func normalizeHookEntry(path, sourceTool string, scope Scope, event string, entryIndex int, rawEntry json.RawMessage) ([]HookItem, []string) {
	var entry struct {
		Matcher    string            `json:"matcher"`
		Sequential *bool             `json:"sequential"`
		Hooks      []json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(rawEntry, &entry); err != nil {
		return nil, []string{fmt.Sprintf("%s: invalid %s hook entry: %v", path, event, err)}
	}
	if len(entry.Hooks) == 0 {
		return nil, []string{fmt.Sprintf("%s: invalid %s hook entry: missing hooks array", path, event)}
	}
	matcher := strings.TrimSpace(entry.Matcher)
	if sourceTool == "codex" && (event == "UserPromptSubmit" || event == "Stop") {
		matcher = ""
	}
	groupID := stableDiscoveryID("hook_group", sourceTool, string(scope), resolvedComparablePath(path), event, matcher)
	collectible, collectReason := hookCollectibility(path, sourceTool)

	var items []HookItem
	var warnings []string
	for i, rawHook := range entry.Hooks {
		hook, warn, ok := normalizeHookAction(path, sourceTool, scope, event, matcher, copyOptionalBool(entry.Sequential), groupID, collectible, collectReason, entryIndex, i, rawHook)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		if !ok {
			continue
		}
		items = append(items, hook)
	}
	return items, warnings
}

func normalizeHookAction(path, sourceTool string, scope Scope, event, matcher string, sequential *bool, groupID string, collectible bool, collectReason string, entryIndex, actionIndex int, rawHook json.RawMessage) (HookItem, string, bool) {
	var action struct {
		Type          string          `json:"type"`
		Name          string          `json:"name"`
		Description   string          `json:"description"`
		Command       string          `json:"command"`
		URL           string          `json:"url"`
		Prompt        string          `json:"prompt"`
		Timeout       json.RawMessage `json:"timeout"`
		TimeoutSec    json.RawMessage `json:"timeoutSec"`
		StatusMessage string          `json:"statusMessage"`
	}
	if err := json.Unmarshal(rawHook, &action); err != nil {
		return HookItem{}, fmt.Sprintf("%s: invalid %s %s action: %v", path, event, matcher, err), false
	}
	actionType := strings.TrimSpace(action.Type)
	name := strings.TrimSpace(action.Name)
	description := strings.TrimSpace(action.Description)
	command := strings.TrimSpace(action.Command)
	url := strings.TrimSpace(action.URL)
	prompt := strings.TrimSpace(action.Prompt)
	timeout, timeoutSeconds, timeoutWarn := parseHookTimeout(sourceTool, action.Timeout, action.TimeoutSec)
	statusMessage := strings.TrimSpace(action.StatusMessage)
	if actionType == "" {
		return HookItem{}, fmt.Sprintf("%s: invalid %s %s action: missing type", path, event, matcher), false
	}
	if timeoutWarn != "" {
		return HookItem{}, timeoutWarn, false
	}
	if sourceTool == "codex" && (event == "UserPromptSubmit" || event == "Stop") {
		matcher = ""
	}
	switch sourceTool {
	case "claude":
		switch actionType {
		case "command":
			if command == "" {
				return HookItem{}, fmt.Sprintf("%s: invalid %s %s action: missing command", path, event, matcher), false
			}
			return HookItem{
				SourceTool:     sourceTool,
				Scope:          scope,
				Event:          event,
				Matcher:        matcher,
				GroupID:        groupID,
				Collectible:    collectible,
				CollectReason:  collectReason,
				Sequential:     copyOptionalBool(sequential),
				Name:           name,
				Description:    description,
				Command:        command,
				Timeout:        timeout,
				TimeoutSeconds: timeoutSeconds,
				StatusMessage:  statusMessage,
				EntryIndex:     entryIndex,
				ActionIndex:    actionIndex,
				ActionType:     actionType,
				Path:           path,
			}, "", true
		case "http":
			if url == "" {
				return HookItem{}, fmt.Sprintf("%s: invalid %s %s action: missing url", path, event, matcher), false
			}
			return HookItem{
				SourceTool:     sourceTool,
				Scope:          scope,
				Event:          event,
				Matcher:        matcher,
				GroupID:        groupID,
				Collectible:    collectible,
				CollectReason:  collectReason,
				Sequential:     copyOptionalBool(sequential),
				Name:           name,
				Description:    description,
				URL:            url,
				Timeout:        timeout,
				TimeoutSeconds: timeoutSeconds,
				StatusMessage:  statusMessage,
				EntryIndex:     entryIndex,
				ActionIndex:    actionIndex,
				ActionType:     actionType,
				Path:           path,
			}, "", true
		case "prompt", "agent":
			if prompt == "" {
				return HookItem{}, fmt.Sprintf("%s: invalid %s %s action: missing prompt", path, event, matcher), false
			}
			return HookItem{
				SourceTool:     sourceTool,
				Scope:          scope,
				Event:          event,
				Matcher:        matcher,
				GroupID:        groupID,
				Collectible:    collectible,
				CollectReason:  collectReason,
				Sequential:     copyOptionalBool(sequential),
				Name:           name,
				Description:    description,
				Prompt:         prompt,
				Timeout:        timeout,
				TimeoutSeconds: timeoutSeconds,
				StatusMessage:  statusMessage,
				EntryIndex:     entryIndex,
				ActionIndex:    actionIndex,
				ActionType:     actionType,
				Path:           path,
			}, "", true
		}
	case "codex":
		if actionType == "command" {
			if command == "" {
				return HookItem{}, fmt.Sprintf("%s: invalid %s %s action: missing command", path, event, matcher), false
			}
			return HookItem{
				SourceTool:     sourceTool,
				Scope:          scope,
				Event:          event,
				Matcher:        matcher,
				GroupID:        groupID,
				Collectible:    collectible,
				CollectReason:  collectReason,
				Sequential:     copyOptionalBool(sequential),
				Name:           name,
				Description:    description,
				Command:        command,
				Timeout:        timeout,
				TimeoutSeconds: timeoutSeconds,
				StatusMessage:  statusMessage,
				EntryIndex:     entryIndex,
				ActionIndex:    actionIndex,
				ActionType:     actionType,
				Path:           path,
			}, "", true
		}
	case "gemini":
		if actionType == "command" {
			if command == "" {
				return HookItem{}, fmt.Sprintf("%s: invalid %s %s action: missing command", path, event, matcher), false
			}
			return HookItem{
				SourceTool:     sourceTool,
				Scope:          scope,
				Event:          event,
				Matcher:        matcher,
				GroupID:        groupID,
				Collectible:    collectible,
				CollectReason:  collectReason,
				Sequential:     copyOptionalBool(sequential),
				Name:           name,
				Description:    description,
				Command:        command,
				Timeout:        timeout,
				TimeoutSeconds: timeoutSeconds,
				StatusMessage:  statusMessage,
				EntryIndex:     entryIndex,
				ActionIndex:    actionIndex,
				ActionType:     actionType,
				Path:           path,
			}, "", true
		}
	}
	if isKnownHookType(sourceTool, actionType) {
		return HookItem{}, fmt.Sprintf("%s: unsupported %s %s hook type %q; only command actions are normalized", path, event, matcher, actionType), false
	}
	return HookItem{}, fmt.Sprintf("%s: invalid %s %s action: unknown type %q", path, event, matcher, actionType), false
}

func parseHookTimeout(sourceTool string, timeoutRaw, timeoutSecRaw json.RawMessage) (string, *int, string) {
	if sourceTool == "codex" {
		if timeoutText, timeoutSeconds, ok := decodeNumericHookTimeoutValue(timeoutSecRaw); ok {
			return timeoutText, timeoutSeconds, ""
		}
		if timeoutText, timeoutSeconds, ok := decodeNumericHookTimeoutValue(timeoutRaw); ok {
			return timeoutText, timeoutSeconds, ""
		}
		if len(timeoutRaw) != 0 || len(timeoutSecRaw) != 0 {
			return "", nil, "invalid codex timeout: expected numeric seconds"
		}
		return "", nil, ""
	}

	if timeoutText, timeoutSeconds, ok := decodeHookTimeoutValue(timeoutRaw); ok {
		return timeoutText, timeoutSeconds, ""
	}
	if timeoutText, timeoutSeconds, ok := decodeHookTimeoutValue(timeoutSecRaw); ok {
		return timeoutText, timeoutSeconds, ""
	}
	return "", nil, ""
}

func decodeNumericHookTimeoutValue(raw json.RawMessage) (string, *int, bool) {
	timeoutText, timeoutSeconds, ok := decodeHookTimeoutValue(raw)
	if !ok || timeoutSeconds == nil {
		return "", nil, false
	}
	return timeoutText, timeoutSeconds, true
}

func decodeHookTimeoutValue(raw json.RawMessage) (string, *int, bool) {
	if len(raw) == 0 || isJSONNull(raw) {
		return "", nil, false
	}

	var numeric int
	if err := json.Unmarshal(raw, &numeric); err == nil {
		text := strconv.Itoa(numeric)
		return text, &numeric, true
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return "", nil, false
		}
		if numeric, err := strconv.Atoi(text); err == nil {
			value := numeric
			return text, &value, true
		}
		return text, nil, true
	}

	return "", nil, false
}

func isKnownHookType(sourceTool, actionType string) bool {
	switch sourceTool {
	case "claude":
		switch actionType {
		case "command", "http", "prompt", "agent":
			return true
		default:
			return false
		}
	case "codex":
		switch actionType {
		case "command":
			return true
		default:
			return false
		}
	case "gemini":
		return actionType == "command"
	default:
		return actionType == "command"
	}
}

func hookCollectibility(path, sourceTool string) (bool, string) {
	if sourceTool == "claude" && filepath.Base(path) == "settings.local.json" && filepath.Base(filepath.Dir(path)) == ".claude" {
		return false, "diagnostics-only: .claude/settings.local.json is not collectible"
	}
	switch strings.ToLower(strings.TrimSpace(sourceTool)) {
	case "claude", "codex", "gemini":
		return true, ""
	}
	return true, ""
}

func copyOptionalBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
