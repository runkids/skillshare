package adapters

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestMergeCodexConfig_PreservesExistingContent(t *testing.T) {
	raw := "[profiles.default]\nmodel = \"gpt-5\"\n\n[ui]\ncompact = true\n"

	merged, err := mergeCodexConfig(raw, true)
	if err != nil {
		t.Fatalf("mergeCodexConfig() error = %v", err)
	}
	if !strings.Contains(merged, "codex_hooks = true") {
		t.Fatalf("merged config missing feature flag: %q", merged)
	}
	if !strings.Contains(merged, "model =") || !strings.Contains(merged, "gpt-5") {
		t.Fatalf("merged config missing original profile content: %q", merged)
	}
	if !strings.Contains(merged, "compact = true") {
		t.Fatalf("merged config missing unrelated content: %q", merged)
	}
}

func TestMergeCodexConfig_PreservesCommentsAndFormatting(t *testing.T) {
	raw := "# top comment\n[profiles.default]\n# keep profile comment\nmodel = \"gpt-5\" # inline comment\n\n[ui]\ncompact = true\n\n# features comment\n[features]\n# keep features comment\nalpha = true\n"

	merged, err := mergeCodexConfig(raw, true)
	if err != nil {
		t.Fatalf("mergeCodexConfig() error = %v", err)
	}
	for _, want := range []string{
		"# top comment",
		"# keep profile comment",
		"# inline comment",
		"# features comment",
		"# keep features comment",
		"[profiles.default]",
		"[ui]",
		"[features]",
		"codex_hooks = true",
		"alpha = true",
	} {
		if !strings.Contains(merged, want) {
			t.Fatalf("merged config missing %q: %q", want, merged)
		}
	}

	profilesIdx := strings.Index(merged, "[profiles.default]")
	uiIdx := strings.Index(merged, "[ui]")
	featuresIdx := strings.Index(merged, "[features]")
	if profilesIdx < 0 || uiIdx < 0 || featuresIdx < 0 {
		t.Fatalf("merged config missing expected table order: %q", merged)
	}
	if !(profilesIdx < uiIdx && uiIdx < featuresIdx) {
		t.Fatalf("expected unrelated table order to be preserved: %q", merged)
	}
}

func TestMergeCodexConfig_UpdatesOnlyActualInlineCodexHooksKey(t *testing.T) {
	raw := "features = { codex_hooks_enabled = true, note = \"codex_hooks\", codex_hooks = false }\n[profiles.default]\nmodel = \"gpt-5\"\n"

	merged, err := mergeCodexConfig(raw, true)
	if err != nil {
		t.Fatalf("mergeCodexConfig() error = %v", err)
	}
	if !strings.Contains(merged, "codex_hooks_enabled = true") {
		t.Fatalf("merged config corrupted near-match key: %q", merged)
	}
	if !strings.Contains(merged, `note = "codex_hooks"`) {
		t.Fatalf("merged config corrupted quoted string value: %q", merged)
	}
	if !strings.Contains(merged, "codex_hooks = true") {
		t.Fatalf("merged config missing actual codex_hooks update: %q", merged)
	}
	if strings.Contains(merged, "codex_hooks = false") {
		t.Fatalf("merged config did not update codex_hooks value: %q", merged)
	}
}

func TestMergeCodexConfig_UpdatesInlineCodexHooksOutsideQuotedString(t *testing.T) {
	raw := "features = { note = \"x, codex_hooks = false\", codex_hooks = false }\n[profiles.default]\nmodel = \"gpt-5\"\n"

	merged, err := mergeCodexConfig(raw, true)
	if err != nil {
		t.Fatalf("mergeCodexConfig() error = %v", err)
	}
	if !strings.Contains(merged, `note = "x, codex_hooks = false"`) {
		t.Fatalf("merged config corrupted quoted string value: %q", merged)
	}
	if !strings.Contains(merged, "codex_hooks = true") {
		t.Fatalf("merged config missing actual codex_hooks update: %q", merged)
	}
	if strings.Contains(merged, "note = \"x, codex_hooks = true\"") {
		t.Fatalf("merged config updated quoted string instead of codex_hooks key: %q", merged)
	}
}

