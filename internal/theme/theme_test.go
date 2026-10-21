package theme

import (
	"io"
	"os"
	"runtime"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Each test calls Reset() before reading Get() because t.Setenv mutations
// must take effect after the sync.Once guard.

func TestResolve_NoColorBeatsEverything(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("SKILLSHARE_THEME", "light")
	Reset()

	tm := Get()
	if !tm.NoColor {
		t.Error("NO_COLOR should force NoColor=true")
	}
	if tm.Source != "no-color" {
		t.Errorf("Source = %q, want %q", tm.Source, "no-color")
	}
}

func TestResolve_ExplicitLight(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("SKILLSHARE_THEME", "light")
	Reset()

	tm := Get()
	if tm.Mode != ModeLight {
		t.Errorf("Mode = %q, want %q", tm.Mode, ModeLight)
	}
	if tm.Source != "env" {
		t.Errorf("Source = %q, want %q", tm.Source, "env")
	}
	if tm.NoColor {
		t.Error("NoColor should be false")
	}
}

func TestResolve_ExplicitDark(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("SKILLSHARE_THEME", "dark")
	Reset()

	tm := Get()
	if tm.Mode != ModeDark {
		t.Errorf("Mode = %q, want %q", tm.Mode, ModeDark)
	}
	if tm.Source != "env" {
		t.Errorf("Source = %q", tm.Source)
	}
}

func TestResolve_InvalidValueFallsBack(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("SKILLSHARE_THEME", "rainbow")
	wantSource := pinAutoTheme(t)

	tm := Get()
	if tm.Mode != ModeDark {
		t.Errorf("Mode = %q, want dark auto theme", tm.Mode)
	}
	if tm.Source != wantSource {
		t.Errorf("Source = %q, want %q", tm.Source, wantSource)
	}
}

func TestResolve_NonTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("SKILLSHARE_THEME", "")
	wantSource := pinAutoTheme(t)

	tm := Get()
	if tm.Mode != ModeDark {
		t.Errorf("Mode = %q", tm.Mode)
	}
	if tm.Source != wantSource {
		t.Errorf("Source = %q, want %q", tm.Source, wantSource)
	}
}

func TestANSI_EmptyWhenStdoutIsNotTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("SKILLSHARE_THEME", "dark")
	pinStdoutTerminal(t, false)

	if a := ANSI(); a.Success != "" || a.Reset != "" {
		t.Errorf("ANSI() with stdout redirected must be empty, got %+v", a)
	}
}

func TestANSI_ColoredWhenStdoutIsTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("SKILLSHARE_THEME", "dark")
	pinStdoutTerminal(t, true)

	if a := ANSI(); a.Success == "" {
		t.Error("ANSI() on a terminal must return color sequences")
	}
}

func pinStdoutTerminal(t *testing.T, isTerminal bool) {
	t.Helper()
	orig := stdoutIsTerminal
	stdoutIsTerminal = func() bool { return isTerminal }
	t.Cleanup(func() {
		stdoutIsTerminal = orig
		Reset()
	})
	Reset()
}

func TestResolve_CIEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("SKILLSHARE_THEME", "")
	t.Setenv("CI", "true")
	wantSource := pinAutoTheme(t)

	tm := Get()
	if tm.Mode != ModeDark {
		t.Errorf("Mode = %q", tm.Mode)
	}
	if tm.Source != wantSource {
		t.Errorf("Source = %q, want %q", tm.Source, wantSource)
	}
}

func TestCanDetect_Windows(t *testing.T) {
	t.Setenv("CI", "true")
	t.Setenv("TERM", "dumb")
	if got, want := canDetect(), runtime.GOOS == "windows"; got != want {
		t.Errorf("canDetect() = %v, want %v", got, want)
	}
}

// pinAutoTheme isolates the terminal and forwarded theme. Windows still probes
// through lipgloss without a TTY; Unix falls back before probing.
func pinAutoTheme(t *testing.T) string {
	t.Helper()
	t.Setenv("_SKILLSHARE_THEME_FORWARDED", "")
	stdout, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = stdout
	origRenderer := lipgloss.DefaultRenderer()
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetHasDarkBackground(true)
	lipgloss.SetDefaultRenderer(renderer)
	t.Cleanup(func() {
		os.Stdout = origStdout
		stdout.Close()
		lipgloss.SetDefaultRenderer(origRenderer)
		Reset()
	})
	Reset()
	if runtime.GOOS == "windows" {
		return "detected"
	}
	return "fallback-dark-no-tty"
}

