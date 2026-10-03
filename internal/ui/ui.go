package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"skillshare/internal/theme"

	"github.com/pterm/pterm"
)

// Deprecated: Use theme.ANSI() instead. These raw escape sequences are
// retained for existing callers. They are variables rather than constants so
// DisableColors can blank them when NO_COLOR is set. New code should use
// theme.ANSI().
var (
	Reset      = "\033[0m"
	Red        = "\033[31m"
	Green      = "\033[32m"
	Yellow     = "\033[33m"
	Blue       = "\033[34m"
	Magenta    = "\033[35m"
	Cyan       = "\033[36m"
	Orange     = "\033[38;5;208m"
	Purple     = "\033[38;5;135m"
	BrightRed  = "\033[38;5;9m"
	OrangeAlt  = "\033[38;5;214m"
	BrightBlue = "\033[38;5;12m"
	Gray       = "\033[90m"
	Dim        = "\x1b[0;2m" // SGR dim — works across all terminal themes
	White      = "\033[97m"
)

// Semantic color aliases for consistent theming
var (
	// Primary brand color (yellow - matches logo)
	Primary = Yellow
	// Accent color for interactive elements
	Accent = Cyan
	// Muted color for secondary information
	Muted = Dim
	// Status colors
	StatusSuccess = Green
	StatusError   = Red
	StatusWarning = Yellow
	StatusInfo    = Cyan
)

// Bold variants
var (
	Bold      = "\033[1m"
	BoldReset = "\033[22m"
)

// DisableColors blanks every raw escape variable above (and pterm's colors)
// so output built from them is plain text. main calls it once at startup when
// the resolved theme has NoColor or Plain set, before any output is produced.
func DisableColors() {
	pterm.DisableColor()
	for _, v := range []*string{
		&Reset, &Red, &Green, &Yellow, &Blue, &Magenta, &Cyan, &Orange,
		&Purple, &BrightRed, &OrangeAlt, &BrightBlue, &Gray, &Dim, &White,
		&Primary, &Accent, &Muted,
		&StatusSuccess, &StatusError, &StatusWarning, &StatusInfo,
		&Bold, &BoldReset,
	} {
		*v = ""
	}
}

// Severity color IDs (256-color palette) — single source of truth for both
// ANSI escape codes (SeverityColor) and lipgloss styles (SeverityColorID).
const (
	SeverityIDCritical = "1"   // red
	SeverityIDHigh     = "208" // orange
	SeverityIDMedium   = "3"   // yellow
	SeverityIDLow      = "12"  // bright blue — visible on dark backgrounds
	SeverityIDInfo     = "244" // medium gray — informational, lowest priority
)

// SeverityColor returns the ANSI color code for a given audit severity level.
func SeverityColor(severity string) string {
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		return Red
	case "HIGH":
		return Orange
	case "MEDIUM":
		return Yellow
	case "LOW":
		return Blue
	case "INFO":
		return Dim
	default:
		return ""
	}
}

// SeverityColorID returns the 256-color palette ID for a severity level.
// Use with lipgloss: lipgloss.Color(ui.SeverityColorID("HIGH"))
func SeverityColorID(severity string) string {
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		return SeverityIDCritical
	case "HIGH":
		return SeverityIDHigh
	case "MEDIUM":
		return SeverityIDMedium
	case "LOW":
		return SeverityIDLow
	case "INFO":
		return SeverityIDInfo
	default:
		return ""
	}
}

// Colorize wraps text with a color code and reset. Returns plain text if
// color is empty or stdout is not a TTY.
func Colorize(color, text string) string {
	if color == "" || !IsTTY() || theme.Get().NoColor {
		return text
	}
	return color + text + Reset
}

// Success prints a success message
func Success(format string, args ...interface{}) {
	a := theme.ANSI()
	fmt.Printf(a.Success+"✓ "+a.Reset+format+"\n", args...)
}

// Error prints an error message
func Error(format string, args ...interface{}) {
	a := theme.ANSI()
	fmt.Printf(a.Danger+"✗ "+a.Reset+format+"\n", args...)
}

// Warning prints a warning message
func Warning(format string, args ...interface{}) {
	a := theme.ANSI()
	fmt.Printf(a.Warning+"! "+a.Reset+format+"\n", args...)
}

