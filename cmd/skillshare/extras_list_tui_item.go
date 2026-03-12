package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// extraTUIItem wraps extrasListEntry to implement bubbles/list.Item.
type extraTUIItem struct {
	entry extrasListEntry
}

func (i extraTUIItem) FilterValue() string { return i.entry.Name }
func (i extraTUIItem) Title() string       { return i.entry.Name }
func (i extraTUIItem) Description() string { return "" }

// extrasListDelegate renders a compact single-line row for the extras list TUI.
type extrasListDelegate struct{}

func (extrasListDelegate) Height() int                             { return 1 }
func (extrasListDelegate) Spacing() int                           { return 0 }
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
	renderExtrasRow(w, extra, width, selected)
}

// renderExtrasRow renders: ▌ name    Nf
// Color bar reflects aggregate sync status across all targets.
func renderExtrasRow(w io.Writer, item extraTUIItem, width int, selected bool) {
	e := item.entry

	// Color bar reflects aggregate sync status (cyan=synced, yellow=drift, red=not synced, gray=no source)
	prefixStyle := lipgloss.NewStyle().Foreground(extrasStatusColor(e))
	bodyStyle := tc.ListRow
	if selected {
		prefixStyle = tc.ListRowPrefixSelected
		bodyStyle = tc.ListRowSelected
	}

	// Build line: name + right-aligned file count
	fileLabel := fmt.Sprintf("%df", e.FileCount)
	if !e.SourceExists {
		fileLabel = "—"
	}

	bodyWidth := width - lipgloss.Width(prefixStyle.Render("▌"))
	if bodyWidth < 10 {
		bodyWidth = 10
	}
	textWidth := bodyWidth - bodyStyle.GetPaddingLeft() - bodyStyle.GetPaddingRight()
	if textWidth < 8 {
		textWidth = 8
	}

	name := e.Name
	badge := extrasStatusBadge(e)

	// Compose: name + gap + badge + fileLabel
	nameWidth := textWidth - len(fileLabel) - 1
	if badge != "" {
		nameWidth -= lipgloss.Width(badge) + 1
	}
	if nameWidth < 4 {
		nameWidth = 4
	}
	if len(name) > nameWidth {
		name = name[:nameWidth-1] + "…"
	}

	gap := textWidth - len(name) - len(fileLabel)
	if badge != "" {
		gap -= lipgloss.Width(badge) + 1
	}
	if gap < 1 {
		gap = 1
	}

	line := name + strings.Repeat(" ", gap)
	if badge != "" {
		line += badge + " "
	}
	line += tc.Dim.Render(fileLabel)

	fmt.Fprint(w, lipgloss.JoinHorizontal(lipgloss.Top,
		prefixStyle.Render("▌"),
		bodyStyle.Width(bodyWidth).MaxWidth(bodyWidth).Render(line),
	))
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
		return tc.Yellow.Render("~")
	}
	return tc.Red.Render("✗")
}

// extrasStatusColor returns the prefix bar color based on aggregate sync status.
func extrasStatusColor(e extrasListEntry) lipgloss.Color {
	if !e.SourceExists {
		return lipgloss.Color("8") // gray
	}
	if len(e.Targets) == 0 {
		return lipgloss.Color("8")
	}

	allSynced := true
	hasDrift := false
	for _, t := range e.Targets {
		if t.Status != "synced" {
			allSynced = false
		}
		if t.Status == "drift" {
			hasDrift = true
		}
	}

	if allSynced {
		return lipgloss.Color("6") // cyan
	}
	if hasDrift {
		return lipgloss.Color("3") // yellow
	}
	return lipgloss.Color("1") // red
}
