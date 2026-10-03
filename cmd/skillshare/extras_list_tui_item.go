package main

import (
	"io"

	"skillshare/internal/theme"

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
func (extrasListDelegate) Spacing() int                            { return 0 }
func (extrasListDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (extrasListDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	extra, ok := item.(extraTUIItem)
	if !ok {
		return
	}
	renderPrefixRow(w, alignRow(extra.entry.Name, extrasStatusBadge(extra.entry), m.Width()-rowIndent), m.Width(), index == m.Index())
}

// extrasStatusBadge returns a short colored status indicator based on aggregate target status.
func extrasStatusBadge(e extrasListEntry) string {
	if !e.SourceExists {
		return theme.Dim().Render("no source")
	}
	if len(e.Targets) == 0 {
		return theme.Dim().Render("no targets")
	}

	synced := 0
	drift := 0
	for _, t := range e.Targets {
		switch t.Status {
		case "synced":
			synced++
		case "drift", "invalid mode":
			drift++
		}
	}

	total := len(e.Targets)
	if synced == total {
		return theme.Success().Render("✓")
	}
	if drift > 0 {
		return theme.Warning().Render("△")
	}
	return theme.Danger().Render("✗")
}
