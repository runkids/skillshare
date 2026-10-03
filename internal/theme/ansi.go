package theme

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// ANSISet contains raw terminal escape sequences keyed by semantic role.
// Returned by ANSI() — use these in fmt.Printf-style plain CLI output
// where a lipgloss.Style is too heavy.
//
// Every field is an empty string when NoColor is enabled, so code can
// concatenate them unconditionally without worrying about NO_COLOR.
type ANSISet struct {
	Reset   string
	Primary string
	Muted   string
	Success string
	Warning string
	Danger  string
	Info    string
	Accent  string
	Dim     string
	Bold    string
}

// ANSI returns the raw escape sequence set for the active theme.
// When NoColor or Plain is set, every field (including Reset) is empty.
func ANSI() ANSISet {
	t := Get()
	if t.NoColor || t.Plain {
		return ANSISet{}
	}
	return ANSISet{
		Reset:   "\033[0m",
		Primary: fg256(t.palette.Primary),
		Muted:   fg256(t.palette.Muted),
		Success: fg256(t.palette.Success),
		Warning: fg256(t.palette.Warning),
		Danger:  fg256(t.palette.Danger),
		Info:    fg256(t.palette.Info),
		Accent:  fg256(t.palette.Accent),
		Dim:     "\x1b[0;2m",
		Bold:    "\033[1m",
	}
}

// fg256 builds a 256-color foreground escape sequence from a lipgloss.Color.
// lipgloss.Color is defined as `type Color string` so the underlying value is
// directly convertible; palette values are always 256-palette integer strings
// like "232", "6", etc.
func fg256(c lipgloss.Color) string {
	s := string(c)
	if s == "" {
		return ""
	}
	return fmt.Sprintf("\033[38;5;%sm", s)
}
