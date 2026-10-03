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
