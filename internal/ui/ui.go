package ui

import (
	"fmt"
)

// Colors for terminal output
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[90m"
)

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
