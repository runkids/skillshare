package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TUI color palette — matches skillshare CLI conventions.
var (
	tuiCyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	tuiGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	tuiRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	tuiYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	tuiDim    = lipgloss.NewStyle().Faint(true)
	tuiBold   = lipgloss.NewStyle().Bold(true)
)

// stepDoneMsg is sent when a step finishes execution.
type stepDoneMsg struct {
	index  int
	result StepResult
}

// tuiModel is the bubbletea model for real-time runbook progress.
type tuiModel struct {
	name    string
	steps   []Step
	results []StepResult
	execFn  func(Step) StepResult

	start time.Time
}

// newTUIModel creates a TUI model with injected execution function.
func newTUIModel(name string, steps []Step, execFn func(Step) StepResult) tuiModel {
	results := make([]StepResult, len(steps))
	for i, s := range steps {
		results[i] = StepResult{Step: s, Status: "pending"}
	}
	return tuiModel{
		name:    name,
		steps:   steps,
		results: results,
		execFn:  execFn,
		start: time.Now(),
	}
}

func (m tuiModel) Init() tea.Cmd {
	if len(m.steps) == 0 {
		return tea.Quit
	}
	return m.runStep(0)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case stepDoneMsg:
		m.results[msg.index] = msg.result
		next := msg.index + 1
		if next < len(m.steps) {
			return m, m.runStep(next)
		}
		return m, tea.Quit
	}

	return m, nil
}

func (m tuiModel) View() string {
	var b strings.Builder

	// Header
	b.WriteString(tuiBold.Render("Runbook: "+m.name) + "\n")
	b.WriteString(tuiDim.Render(strings.Repeat("─", 50)) + "\n")

	// Step list
	for _, r := range m.results {
		icon := statusIcon(r.Status)
		title := truncateText(r.Step.Title, 38)
		line := fmt.Sprintf(" %s %2d. %s", icon, r.Step.Number, title)

		if r.Status == StatusPassed || r.Status == StatusFailed {
			line += tuiDim.Render(fmt.Sprintf("  %s", formatDurationMs(r.DurationMs)))
		}
		b.WriteString(line + "\n")

		// Show failure reason
		if r.Status == StatusFailed {
			reason := stepFailReason(r)
			if reason != "" {
				b.WriteString(tuiRed.Render("      "+reason) + "\n")
			}
		}
	}

	// Footer
	b.WriteString(tuiDim.Render(strings.Repeat("─", 50)) + "\n")
	b.WriteString(m.summaryLine() + "\n")

	return b.String()
}

// runStep returns a tea.Cmd that executes step at index i.
func (m tuiModel) runStep(i int) tea.Cmd {
	step := m.steps[i]
	execFn := m.execFn
	return func() tea.Msg {
		result := execFn(step)
		return stepDoneMsg{index: i, result: result}
	}
}

// statusIcon returns the colored icon for a step status.
func statusIcon(status string) string {
	switch status {
	case StatusPassed:
		return tuiGreen.Render("✓")
	case StatusFailed:
		return tuiRed.Render("✗")
	case StatusRunning:
		return tuiCyan.Render("●")
	default: // pending
		return tuiDim.Render("○")
	}
}

// summaryLine returns the footer summary string.
func (m tuiModel) summaryLine() string {
	var passed, failed, running int
	for _, r := range m.results {
		switch r.Status {
		case StatusPassed:
			passed++
		case StatusFailed:
			failed++
		case StatusRunning:
			running++
		}
	}

	elapsed := time.Since(m.start)
	parts := []string{
		fmt.Sprintf("%d/%d passed", passed, len(m.steps)),
	}
	if failed > 0 {
		parts = append(parts, tuiRed.Render(fmt.Sprintf("%d failed", failed)))
	} else {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if running > 0 {
		parts = append(parts, tuiCyan.Render(fmt.Sprintf("%d running", running)))
	}
	parts = append(parts, tuiDim.Render(fmt.Sprintf("%.1fs", elapsed.Seconds())))

	return strings.Join(parts, "  ")
}