// --- helpers used by later tests ---

func mustBeColor(t *testing.T, c lipgloss.Color, name string) {
	t.Helper()
	if string(c) == "" {
		t.Errorf("palette field %s is empty", name)
	}
}

func TestPaletteCompleteness(t *testing.T) {
	cases := map[string]palette{
		"dark":  darkPalette,
		"light": lightPalette,
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			mustBeColor(t, p.Primary, "Primary")
			mustBeColor(t, p.Muted, "Muted")
			mustBeColor(t, p.Accent, "Accent")
			mustBeColor(t, p.Selected, "Selected")
			mustBeColor(t, p.SelectedBg, "SelectedBg")
			mustBeColor(t, p.Success, "Success")
			mustBeColor(t, p.Warning, "Warning")
			mustBeColor(t, p.Danger, "Danger")
			mustBeColor(t, p.Info, "Info")
			mustBeColor(t, p.BadgeFg, "BadgeFg")
			mustBeColor(t, p.BadgeBg, "BadgeBg")
			mustBeColor(t, p.SeverityCritical, "SeverityCritical")
			mustBeColor(t, p.SeverityHigh, "SeverityHigh")
			mustBeColor(t, p.SeverityMedium, "SeverityMedium")
			mustBeColor(t, p.SeverityLow, "SeverityLow")
			mustBeColor(t, p.SeverityInfo, "SeverityInfo")
		})
	}
}

func TestNoColorStripsStyles(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	Reset()

	rendered := Primary().Render("hello")
	if rendered != "hello" {
		t.Errorf("Primary().Render with NO_COLOR = %q, want %q", rendered, "hello")
	}

	a := ANSI()
	if a.Reset != "" || a.Primary != "" || a.Success != "" {
		t.Errorf("ANSI() with NO_COLOR must return empty strings, got %+v", a)
	}
}

func TestSeverityUnknownFallsBackToDim(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("SKILLSHARE_THEME", "dark")
	Reset()

	unknown := Severity("unknown").Render("x")
	dim := Dim().Render("x")
	if unknown != dim {
		t.Errorf("Severity(\"unknown\") = %q, Dim() = %q; should match", unknown, dim)
	}
}

func TestContrastRatios(t *testing.T) {
	cases := []struct {
		name     string
		fg       lipgloss.Color
		bg       lipgloss.Color
		minRatio float64
	}{
		// Dark palette on pure black
		{"dark/Primary on black", darkPalette.Primary, lipgloss.Color("#000000"), 10.0},
		{"dark/Muted on black", darkPalette.Muted, lipgloss.Color("#000000"), 4.5},
		{"dark/Warning on black", darkPalette.Warning, lipgloss.Color("#000000"), 4.5},
		{"dark/SeverityMedium on black", darkPalette.SeverityMedium, lipgloss.Color("#000000"), 4.5},

		// Light palette on pure white
		{"light/Primary on white", lightPalette.Primary, lipgloss.Color("#FFFFFF"), 10.0},
		{"light/Muted on white", lightPalette.Muted, lipgloss.Color("#FFFFFF"), 4.5},
		{"light/Warning on white", lightPalette.Warning, lipgloss.Color("#FFFFFF"), 4.5},
		{"light/Danger on white", lightPalette.Danger, lipgloss.Color("#FFFFFF"), 4.5},
		{"light/Success on white", lightPalette.Success, lipgloss.Color("#FFFFFF"), 4.5},
		{"light/SeverityCritical on white", lightPalette.SeverityCritical, lipgloss.Color("#FFFFFF"), 4.5},
		{"light/SeverityMedium on white", lightPalette.SeverityMedium, lipgloss.Color("#FFFFFF"), 4.5},
		{"light/SeverityLow on white", lightPalette.SeverityLow, lipgloss.Color("#FFFFFF"), 4.5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ratio := contrastRatio(tc.fg, tc.bg)
			if ratio < tc.minRatio {
				t.Errorf("contrast ratio %.2f < %.2f", ratio, tc.minRatio)
			}
		})
	}
}

// Regression guard: Primary must NOT be pure white (15) on dark palette.
// This was the root cause of issue #125.
func TestDarkPrimaryIsNotPureWhite(t *testing.T) {
	if darkPalette.Primary == lipgloss.Color("15") {
		t.Error("darkPalette.Primary reverted to pure white (15) — this breaks light terminals")
	}
}
