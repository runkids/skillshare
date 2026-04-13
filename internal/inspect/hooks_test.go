package inspect

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func findHookItem(t *testing.T, items []HookItem, path string) HookItem {
	t.Helper()
	for _, item := range items {
		if item.Path == path {
			return item
		}
	}
	t.Fatalf("hook item with path %q not found", path)
	return HookItem{}
}

func TestScanHooks_GlobalAndProjectLocations(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	files := map[string]string{
		filepath.Join(home, ".claude", "settings.json"):          `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./scripts/check.sh"}]}]}}`,
		filepath.Join(home, ".gemini", "settings.json"):          `{"hooks":{"BeforeTool":[{"matcher":"Read","hooks":[{"type":"command","command":"./scripts/gemini.sh"}]}]}}`,
		filepath.Join(project, ".claude", "settings.json"):       `{"hooks":{"PostToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./project/post.sh"}]}]}}`,
		filepath.Join(project, ".claude", "settings.local.json"): `{"hooks":{"PreToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"./project/local.sh"}]}]}}`,
		filepath.Join(project, ".gemini", "settings.json"):       `{"hooks":{"BeforeTool":[{"matcher":"Write","hooks":[{"type":"command","command":"./project/lint.sh"}]}]}}`,
	}
	for path, content := range files {
		mustWriteFile(t, path, content)
	}

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != len(files) {
		t.Fatalf("expected %d items, got %d", len(files), len(items))
	}

	globalClaude := findHookItem(t, items, filepath.Join(home, ".claude", "settings.json"))
	if globalClaude.SourceTool != "claude" {
		t.Fatalf("sourceTool = %q, want claude", globalClaude.SourceTool)
	}
	if globalClaude.Scope != ScopeUser {
		t.Fatalf("scope = %q, want user", globalClaude.Scope)
	}
	if globalClaude.Event != "PreToolUse" {
		t.Fatalf("event = %q, want PreToolUse", globalClaude.Event)
	}
	if globalClaude.Matcher != "Bash" {
		t.Fatalf("matcher = %q, want Bash", globalClaude.Matcher)
	}
	if globalClaude.Command != "./scripts/check.sh" {
		t.Fatalf("command = %q, want ./scripts/check.sh", globalClaude.Command)
	}
	if globalClaude.ActionType != "command" {
		t.Fatalf("actionType = %q, want command", globalClaude.ActionType)
	}

	projectGemini := findHookItem(t, items, filepath.Join(project, ".gemini", "settings.json"))
	if projectGemini.SourceTool != "gemini" {
		t.Fatalf("sourceTool = %q, want gemini", projectGemini.SourceTool)
	}
	if projectGemini.Scope != ScopeProject {
		t.Fatalf("scope = %q, want project", projectGemini.Scope)
	}
	if projectGemini.Event != "BeforeTool" {
		t.Fatalf("event = %q, want BeforeTool", projectGemini.Event)
	}
	if projectGemini.Command != "./project/lint.sh" {
		t.Fatalf("command = %q, want ./project/lint.sh", projectGemini.Command)
	}
}

