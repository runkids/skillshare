package main

import (
	"fmt"
	"io"

	"skillshare/internal/config"
	"skillshare/internal/sync"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// targetTUIItem wraps a target entry for the bubbles/list widget.
type targetTUIItem struct {
	name   string
	target config.TargetConfig
}

func (i targetTUIItem) FilterValue() string { return i.name }
func (i targetTUIItem) Title() string       { return i.name }
func (i targetTUIItem) Description() string { return "" }

// targetListDelegate renders each target row in the list.
type targetListDelegate struct{}

func (targetListDelegate) Height() int                             { return 1 }
func (targetListDelegate) Spacing() int                            { return 0 }
func (targetListDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (targetListDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ti, ok := item.(targetTUIItem)
	if !ok {
		return
	}
	width := m.Width()
	if width <= 0 {
		width = 40
	}
	mode := sync.EffectiveMode(ti.target.Mode)
	line := fmt.Sprintf("%s  (%s)", ti.name, mode)
	selected := index == m.Index()
	renderPrefixRow(w, line, width, selected)
}
