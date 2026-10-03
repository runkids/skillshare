// Package theme provides a unified light/dark terminal theme system.
//
// The theme is resolved once at first access via theme.Get() and cached
// for the remainder of the process. Resolution precedence:
//
//  1. NO_COLOR env var set → disable all colors
//  2. SKILLSHARE_THEME=light|dark → explicit override
//  3. SKILLSHARE_THEME=auto (or unset) → probe terminal background
//  4. Fallback to dark palette
//
// Usage from TUI code:
//
//	style := theme.Primary()
//	rendered := style.Render("text")
//
// Usage from plain CLI output:
//
//	ansi := theme.ANSI()
//	fmt.Printf("%s%s%s\n", ansi.Success, "done", ansi.Reset)
package theme

import "sync"

// Mode represents the user's resolved theme preference.
type Mode string

const (
	// ModeAuto is the default before resolution; never appears on a resolved Theme.
	ModeAuto Mode = "auto"
	// ModeLight means the palette is tuned for light terminal backgrounds.
	ModeLight Mode = "light"
	// ModeDark means the palette is tuned for dark terminal backgrounds.
	ModeDark Mode = "dark"
)

// Theme holds the resolved, immutable theme state for the current process.
type Theme struct {
	// Mode is the resolved palette mode (never ModeAuto after Get()).
	Mode Mode
	// Source describes how Mode was decided. See resolve() for valid values.
	Source string
	// NoColor is true when NO_COLOR env var was set; all style/ansi
	// constructors must return plain text when this is true.
	NoColor bool
	// Plain is true when stdout is not a terminal (pipe, file, CI log).
	// Raw escape sequences from ANSI() are suppressed so redirected output
	// stays readable; lipgloss styles already degrade on their own.
	Plain bool

	palette palette
}

var (
	current *Theme
	once    sync.Once
)

// Get returns the process-wide theme singleton, resolving it on first call.
// Safe for concurrent use.
func Get() *Theme {
	once.Do(func() {
		current = resolve()
		current.Plain = !stdoutIsTerminal()
	})
	return current
}

// Reset clears the cached theme so the next Get() re-resolves. Intended
// for tests that need to change env vars and observe a new resolution.
func Reset() {
	once = sync.Once{}
	current = nil
}