func TestMergeCodexConfig_UpdatesNestedInlineTablesWithoutCorruption(t *testing.T) {
	raw := "features = { nested = { enabled = true }, codex_hooks = false }\n[profiles.default]\nmodel = \"gpt-5\"\n"

	merged, err := mergeCodexConfig(raw, true)
	if err != nil {
		t.Fatalf("mergeCodexConfig() error = %v", err)
	}
	if !strings.Contains(merged, "nested = { enabled = true }") {
		t.Fatalf("merged config corrupted nested inline table: %q", merged)
	}
	if !strings.Contains(merged, "codex_hooks = true") {
		t.Fatalf("merged config missing codex_hooks update: %q", merged)
	}
}

func TestPatchInlineCodexFeaturesLine_RejectsTrailingInlineTableTokens(t *testing.T) {
	line := "features = { codex_hooks = false }, other = { enabled = true }"
	if _, ok := patchInlineCodexFeaturesLine(line, true); ok {
		t.Fatalf("patchInlineCodexFeaturesLine() = ok for trailing tokens, want reject")
	}
}

func TestMergeCodexConfig_FormatsCodexHooksBeforeFollowingTable(t *testing.T) {
	raw := "[features]\nalpha = true\n\n[ui]\ncompact = true\n"

	merged, err := mergeCodexConfig(raw, true)
	if err != nil {
		t.Fatalf("mergeCodexConfig() error = %v", err)
	}
	if !strings.Contains(merged, "alpha = true\ncodex_hooks = true") {
		t.Fatalf("merged config missing codex_hooks line in features block: %q", merged)
	}
	if idx := strings.Index(merged, "[ui]"); idx < 0 || !strings.Contains(merged[:idx], "codex_hooks = true") {
		t.Fatalf("merged config did not preserve table boundary formatting: %q", merged)
	}
}

func TestMergeCodexConfig_FormatsCodexHooksBeforeArrayOfTables(t *testing.T) {
	raw := "[features]\nalpha = true\n\n[[rules]]\nname = \"one\"\n"

	merged, err := mergeCodexConfig(raw, false)
	if err != nil {
		t.Fatalf("mergeCodexConfig() error = %v", err)
	}
	featuresIdx := strings.Index(merged, "[features]")
	rulesIdx := strings.Index(merged, "[[rules]]")
	flagIdx := strings.Index(merged, "codex_hooks = false")
	if featuresIdx < 0 || rulesIdx < 0 || flagIdx < 0 {
		t.Fatalf("merged config missing expected blocks: %q", merged)
	}
	if !(featuresIdx < flagIdx && flagIdx < rulesIdx) {
		t.Fatalf("expected codex_hooks to stay inside features block before array table: %q", merged)
	}
}

func TestMergeCodexConfig_PatchesIndentedFeaturesTable(t *testing.T) {
	raw := "  [features]\n  alpha = true\n\n  [ui]\n  compact = true\n"

	merged, err := mergeCodexConfig(raw, true)
	if err != nil {
		t.Fatalf("mergeCodexConfig() error = %v", err)
	}
	if strings.Count(merged, "[features]") != 1 {
		t.Fatalf("expected one features table, got %q", merged)
	}
	if !strings.Contains(merged, "codex_hooks = true") {
		t.Fatalf("merged config missing codex_hooks line: %q", merged)
	}
	var parsed map[string]any
	if err := toml.Unmarshal([]byte(merged), &parsed); err != nil {
		t.Fatalf("merged config did not parse as TOML: %v; output=%q", err, merged)
	}
	if _, ok := parsed["features"]; !ok {
		t.Fatalf("parsed TOML missing features table: %#v", parsed)
	}
}

