package ui

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Base colors for terminal output
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Orange  = "\033[38;5;208m"
	Gray    = "\033[90m"
	White   = "\033[97m"
)

// Semantic color aliases for consistent theming
const (
	// Primary brand color (yellow - matches logo)
	Primary = Yellow
	// Accent color for interactive elements
	Accent = Cyan
	// Muted color for secondary information
	Muted = Gray
	// Status colors
	StatusSuccess = Green
	StatusError   = Red
	StatusWarning = Yellow
	StatusInfo    = Cyan
)

// Bold variants
const (
	Bold      = "\033[1m"
	BoldReset = "\033[22m"
)

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
		return Gray
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
	if color == "" || !IsTTY() {
		return text
	}
	return color + text + Reset
}

// Success prints a success message
func Success(format string, args ...interface{}) {
	fmt.Printf(Green+"✓ "+Reset+format+"\n", args...)
}

// Error prints an error message
func Error(format string, args ...interface{}) {
	fmt.Printf(Red+"✗ "+Reset+format+"\n", args...)
}

// Warning prints a warning message
func Warning(format string, args ...interface{}) {
	fmt.Printf(Yellow+"! "+Reset+format+"\n", args...)
}

// Info prints an info message
func Info(format string, args ...interface{}) {
	fmt.Printf(Cyan+"→ "+Reset+format+"\n", args...)
}

// Status prints a status line
func Status(name, status, detail string) {
	statusColor := Gray
	switch status {
	case "linked":
		statusColor = Green
	case "not exist":
		statusColor = Yellow
	case "has files":
		statusColor = Blue
	case "conflict", "broken":
		statusColor = Red
	}

	fmt.Printf("  %-12s %s%-12s%s %s\n", name, statusColor, status, Reset, Gray+detail+Reset)
}

// Header prints a section header
func Header(text string) {
	fmt.Printf("\n%s%s%s\n", Cyan, text, Reset)
	fmt.Println(Gray + "─────────────────────────────────────────" + Reset)
}

// Checkbox returns a formatted checkbox string
func Checkbox(checked bool) string {
	if checked {
		return Green + "[x]" + Reset
	}
	return "[ ]"
}

// CheckboxItem prints a checkbox item with name and description
func CheckboxItem(checked bool, name, description string) {
	checkbox := Checkbox(checked)
	if description != "" {
		fmt.Printf("  %s %-12s %s%s%s\n", checkbox, name, Gray, description, Reset)
	} else {
		fmt.Printf("  %s %s\n", checkbox, name)
	}
}

// DiffItem prints a diff-style list item (+/-/~)
func DiffItem(action, name, detail string) {
	var icon, color string
	switch action {
	case "add":
		icon, color = "+", Green
	case "modify":
		icon, color = "~", Yellow
	case "remove":
		icon, color = "-", Cyan
	default:
		icon, color = " ", Reset
	}
	if detail != "" {
		fmt.Printf("  %s%s%s %s %s%s%s\n", color, icon, Reset, name, Gray, detail, Reset)
	} else {
		fmt.Printf("  %s%s%s %s\n", color, icon, Reset, name)
	}
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
	lines := []string{
		Primary + `     _    _ _ _     _` + Reset,
		Primary + ` ___| | _(_) | |___| |__   __ _ _ __ ___` + Reset,
		Primary + `/ __| |/ / | | / __| '_ \ / _` + "`" + ` | '__/ _ \` + Reset,
		Primary + `\__ \   <| | | \__ \ | | | (_| | | |  __/` + Reset + `  ` + Muted + `https://github.com/runkids/skillshare` + Reset,
	}

	// Last line varies based on version
	suffix := ""
	if ModeLabel != "" {
		suffix = `  ` + Accent + `(` + ModeLabel + `)` + Reset
	}
	if version != "" {
		lines = append(lines, Primary+`|___/_|\_\_|_|_|___/_| |_|\__,_|_|  \___|`+Reset+`  `+Muted+`v`+version+Reset+suffix)
	} else {
		lines = append(lines, Primary+`|___/_|\_\_|_|_|___/_| |_|\__,_|_|  \___|`+Reset+`  `+Muted+`Sync skills across all AI CLI tools`+Reset+suffix)
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