func TestScanHooks_IgnoresHomeClaudeSettingsLocal(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(home, ".claude", "settings.local.json"), `{"hooks":{"PreToolUse":[{"matcher":"Edit","hooks":[{"type":"command","command":"./scripts/check-local.sh"}]}]}}`)
	mustWriteFile(t, filepath.Join(project, ".claude", "settings.json"), `{"hooks":{"PostToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./project/post.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item from project config, got %d", len(items))
	}
	if items[0].Path != filepath.Join(project, ".claude", "settings.json") {
		t.Fatalf("path = %q, want project settings.json", items[0].Path)
	}
	if items[0].Command != "./project/post.sh" {
		t.Fatalf("command = %q, want ./project/post.sh", items[0].Command)
	}
}

func TestScanHooks_HomeRootIncludesProjectClaudeSettingsLocal(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "workspace")

	mustWriteFile(t, filepath.Join(home, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./shared.sh"}]}]}}`)
	mustWriteFile(t, filepath.Join(home, ".claude", "settings.local.json"), `{"hooks":{"PreToolUse":[{"matcher":"Edit","hooks":[{"type":"command","command":"./local.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(home)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items from shared home/project config, got %d", len(items))
	}
	got := map[string]HookItem{}
	for _, item := range items {
		got[item.Path] = item
	}
	shared := got[filepath.Join(home, ".claude", "settings.json")]
	if shared.Command != "./shared.sh" {
		t.Fatalf("shared command = %q, want ./shared.sh", shared.Command)
	}
	if shared.Scope != ScopeProject {
		t.Fatalf("shared scope = %q, want project", shared.Scope)
	}
	local := got[filepath.Join(home, ".claude", "settings.local.json")]
	if local.Command != "./local.sh" {
		t.Fatalf("local command = %q, want ./local.sh", local.Command)
	}
	if local.Scope != ScopeProject {
		t.Fatalf("local scope = %q, want project", local.Scope)
	}
}

func TestScanHooks_DirectKnownPathFIFOReturnsPromptly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fifo behavior is platform-dependent on windows")
	}

	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	fifoPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(fifoPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", fifoPath, err)
	}
	if err := createTestFIFO(fifoPath, 0o644); err != nil {
		t.Skipf("unable to create fifo: %v", err)
	}

	t.Setenv("HOME", home)

	type result struct {
		items    []HookItem
		warnings []string
		err      error
	}
	done := make(chan result, 1)
	go func() {
		items, warnings, err := ScanHooks("")
		done <- result{items: items, warnings: warnings, err: err}
	}()

	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("ScanHooks() error = %v", res.err)
		}
		if len(res.items) != 0 {
			t.Fatalf("expected 0 items, got %d", len(res.items))
		}
		if len(res.warnings) == 0 {
			t.Fatal("expected warning for fifo hook config")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ScanHooks() hung on fifo hook config")
	}
}

func TestScanHooks_UnsupportedShapesAreSkippedAndWarningsCollected(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(home, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./scripts/check.sh"}]}]}}`)
	mustWriteFile(t, filepath.Join(project, ".gemini", "settings.json"), `{"hooks":{"BeforeTool":[{"matcher":"Write","hooks":[{"type":"unknown","command":"./skip.sh"},{"type":"command"}]},{"matcher":"SkipMe","hooks":"not-an-array"}]}}`)
	mustWriteFile(t, filepath.Join(project, ".claude", "settings.local.json"), `not json`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 supported hook item, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings for malformed hook config")
	}
	for _, item := range items {
		if item.Command == "./skip.sh" {
			t.Fatal("unsupported hook shapes should be skipped")
		}
	}
}

func TestScanHooks_MalformedInFileConfigEmitsWarnings(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./scripts/check.sh"}]},{"matcher":"Write","hooks":"not-an-array"},{"matcher":"Edit","hooks":[{"type":"command"}]},{"matcher":"Skip","hooks":[{"type":"unknown","command":"./skip.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 supported hook item, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings for malformed in-file hook config")
	}

	item := items[0]
	if item.Command != "./scripts/check.sh" {
		t.Fatalf("command = %q, want ./scripts/check.sh", item.Command)
	}
	if item.Scope != ScopeProject {
		t.Fatalf("scope = %q, want project", item.Scope)
	}
}

func TestScanHooks_MissingHooksArrayEmitsWarning(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash"},{"matcher":"Edit","hooks":[{"type":"command","command":"./scripts/check.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 supported hook item, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for missing hooks array")
	}

	item := items[0]
	if item.Command != "./scripts/check.sh" {
		t.Fatalf("command = %q, want ./scripts/check.sh", item.Command)
	}
	if item.Scope != ScopeProject {
		t.Fatalf("scope = %q, want project", item.Scope)
	}
}

func TestScanHooks_ReadsSymlinkedConfigFile(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	target := filepath.Join(tmp, "outside.json")
	mustWriteFile(t, target, `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./leak.sh"}]}]}}`)

	link := filepath.Join(project, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", link, err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if item.Path != link {
		t.Fatalf("path = %q, want symlink path %q", item.Path, link)
	}
	if item.Command != "./leak.sh" {
		t.Fatalf("command = %q, want ./leak.sh", item.Command)
	}
	if item.Scope != ScopeProject {
		t.Fatalf("scope = %q, want project", item.Scope)
	}
}

func TestScanHooks_SkipsOversizedConfigFile(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	path := filepath.Join(project, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("a"), 512*1024+1), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for oversized hook config")
	}
}

func TestScanHooks_SkipsNonRegularConfigFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix domain sockets are not supported on windows")
	}

	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	path := filepath.Join(project, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Skipf("unable to create unix socket: %v", err)
	}
	t.Cleanup(func() {
		listener.Close()
		_ = os.Remove(path)
	})

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for non-regular hook config")
	}
}

