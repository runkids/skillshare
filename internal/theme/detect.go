package theme

import (
	"os"
	"runtime"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// probeTimeout is the maximum time resolve() waits for an OSC 11 background
// color query response. Chosen empirically: long enough for tmux passthrough
// and slow SSH links, short enough to not stall CLI startup.
const probeTimeout = 300 * time.Millisecond

// resolve decides the theme at process start. It never returns nil.
//
// Precedence (highest first):
//
//  1. NO_COLOR set (any value) → NoColor=true, palette arbitrary.
//  2. SKILLSHARE_THEME=light → light palette, Source="env".
//  3. SKILLSHARE_THEME=dark → dark palette, Source="env".
//  4. SKILLSHARE_THEME=auto / unset / unknown → probe terminal.
//  5. Probe skipped or failed → dark palette fallback.
func resolve() *Theme {
	// 1. NO_COLOR — https://no-color.org/ (only when non-empty per spec)
	if v, ok := os.LookupEnv("NO_COLOR"); ok && v != "" {
		return &Theme{
			Mode:    ModeDark,
			Source:  "no-color",
			NoColor: true,
			palette: darkPalette,
		}
	}

	// 2-3. Explicit SKILLSHARE_THEME override
	switch os.Getenv("SKILLSHARE_THEME") {
	case "light":
		return &Theme{Mode: ModeLight, Source: "env", palette: lightPalette}
	case "dark":
		return &Theme{Mode: ModeDark, Source: "env", palette: darkPalette}
	}
	// "", "auto", or unknown values fall through to detection.

	// 3b. Server-forwarded detection — the server already detected theme
	// from its TTY and passes the result so subprocess doesn't need a TTY.
	// Source is "detected" (not "env") because the user didn't set this.
	switch os.Getenv("_SKILLSHARE_THEME_FORWARDED") {
	case "light":
		return &Theme{Mode: ModeLight, Source: "detected", palette: lightPalette}
	case "dark":
		return &Theme{Mode: ModeDark, Source: "detected", palette: darkPalette}
	}

	// 4. Auto detection gate
	if !canDetect() {
		return &Theme{
			Mode:    ModeDark,
			Source:  "fallback-dark-no-tty",
			palette: darkPalette,
		}
	}

	// 5. OSC 11 probe with timeout
	mode, ok := probeBackgroundWithTimeout(probeTimeout)
	if !ok {
		return &Theme{
			Mode:    ModeDark,
			Source:  "fallback-dark-probe-failed",
			palette: darkPalette,
		}
	}
	if mode == ModeLight {
		return &Theme{Mode: ModeLight, Source: "detected", palette: lightPalette}
	}
	return &Theme{Mode: ModeDark, Source: "detected", palette: darkPalette}
}

// stdoutIsTerminal reports whether stdout is attached to a terminal.
// A variable so tests can pin the answer.
var stdoutIsTerminal = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

// canDetect reports whether it is safe to send an OSC 11 query. Guards
// against:
//   - Non-TTY stdin/stdout (piped, redirected, CI)
//   - CI env var set
//   - TERM=dumb or empty
//
// On Windows returns true unconditionally because lipgloss uses WinAPI
// instead of OSC 11 there.
func canDetect() bool {
	if runtime.GOOS == "windows" {
		return true
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}
	if os.Getenv("CI") != "" {
		return false
	}
	t := os.Getenv("TERM")
	if t == "" || t == "dumb" {
		return false
	}
	return true
}

// probeBackgroundWithTimeout calls lipgloss.HasDarkBackground in a goroutine
// and returns ("dark"|"light", true) on success or ("", false) on timeout.
//
// KNOWN LEAK: When the probe times out, the goroutine remains blocked
// reading from stdin until the process exits. This is acceptable because
// the leak occurs at most once per process and holds only a goroutine
// (no file handles or locks). The alternative — a custom OSC 11 probe
// with SetReadDeadline — would add ~50 lines of terminal protocol code
// without a meaningful benefit.
func probeBackgroundWithTimeout(timeout time.Duration) (Mode, bool) {
	ch := make(chan bool, 1)
	go func() {
		ch <- lipgloss.HasDarkBackground()
	}()
	select {
	case isDark := <-ch:
		if isDark {
			return ModeDark, true
		}
		return ModeLight, true
	case <-time.After(timeout):
		return "", false
	}
}
