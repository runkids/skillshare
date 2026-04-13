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

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func findRuleItem(t *testing.T, items []RuleItem, pathSuffix string) RuleItem {
	t.Helper()
	for _, item := range items {
		if strings.HasSuffix(item.Path, pathSuffix) {
			return item
		}
	}
	t.Fatalf("rule item with path suffix %q not found", pathSuffix)
	return RuleItem{}
}

func TestScanRules_GlobalAndProjectLocations(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	files := map[string]string{
		filepath.Join(home, ".claude", "CLAUDE.md"):               "# Global Claude",
		filepath.Join(home, ".codex", "AGENTS.md"):                "# Global Codex",
		filepath.Join(home, ".gemini", "GEMINI.md"):               "# Global Gemini",
		filepath.Join(home, ".claude", "rules", "global.md"):      "# Global Rule",
		filepath.Join(project, "CLAUDE.md"):                       "# Project Claude",
		filepath.Join(project, "AGENTS.md"):                       "# Project Codex",
		filepath.Join(project, "GEMINI.md"):                       "# Project Gemini",
		filepath.Join(project, ".claude", "CLAUDE.md"):            "# Project Claude Hidden",
		filepath.Join(project, ".codex", "AGENTS.md"):             "# Project Codex Hidden",
		filepath.Join(project, ".gemini", "GEMINI.md"):            "# Project Gemini Hidden",
		filepath.Join(project, ".claude", "rules", "backend.md"):  "---\npaths:\n  - src/**\n  - lib/**\n---\n# Backend",
		filepath.Join(project, ".gemini", "rules", "frontend.md"): "# Frontend Rule",
	}
	for path, content := range files {
		mustWriteFile(t, path, content)
	}

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != len(files) {
		t.Fatalf("expected %d items, got %d", len(files), len(items))
	}

	scoped := findRuleItem(t, items, filepath.Join(".claude", "rules", "backend.md"))
	if !scoped.IsScoped {
		t.Fatal("expected backend rule to be scoped")
	}
	wantPaths := []string{"src/**", "lib/**"}
	if len(scoped.ScopedPaths) != len(wantPaths) {
		t.Fatalf("scoped paths = %v, want %v", scoped.ScopedPaths, wantPaths)
	}
	for i, want := range wantPaths {
		if scoped.ScopedPaths[i] != want {
			t.Fatalf("scoped path[%d] = %q, want %q", i, scoped.ScopedPaths[i], want)
		}
	}
	if scoped.SourceTool != "claude" {
		t.Fatalf("sourceTool = %q, want claude", scoped.SourceTool)
	}
	if scoped.Scope != ScopeProject {
		t.Fatalf("scope = %q, want project", scoped.Scope)
	}
	if scoped.Path != filepath.Join(project, ".claude", "rules", "backend.md") {
		t.Fatalf("path = %q, want project backend path", scoped.Path)
	}
	if !scoped.Exists {
		t.Fatal("expected scoped rule to exist")
	}
	if scoped.Size == 0 {
		t.Fatal("expected scoped rule size to be > 0")
	}
}

func TestScanRules_IncludesPiInstructionFiles(t *testing.T) {
	project := t.TempDir()
	mustWriteFile(t, filepath.Join(project, ".pi", "SYSTEM.md"), "# System")
	mustWriteFile(t, filepath.Join(project, ".pi", "APPEND_SYSTEM.md"), "# Append")

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("ScanRules() warnings = %v", warnings)
	}

	var foundSystem, foundAppend bool
	for _, item := range items {
		if item.SourceTool == "pi" && item.Path == filepath.Join(project, ".pi", "SYSTEM.md") {
			foundSystem = true
		}
		if item.SourceTool == "pi" && item.Path == filepath.Join(project, ".pi", "APPEND_SYSTEM.md") {
			foundAppend = true
		}
	}
	if !foundSystem || !foundAppend {
		t.Fatalf("items = %#v, want pi SYSTEM and APPEND_SYSTEM entries", items)
	}
}

