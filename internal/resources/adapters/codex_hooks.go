package adapters

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

var (
	codexFeaturesHeaderRE  = regexp.MustCompile(`^\s*\[features\]\s*(?:#.*)?$`)
	codexAnyTableHeaderRE  = regexp.MustCompile(`^\s*\[\[?[^\[\]]+\]\]?\s*(?:#.*)?$`)
	codexHooksAssignmentRE = regexp.MustCompile(`^(\s*codex_hooks\s*=\s*)([^#\r\n]*?)(\s*(#.*)?)$`)
)

// CompileCodexHooks compiles managed codex hooks into target-native files.
func CompileCodexHooks(records []HookRecord, projectRoot, rawConfig string) ([]CompiledFile, []string, error) {
	filtered := make([]HookRecord, 0, len(records))
	warnings := make([]string, 0)
	for _, record := range records {
		if !isSupportedCodexEvent(record.Event) {
			warnings = append(warnings, "skipping hook "+strings.TrimSpace(record.ID)+": unsupported codex event "+strings.TrimSpace(record.Event))
			continue
		}
		filtered = append(filtered, record)
	}

	document, buildWarnings, err := buildHookDocument(filtered, func(handler HookHandler) (hookJSONAction, string, bool) {
		if handler.Type != "command" {
			return hookJSONAction{}, "skipping unsupported codex hook type " + handler.Type, false
		}
		if handler.Command == "" {
			return hookJSONAction{}, "", false
		}
		timeout, ok := codexTimeoutJSONValue(handler)
		if !ok {
			return hookJSONAction{}, "skipping codex hook with non-numeric timeout", false
		}
		return hookJSONAction{
			Type:          "command",
			Command:       handler.Command,
			Timeout:       timeout,
			StatusMessage: handler.StatusMessage,
		}, "", true
	})
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, buildWarnings...)
	if document == nil {
		document = map[string][]hookJSONEntry{}
	}

	config, err := mergeCodexConfig(rawConfig, len(document) > 0)
	if err != nil {
		return nil, nil, err
	}

	hooksJSON, err := json.Marshal(map[string]any{"hooks": document})
	if err != nil {
		return nil, nil, err
	}

	return []CompiledFile{
		{
			Path:    filepath.Join(projectRoot, ".codex", "config.toml"),
			Content: config,
			Format:  "toml",
		},
		{
			Path:    filepath.Join(projectRoot, ".codex", "hooks.json"),
			Content: string(hooksJSON),
			Format:  "json",
		},
	}, warnings, nil
}

func isSupportedCodexEvent(event string) bool {
	switch strings.TrimSpace(event) {
	case "SessionStart", "PreToolUse", "PostToolUse", "UserPromptSubmit", "Stop":
		return true
	default:
		return false
	}
}

func mergeCodexConfig(raw string, enabled bool) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return minimalCodexConfig(enabled), nil
	}
	if err := validateCodexConfigFeatures(raw); err != nil {
		return "", err
	}
	if merged, ok := patchExplicitCodexFeaturesTable(raw, enabled); ok {
		return merged, nil
	}
	if merged, ok := patchInlineCodexFeaturesTable(raw, enabled); ok {
		return merged, nil
	}
	return raw + newlineFor(raw) + minimalCodexConfig(enabled), nil
}

func codexTimeoutJSONValue(handler HookHandler) (any, bool) {
	if handler.TimeoutSeconds != nil {
		return *handler.TimeoutSeconds, true
	}

	timeout := strings.TrimSpace(handler.Timeout)
	if timeout == "" {
		return nil, true
	}
	seconds, err := strconv.Atoi(timeout)
	if err != nil {
		return nil, false
	}
	return seconds, true
}

func validateCodexConfigFeatures(raw string) error {
	doc := map[string]any{}
	if err := toml.Unmarshal([]byte(raw), &doc); err != nil {
		return err
	}
	featuresValue, exists := doc["features"]
	if !exists {
		return nil
	}
	if _, ok := featuresValue.(map[string]any); !ok {
		return fmt.Errorf("codex config features must be a table")
	}
	return nil
}

