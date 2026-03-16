package main

import (
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// extraTUIItem wraps extrasListEntry to implement bubbles/list.Item.
type extraTUIItem struct {
	entry extrasListEntry
}

func (i extraTUIItem) FilterValue() string { return i.entry.Name }
func (i extraTUIItem) Title() string       { return i.entry.Name }
func (i extraTUIItem) Description() string { return "" }

// extrasListDelegate renders a compact single-line row for the extras list TUI.
// Uses the shared renderPrefixRow for consistent styling with list TUI.
type extrasListDelegate struct{}

func (extrasListDelegate) Height() int                             { return 1 }
func (extrasListDelegate) Spacing() int                            { return 1 }
func (extrasListDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (extrasListDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	extra, ok := item.(extraTUIItem)
	if !ok {
		return
	}
	width := m.Width()
	if width <= 0 {
		width = 40
	}
	selected := index == m.Index()
	line := extra.entry.Name + "  " + extrasStatusBadge(extra.entry)
	renderPrefixRow(w, line, width, selected)
}

// extrasStatusBadge returns a short colored status indicator based on aggregate target status.
func extrasStatusBadge(e extrasListEntry) string {
	if !e.SourceExists {
		return tc.Dim.Render("no source")
	}
	if len(e.Targets) == 0 {
		return tc.Dim.Render("no targets")
	}

	synced := 0
	drift := 0
	for _, t := range e.Targets {
		switch t.Status {
		case "synced":
			synced++
		case "drift":
			drift++
		}
	}

	total := len(e.Targets)
	if synced == total {
		return tc.Green.Render("✓")
	}
	if drift > 0 {
		return tc.Yellow.Render("△")
	}
	return tc.Red.Render("✗")
}