func TestScanRules_IncludesGlobalPiInstructionFiles(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")

	mustWriteFile(t, filepath.Join(home, ".pi", "agent", "AGENTS.md"), "# Global Pi Agents")
	mustWriteFile(t, filepath.Join(home, ".pi", "agent", "SYSTEM.md"), "# Global Pi System")
	mustWriteFile(t, filepath.Join(home, ".pi", "agent", "APPEND_SYSTEM.md"), "# Global Pi Append")

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules("")
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("ScanRules() warnings = %v", warnings)
	}

	agents := findRuleItem(t, items, filepath.Join(".pi", "agent", "AGENTS.md"))
	if agents.SourceTool != "pi" {
		t.Fatalf("global pi agents sourceTool = %q, want pi", agents.SourceTool)
	}
	if agents.Scope != ScopeUser {
		t.Fatalf("global pi agents scope = %q, want user", agents.Scope)
	}

	system := findRuleItem(t, items, filepath.Join(".pi", "agent", "SYSTEM.md"))
	if system.SourceTool != "pi" {
		t.Fatalf("global pi system sourceTool = %q, want pi", system.SourceTool)
	}
	if system.Scope != ScopeUser {
		t.Fatalf("global pi system scope = %q, want user", system.Scope)
	}

	appendSystem := findRuleItem(t, items, filepath.Join(".pi", "agent", "APPEND_SYSTEM.md"))
	if appendSystem.SourceTool != "pi" {
		t.Fatalf("global pi append sourceTool = %q, want pi", appendSystem.SourceTool)
	}
	if appendSystem.Scope != ScopeUser {
		t.Fatalf("global pi append scope = %q, want user", appendSystem.Scope)
	}
}

func TestScanRules_ProjectRootAgentsStaysCodexWhenPiFilesExist(t *testing.T) {
	project := t.TempDir()
	agentsPath := filepath.Join(project, "AGENTS.md")
	mustWriteFile(t, agentsPath, "# Project Codex")
	mustWriteFile(t, filepath.Join(project, ".pi", "SYSTEM.md"), "# System")

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("ScanRules() warnings = %v", warnings)
	}

	var codexRootCount, piRootCount int
	for _, item := range items {
		if item.Path != agentsPath {
			continue
		}
		switch item.SourceTool {
		case "codex":
			codexRootCount++
		case "pi":
			piRootCount++
		}
	}

	if codexRootCount != 1 {
		t.Fatalf("root AGENTS codex count = %d, want 1; items = %#v", codexRootCount, items)
	}
	if piRootCount != 0 {
		t.Fatalf("root AGENTS pi count = %d, want 0; items = %#v", piRootCount, items)
	}
}

func TestScanRules_MalformedFrontmatterDegradesToUnscoped(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, ".claude", "rules", "broken.md"), "---\npaths: [src/**\n---\n# Broken")

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings for malformed frontmatter")
	}
	item := items[0]
	if item.IsScoped {
		t.Fatal("malformed frontmatter should degrade to unscoped")
	}
	if len(item.ScopedPaths) != 0 {
		t.Fatalf("scoped paths = %v, want none", item.ScopedPaths)
	}
}

func TestScanRules_LaterBodyDelimiterDoesNotCreateScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	path := filepath.Join(project, ".claude", "rules", "later.md")
	mustWriteFile(t, path, "# Body first\n---\npaths:\n  - src/**\n---")

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if item.Path != path {
		t.Fatalf("path = %q, want %q", item.Path, path)
	}
	if item.IsScoped {
		t.Fatal("later body delimiter should not create scope")
	}
	if len(item.ScopedPaths) != 0 {
		t.Fatalf("scoped paths = %v, want none", item.ScopedPaths)
	}
}

func TestScanRules_UnclosedLeadingFrontmatterDoesNotCreateScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	path := filepath.Join(project, ".claude", "rules", "unclosed.md")
	mustWriteFile(t, path, "---\npaths:\n  - src/**\n# missing closing delimiter")

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings for unclosed frontmatter")
	}
	item := items[0]
	if item.Path != path {
		t.Fatalf("path = %q, want %q", item.Path, path)
	}
	if item.IsScoped {
		t.Fatal("unclosed frontmatter should not create scope")
	}
	if len(item.ScopedPaths) != 0 {
		t.Fatalf("scoped paths = %v, want none", item.ScopedPaths)
	}
}

func TestScanRules_NonFrontmatterPrefixDoesNotCreateScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	path := filepath.Join(project, ".claude", "rules", "prefix.md")
	mustWriteFile(t, path, strings.Join([]string{
		"---not-frontmatter",
		"Body text before a real-looking block.",
		"---",
		"paths:",
		"  - src/**",
		"---",
		"# Trailing body",
	}, "\n"))

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if item.Path != path {
		t.Fatalf("path = %q, want %q", item.Path, path)
	}
	if item.IsScoped {
		t.Fatal("prefix line should not be treated as frontmatter")
	}
	if len(item.ScopedPaths) != 0 {
		t.Fatalf("scoped paths = %v, want none", item.ScopedPaths)
	}
}