func TestScanHooks_DedupesOverlappingHomeAndProjectRoots(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "workspace")

	mustWriteFile(t, filepath.Join(home, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./shared.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(home)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.Path != filepath.Join(home, ".claude", "settings.json") {
		t.Fatalf("path = %q, want shared path", item.Path)
	}
	if item.Scope != ScopeProject {
		t.Fatalf("scope = %q, want project scope to win", item.Scope)
	}
}

func TestScanHooks_UnknownEventNamesAreIgnored(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./ok.sh"}]}],"BogusEvent":[{"matcher":"Bash","hooks":[{"type":"command","command":"./skip.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Event != "PreToolUse" {
		t.Fatalf("event = %q, want PreToolUse", items[0].Event)
	}
	if items[0].Command != "./ok.sh" {
		t.Fatalf("command = %q, want ./ok.sh", items[0].Command)
	}
}

func TestScanHooks_ClaudeSupportedHandlerTypesProduceItems(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"http","url":"https://example.com/hook"},{"type":"prompt","prompt":"Evaluate the input"},{"type":"agent","prompt":"Verify the input"},{"type":"command","command":"./ok.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 hook items, got %d", len(items))
	}
	got := map[string]HookItem{}
	for _, item := range items {
		got[item.ActionType] = item
		if item.Event != "PreToolUse" {
			t.Fatalf("event = %q, want PreToolUse", item.Event)
		}
		if item.Scope != ScopeProject {
			t.Fatalf("scope = %q, want project", item.Scope)
		}
	}
	if got["command"].Command != "./ok.sh" {
		t.Fatalf("command = %q, want ./ok.sh", got["command"].Command)
	}
	if got["http"].ActionType != "http" {
		t.Fatalf("http actionType = %q, want http", got["http"].ActionType)
	}
	if got["http"].URL != "https://example.com/hook" {
		t.Fatalf("http url = %q, want https://example.com/hook", got["http"].URL)
	}
	if got["prompt"].ActionType != "prompt" {
		t.Fatalf("prompt actionType = %q, want prompt", got["prompt"].ActionType)
	}
	if got["prompt"].Prompt != "Evaluate the input" {
		t.Fatalf("prompt payload = %q, want Evaluate the input", got["prompt"].Prompt)
	}
	if got["agent"].ActionType != "agent" {
		t.Fatalf("agent actionType = %q, want agent", got["agent"].ActionType)
	}
	if got["agent"].Prompt != "Verify the input" {
		t.Fatalf("agent payload = %q, want Verify the input", got["agent"].Prompt)
	}
}

func TestScanHooks_ClaudeUnknownHandlerTypeEmitsWarning(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"commmand","command":"./bad.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for unknown Claude handler type")
	}
}

