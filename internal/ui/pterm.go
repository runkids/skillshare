package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// IsTTY returns true if stdout is a terminal
func IsTTY() bool {
	fi, _ := os.Stdout.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Box prints content in a styled box
func Box(title string, lines ...string) {
	if !IsTTY() {
		if title != "" {
			fmt.Printf("── %s ──\n", title)
		}
		for _, line := range lines {
			fmt.Println(line)
		}
		return
	}

	// Find max line length for consistent box width
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	// Pad lines to same length
	content := ""
	for i, line := range lines {
		padded := line
		if len(line) < maxLen {
			padded = line + strings.Repeat(" ", maxLen-len(line))
		}
		content += padded
		if i < len(lines)-1 {
			content += "\n"
		}
	}

	box := pterm.DefaultBox.WithTitle(title)
	box.Println(content)
}

// HeaderBox prints command header box
func HeaderBox(command, subtitle string) {
	if !IsTTY() {
		fmt.Printf("%s\n%s\n", command, subtitle)
		return
	}

	box := pterm.DefaultBox.
		WithTitle(pterm.Cyan(command)).
		WithTitleTopCenter()
	box.Println(subtitle)
}

// Spinner wraps pterm spinner
type Spinner struct {
	spinner *pterm.SpinnerPrinter
	start   time.Time
}

// StartSpinner starts a spinner with message
func StartSpinner(message string) *Spinner {
	if !IsTTY() {
		fmt.Printf("... %s\n", message)
		return &Spinner{start: time.Now()}
	}

	s, _ := pterm.DefaultSpinner.Start(message)
	return &Spinner{spinner: s, start: time.Now()}
}

// Update updates spinner text
func (s *Spinner) Update(message string) {
	if s.spinner != nil {
		s.spinner.UpdateText(message)
	} else {
		fmt.Printf("... %s\n", message)
	}
}

// Success stops spinner with success
func (s *Spinner) Success(message string) {
	elapsed := time.Since(s.start)
	msg := fmt.Sprintf("%s (%.1fs)", message, elapsed.Seconds())
	if s.spinner != nil {
		s.spinner.Success(msg)
	} else {
		fmt.Printf("✓ %s\n", msg)
	}
}

// Fail stops spinner with failure
func (s *Spinner) Fail(message string) {
	if s.spinner != nil {
		s.spinner.Fail(message)
	} else {
		fmt.Printf("✗ %s\n", message)
	}
}

// Stop stops spinner without message
func (s *Spinner) Stop() {
	if s.spinner != nil {
		s.spinner.Stop()
	}
}

// SuccessMsg prints success message
func SuccessMsg(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if IsTTY() {
		pterm.Success.Println(msg)
	} else {
		fmt.Printf("✓ %s\n", msg)
	}
}

// ErrorMsg prints error message
func ErrorMsg(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if IsTTY() {
		pterm.Error.Println(msg)
	} else {
		fmt.Printf("✗ %s\n", msg)
	}
}

// WarningBox prints warning in a box
func WarningBox(title string, lines ...string) {
	if !IsTTY() {
		fmt.Printf("! %s\n", title)
		for _, line := range lines {
			fmt.Printf("  %s\n", line)
		}
		return
	}

	// Find max line length for consistent box width
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	// Pad lines to same length
	content := ""
	for i, line := range lines {
		padded := line
		if len(line) < maxLen {
			padded = line + strings.Repeat(" ", maxLen-len(line))
		}
		content += padded
		if i < len(lines)-1 {
			content += "\n"
		}
	}

	box := pterm.DefaultBox.
		WithTitle(pterm.Yellow(title)).
		WithBoxStyle(pterm.NewStyle(pterm.FgYellow))
	box.Println(content)
}

// SummaryBox prints a summary box with key-value pairs
func SummaryBox(title string, items map[string]string) {
	if !IsTTY() {
		fmt.Printf("── %s ──\n", title)
		for k, v := range items {
			fmt.Printf("  %s: %s\n", k, v)
		}
		return
	}

	var lines []string
	for k, v := range items {
		lines = append(lines, fmt.Sprintf("  %-10s %s", k+":", v))
	}

	// Find max line length for consistent box width
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	// Pad lines to same length
	content := ""
	for i, line := range lines {
		padded := line
		if len(line) < maxLen {
			padded = line + strings.Repeat(" ", maxLen-len(line))
		}
		content += padded
		if i < len(lines)-1 {
			content += "\n"
		}
	}

	box := pterm.DefaultBox.WithTitle(title)
	box.Println(content)
}

// ProgressBar wraps pterm progress bar
type ProgressBar struct {
	bar   *pterm.ProgressbarPrinter
	total int
}

// StartProgress starts a progress bar
func StartProgress(title string, total int) *ProgressBar {
	if !IsTTY() {
		fmt.Printf("%s (0/%d)\n", title, total)
		return &ProgressBar{total: total}
	}

	bar, _ := pterm.DefaultProgressbar.
		WithTotal(total).
		WithTitle(title).
		Start()
	return &ProgressBar{bar: bar, total: total}
}

// Increment increments progress
func (p *ProgressBar) Increment() {
	if p.bar != nil {
		p.bar.Increment()
	}
}

// UpdateTitle updates progress bar title
func (p *ProgressBar) UpdateTitle(title string) {
	if p.bar != nil {
		p.bar.UpdateTitle(title)
	} else {
		fmt.Printf("  %s\n", title)
	}
}

// Stop stops the progress bar
func (p *ProgressBar) Stop() {
	if p.bar != nil {
		p.bar.Stop()
	}
}

// UpdateNotification prints a colorful update notification
func UpdateNotification(currentVersion, latestVersion string) {
	if !IsTTY() {
		fmt.Printf("\n! Update available: %s -> %s\n", currentVersion, latestVersion)
		fmt.Println("  Run 'skillshare upgrade' to update")
		return
	}

	fmt.Println()
	box := pterm.DefaultBox.
		WithTitle(pterm.Yellow("Update Available")).
		WithBoxStyle(pterm.NewStyle(pterm.FgYellow))
	content := fmt.Sprintf(
		"\n  %s %s → %s\n\n  Run: %s\n",
		pterm.Gray("Version:"),
		pterm.Red(currentVersion),
		pterm.Green(latestVersion),
		pterm.Cyan("skillshare upgrade"),
	)
	box.Println(content)
}

// ListItem prints a list item with status
func ListItem(status, name, detail string) {
	var statusIcon string
	var style pterm.Style

	switch status {
	case "success":
		statusIcon = "✓"
		style = *pterm.NewStyle(pterm.FgGreen)
	case "error":
		statusIcon = "✗"
		style = *pterm.NewStyle(pterm.FgRed)
	case "warning":
		statusIcon = "!"
		style = *pterm.NewStyle(pterm.FgYellow)
	default:
		statusIcon = "→"
		style = *pterm.NewStyle(pterm.FgCyan)
	}

	if IsTTY() {
		fmt.Printf("  %s %-20s %s\n", style.Sprint(statusIcon), name, pterm.Gray(detail))
	} else {
		fmt.Printf("  %s %-20s %s\n", statusIcon, name, detail)
	}
}
