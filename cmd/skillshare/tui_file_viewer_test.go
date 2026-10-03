package main

import (
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
