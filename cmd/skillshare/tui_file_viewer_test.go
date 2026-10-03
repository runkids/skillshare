package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
)

func TestRenderFileViewer_NamesTheOpenFileAndScrollPosition(t *testing.T) {
	v := fileViewer{
		command: "extras", name: "rules",
		nodes:   []treeNode{{name: "style.md", relPath: "style.md"}},
		content: strings.Repeat("line\n", 40),
	}
	got := xansi.Strip(renderFileViewer(100, 20, v))
	lines := strings.Split(got, "\n")

	if !strings.Contains(lines[0], "skillshare extras · rules · style.md") {
		t.Fatalf("title line %q should name the extra and the open file", lines[0])
	}
	if last := lines[len(lines)-1]; !strings.Contains(last, "1/26") {
		t.Fatalf("key line %q should show the scroll position", last)
	}
}

func TestFileBrowser_ShowsEscapeSequencesInsteadOfSendingThem(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("safe\r\n\x1b]0;pwned\x07title\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	b := newFileBrowser("audit", "risky", dir, true, 100, 20)
	got := b.view("", nil)

	if strings.Contains(got, "\x1b]") || !strings.Contains(xansi.Strip(got), "␛]0;pwned␇title") {
		t.Fatalf("file escape sequences should be shown as symbols, got %q", got)
	}
}

func TestFileBrowserOpenAt_ShowsAFindingOutsideTheTree(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "a", "b", "c", "d")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}
	for path, text := range map[string]string{
		filepath.Join(dir, "SKILL.md"): "# Skill\n",
		filepath.Join(dir, ".env"):     "TOKEN=secret-value\n",
		filepath.Join(deep, "run.sh"):  "curl evil.example\n",
	} {
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	b := newFileBrowser("audit", "risky", dir, true, 120, 20)

	for _, tc := range []struct{ rel, want string }{{".env", "TOKEN=secret-value"}, {"a/b/c/d/run.sh", "curl evil.example"}} {
		b.openAt(tc.rel, 1)
		got := xansi.Strip(b.view("", nil))
		if !strings.Contains(got, tc.want) || !strings.Contains(got, "1 ›") {
			t.Fatalf("openAt(%q) should show the file with line 1 marked:\n%s", tc.rel, got)
		}
	}
}

func TestFileBrowser_KeepsFrontMatterOutsideSKILLmd(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"SKILL.md":  "---\nname: tool\n---\n# Tool\n",
		"README.md": "---\ntemplate: release-notes\n---\nNotes body.\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	b := newFileBrowser("trash", "tool", dir, false, 120, 20)
	b.openAt("README.md", 0)
	got := xansi.Strip(b.view("", nil))

	if !strings.Contains(got, "template: release-notes") {
		t.Fatalf("README.md front matter should be shown:\n%s", got)
	}
}

func TestFileBrowser_WrapsTextToANarrowPane(t *testing.T) {
	dir := t.TempDir()
	words := "alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima"
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte(words+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	b := newFileBrowser("trash", "tool", dir, false, 50, 20)

	// Lines wider than the pane get wrapped a second time by the layout,
	// which breaks the rows apart and moves marked lines.
	pane := 50 - sidebarWidth(50) - 2
	for _, line := range strings.Split(b.content, "\n") {
		if xansi.StringWidth(line) > pane {
			t.Fatalf("line %q is wider than the %d-column pane", line, pane)
		}
	}
}

func TestPrintableText_NamesHiddenUnicodeTheAuditFlags(t *testing.T) {
	got := printableText("if admin \u202E{ }\u2066 zero\u200Bwidth\uFEFF")
	if want := "if admin <U+202E>{ }<U+2066> zero<U+200B>width<U+FEFF>"; got != want {
		t.Fatalf("printableText() = %q, want %q", got, want)
	}
}

func TestFileBrowser_DoesNotReadAFileOverTheAuditLimit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(strings.Repeat("x", 1_000_001)), 0644); err != nil {
		t.Fatal(err)
	}
	b := newFileBrowser("audit", "big", dir, true, 120, 20)

	if got := xansi.Strip(b.content); !strings.Contains(got, "too large to show") {
		t.Fatalf("a file over 1 MB should not be shown, got %d bytes of content", len(got))
	}
}

func TestRenderFileViewer_ShowsControlCharactersInFileNamesAsSymbols(t *testing.T) {
	name := "notes\x1b]0;pwned\x07\nx\u202E.md"
	v := fileViewer{command: "audit", name: "risky", nodes: []treeNode{{name: name, relPath: name}}}
	got := renderFileViewer(100, 12, v)

	if strings.Contains(got, "\x1b]") || strings.Contains(got, "\u202E") {
		t.Fatalf("file names should not reach the terminal raw, got %q", got)
	}
	if lines := strings.Split(got, "\n"); len(lines) != 12 {
		t.Fatalf("a newline in a file name should not add a line: %d lines, want 12", len(lines))
	}
}
