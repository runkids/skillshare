package main

import (
	"strings"

	"skillshare/internal/ui"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

// tuiBrandYellow is the logo yellow used for active/selected item borders across all TUIs.
const tuiBrandYellow = lipgloss.Color("#D4D93C")

// tc centralizes shared color styles used across all TUI views.
// Domain-specific structs (ac, lc) reference tc for base colors.
var tc = struct {
	BrandYellow lipgloss.Color

	// Semantic
	Title    lipgloss.Style // section headings — bold cyan
	Emphasis lipgloss.Style // primary values, bright text — bright white (15)
	Dim      lipgloss.Style // secondary info, labels, descriptions — gray
	Faint    lipgloss.Style // decorative chrome, borders, help — darker gray
	Cyan     lipgloss.Style // emphasis, targets — cyan
	Green    lipgloss.Style // ok, passed
	Yellow   lipgloss.Style // warning
	Red      lipgloss.Style // error, blocked
	Magenta  lipgloss.Style // category accent (e.g. discovery cache)

	// Detail panel
	Label     lipgloss.Style // row labels (width 14)
	Value     lipgloss.Style // default foreground
	File      lipgloss.Style // file names — dim gray
	Target    lipgloss.Style // target names — cyan
	Separator lipgloss.Style // horizontal rules — faint
	Border    lipgloss.Style // panel borders — faint

	// Filter & help
	Filter lipgloss.Style // filter prompt/cursor — cyan
	Help   lipgloss.Style // help bar — faint, left margin

	// Delegate styles — shared by all list TUIs
	NormalTitle   lipgloss.Style
	NormalDesc    lipgloss.Style
	SelectedTitle lipgloss.Style
	SelectedDesc  lipgloss.Style

	// Severity — shared across all TUIs (audit, log, etc.)
	Critical lipgloss.Style // red, bold
	High     lipgloss.Style // orange
	Medium   lipgloss.Style // yellow
	Low      lipgloss.Style // bright blue
	Info     lipgloss.Style // medium gray

	// List chrome
	ListTitle    lipgloss.Style // list title — bold cyan
	SpinnerStyle lipgloss.Style // loading spinner — cyan
}{
	BrandYellow: lipgloss.Color("#D4D93C"),

	Title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")),
	Emphasis: lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
	Dim:      lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	Faint:    lipgloss.NewStyle().Foreground(lipgloss.Color("239")),
	Cyan:     lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	Green:    lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
	Yellow:   lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
	Red:      lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
	Magenta:  lipgloss.NewStyle().Foreground(lipgloss.Color("5")),

	Label:     lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(14),
	Value:     lipgloss.NewStyle(),
	File:      lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	Target:    lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	Separator: lipgloss.NewStyle().Foreground(lipgloss.Color("239")),
	Border:    lipgloss.NewStyle().Foreground(lipgloss.Color("239")),

	Filter: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	Help:   lipgloss.NewStyle().MarginLeft(2).Foreground(lipgloss.Color("239")),

	NormalTitle: lipgloss.NewStyle().PaddingLeft(2),
	NormalDesc:  lipgloss.NewStyle().Foreground(lipgloss.Color("245")).PaddingLeft(2),
	SelectedTitle: lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color("#D4D93C")).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#D4D93C")).PaddingLeft(1),
	SelectedDesc: lipgloss.NewStyle().Foreground(lipgloss.Color("245")).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#D4D93C")).PaddingLeft(1),

	Critical: lipgloss.NewStyle().Foreground(lipgloss.Color(ui.SeverityIDCritical)).Bold(true),
	High:     lipgloss.NewStyle().Foreground(lipgloss.Color(ui.SeverityIDHigh)),
	Medium:   lipgloss.NewStyle().Foreground(lipgloss.Color(ui.SeverityIDMedium)),
	Low:      lipgloss.NewStyle().Foreground(lipgloss.Color(ui.SeverityIDLow)),
	Info:     lipgloss.NewStyle().Foreground(lipgloss.Color(ui.SeverityIDInfo)),

	ListTitle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")),
	SpinnerStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
}

// tcSevStyle returns the severity lipgloss style from the centralized tc config.
func tcSevStyle(severity string) lipgloss.Style {
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		return tc.Critical
	case "HIGH":
		return tc.High
	case "MEDIUM":
		return tc.Medium
	case "LOW":
		return tc.Low
	case "INFO":
		return tc.Info
	default:
		return tc.Dim
	}
}

// riskLabelStyle maps a lowercase risk label to the matching lipgloss style.
func riskLabelStyle(label string) lipgloss.Style {
	switch strings.ToLower(label) {
	case "clean":
		return tc.Green
	case "low":
		return tc.Low
	case "medium":
		return tc.Medium
	case "high":
		return tc.High
	case "critical":
		return tc.Critical
	default:
		return tc.Dim
	}
}

// formatRiskBadgeLipgloss returns a colored risk badge for TUI list items.
func formatRiskBadgeLipgloss(label string) string {
	if label == "" {
		return ""
	}
	return " " + riskLabelStyle(label).Render("["+label+"]")
}

// configureDelegate applies shared delegate styles to a list.DefaultDelegate.
// showDesc toggles description row (2-line items when true, 1-line when false).
func configureDelegate(d *list.DefaultDelegate, showDesc bool) {
	d.ShowDescription = showDesc
	d.SetSpacing(0)
	d.Styles.NormalTitle = tc.NormalTitle
	d.Styles.SelectedTitle = tc.SelectedTitle
	if showDesc {
		d.Styles.NormalDesc = tc.NormalDesc
		d.Styles.SelectedDesc = tc.SelectedDesc
	} else {
		d.SetHeight(1)
	}
}
