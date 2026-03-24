package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"skillshare/internal/config"
)

type extrasInitPhase int

const (
	extrasPhaseNameInput     extrasInitPhase = iota
	extrasPhaseSourceInput                   // ask for custom source directory
	extrasPhaseTargetInput                   // ask for target path
	extrasPhaseModeSelect                    // choose sync mode
	extrasPhaseFlattenToggle                 // flatten files into target root?
	extrasPhaseAddMore                       // add another target?
	extrasPhaseConfirm                       // show summary and confirm
)

type extrasInitTarget struct {
	path    string
	mode    string
	flatten bool
}

type extrasInitTUIModel struct {
	phase       extrasInitPhase
	name        string
	sourceValue string
	targets     []extrasInitTarget
	currMode    int // cursor index into syncModes

	textInput   textinput.Model
	sourceInput textinput.Model
	done        bool
	cancelled   bool
	err         error
}

var syncModes = config.ExtraSyncModes

func newExtrasInitTUIModel() extrasInitTUIModel {
	ti := textinput.New()
	ti.Placeholder = "rules"
	ti.Focus()
	ti.PromptStyle = tc.Cyan
	ti.Cursor.Style = tc.Cyan

	si := textinput.New()
	si.Placeholder = "Leave empty to use default"
	si.CharLimit = 256
	si.PromptStyle = tc.Cyan
	si.Cursor.Style = tc.Cyan

	return extrasInitTUIModel{
		phase:       extrasPhaseNameInput,
		textInput:   ti,
		sourceInput: si,
	}
}