// Info prints an info message
func Info(format string, args ...interface{}) {
	a := theme.ANSI()
	fmt.Printf(a.Info+"→ "+a.Reset+format+"\n", args...)
}

// Header prints a section header
func Header(text string) {
	a := theme.ANSI()
	fmt.Printf("\n%s%s%s\n", a.Info, text, a.Reset)
	fmt.Println(a.Dim + "─────────────────────────────────────────" + a.Reset)
}

// Checkbox returns a formatted checkbox string
func Checkbox(checked bool) string {
	a := theme.ANSI()
	if checked {
		return a.Success + "[x]" + a.Reset
	}
	return "[ ]"
}

// CheckboxItem prints a checkbox item with name and description
func CheckboxItem(checked bool, name, description string) {
	a := theme.ANSI()
	checkbox := Checkbox(checked)
	if description != "" {
		fmt.Printf("  %s %-12s %s%s%s\n", checkbox, name, a.Dim, description, a.Reset)
	} else {
		fmt.Printf("  %s %s\n", checkbox, name)
	}
}

// ActionLine prints an action-oriented diff line with symbol prefix.
//
//	kind: "new"/"restore" (+ green), "modified" (~ cyan), "override" (! yellow),
//	      "orphan" (- red), "local" (← gray), "warn" (⚠ yellow)
func ActionLine(kind, text string) {
	a := theme.ANSI()
	var icon, color string
	switch kind {
	case "new", "restore":
		icon, color = "+", a.Success
	case "modified":
		icon, color = "~", a.Info
	case "override":
		icon, color = "!", a.Warning
	case "orphan":
		icon, color = "-", a.Danger
	case "local":
		icon, color = "←", a.Dim
	// Legacy kinds for backward compatibility
	case "sync":
		icon, color = "→", a.Info
	case "force":
		icon, color = "⚠", a.Warning
	case "collect":
		icon, color = "←", a.Dim
	case "warn":
		icon, color = "⚠", a.Warning
	default:
		icon, color = " ", a.Reset
	}
	fmt.Printf("  %s%s%s %s\n", color, icon, a.Reset, text)
}

// isTTY checks if stdout is a terminal (for animation support)
func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Logo prints the ASCII art logo with optional version and animation
// ModeLabel is an optional label appended after the version in the logo.
// Set to "project" to display "(project)" next to the version.
var ModeLabel string

// WithModeLabel appends " (project)" to text when ModeLabel is set.
func WithModeLabel(text string) string {
	if ModeLabel != "" {
		return text + " (" + ModeLabel + ")"
	}
	return text
}

func Logo(version string) {
	LogoAnimated(version, isTTY())
}

// LogoAnimated prints the ASCII art logo with optional animation
func LogoAnimated(version string, animate bool) {
	a := theme.ANSI()
	lines := []string{
		a.Warning + `     _    _ _ _     _` + a.Reset,
		a.Warning + ` ___| | _(_) | |___| |__   __ _ _ __ ___` + a.Reset,
		a.Warning + `/ __| |/ / | | / __| '_ \ / _` + "`" + ` | '__/ _ \` + a.Reset,
		a.Warning + `\__ \   <| | | \__ \ | | | (_| | | |  __/` + a.Reset + `  ` + a.Dim + `https://github.com/runkids/skillshare` + a.Reset,
	}

	// Last line varies based on version
	suffix := ""
	if ModeLabel != "" {
		suffix = `  ` + a.Info + `(` + ModeLabel + `)` + a.Reset
	}
	if version != "" {
		lines = append(lines, a.Warning+`|___/_|\_\_|_|_|___/_| |_|\__,_|_|  \___|`+a.Reset+`  `+a.Dim+`v`+version+a.Reset+suffix)
	} else {
		lines = append(lines, a.Warning+`|___/_|\_\_|_|_|___/_| |_|\__,_|_|  \___|`+a.Reset+`  `+a.Dim+`Your AI coding setup, everywhere.`+a.Reset+suffix)
	}

	if animate {
		// Animated: fade in line by line (30ms per line = 150ms total)
		for _, line := range lines {
			fmt.Println(line)
			time.Sleep(30 * time.Millisecond)
		}
	} else {
		// Non-TTY: print all at once
		for _, line := range lines {
			fmt.Println(line)
		}
	}
	fmt.Println()
}
