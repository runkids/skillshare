package main

import (
	"io"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/targetsummary"
	"skillshare/internal/theme"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// targetTUIItem wraps a target entry for the bubbles/list widget.
type targetTUIItem struct {
	name        string
	target      config.TargetConfig
	displayPath string
	skillSync   string
	// skillSyncText is skillSync as the plain list prints it.
	skillSyncText string
	agentConfig   config.ResourceTargetConfig
	agentSummary  *targetsummary.AgentSummary
	// namingErr is why sync will fail for this target's naming and mode, with the fix.
	namingErr error
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
	mode := sync.EffectiveMode(ti.target.SkillsConfig().Mode)
	if !ti.target.SkillsConfig().IsEnabled() {
		mode = "skills off"
	}
	modeStyle := theme.Dim()
	if ti.namingErr != nil {
		modeStyle, mode = theme.Warning(), "! "+mode
	}
	renderPrefixRow(w, alignRow(ti.name, modeStyle.Render(mode), m.Width()-rowIndent), m.Width(), index == m.Index())
}
