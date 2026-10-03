package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"

	"skillshare/internal/theme"
)

// contentGlamourStyle must pick the glamour base style that matches the
// resolved terminal theme. Otherwise body prose rendered on a light
// terminal uses the dark-base near-white color ("252") and disappears.
func TestContentGlamourStyle_MatchesTheme(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	cases := []struct {
		mode     string
		wantBody string // expected Document.StylePrimitive.Color
	}{
		{"light", "234"}, // readable dark gray on light terminals
		{"dark", "252"},  // readable near-white on dark terminals
	}

	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			t.Setenv("SKILLSHARE_THEME", tc.mode)
			theme.Reset()
			defer theme.Reset()

			s := contentGlamourStyle()
			if s.Document.StylePrimitive.Color == nil {
				t.Fatal("Document.Color is nil")
			}
			if got := *s.Document.StylePrimitive.Color; got != tc.wantBody {
				t.Errorf("body Color = %q, want %q", got, tc.wantBody)
			}

			// Shared customizations must still apply regardless of base.
			if s.Document.Margin == nil || *s.Document.Margin != 0 {
				t.Errorf("Document.Margin = %v, want 0", s.Document.Margin)
			}
			if s.H1.StylePrimitive.BackgroundColor != nil {
				t.Errorf("H1.BackgroundColor = %v, want nil", s.H1.StylePrimitive.BackgroundColor)
			}
			if s.H1.StylePrimitive.Color == nil || *s.H1.StylePrimitive.Color != "6" {
				t.Errorf("H1.Color = %v, want \"6\"", s.H1.StylePrimitive.Color)
			}
		})
	}
}

func TestLoadContentForSkill_AgentLeavesOutFrontMatter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "reviewer.md"), []byte("---\nname: reviewer\ndescription: Reviews code changes\n---\nReview the diff.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := listTUIModel{agentsSourcePath: dir, termWidth: 100, termHeight: 30}
	loadContentForSkill(&m, skillEntry{Name: "reviewer", Kind: "agent", RelPath: "reviewer.md"})

	got := xansi.Strip(m.contentText)
	if strings.Contains(got, "name: reviewer") || !strings.Contains(got, "Review the diff.") {
		t.Fatalf("agent content should show the body without front matter, got %q", got)
	}
}

func TestHandleContentMouse_WheelScrollsAnAgentAnywhere(t *testing.T) {
	m := listTUIModel{contentKind: "agent", termWidth: 100, termHeight: 10, contentText: strings.Repeat("line\n", 40)}
	next, _ := m.handleContentMouse(tea.MouseMsg{X: 2, Button: tea.MouseButtonWheelDown})

	if got := next.(listTUIModel).contentScroll; got != 1 {
		t.Fatalf("contentScroll = %d, want 1: an agent has no tree, so the wheel scrolls its text", got)
	}
}