func TestScanRules_TrailingSpaceDelimitersDoNotCount(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "opening delimiter with trailing space",
			content: strings.Join([]string{
				"--- ",
				"paths:",
				"  - src/**",
				"---",
				"# Body",
			}, "\n"),
		},
		{
			name: "closing delimiter with trailing space",
			content: strings.Join([]string{
				"---",
				"paths:",
				"  - src/**",
				"--- ",
				"# Body",
			}, "\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			home := filepath.Join(tmp, "home")
			project := filepath.Join(tmp, "project")

			path := filepath.Join(project, ".claude", "rules", "trailing-space.md")
			mustWriteFile(t, path, tt.content)

			t.Setenv("HOME", home)

			items, _, err := ScanRules(project)
			if err != nil {
				t.Fatalf("ScanRules() error = %v", err)
			}
			if len(items) != 1 {
				t.Fatalf("expected 1 item, got %d", len(items))
			}
			item := items[0]
			if item.IsScoped {
				t.Fatal("trailing-space delimiter should not create scope")
			}
			if len(item.ScopedPaths) != 0 {
				t.Fatalf("scoped paths = %v, want none", item.ScopedPaths)
			}
		})
	}
}

func TestScanRules_IndentedOpenerDoesNotCreateScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	path := filepath.Join(project, ".claude", "rules", "indented.md")
	mustWriteFile(t, path, strings.Join([]string{
		"  ---",
		"paths:",
		"  - src/**",
		"---",
		"# Body",
	}, "\n"))

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if item.Path != path {
		t.Fatalf("path = %q, want %q", item.Path, path)
	}
	if item.IsScoped {
		t.Fatal("indented opener should not create scope")
	}
	if len(item.ScopedPaths) != 0 {
		t.Fatalf("scoped paths = %v, want none", item.ScopedPaths)
	}
}

func TestScanRules_MixedTypePathsDegradeSafely(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	path := filepath.Join(project, ".claude", "rules", "mixed.md")
	mustWriteFile(t, path, strings.Join([]string{
		"---",
		"paths: [src/**, 123]",
		"---",
		"# Body",
	}, "\n"))

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for mixed-type paths frontmatter")
	}
	item := items[0]
	if item.IsScoped {
		t.Fatal("mixed-type paths should not create scope")
	}
	if len(item.ScopedPaths) != 0 {
		t.Fatalf("scoped paths = %v, want none", item.ScopedPaths)
	}
}

func TestScanRules_SkipsNonTextRulesDirectoryFile(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	path := filepath.Join(project, ".claude", "rules", "binary.bin")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for non-text rules directory file")
	}
}

func TestScanRules_SkipsBinaryLikeRulesDirectoryFile(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	path := filepath.Join(project, ".claude", "rules", "artifact.pdf")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	content := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n%%EOF")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for binary-like rules directory file")
	}
}

func TestScanRules_SkipsOversizedRulesDirectoryFile(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	path := filepath.Join(project, ".claude", "rules", "oversized.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("a"), maxRuleFileSize+1), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for oversized rules directory file")
	}
}

func TestScanRules_ReadsSymlinkedRulesDirectoryFile(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	target := filepath.Join(tmp, "outside.md")
	mustWriteFile(t, target, "# Outside content")

	link := filepath.Join(project, ".claude", "rules", "linked.md")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", link, err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
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
	if item.Content != "# Outside content" {
		t.Fatalf("content = %q, want %q", item.Content, "# Outside content")
	}
	if item.Scope != ScopeProject {
		t.Fatalf("scope = %q, want project", item.Scope)
	}
	if item.SourceTool != "claude" {
		t.Fatalf("sourceTool = %q, want claude", item.SourceTool)
	}
}

func TestScanRules_ParsesCRLFScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	path := filepath.Join(project, ".claude", "rules", "crlf.md")
	mustWriteFile(t, path, strings.Join([]string{
		"---",
		"paths:",
		"  - src/**",
		"---",
		"# Body",
	}, "\r\n"))

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if !item.IsScoped {
		t.Fatal("expected CRLF rule to be scoped")
	}
	if len(item.ScopedPaths) != 1 || item.ScopedPaths[0] != "src/**" {
		t.Fatalf("scoped paths = %v, want [src/**]", item.ScopedPaths)
	}
}

func TestScanRules_ParsesLongFrontmatterLine(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	longPath := strings.Repeat("a", 70*1024)
	path := filepath.Join(project, ".claude", "rules", "long.md")
	mustWriteFile(t, path, strings.Join([]string{
		"---",
		"paths:",
		"  - " + longPath,
		"---",
		"# Body",
	}, "\n"))

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if !item.IsScoped {
		t.Fatal("expected long frontmatter rule to be scoped")
	}
	if len(item.ScopedPaths) != 1 || item.ScopedPaths[0] != longPath {
		t.Fatalf("scoped paths = %v, want [%s]", item.ScopedPaths, longPath)
	}
}

func TestScanRules_SkipsNonRegularRulesDirectoryEntry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix domain sockets are not supported on windows")
	}

	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	path := filepath.Join(project, ".claude", "rules", "socket.sock")
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

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for non-regular rules directory entry")
	}
}

