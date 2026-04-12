package adapters

import (
	"path/filepath"
	"strconv"
	"strings"
)

// CompileGeminiHooks compiles managed gemini hooks into target-native files.
func CompileGeminiHooks(records []HookRecord, projectRoot, rawConfig string) ([]CompiledFile, []string, error) {
	filtered := make([]HookRecord, 0, len(records))
	warnings := make([]string, 0)
	for _, record := range records {
		if !isSupportedGeminiEvent(record.Event) {
			warnings = append(warnings, "skipping hook "+strings.TrimSpace(record.ID)+": unsupported gemini event "+strings.TrimSpace(record.Event))
			continue
		}
		filtered = append(filtered, record)
	}

	document, buildWarnings, err := buildHookDocument(filtered, func(handler HookHandler) (hookJSONAction, string, bool) {
		if handler.Type != "command" {
			return hookJSONAction{}, "skipping unsupported gemini hook type " + handler.Type, false
		}
		if strings.TrimSpace(handler.Command) == "" {
			return hookJSONAction{}, "", false
		}
		timeout, ok := geminiTimeoutJSONValue(handler)
		if !ok {
			return hookJSONAction{}, "skipping gemini hook with non-numeric timeout", false
		}
		return hookJSONAction{
			Type:          "command",
			Name:          strings.TrimSpace(handler.Name),
			Description:   strings.TrimSpace(handler.Description),
			Command:       strings.TrimSpace(handler.Command),
			Timeout:       timeout,
			StatusMessage: strings.TrimSpace(handler.StatusMessage),
		}, "", true
	})
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, buildWarnings...)
	if document == nil {
		document = map[string][]hookJSONEntry{}
	}

	mergedConfig, err := mergeJSONConfig(rawConfig, map[string]any{"hooks": document})
	if err != nil {
		return nil, nil, err
	}

	return []CompiledFile{{
		Path:    filepath.Join(projectRoot, ".gemini", "settings.json"),
		Content: mergedConfig,
		Format:  "json",
	}}, warnings, nil
}

func isSupportedGeminiEvent(event string) bool {
	switch strings.TrimSpace(event) {
	case "SessionStart", "SessionEnd", "BeforeAgent", "AfterAgent", "BeforeModel", "AfterModel", "BeforeToolSelection", "BeforeTool", "AfterTool", "PreCompress", "Notification":
		return true
	default:
		return false
	}
}

func geminiTimeoutJSONValue(handler HookHandler) (any, bool) {
	timeout := strings.TrimSpace(handler.Timeout)
	if timeout == "" {
		if handler.TimeoutSeconds == nil {
			return nil, true
		}
		return *handler.TimeoutSeconds, true
	}
	milliseconds, err := strconv.Atoi(timeout)
	if err != nil {
		return nil, false
	}
	return milliseconds, true
}