func TestScanHooks_GeminiUnknownHandlerTypeEmitsWarning(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".gemini", "settings.json"), `{"hooks":{"BeforeTool":[{"matcher":"Read","hooks":[{"type":"commmand","command":"./bad.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for unknown Gemini handler type")
	}
}

func TestScanHooks_GeminiHttpHandlerEmitsInvalidWarning(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".gemini", "settings.json"), `{"hooks":{"BeforeTool":[{"matcher":"Read","hooks":[{"type":"http","url":"https://example.com/hook"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for Gemini http handler type")
	}
	if !strings.Contains(warnings[0], "invalid") && !strings.Contains(warnings[0], "unknown type") {
		t.Fatalf("warning = %q, want invalid/unknown type warning", warnings[0])
	}
}

func TestScanHooks_NullHooksBlockEmitsWarning(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".claude", "settings.json"), `{"hooks":null}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for null hooks block")
	}
}

func TestScanHooks_NullHookEventEmitsWarning(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".claude", "settings.json"), `{"hooks":{"PreToolUse":null}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for null hook event")
	}
}

func TestScanHooks_ClaudeDocumentedEventIsRecognized(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".claude", "settings.json"), `{"hooks":{"UserPromptSubmit":[{"matcher":"Bash","hooks":[{"type":"command","command":"./submit.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Event != "UserPromptSubmit" {
		t.Fatalf("event = %q, want UserPromptSubmit", items[0].Event)
	}
	if items[0].Command != "./submit.sh" {
		t.Fatalf("command = %q, want ./submit.sh", items[0].Command)
	}
}

func TestScanHooks_GeminiDocumentedEventIsRecognized(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".gemini", "settings.json"), `{"hooks":{"BeforeAgent":[{"matcher":"Read","hooks":[{"type":"command","command":"./agent.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Event != "BeforeAgent" {
		t.Fatalf("event = %q, want BeforeAgent", items[0].Event)
	}
	if items[0].Command != "./agent.sh" {
		t.Fatalf("command = %q, want ./agent.sh", items[0].Command)
	}
}

func TestScanHooks_OverlappingRootPreservesMultipleHooksPerFile(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "workspace")

	mustWriteFile(t, filepath.Join(home, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./pre.sh"}]}],"PostToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./post.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(home)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	got := map[string]HookItem{}
	for _, item := range items {
		got[item.Event] = item
		if item.Scope != ScopeProject {
			t.Fatalf("scope for %s = %q, want project", item.Event, item.Scope)
		}
		if item.Path != filepath.Join(home, ".claude", "settings.json") {
			t.Fatalf("path for %s = %q, want shared path", item.Event, item.Path)
		}
	}
	if got["PreToolUse"].Command != "./pre.sh" {
		t.Fatalf("pre command = %q, want ./pre.sh", got["PreToolUse"].Command)
	}
	if got["PostToolUse"].Command != "./post.sh" {
		t.Fatalf("post command = %q, want ./post.sh", got["PostToolUse"].Command)
	}
}

func TestScanHooks_CodexAndPrivateLocal(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	userCodexPath := filepath.Join(home, ".codex", "hooks.json")
	projectCodexPath := filepath.Join(project, ".codex", "hooks.json")
	projectClaudeLocalPath := filepath.Join(project, ".claude", "settings.local.json")

	mustWriteFile(t, userCodexPath, `{"hooks":{"PreToolUse":[{"matcher":"Read","hooks":[{"type":"command","command":"./user-codex.sh"}]}]}}`)
	mustWriteFile(t, projectCodexPath, `{"hooks":{"PostToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"./project-codex.sh"}]}]}}`)
	mustWriteFile(t, projectClaudeLocalPath, `{"hooks":{"PreToolUse":[{"matcher":"Edit","hooks":[{"type":"command","command":"./project-local.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	var userCodex, projectCodex, projectLocal HookItem
	for _, item := range items {
		switch item.Path {
		case userCodexPath:
			userCodex = item
		case projectCodexPath:
			projectCodex = item
		case projectClaudeLocalPath:
			projectLocal = item
		}
	}

	if userCodex.Path == "" {
		t.Fatal("expected user codex hook item")
	}
	if userCodex.SourceTool != "codex" {
		t.Fatalf("sourceTool = %q, want codex", userCodex.SourceTool)
	}
	if userCodex.Scope != ScopeUser {
		t.Fatalf("scope = %q, want user", userCodex.Scope)
	}
	if userCodex.Command != "./user-codex.sh" {
		t.Fatalf("command = %q, want ./user-codex.sh", userCodex.Command)
	}
	if strings.TrimSpace(userCodex.GroupID) == "" {
		t.Fatal("expected non-empty groupID for user codex hook")
	}
	if !userCodex.Collectible {
		t.Fatal("expected user codex hook to be collectible")
	}
	if userCodex.CollectReason != "" {
		t.Fatalf("collectReason = %q, want empty", userCodex.CollectReason)
	}

	if projectCodex.Path == "" {
		t.Fatal("expected project codex hook item")
	}
	if projectCodex.SourceTool != "codex" {
		t.Fatalf("sourceTool = %q, want codex", projectCodex.SourceTool)
	}
	if projectCodex.Scope != ScopeProject {
		t.Fatalf("scope = %q, want project", projectCodex.Scope)
	}
	if projectCodex.Command != "./project-codex.sh" {
		t.Fatalf("command = %q, want ./project-codex.sh", projectCodex.Command)
	}
	if strings.TrimSpace(projectCodex.GroupID) == "" {
		t.Fatal("expected non-empty groupID for project codex hook")
	}
	if !projectCodex.Collectible {
		t.Fatal("expected project codex hook to be collectible")
	}
	if projectCodex.CollectReason != "" {
		t.Fatalf("collectReason = %q, want empty", projectCodex.CollectReason)
	}

	if projectLocal.Path == "" {
		t.Fatal("expected project local claude hook item")
	}
	if projectLocal.SourceTool != "claude" {
		t.Fatalf("sourceTool = %q, want claude", projectLocal.SourceTool)
	}
	if projectLocal.Scope != ScopeProject {
		t.Fatalf("scope = %q, want project", projectLocal.Scope)
	}
	if projectLocal.Command != "./project-local.sh" {
		t.Fatalf("command = %q, want ./project-local.sh", projectLocal.Command)
	}
	if strings.TrimSpace(projectLocal.GroupID) == "" {
		t.Fatal("expected non-empty groupID for local hook")
	}
	if projectLocal.Collectible {
		t.Fatal("expected project local claude hook to be non-collectible")
	}
	if !strings.Contains(strings.ToLower(projectLocal.CollectReason), "local") {
		t.Fatalf("collectReason = %q, want reason mentioning local/private scope", projectLocal.CollectReason)
	}
}

func TestScanHooks_CodexUnsupportedEventIgnored(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".codex", "hooks.json"), `{"hooks":{"FileChanged":[{"matcher":"Write","hooks":[{"type":"command","command":"./file-changed.sh"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings for unsupported codex event, got %v", warnings)
	}
	if len(items) != 0 {
		t.Fatalf("expected unsupported codex event to be ignored, got %d items", len(items))
	}
}

func TestScanHooks_CodexUnsupportedActionTypeWarned(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".codex", "hooks.json"), `{"hooks":{"PreToolUse":[{"matcher":"Write","hooks":[{"type":"http","url":"https://example.com/hook"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected unsupported codex action type to be skipped, got %d items", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for unsupported codex action type")
	}
}

func TestScanHooks_CodexEmptyMatcherAndNumericTimeout(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".codex", "hooks.json"), `{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":"./submit.sh","timeout":30}]}],"Stop":[{"matcher":"","hooks":[{"type":"command","command":"./stop.sh","timeoutSec":45}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 codex hook items, got %d", len(items))
	}

	got := map[string]HookItem{}
	for _, item := range items {
		got[item.Event] = item
		if item.Matcher != "" {
			t.Fatalf("matcher = %q, want empty for codex event %s", item.Matcher, item.Event)
		}
		if item.TimeoutSeconds == nil {
			t.Fatalf("timeoutSeconds = nil for event %s", item.Event)
		}
	}
	if got["UserPromptSubmit"].TimeoutSeconds == nil || *got["UserPromptSubmit"].TimeoutSeconds != 30 {
		t.Fatalf("UserPromptSubmit timeoutSeconds = %#v, want 30", got["UserPromptSubmit"].TimeoutSeconds)
	}
	if got["Stop"].TimeoutSeconds == nil || *got["Stop"].TimeoutSeconds != 45 {
		t.Fatalf("Stop timeoutSeconds = %#v, want 45", got["Stop"].TimeoutSeconds)
	}
}

func TestScanHooks_CodexRejectsNonNumericTimeoutString(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".codex", "hooks.json"), `{"hooks":{"PreToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"./check.sh","timeout":"30s"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected invalid codex timeout to be skipped, got %d items", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for invalid codex timeout string")
	}
}

func TestScanHooks_CodexPrefersNumericTimeoutSecOverInvalidTimeout(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".codex", "hooks.json"), `{"hooks":{"PreToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"./check.sh","timeout":"30s","timeoutSec":30}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 codex hook item, got %d", len(items))
	}
	if items[0].TimeoutSeconds == nil || *items[0].TimeoutSeconds != 30 {
		t.Fatalf("timeoutSeconds = %#v, want 30", items[0].TimeoutSeconds)
	}
}

func TestScanHooks_GeminiHooksAreCollectibleAndPreserveMetadata(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".gemini", "settings.json"), `{"hooks":{"BeforeTool":[{"matcher":"Read","sequential":true,"hooks":[{"type":"command","name":"lint-read","description":"Run read lint","command":"./gemini.sh","timeout":30000}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 gemini hook item, got %d", len(items))
	}
	if !items[0].Collectible {
		t.Fatal("expected gemini hook to be collectible")
	}
	if items[0].CollectReason != "" {
		t.Fatalf("collectReason = %q, want empty", items[0].CollectReason)
	}
	if items[0].Sequential == nil || !*items[0].Sequential {
		t.Fatalf("sequential = %#v, want true", items[0].Sequential)
	}
	if items[0].Name != "lint-read" {
		t.Fatalf("name = %q, want lint-read", items[0].Name)
	}
	if items[0].Description != "Run read lint" {
		t.Fatalf("description = %q, want Run read lint", items[0].Description)
	}
	if items[0].Timeout != "30000" {
		t.Fatalf("timeout = %q, want 30000", items[0].Timeout)
	}
}

func TestScanHooks_PreservesHookMetadata(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./check.sh","timeout":"30s","statusMessage":"Running check"}]}]}}`)

	t.Setenv("HOME", home)

	items, warnings, err := ScanHooks(project)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Timeout != "30s" {
		t.Fatalf("timeout = %q, want 30s", items[0].Timeout)
	}
	if items[0].StatusMessage != "Running check" {
		t.Fatalf("statusMessage = %q, want Running check", items[0].StatusMessage)
	}
}