func patchExplicitCodexFeaturesTable(raw string, enabled bool) (string, bool) {
	spans := codexLineSpans(raw)
	for i, span := range spans {
		body := trimCodexLineEnding(raw[span.start:span.end])
		if !codexFeaturesHeaderRE.MatchString(body) {
			continue
		}
		blockEnd := len(raw)
		for j := i + 1; j < len(spans); j++ {
			nextBody := trimCodexLineEnding(raw[spans[j].start:spans[j].end])
			if codexAnyTableHeaderRE.MatchString(nextBody) {
				blockEnd = spans[j].start
				break
			}
		}
		block := raw[span.start:blockEnd]
		if updated, ok := patchCodexHooksLineInBlock(block, enabled); ok {
			return raw[:span.start] + updated + raw[blockEnd:], true
		}
		insertAt := span.start + lastCodexNonBlankLineEnd(block)
		insert := minimalCodexAssignment(enabled) + newlineFor(raw)
		return raw[:insertAt] + insert + raw[insertAt:], true
	}
	return "", false
}

func patchInlineCodexFeaturesTable(raw string, enabled bool) (string, bool) {
	spans := codexLineSpans(raw)
	for _, span := range spans {
		body := trimCodexLineEnding(raw[span.start:span.end])
		if codexAnyTableHeaderRE.MatchString(body) {
			return "", false
		}
		updated, ok := patchInlineCodexFeaturesLine(body, enabled)
		if !ok {
			continue
		}
		return raw[:span.start] + updated + raw[span.end:], true
	}
	return "", false
}

func patchInlineCodexFeaturesLine(line string, enabled bool) (string, bool) {
	openBrace, closeBrace, ok := locateInlineCodexFeaturesTable(line)
	if !ok {
		return "", false
	}
	inside := line[openBrace+1 : closeBrace]
	patchedInside, ok := patchInlineCodexFeaturesBody(inside, enabled)
	if !ok {
		return "", false
	}
	return line[:openBrace+1] + patchedInside + line[closeBrace:], true
}

func locateInlineCodexFeaturesTable(line string) (int, int, bool) {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if !strings.HasPrefix(line[i:], "features") {
		return 0, 0, false
	}
	i += len("features")
	if i < len(line) && isInlineCodexIdentifierChar(line[i]) {
		return 0, 0, false
	}
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) || line[i] != '=' {
		return 0, 0, false
	}
	i++
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) || line[i] != '{' {
		return 0, 0, false
	}

	openBrace := i
	closeBrace, ok := findInlineCodexTableClose(line, openBrace)
	if !ok {
		return 0, 0, false
	}
	if !inlineCodexFeaturesSuffixAllowed(line[closeBrace+1:]) {
		return 0, 0, false
	}
	return openBrace, closeBrace, true
}

func findInlineCodexTableClose(line string, openBrace int) (int, bool) {
	quote := byte(0)
	escape := false
	braceDepth := 0
	bracketDepth := 0

	for i := openBrace; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			if quote == '"' {
				if escape {
					escape = false
					continue
				}
				if c == '\\' {
					escape = true
					continue
				}
			}
			if c == quote {
				quote = 0
			}
			continue
		}

		switch c {
		case '"', '\'':
			quote = c
		case '{':
			braceDepth++
		case '}':
			braceDepth--
			if braceDepth == 0 {
				return i, true
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		}
	}

	return 0, false
}

func inlineCodexFeaturesSuffixAllowed(suffix string) bool {
	trimmed := strings.TrimSpace(suffix)
	return trimmed == "" || strings.HasPrefix(trimmed, "#")
}

func isInlineCodexIdentifierChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-'
}

func patchInlineCodexFeaturesBody(body string, enabled bool) (string, bool) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return " codex_hooks = " + strconv.FormatBool(enabled) + " ", true
	}
	if valueStart, valueEnd, ok := findInlineCodexHooksValueSpan(body); ok {
		return body[:valueStart] + strconv.FormatBool(enabled) + body[valueEnd:], true
	}
	if strings.HasSuffix(trimmed, ",") {
		return body + " codex_hooks = " + strconv.FormatBool(enabled) + " ", true
	}
	return body + ", codex_hooks = " + strconv.FormatBool(enabled), true
}

