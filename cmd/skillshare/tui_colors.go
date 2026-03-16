package main

import (
	"strings"

	"skillshare/internal/ui"

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
	Dim      lipgloss.Style // secondary info, labels, descriptions — SGR dim
	Faint    lipgloss.Style // decorative chrome, borders, help — SGR dim
	Cyan     lipgloss.Style // emphasis, targets — cyan
	Green    lipgloss.Style // ok, passed
	Yellow   lipgloss.Style // warning
	Red      lipgloss.Style // error, blocked

	// Detail panel
	Label     lipgloss.Style // row labels (width 14)
	Value     lipgloss.Style // default foreground
	File      lipgloss.Style // file names — dim
	Target    lipgloss.Style // target names — cyan
	Separator lipgloss.Style // horizontal rules — faint
	Border    lipgloss.Style // panel borders — faint

	// Filter & help
	Filter lipgloss.Style // filter prompt/cursor — cyan
	Help   lipgloss.Style // help bar — faint, left margin

	// List browser chrome
	ListRow               lipgloss.Style
	ListMeta              lipgloss.Style
	ListRowSelected       lipgloss.Style
	ListMetaSelected      lipgloss.Style
	ListRowPrefix         lipgloss.Style
	ListRowPrefixSelected lipgloss.Style
	BadgeLocal            lipgloss.Style
	BadgeRemote           lipgloss.Style

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
	Dim:      lipgloss.NewStyle().Faint(true),
	Faint:    lipgloss.NewStyle().Faint(true),
	Cyan:     lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	Green:    lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
	Yellow:   lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
	Red:      lipgloss.NewStyle().Foreground(lipgloss.Color("1")),

	Label:     lipgloss.NewStyle().Faint(true).Width(14),
	Value:     lipgloss.NewStyle(),
	File:      lipgloss.NewStyle().Faint(true),
	Target:    lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	Separator: lipgloss.NewStyle().Faint(true),
	Border:    lipgloss.NewStyle().Faint(true),

	Filter: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	Help:   lipgloss.NewStyle().MarginLeft(2).Faint(true),

	ListRow:               lipgloss.NewStyle().PaddingLeft(1),
	ListMeta:              lipgloss.NewStyle().PaddingLeft(1).Faint(true),
	ListRowSelected:       lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("237")).Bold(true),
	ListMetaSelected:      lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("250")).Background(lipgloss.Color("237")),
	ListRowPrefix:         lipgloss.NewStyle().Foreground(lipgloss.Color("235")),
	ListRowPrefixSelected: lipgloss.NewStyle().Foreground(tuiBrandYellow),
	BadgeLocal: lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("237")).
		Padding(0, 1),
	BadgeRemote: lipgloss.NewStyle().Foreground(lipgloss.Color("228")).Background(lipgloss.Color("239")).
		Padding(0, 1),

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
