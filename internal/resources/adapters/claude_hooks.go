package adapters

import (
	"path/filepath"
)

// CompileClaudeHooks compiles managed claude hooks into target-native files.
func CompileClaudeHooks(records []HookRecord, projectRoot, rawConfig string) ([]CompiledFile, []string, error) {
	document, warnings, err := buildHookDocument(records, func(handler HookHandler) (hookJSONAction, string, bool) {
		actionType := handler.Type
		switch actionType {
		case "command":
			if handler.Command == "" {
				return hookJSONAction{}, "", false
			}
			return hookJSONAction{
				Type:          "command",
				Command:       handler.Command,
				Timeout:       handler.Timeout,
				StatusMessage: handler.StatusMessage,
			}, "", true
		case "http":
			if handler.URL == "" {
				return hookJSONAction{}, "", false
			}
			return hookJSONAction{
				Type:          "http",
				URL:           handler.URL,
				Timeout:       handler.Timeout,
				StatusMessage: handler.StatusMessage,
			}, "", true
		case "prompt", "agent":
			if handler.Prompt == "" {
				return hookJSONAction{}, "", false
			}
			return hookJSONAction{
				Type:          actionType,
				Prompt:        handler.Prompt,
				Timeout:       handler.Timeout,
				StatusMessage: handler.StatusMessage,
			}, "", true
		default:
			return hookJSONAction{}, "skipping unsupported claude hook type " + actionType, false
		}
	})
	if err != nil {
		return nil, nil, err
	}
	if document == nil {
		document = map[string][]hookJSONEntry{}
	}

	mergedConfig, err := mergeJSONConfig(rawConfig, map[string]any{"hooks": document})
	if err != nil {
		return nil, nil, err
	}

	return []CompiledFile{{
		Path:    filepath.Join(projectRoot, ".claude", "settings.json"),
		Content: mergedConfig,
		Format:  "json",
	}}, warnings, nil
}