func (m extrasInitTUIModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m extrasInitTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "esc":
			// esc on first phase cancels; on later phases go back one step
			switch m.phase {
			case extrasPhaseNameInput:
				m.cancelled = true
				return m, tea.Quit
			case extrasPhaseSourceInput:
				m.phase = extrasPhaseNameInput
				m.textInput.SetValue(m.name)
				m.textInput.Placeholder = "rules"
				return m, nil
			case extrasPhaseTargetInput:
				if len(m.targets) == 0 {
					m.phase = extrasPhaseSourceInput
					m.sourceInput.SetValue(m.sourceValue)
					m.sourceInput.Focus()
					return m, nil
				}
				m.phase = extrasPhaseAddMore
				return m, nil
			case extrasPhaseModeSelect:
				m.targets = m.targets[:len(m.targets)-1] // remove the pending target
				m.phase = extrasPhaseTargetInput
				m.textInput.SetValue("")
				m.textInput.Placeholder = targetPlaceholder(len(m.targets))
				return m, nil
			case extrasPhaseFlattenToggle:
				m.phase = extrasPhaseModeSelect
				return m, nil
			case extrasPhaseAddMore:
				if m.targets[len(m.targets)-1].mode == "symlink" {
					m.phase = extrasPhaseModeSelect
				} else {
					m.phase = extrasPhaseFlattenToggle
				}
				return m, nil
			case extrasPhaseConfirm:
				m.phase = extrasPhaseAddMore
				return m, nil
			}
		}

		switch m.phase {
		case extrasPhaseNameInput:
			switch msg.String() {
			case "enter":
				name := strings.TrimSpace(m.textInput.Value())
				if name == "" {
					return m, nil
				}
				if err := config.ValidateExtraName(name); err != nil {
					m.err = err
					return m, nil
				}
				m.name = name
				m.err = nil
				m.phase = extrasPhaseSourceInput
				m.sourceInput.Focus()
				return m, nil
			}

		case extrasPhaseSourceInput:
			switch msg.String() {
			case "enter":
				m.sourceValue = strings.TrimSpace(m.sourceInput.Value())
				m.phase = extrasPhaseTargetInput
				m.textInput.SetValue("")
				m.textInput.Placeholder = targetPlaceholder(0)
				return m, nil
			}

		case extrasPhaseTargetInput:
			switch msg.String() {
			case "enter":
				path := strings.TrimSpace(m.textInput.Value())
				if path == "" {
					return m, nil
				}
				m.targets = append(m.targets, extrasInitTarget{path: path})
				m.currMode = 0
				m.phase = extrasPhaseModeSelect
				return m, nil
			}

		case extrasPhaseModeSelect:
			switch msg.String() {
			case "up", "k":
				if m.currMode > 0 {
					m.currMode--
				}
				return m, nil
			case "down", "j":
				if m.currMode < len(syncModes)-1 {
					m.currMode++
				}
				return m, nil
			case "enter", " ":
				m.targets[len(m.targets)-1].mode = syncModes[m.currMode]
				if syncModes[m.currMode] == "symlink" {
					m.phase = extrasPhaseAddMore // skip flatten for symlink
				} else {
					m.phase = extrasPhaseFlattenToggle
				}
				return m, nil
			}
			return m, nil

		case extrasPhaseFlattenToggle:
			switch msg.String() {
			case "y", "Y":
				m.targets[len(m.targets)-1].flatten = true
				m.phase = extrasPhaseAddMore
				return m, nil
			case "n", "N", "enter":
				m.phase = extrasPhaseAddMore
				return m, nil
			}
			return m, nil

		case extrasPhaseAddMore:
			switch msg.String() {
			case "y", "Y":
				m.phase = extrasPhaseTargetInput
				m.textInput.SetValue("")
				m.textInput.Placeholder = targetPlaceholder(len(m.targets))
				return m, nil
			case "n", "N", "enter":
				m.phase = extrasPhaseConfirm
				return m, nil
			}
			return m, nil

		case extrasPhaseConfirm:
			switch msg.String() {
			case "y", "Y", "enter":
				m.done = true
				return m, tea.Quit
			case "n", "N":
				m.cancelled = true
				return m, tea.Quit
			}
			return m, nil
		}
	}

	// Delegate to textinput for typing phases
	switch m.phase {
	case extrasPhaseNameInput, extrasPhaseTargetInput:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	case extrasPhaseSourceInput:
		var cmd tea.Cmd
		m.sourceInput, cmd = m.sourceInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m extrasInitTUIModel) View() string {
	var b strings.Builder

	b.WriteString(tc.Title.Render("Extras Init"))
	b.WriteString("\n\n")

	switch m.phase {
	case extrasPhaseNameInput:
		b.WriteString(tc.Cyan.Render("Extra name: "))
		b.WriteString(m.textInput.View())
		if m.err != nil {
			b.WriteString("\n" + tc.Red.Render(m.err.Error()))
		}
		b.WriteString("\n\n")
		b.WriteString(tc.Help.Render("enter confirm  esc cancel"))

	case extrasPhaseSourceInput:
		b.WriteString(tc.Dim.Render(fmt.Sprintf("Name: %s", m.name)))
		b.WriteString("\n\n")
		b.WriteString(tc.Cyan.Render("Source directory (optional): "))
		b.WriteString(m.sourceInput.View())
		b.WriteString("\n\n")
		b.WriteString(tc.Help.Render("enter to skip (use default)  esc back"))

	case extrasPhaseTargetInput:
		b.WriteString(tc.Dim.Render(fmt.Sprintf("Name: %s", m.name)))
		if m.sourceValue != "" {
			b.WriteString("\n")
			b.WriteString(tc.Dim.Render(fmt.Sprintf("Source: %s", m.sourceValue)))
		}
		if len(m.targets) > 0 {
			b.WriteString("\n")
			for _, t := range m.targets {
				modeLabel := t.mode
				if t.flatten {
					modeLabel += ", flatten"
				}
				b.WriteString(tc.Dim.Render(fmt.Sprintf("  → %s (%s)", t.path, modeLabel)))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(tc.Cyan.Render(fmt.Sprintf("Target #%d path: ", len(m.targets)+1)))
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(tc.Help.Render("enter confirm  esc back"))

	case extrasPhaseModeSelect:
		b.WriteString(tc.Dim.Render(fmt.Sprintf("Name: %s", m.name)))
		b.WriteString("\n")
		b.WriteString(tc.Dim.Render(fmt.Sprintf("Target: %s", m.targets[len(m.targets)-1].path)))
		b.WriteString("\n\n")
		b.WriteString(tc.Cyan.Render("Sync mode:"))
		b.WriteString("\n")
		for i, mode := range syncModes {
			cursor := "  "
			if i == m.currMode {
				cursor = "▸ "
			}
			var desc string
			switch mode {
			case "merge":
				desc = " (per-file symlinks, default)"
			case "copy":
				desc = " (file copies)"
			case "symlink":
				desc = " (directory symlink)"
			}
			if i == m.currMode {
				b.WriteString(tc.Cyan.Render(cursor+mode) + tc.Dim.Render(desc))
			} else {
				b.WriteString(tc.Dim.Render(cursor + mode + desc))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(tc.Help.Render("↑↓/jk navigate  enter/space select  esc back"))

	case extrasPhaseFlattenToggle:
		b.WriteString(tc.Dim.Render(fmt.Sprintf("Name: %s", m.name)))
		b.WriteString("\n")
		lastTarget := m.targets[len(m.targets)-1]
		b.WriteString(tc.Dim.Render(fmt.Sprintf("Target: %s (%s)", lastTarget.path, lastTarget.mode)))
		b.WriteString("\n\n")
		b.WriteString(tc.Cyan.Render("Flatten files into target root? (y/N) "))
		b.WriteString("\n\n")
		b.WriteString(tc.Help.Render("y yes  n/enter no  esc back"))

	case extrasPhaseAddMore:
		b.WriteString(tc.Dim.Render(fmt.Sprintf("Name: %s", m.name)))
		b.WriteString("\n")
		for _, t := range m.targets {
			modeLabel := t.mode
			if t.flatten {
				modeLabel += ", flatten"
			}
			b.WriteString(tc.Dim.Render(fmt.Sprintf("  → %s (%s)", t.path, modeLabel)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(tc.Cyan.Render("Add another target? (y/N) "))
		b.WriteString("\n\n")
		b.WriteString(tc.Help.Render("y yes  n/enter no  esc back"))

	case extrasPhaseConfirm:
		b.WriteString(tc.Cyan.Render("Summary:"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Name: %s\n", m.name))
		if m.sourceValue != "" {
			b.WriteString(fmt.Sprintf("  Source: %s\n", m.sourceValue))
		}
		for _, t := range m.targets {
			modeLabel := t.mode
			if t.flatten {
				modeLabel += ", flatten"
			}
			b.WriteString(fmt.Sprintf("  → %s (%s)\n", t.path, modeLabel))
		}
		b.WriteString("\n")
		b.WriteString(tc.Cyan.Render("Create this extra? (Y/n) "))
		b.WriteString("\n\n")
		b.WriteString(tc.Help.Render("y/enter confirm  n cancel"))
	}

	return b.String()
}

// targetPlaceholder returns a contextual placeholder for the target input.
func targetPlaceholder(n int) string {
	placeholders := []string{
		"~/.claude/rules",
		"~/.cursor/rules",
		"~/.codex/rules",
	}
	if n < len(placeholders) {
		return placeholders[n]
	}
	return "~/.<tool>/rules"
}

// cmdExtrasInitTUI launches the interactive wizard when no arguments are provided.
func cmdExtrasInitTUI(mode runMode, cwd string) error {
	m := newExtrasInitTUIModel()
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	result, ok := finalModel.(extrasInitTUIModel)
	if !ok || result.cancelled || !result.done {
		return nil
	}

	// Collect targets and the first non-empty mode (mode applies globally)
	var targetPaths []string
	syncMode := ""
	flatten := false
	for _, t := range result.targets {
		targetPaths = append(targetPaths, t.path)
		if syncMode == "" && t.mode != "" && t.mode != "merge" {
			syncMode = t.mode
		}
		if t.flatten {
			flatten = true
		}
	}

	start := time.Now()
	if mode == modeProject {
		return extrasInitProject(cwd, result.name, targetPaths, syncMode, flatten, false, start)
	}
	return extrasInitGlobal(result.name, targetPaths, syncMode, result.sourceValue, flatten, false, start)
}