func findInlineCodexHooksValueSpan(body string) (int, int, bool) {
	const key = "codex_hooks"
	quote := byte(0)
	escape := false
	braceDepth := 0
	bracketDepth := 0
	for i := 0; i < len(body); i++ {
		c := body[i]
		if quote != 0 {
			if quote == '"' {
				if escape {
					escape = false
					continue
				}
				if c == '\\' {
					escape = true
					continue
				}
			}
			if c == quote {
				quote = 0
			}
			continue
		}

		switch c {
		case '"', '\'':
			quote = c
			continue
		case '{':
			braceDepth++
			continue
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
			continue
		case '[':
			bracketDepth++
			continue
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
			continue
		}

		if braceDepth != 0 || bracketDepth != 0 || c != key[0] {
			continue
		}
		if !hasInlineCodexHooksKeyPrefix(body, i) {
			continue
		}

		j := i + len(key)
		for j < len(body) && isInlineSpace(body[j]) {
			j++
		}
		if j >= len(body) || body[j] != '=' {
			continue
		}
		j++
		for j < len(body) && isInlineSpace(body[j]) {
			j++
		}
		return j, findInlineCodexHooksValueEnd(body, j), true
	}
	return 0, 0, false
}

func hasInlineCodexHooksKeyPrefix(body string, idx int) bool {
	const key = "codex_hooks"
	if !strings.HasPrefix(body[idx:], key) {
		return false
	}
	if idx > 0 {
		prev := body[idx-1]
		if isInlineIdentChar(prev) || prev == '.' {
			return false
		}
	}
	if next := idx + len(key); next < len(body) {
		if isInlineIdentChar(body[next]) {
			return false
		}
	}
	return true
}

func findInlineCodexHooksValueEnd(body string, start int) int {
	quote := byte(0)
	escape := false
	braceDepth := 0
	bracketDepth := 0
	for i := start; i < len(body); i++ {
		c := body[i]
		if quote != 0 {
			if quote == '"' {
				if escape {
					escape = false
					continue
				}
				if c == '\\' {
					escape = true
					continue
				}
			}
			if c == quote {
				quote = 0
			}
			continue
		}

		switch c {
		case '"', '\'':
			quote = c
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case ',':
			if braceDepth == 0 && bracketDepth == 0 {
				return i
			}
		}
	}
	return len(body)
}

func isInlineSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

func isInlineIdentChar(b byte) bool {
	return b == '_' || b == '-' || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func patchCodexHooksLineInBlock(block string, enabled bool) (string, bool) {
	spans := codexLineSpans(block)
	for _, span := range spans {
		line := block[span.start:span.end]
		body := trimCodexLineEnding(line)
		updatedBody, ok := patchCodexHooksLine(body, enabled)
		if !ok {
			continue
		}
		return block[:span.start] + updatedBody + line[len(body):] + block[span.end:], true
	}
	return "", false
}

func patchCodexHooksLine(body string, enabled bool) (string, bool) {
	match := codexHooksAssignmentRE.FindStringSubmatchIndex(body)
	if match == nil {
		return "", false
	}
	return body[:match[2]] + strconv.FormatBool(enabled) + body[match[3]:], true
}

func lastCodexNonBlankLineEnd(block string) int {
	spans := codexLineSpans(block)
	for i := len(spans) - 1; i >= 0; i-- {
		line := trimCodexLineEnding(block[spans[i].start:spans[i].end])
		if strings.TrimSpace(line) != "" {
			return spans[i].end
		}
	}
	return len(block)
}

func codexLineSpans(raw string) []struct{ start, end int } {
	if raw == "" {
		return nil
	}
	spans := make([]struct{ start, end int }, 0, strings.Count(raw, "\n")+1)
	start := 0
	for start < len(raw) {
		next := strings.IndexByte(raw[start:], '\n')
		if next < 0 {
			spans = append(spans, struct{ start, end int }{start: start, end: len(raw)})
			break
		}
		end := start + next + 1
		spans = append(spans, struct{ start, end int }{start: start, end: end})
		start = end
	}
	return spans
}

func trimCodexLineEnding(line string) string {
	return strings.TrimRight(line, "\r\n")
}

func minimalCodexConfig(enabled bool) string {
	return "[features]\n" + minimalCodexAssignment(enabled) + "\n"
}

func minimalCodexAssignment(enabled bool) string {
	return "codex_hooks = " + strconv.FormatBool(enabled)
}

func newlineFor(raw string) string {
	if strings.Contains(raw, "\r\n") {
		return "\r\n"
	}
	return "\n"
}