func TestMergeCodexConfig_DisablesFeatureFlagWhenEmpty(t *testing.T) {
	merged, err := mergeCodexConfig("", false)
	if err != nil {
		t.Fatalf("mergeCodexConfig() error = %v", err)
	}
	if !strings.Contains(merged, "codex_hooks = false") {
		t.Fatalf("merged config missing disabled feature flag: %q", merged)
	}
}

func TestMergeCodexConfig_RejectsFeatureTypeConflict(t *testing.T) {
	_, err := mergeCodexConfig("features = true\n", true)
	if err == nil {
		t.Fatal("mergeCodexConfig() error = nil, want conflict error")
	}
}

func TestCompileClaudeHooks_EmitsEmptyHooksSurface(t *testing.T) {
	projectRoot := "/tmp/project"

	files, warnings, err := CompileClaudeHooks(nil, projectRoot, `{"profiles":{"default":{"model":"gpt-5"}}}`)
	if err != nil {
		t.Fatalf("CompileClaudeHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	compiled := findHookCompiledFile(t, files, filepath.Join(projectRoot, ".claude", "settings.json"))
	if !strings.Contains(compiled.Content, `"hooks":{}`) {
		t.Fatalf("expected empty hooks object in claude output: %q", compiled.Content)
	}
	if !strings.Contains(compiled.Content, `"model":"gpt-5"`) {
		t.Fatalf("expected raw config to be preserved: %q", compiled.Content)
	}
}

func TestCompileClaudeHooks_SupportsJSONWithComments(t *testing.T) {
	projectRoot := "/tmp/project"
	raw := "{\n  // keep default profile\n  \"profiles\": {\n    \"default\": {\n      \"model\": \"gpt-5\",\n      \"endpoint\": \"https://example.com/api\"\n    }\n  },\n  /* keep UI */\n  \"ui\": { \"compact\": true }\n}\n"

	files, warnings, err := CompileClaudeHooks(nil, projectRoot, raw)
	if err != nil {
		t.Fatalf("CompileClaudeHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	compiled := findHookCompiledFile(t, files, filepath.Join(projectRoot, ".claude", "settings.json"))
	if !strings.Contains(compiled.Content, `"endpoint":"https://example.com/api"`) {
		t.Fatalf("expected URL string to survive JSONC parsing: %q", compiled.Content)
	}
	if !strings.Contains(compiled.Content, `"hooks":{}`) {
		t.Fatalf("expected merged hooks object in JSONC output: %q", compiled.Content)
	}
}

func TestCompileCodexHooks_EmitsEmptyHooksSurface(t *testing.T) {
	projectRoot := "/tmp/project"
	raw := "[profiles.default]\nmodel = \"gpt-5\"\n"

	files, warnings, err := CompileCodexHooks(nil, projectRoot, raw)
	if err != nil {
		t.Fatalf("CompileCodexHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	config := findHookCompiledFile(t, files, filepath.Join(projectRoot, ".codex", "config.toml"))
	if !strings.Contains(config.Content, "codex_hooks = false") {
		t.Fatalf("expected disabled codex feature flag: %q", config.Content)
	}
	if !strings.Contains(config.Content, "gpt-5") {
		t.Fatalf("expected raw config to be preserved: %q", config.Content)
	}

	hooksJSON := findHookCompiledFile(t, files, filepath.Join(projectRoot, ".codex", "hooks.json"))
	if !strings.Contains(hooksJSON.Content, `"hooks":{}`) {
		t.Fatalf("expected empty hooks object in codex output: %q", hooksJSON.Content)
	}
}

func TestCompileCodexHooks_EmitsEmptyMatcherAndNumericTimeout(t *testing.T) {
	projectRoot := "/tmp/project"
	records := []HookRecord{
		{
			ID:           "codex/user-prompt-submit/matcher.yaml",
			RelativePath: "codex/user-prompt-submit/matcher.yaml",
			Tool:         "codex",
			Event:        "UserPromptSubmit",
			Matcher:      "",
			Handlers: []HookHandler{
				{Type: "command", Command: "./submit.sh", TimeoutSeconds: intPtr(30)},
			},
		},
		{
			ID:           "codex/stop/matcher.yaml",
			RelativePath: "codex/stop/matcher.yaml",
			Tool:         "codex",
			Event:        "Stop",
			Matcher:      "",
			Handlers: []HookHandler{
				{Type: "command", Command: "./stop.sh", TimeoutSeconds: intPtr(45)},
			},
		},
	}

	files, warnings, err := CompileCodexHooks(records, projectRoot, "")
	if err != nil {
		t.Fatalf("CompileCodexHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	compiled := findHookCompiledFile(t, files, filepath.Join(projectRoot, ".codex", "hooks.json"))
	if strings.Contains(compiled.Content, `"matcher"`) {
		t.Fatalf("expected empty codex matcher to be omitted: %q", compiled.Content)
	}
	if !strings.Contains(compiled.Content, `"timeout":30`) || !strings.Contains(compiled.Content, `"timeout":45`) {
		t.Fatalf("expected numeric timeout values in codex output: %q", compiled.Content)
	}
	if !strings.Contains(compiled.Content, "UserPromptSubmit") || !strings.Contains(compiled.Content, "Stop") {
		t.Fatalf("expected codex events in output: %q", compiled.Content)
	}
}

func TestCompileCodexHooks_SkipsUnsupportedHandlers(t *testing.T) {
	projectRoot := "/tmp/project"
	records := []HookRecord{
		{
			ID:           "codex/pre-tool-use/bash.yaml",
			RelativePath: "codex/pre-tool-use/bash.yaml",
			Tool:         "codex",
			Event:        "PreToolUse",
			Matcher:      "Bash",
			Handlers: []HookHandler{
				{Type: "command", Command: "./bin/check"},
				{Type: "http", URL: "https://example.com/hook"},
				{Type: "prompt", Prompt: "Summarize the tool input"},
			},
		},
	}

	files, warnings, err := CompileCodexHooks(records, projectRoot, "")
	if err != nil {
		t.Fatalf("CompileCodexHooks() error = %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for unsupported handlers")
	}

	compiled := findHookCompiledFile(t, files, filepath.Join(projectRoot, ".codex", "hooks.json"))
	if strings.Contains(compiled.Content, `"type":"http"`) || strings.Contains(compiled.Content, `"type":"prompt"`) {
		t.Fatalf("unsupported handlers leaked into codex output: %q", compiled.Content)
	}
	if !strings.Contains(compiled.Content, `"type":"command"`) {
		t.Fatalf("codex output missing command handler: %q", compiled.Content)
	}
}

func TestCompileCodexHooks_SkipsUnsupportedEvents(t *testing.T) {
	projectRoot := "/tmp/project"
	records := []HookRecord{
		{
			ID:           "codex/file-changed/bash.yaml",
			RelativePath: "codex/file-changed/bash.yaml",
			Tool:         "codex",
			Event:        "FileChanged",
			Matcher:      "Bash",
			Handlers:     []HookHandler{{Type: "command", Command: "./bin/check"}},
		},
		{
			ID:           "codex/pre-tool-use/bash.yaml",
			RelativePath: "codex/pre-tool-use/bash.yaml",
			Tool:         "codex",
			Event:        "PreToolUse",
			Matcher:      "Bash",
			Handlers:     []HookHandler{{Type: "command", Command: "./bin/ok"}},
		},
	}

	files, warnings, err := CompileCodexHooks(records, projectRoot, "")
	if err != nil {
		t.Fatalf("CompileCodexHooks() error = %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for unsupported codex event")
	}

	compiled := findHookCompiledFile(t, files, filepath.Join(projectRoot, ".codex", "hooks.json"))
	if strings.Contains(compiled.Content, "FileChanged") {
		t.Fatalf("unsupported codex event leaked into compiled output: %q", compiled.Content)
	}
	if !strings.Contains(compiled.Content, "PreToolUse") {
		t.Fatalf("supported codex event missing from compiled output: %q", compiled.Content)
	}
}

func TestCompileGeminiHooks_WritesSettingsJSON(t *testing.T) {
	projectRoot := "/tmp/project"
	sequential := true
	records := []HookRecord{
		{
			ID:           "gemini/before-tool/read.yaml",
			RelativePath: "gemini/before-tool/read.yaml",
			Tool:         "gemini",
			Event:        "BeforeTool",
			Matcher:      "Read",
			Sequential:   &sequential,
			Handlers: []HookHandler{{
				Type:        "command",
				Name:        "lint-read",
				Description: "Run read lint",
				Command:     "./bin/gemini-lint",
				Timeout:     "30000",
			}},
		},
	}

	files, warnings, err := CompileGeminiHooks(records, projectRoot, `{"theme":"light"}`)
	if err != nil {
		t.Fatalf("CompileGeminiHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("CompileGeminiHooks() warnings = %v, want none", warnings)
	}

	compiled := findHookCompiledFile(t, files, filepath.Join(projectRoot, ".gemini", "settings.json"))
	for _, want := range []string{
		`"theme":"light"`,
		`"BeforeTool"`,
		`"matcher":"Read"`,
		`"sequential":true`,
		`"type":"command"`,
		`"name":"lint-read"`,
		`"description":"Run read lint"`,
		`"command":"./bin/gemini-lint"`,
		`"timeout":30000`,
	} {
		if !strings.Contains(compiled.Content, want) {
			t.Fatalf("compiled content missing %q: %q", want, compiled.Content)
		}
	}
}

func TestCompileGeminiHooks_SkipsUnsupportedEventsAndHandlers(t *testing.T) {
	projectRoot := "/tmp/project"
	records := []HookRecord{
		{
			ID:           "gemini/before-tool/read.yaml",
			RelativePath: "gemini/before-tool/read.yaml",
			Tool:         "gemini",
			Event:        "BeforeTool",
			Matcher:      "Read",
			Handlers: []HookHandler{
				{Type: "command", Command: "./bin/gemini-lint"},
				{Type: "http", URL: "https://example.com/hook"},
			},
		},
		{
			ID:           "gemini/file-changed/read.yaml",
			RelativePath: "gemini/file-changed/read.yaml",
			Tool:         "gemini",
			Event:        "FileChanged",
			Matcher:      "Read",
			Handlers:     []HookHandler{{Type: "command", Command: "./bin/skip"}},
		},
	}

	files, warnings, err := CompileGeminiHooks(records, projectRoot, "")
	if err != nil {
		t.Fatalf("CompileGeminiHooks() error = %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings for unsupported gemini hook content")
	}

	compiled := findHookCompiledFile(t, files, filepath.Join(projectRoot, ".gemini", "settings.json"))
	if strings.Contains(compiled.Content, `"type":"http"`) {
		t.Fatalf("unsupported gemini handler leaked into output: %q", compiled.Content)
	}
	if strings.Contains(compiled.Content, "FileChanged") {
		t.Fatalf("unsupported gemini event leaked into output: %q", compiled.Content)
	}
	if !strings.Contains(compiled.Content, "BeforeTool") {
		t.Fatalf("supported gemini event missing from output: %q", compiled.Content)
	}
}

func findHookCompiledFile(t *testing.T, files []CompiledFile, wantPath string) CompiledFile {
	t.Helper()
	for _, file := range files {
		if file.Path == wantPath {
			return file
		}
	}
	t.Fatalf("compiled output missing %q", wantPath)
	return CompiledFile{}
}

func intPtr(v int) *int {
	return &v
}