func TestScanRules_ReadsSymlinkedRulesDirectoryRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior is platform-dependent on windows")
	}

	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")
	externalRules := filepath.Join(tmp, "external-rules")

	mustWriteFile(t, filepath.Join(externalRules, "leak.md"), "# Leaked content")

	rulesRoot := filepath.Join(project, ".claude", "rules")
	if err := os.MkdirAll(filepath.Dir(rulesRoot), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rulesRoot, err)
	}
	if err := os.Symlink(externalRules, rulesRoot); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item from symlinked rules root, got %d", len(items))
	}
	item := items[0]
	wantPath := filepath.Join(project, ".claude", "rules", "leak.md")
	if item.Path != wantPath {
		t.Fatalf("path = %q, want logical path %q", item.Path, wantPath)
	}
	if item.Content != "# Leaked content" {
		t.Fatalf("content = %q, want %q", item.Content, "# Leaked content")
	}
	if item.Scope != ScopeProject {
		t.Fatalf("scope = %q, want project", item.Scope)
	}
}

func TestScanRules_DedupesOverlappingHomeAndProjectRoots(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "workspace")

	mustWriteFile(t, filepath.Join(home, ".claude", "CLAUDE.md"), "# Shared Claude")

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(home)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.Path != filepath.Join(home, ".claude", "CLAUDE.md") {
		t.Fatalf("path = %q, want shared path", item.Path)
	}
	if item.Scope != ScopeProject {
		t.Fatalf("scope = %q, want project scope to win", item.Scope)
	}
}

func TestScanRules_HomeAliasRootStillSkipsUserDuplicates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior is platform-dependent on windows")
	}

	tmp := t.TempDir()
	home := filepath.Join(tmp, "workspace")
	alias := filepath.Join(tmp, "workspace-alias")

	mustWriteFile(t, filepath.Join(home, ".claude", "CLAUDE.md"), "# Shared Claude")
	mustWriteFile(t, filepath.Join(home, ".claude", "rules", "shared.md"), "# Shared Rule")

	if err := os.Symlink(home, alias); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	t.Setenv("HOME", home)

	items, warnings, err := ScanRules(alias)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 project items from alias root, got %d", len(items))
	}
	for _, item := range items {
		if item.Scope != ScopeProject {
			t.Fatalf("scope = %q, want project for %s", item.Scope, item.Path)
		}
		if !strings.HasPrefix(item.Path, alias) {
			t.Fatalf("path = %q, want alias-root path", item.Path)
		}
	}
}

func TestScanRules_DirectKnownPathFIFOReturnsPromptly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fifo behavior is platform-dependent on windows")
	}

	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	fifoPath := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(fifoPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", fifoPath, err)
	}
	if err := createTestFIFO(fifoPath, 0o644); err != nil {
		t.Skipf("unable to create fifo: %v", err)
	}

	t.Setenv("HOME", home)

	type result struct {
		items    []RuleItem
		warnings []string
		err      error
	}
	done := make(chan result, 1)
	go func() {
		items, warnings, err := ScanRules("")
		done <- result{items: items, warnings: warnings, err: err}
	}()

	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("ScanRules() error = %v", res.err)
		}
		if len(res.items) != 0 {
			t.Fatalf("expected 0 items, got %d", len(res.items))
		}
		if len(res.warnings) == 0 {
			t.Fatal("expected warning for fifo rule file")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ScanRules() hung on fifo rule file")
	}
}

func TestScanRules_CollectibleMetadata(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")

	mustWriteFile(t, filepath.Join(project, "AGENTS.md"), "# Project Codex")
	mustWriteFile(t, filepath.Join(project, ".claude", "rules", "backend.md"), "---\npaths:\n  - src/**\n---\n# Backend")

	t.Setenv("HOME", home)

	first, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	second, warnings, err := ScanRules(project)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	firstByPath := make(map[string]RuleItem, len(first))
	for _, item := range first {
		firstByPath[item.Path] = item
		if strings.TrimSpace(item.ID) == "" {
			t.Fatalf("id for %s should be non-empty", item.Path)
		}
		if !item.Collectible {
			t.Fatalf("collectible for %s = false, want true", item.Path)
		}
		if item.CollectReason != "" {
			t.Fatalf("collectReason for %s = %q, want empty", item.Path, item.CollectReason)
		}
	}

	for _, item := range second {
		before, ok := firstByPath[item.Path]
		if !ok {
			t.Fatalf("path %s missing from first scan", item.Path)
		}
		if before.ID != item.ID {
			t.Fatalf("id for %s changed across scans: %q != %q", item.Path, before.ID, item.ID)
		}
	}
}
