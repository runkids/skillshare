package ui

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/pterm/pterm"
)

// ansiRegex matches ANSI escape sequences (CSI sequences ending with any letter).
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI removes all ANSI escape sequences from a string.
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// gitProgressPercentRegex extracts "Stage: NN%" from git progress lines.
var gitProgressPercentRegex = regexp.MustCompile(`^([^:]+):\s*([0-9]{1,3}%)`)

const spinnerGitUpdateMinInterval = 120 * time.Millisecond
const minProgressWidth = 40

func init() {
	// Unify spinner style: braille dot pattern (matches bubbletea spinner.Dot), cyan.
	pterm.DefaultSpinner.Sequence = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
	pterm.DefaultSpinner.Style = pterm.NewStyle(pterm.FgCyan)
	// Disable built-in timer to prevent flicker: the animation goroutine
	// appends "(Ns)" but UpdateText does not, causing per-frame length
	// differences. Our Success()/Warn() already print elapsed time.
	pterm.DefaultSpinner.ShowTimer = false
}

// DimText wraps text with SGR dim attribute. Use instead of pterm.Gray()
// for consistent appearance across terminal themes.
func DimText(s string) string { return Dim + s + Reset }

// DisplayWidth returns the visible width of a string (excluding ANSI codes, handling wide chars)
func DisplayWidth(s string) int {
	return runewidth.StringWidth(StripANSI(s))
}

// fileTTY returns true if the given file descriptor is a terminal.
func fileTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// IsTTY returns true if stdout is a terminal.
func IsTTY() bool { return fileTTY(os.Stdout) }

// ProgressWriter is the destination for spinner and progress bar output.
// Defaults to os.Stdout; set to os.Stderr for structured output modes
// (e.g. --format json/sarif) so progress indicators don't corrupt data.
var ProgressWriter *os.File = os.Stdout

// SetProgressWriter redirects spinner and progress bar output.
func SetProgressWriter(w *os.File) {
	ProgressWriter = w
}

// progressSuppressed disables all pterm spinners/progress bars, forcing
// text-only fallback. This prevents pterm's cursor package from emitting
// ANSI hide/show codes to os.Stdout (hardcoded upstream).
var progressSuppressed bool

// SuppressProgress forces all spinners and progress bars into non-TTY
// (text-only) mode, preventing pterm from emitting ANSI cursor codes.
func SuppressProgress() { progressSuppressed = true }

// RestoreProgress re-enables TTY detection for progress output.
func RestoreProgress() { progressSuppressed = false }

// SuppressProgressToStderr redirects progress output to stderr and suppresses
// pterm TTY detection. Returns a restore function suitable for defer.
// Use this for structured output modes (--format json/sarif/markdown) to
// prevent ANSI cursor codes from leaking into stdout.
func SuppressProgressToStderr() func() {
	prev := ProgressWriter
	SetProgressWriter(os.Stderr)
	SuppressProgress()
	return func() {
		SetProgressWriter(prev)
		RestoreProgress()
	}
}

// isProgressTTY returns true if ProgressWriter is a terminal and
// progress output has not been suppressed.
func isProgressTTY() bool { return !progressSuppressed && fileTTY(ProgressWriter) }

// Box prints content in a styled box
func Box(title string, lines ...string) {
	BoxWithMinWidth(title, 0, lines...)
}

// BoxWithMinWidth prints a titled section with a separator line, enforcing a
// minimum separator width. Use this to align multiple sections visually.
func BoxWithMinWidth(title string, minWidth int, lines ...string) {
	if !IsTTY() {
		if title != "" {
			fmt.Printf("── %s ──\n", title)
		}
		for _, line := range lines {
			fmt.Println(line)
		}
		return
	}

	maxLen := minWidth
	for _, line := range lines {
		if w := DisplayWidth(line); w > maxLen {
			maxLen = w
		}
	}
	if title != "" {
		fmt.Println(pterm.Cyan(title))
	}
	fmt.Println(DimText(strings.Repeat("─", maxLen)))
	for _, line := range lines {
		fmt.Println(line)
	}
}

// HeaderBox prints command header box
func HeaderBox(command, subtitle string) {
	HeaderBoxWithMinWidth(command, subtitle, 0)
}

// HeaderBoxWithMinWidth prints a command header with a separator line.
func HeaderBoxWithMinWidth(command, subtitle string, minWidth int) {
	if !IsTTY() {
		fmt.Printf("%s\n%s\n", command, subtitle)
		return
	}

	maxLen := minWidth
	for _, line := range strings.Split(subtitle, "\n") {
		if w := DisplayWidth(line); w > maxLen {
			maxLen = w
		}
	}
	if w := DisplayWidth(command); w > maxLen {
		maxLen = w
	}

	fmt.Println(pterm.Cyan(command))
	fmt.Println(DimText(strings.Repeat("─", maxLen)))
	fmt.Println(subtitle)
}

// liveSpinner is a pterm spinner that leaves nothing behind once stopped.
// pterm's render goroutine checks IsActive and then writes, so a Stop that
// lands in between has its line cleared and redrawn; writes go through a gate
// that Stop closes before clearing the line.
type liveSpinner struct {
	printer *pterm.SpinnerPrinter
	mu      sync.Mutex
	stopped bool
	out     io.Writer
}

func startLiveSpinner(text string) *liveSpinner {
	ls := &liveSpinner{out: ProgressWriter}
	ls.printer, _ = pterm.DefaultSpinner.
		WithRemoveWhenDone(true).
		WithWriter(ls).
		Start(text)
	return ls
}

func (ls *liveSpinner) Write(p []byte) (int, error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if ls.stopped {
		return len(p), nil
	}
	return ls.out.Write(p)
}

func (ls *liveSpinner) UpdateText(text string) { ls.printer.UpdateText(text) }

// Stop clears the spinner's line; later writes from pterm are dropped.
func (ls *liveSpinner) Stop() error {
	ls.mu.Lock()
	if !ls.stopped {
		ls.stopped = true
		fmt.Fprint(ls.out, "\r\x1b[2K")
	}
	ls.mu.Unlock()
	return ls.printer.Stop()
}

// Spinner wraps pterm spinner with step tracking
type Spinner struct {
	spinner     *liveSpinner
	start       time.Time
	currentStep int
	totalSteps  int
	stepPrefix  string
	lastUpdate  time.Time
	lastMessage string
}

// StartSpinner starts a spinner with message. Without a terminal it shows
// nothing; only the final Success, Fail or Warn line is printed.
func StartSpinner(message string) *Spinner {
	if !isProgressTTY() {
		return &Spinner{start: time.Now()}
	}

	return &Spinner{spinner: startLiveSpinner(message), start: time.Now()}
}

// StartSpinnerWithSteps starts a spinner that shows step progress
func StartSpinnerWithSteps(message string, totalSteps int) *Spinner {
	if !isProgressTTY() {
		return &Spinner{start: time.Now(), currentStep: 1, totalSteps: totalSteps}
	}

	stepPrefix := fmt.Sprintf("[1/%d] ", totalSteps)
	return &Spinner{
		spinner:     startLiveSpinner(stepPrefix + message),
		start:       time.Now(),
		currentStep: 1,
		totalSteps:  totalSteps,
		stepPrefix:  stepPrefix,
	}
}

// Update updates spinner text
func (s *Spinner) Update(message string) {
	message, ok := normalizeSpinnerUpdate(message, s.lastMessage, s.lastUpdate)
	if !ok {
		return
	}
	s.lastMessage = message
	s.lastUpdate = time.Now()

	if s.spinner != nil {
		s.spinner.UpdateText(s.stepPrefix + message)
	}
}

// NextStep advances to next step and updates message
func (s *Spinner) NextStep(message string) {
	if s.totalSteps > 0 && s.currentStep < s.totalSteps {
		s.currentStep++
		s.stepPrefix = fmt.Sprintf("[%d/%d] ", s.currentStep, s.totalSteps)
	}
	s.Update(message)
}

// Success stops spinner with success
func (s *Spinner) Success(message string) {
	msg := message + elapsedNote(s.start)
	if s.spinner != nil {
		s.spinner.Stop() //nolint:errcheck
		fmt.Fprintf(ProgressWriter, "%s %s\n", pterm.Green("✓"), msg)
	} else {
		fmt.Fprintf(ProgressWriter, "✓ %s\n", msg)
	}
}

// Fail stops spinner with failure (red)
func (s *Spinner) Fail(message string) {
	if s.spinner != nil {
		s.spinner.Stop() //nolint:errcheck
		fmt.Fprintf(ProgressWriter, "%s %s\n", pterm.Red("✗"), message)
	} else {
		fmt.Fprintf(ProgressWriter, "✗ %s\n", message)
	}
}

// Warn stops spinner with warning (yellow)
func (s *Spinner) Warn(message string) {
	msg := message + elapsedNote(s.start)
	if s.spinner != nil {
		s.spinner.Stop() //nolint:errcheck
		fmt.Fprintf(ProgressWriter, "%s %s\n", pterm.Yellow("!"), msg)
	} else {
		fmt.Fprintf(ProgressWriter, "! %s\n", msg)
	}
}

// elapsedNote is Took for a step that started at start.
func elapsedNote(start time.Time) string {
	return Took(time.Since(start))
}

// Started is when the spinner started, for timing the step it covers.
func (s *Spinner) Started() time.Time { return s.start }

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
		fmt.Printf("%s %s\n", pterm.Green("✓"), msg)
	} else {
		fmt.Printf("✓ %s\n", msg)
	}
}

// ErrorMsg prints error message
func ErrorMsg(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if IsTTY() {
		fmt.Printf("%s %s\n", pterm.Red("✗"), msg)
	} else {
		fmt.Printf("✗ %s\n", msg)
	}
}

// WarningBox prints a warning section with a separator line.
func WarningBox(title string, lines ...string) {
	if !IsTTY() {
		fmt.Printf("! %s\n", title)
		for _, line := range lines {
			fmt.Printf("  %s\n", line)
		}
		return
	}

	maxLen := 0
	for _, line := range lines {
		if w := DisplayWidth(line); w > maxLen {
			maxLen = w
		}
	}

	fmt.Println(pterm.Yellow(title))
	fmt.Println(pterm.Yellow(strings.Repeat("─", maxLen)))
	for _, line := range lines {
		fmt.Println(line)
	}
}

// ProgressBar renders a fixed-width progress bar with block characters.
//
// TTY output (single-line, \r overwrite):
//
//	■■■■■■■■■■■■■■■■■■■■･････････ 69%  Scanning skills       5/10
//
// Non-TTY output falls back to plain text lines.
type ProgressBar struct {
	mu         sync.Mutex
	total      int
	current    int
	title      string
	header     string // optional phase header rendered above the bar
	hasHeader  bool   // true once SetHeader has been called
	tty        bool
	stopped    bool
	lastRender time.Time
	dirty      bool // state changed since last render
}

const (
	barWidth         = 36           // fixed visible width (character count)
	barFill          = "■"          // U+25A0 filled block
	barEmpty         = "･"          // U+FF65 half-width dot
	barColor         = "\033[0;36m" // reset + cyan (project accent)
	barDim           = "\033[36;2m" // cyan + dim for empty dots
	barMuted         = "\x1b[0;2m"  // dim attribute for label + count
	barReset         = "\033[0m"
	hideCursor       = "\x1b[?25l"
	showCursor       = "\x1b[?25h"
	clearLine        = "\r\x1b[2K"
	barThrottleMs    = 50 * time.Millisecond // min interval between renders
	barFixedOverhead = barWidth + 8          // bar chars + " NNN%  " + padding
)

// StartProgress starts a progress bar with the given title and total count.
func StartProgress(title string, total int) *ProgressBar {
	// Off a terminal the bar prints nothing, like a spinner: it would only
	// leave a stale line where a terminal shows a frame that clears itself.
	if !isProgressTTY() {
		return &ProgressBar{total: total, title: title}
	}

	fmt.Fprint(ProgressWriter, hideCursor)
	p := &ProgressBar{total: total, title: title, tty: true}
	p.renderNow()
	return p
}

// Increment increments progress by 1. Safe for concurrent use.
func (p *ProgressBar) Increment() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return
	}
	p.current++
	if p.current > p.total {
		p.current = p.total
	}
	if p.current >= p.total {
		p.title = "Done"
		if p.tty {
			p.renderNow() // always render the final frame
		}
		return
	}
	if p.tty {
		p.renderThrottled()
	}
}

// Add increments progress by n. Safe for concurrent use.
func (p *ProgressBar) Add(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return
	}
	p.current += n
	if p.current > p.total {
		p.current = p.total
	}
	if p.tty {
		p.renderThrottled()
	}
}

// UpdateTitle updates the label shown after the percentage.
// Git progress messages are automatically normalized (strip "remote:" prefix,
// transfer rates, and verbose object counts). Safe for concurrent use.
func (p *ProgressBar) UpdateTitle(title string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.title = strings.TrimSpace(normalizeGitProgressMessage(title))

	if p.tty {
		p.renderThrottled()
	}
}

// SetHeader sets (or updates) a phase header line rendered above the progress bar.
// On the first call it pushes the bar down one line to make room for the header.
// Subsequent calls update the header in place. Safe for concurrent use.
func (p *ProgressBar) SetHeader(header string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return
	}
	if !p.hasHeader {
		// First header: push bar down one line so the header can occupy the line above.
		if p.tty {
			fmt.Fprint(ProgressWriter, "\n")
		}
		p.hasHeader = true
	}
	p.header = header
	if p.tty {
		p.renderNow()
	}
}

// Stop clears the progress bar and its phase header and restores the cursor;
// the result that follows says what happened. Safe for concurrent use.
func (p *ProgressBar) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return
	}
	p.stopped = true
	if p.tty {
		fmt.Fprint(ProgressWriter, clearLine)
		if p.hasHeader {
			fmt.Fprint(ProgressWriter, "\x1b[1A"+clearLine)
		}
		fmt.Fprint(ProgressWriter, showCursor)
	}
}

// renderThrottled renders at most once per barThrottleMs. Marks dirty if skipped.
func (p *ProgressBar) renderThrottled() {
	now := time.Now()
	if now.Sub(p.lastRender) < barThrottleMs {
		p.dirty = true
		return
	}
	p.renderNow()
}

// renderNow draws the progress bar unconditionally.
// In 2-line mode (hasHeader), it also rewrites the header line above the bar.
func (p *ProgressBar) renderNow() {
	p.dirty = false
	p.lastRender = time.Now()

	pct := 0
	if p.total > 0 {
		pct = p.current * 100 / p.total
	}
	if pct > 100 {
		pct = 100
	}

	fill := pct * barWidth / 100
	empty := barWidth - fill

	// Layout: bar(36) + " NNN%" (5) + "  " + count + "  " + title
	// Count is at a fixed position right after %, title fills remaining space.
	countStr := fmt.Sprintf("%d/%d", p.current, p.total)
	// barFixedOverhead covers bar(36) + " NNN%  "(8); add count + "  " gap
	titleWidth := pterm.GetTerminalWidth() - barFixedOverhead - len(countStr) - 4
	if titleWidth < 12 {
		titleWidth = 12
	}

	bar := fmt.Sprintf("%s%s%s%s%s %3d%%%s  %s  %s%s",
		barColor, strings.Repeat(barFill, fill),
		barDim, strings.Repeat(barEmpty, empty),
		barColor,
		pct,
		barMuted,
		countStr,
		runewidth.Truncate(p.title, titleWidth, "..."),
		barReset,
	)

	if p.hasHeader && p.tty {
		// 2-line mode: cursor is on bar line (N+1).
		// Move up to header line (N), clear & print header, move back down, clear & print bar.
		fmt.Fprintf(ProgressWriter, "\x1b[1A%s%s\n%s%s", clearLine, p.header, clearLine, bar)
	} else {
		fmt.Fprintf(ProgressWriter, "%s%s", clearLine, bar)
	}
}

// RenderInlineBar renders a compact inline progress bar for TUI area printers.
// Style: cyan filled + dim empty, 30 chars wide. Used by batch operations
// (diff, search install) where the bar is embedded in a multi-line area update.
func RenderInlineBar(done, total int) string {
	const barWidth = 30
	if total <= 0 {
		emptyBar := DimText(strings.Repeat("█", barWidth))
		return fmt.Sprintf("%s %s 0%%", emptyBar, DimText("0/0"))
	}
	filled := done * barWidth / total
	if filled > barWidth {
		filled = barWidth
	}
	pct := (done*100 + total/2) / total // rounded integer division
	filledBar := pterm.Cyan(strings.Repeat("█", filled))
	emptyBar := DimText(strings.Repeat("█", barWidth-filled))
	count := fmt.Sprintf("%d/%d", done, total)
	return fmt.Sprintf("%s%s %s %d%%", filledBar, emptyBar, DimText(count), pct)
}

// UpdateNotification prints a colorful update notification.
// upgradeCmd is the command the user should run (e.g. "brew upgrade skillshare"
// or "skillshare upgrade").
func UpdateNotification(currentVersion, latestVersion, upgradeCmd string) {
	if !IsTTY() {
		fmt.Fprintf(os.Stderr, "\n! Update available: %s -> %s\n", currentVersion, latestVersion)
		fmt.Fprintf(os.Stderr, "  Run '%s' to update\n", upgradeCmd)
		return
	}

	fmt.Fprintln(os.Stderr)
	versionLine := fmt.Sprintf("  Version: %s -> %s", currentVersion, latestVersion)
	runLine := fmt.Sprintf("  Run: %s", upgradeCmd)
	w := DisplayWidth(versionLine)
	if rw := DisplayWidth(runLine); rw > w {
		w = rw
	}

	fmt.Fprintln(os.Stderr, pterm.Yellow("Update Available"))
	fmt.Fprintln(os.Stderr, pterm.Yellow(strings.Repeat("─", w)))
	fmt.Fprintln(os.Stderr, versionLine)
	fmt.Fprintln(os.Stderr, runLine)
}

// UpdateSummary prints the closing line of an update, leaving out zero counts.
func UpdateSummary(stats UpdateStats) {
	text := "Nothing to update"
	if stats.Updated > 0 {
		text = fmt.Sprintf("Updated %d", stats.Updated)
	} else if stats.Skipped > 0 || stats.Pruned > 0 || stats.SecurityFailed > 0 {
		text = "Nothing updated"
	}
	for _, m := range []struct {
		n     int
		label string
	}{{stats.Skipped, "skipped"}, {stats.Pruned, "pruned"}, {stats.SecurityFailed, "blocked by security audit"}} {
		if m.n > 0 {
			text += fmt.Sprintf(", %d %s", m.n, m.label)
		}
	}
	mark := MarkOK
	if stats.SecurityFailed > 0 {
		mark = MarkWarn
	}
	fmt.Println()
	Done(mark, text, stats.Duration)
}

// UpdateStats holds statistics for update summary
type UpdateStats struct {
	Updated        int
	Skipped        int
	Pruned         int
	SecurityFailed int
	Duration       time.Duration
}

// Metric is a labeled count for OperationSummary.
type Metric struct {
	Label string
	Count int
	// HighlightColor is the pterm color func used when Count > 0 in TTY mode.
	// When nil, defaults to pterm.Gray.
	HighlightColor func(a ...any) string
}

// formatSummaryLine builds the plain-text (non-TTY) summary line.
func formatSummaryLine(action string, duration time.Duration, metrics ...Metric) string {
	parts := make([]string, len(metrics))
	for i, m := range metrics {
		parts[i] = fmt.Sprintf("%d %s", m.Count, m.Label)
	}
	line := fmt.Sprintf("%s complete: %s", action, strings.Join(parts, ", "))
	if duration > 0 {
		line += fmt.Sprintf(" (%.1fs)", duration.Seconds())
	}
	return line
}

// OperationSummary prints a generic operation summary line.
//
// TTY output:  ✓ Collect complete  5 collected  2 skipped  0 failed  (1.2s)
// Pipe output: Collect complete: 5 collected, 2 skipped, 0 failed (1.2s)
func OperationSummary(action string, duration time.Duration, metrics ...Metric) {
	fmt.Println() // spacing before summary

	if !IsTTY() {
		fmt.Println(formatSummaryLine(action, duration, metrics...))
		return
	}

	parts := []string{pterm.Green("✓ " + action + " complete")}
	dimFn := func(a ...any) string { return DimText(fmt.Sprint(a...)) }
	for _, m := range metrics {
		colorFn := dimFn
		if m.Count > 0 && m.HighlightColor != nil {
			colorFn = m.HighlightColor
		}
		parts = append(parts, colorFn(fmt.Sprint(m.Count))+" "+m.Label)
	}
	line := strings.Join(parts, "  ")
	if duration > 0 {
		line += "  " + DimText(fmt.Sprintf("(%.1fs)", duration.Seconds()))
	}
	fmt.Println(line)
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
		fmt.Printf("  %s %-20s %s\n", style.Sprint(statusIcon), name, DimText(detail))
	} else {
		fmt.Printf("  %s %-20s %s\n", statusIcon, name, detail)
	}
}

// Step-based UI components for install flow

const (
	StepArrow  = "▸"
	StepCheck  = "✓"
	StepCross  = "✗"
	StepSkipCh = "⊘"
	StepBullet = "●"
	StepLine   = "│"
	StepCorner = "└"
)

// TreeLine returns the tree continuation character (│) with TTY coloring
func TreeLine() string {
	if IsTTY() {
		return DimText(StepLine)
	}
	return StepLine
}

// ClearLines moves the cursor up n lines and clears each, effectively
// erasing the last n lines of terminal output. No-op when stdout is not a TTY.
func ClearLines(n int) {
	if !IsTTY() {
		return
	}
	for range n {
		fmt.Print("\033[A\033[2K")
	}
}

// StepStart, StepContinue and StepEnd print one detail of a running step
// as a plain row: "  Source    ~/work/sql-helper".
func StepStart(label, value string) { Row(MarkNone, label, value, RowWidth(label)) }

// StepContinue prints a detail row; see StepStart.
func StepContinue(label, value string) { Row(MarkNone, label, value, RowWidth(label)) }

// StepResult prints a command's closing line after a blank line. status is
// "success", "error" or anything else for a warning.
func StepResult(status, message string, duration time.Duration) {
	mark := MarkWarn
	switch status {
	case "success":
		mark = MarkOK
	case "error":
		mark = MarkFail
	}
	fmt.Println()
	Done(mark, message, duration)
}

// StepEnd prints a detail row; see StepStart.
func StepEnd(label, value string) { Row(MarkNone, label, value, RowWidth(label)) }

// TreeSpinner is a spinner that fits into tree structure
type TreeSpinner struct {
	spinner     *liveSpinner
	start       time.Time
	isLast      bool
	lastUpdate  time.Time
	lastMessage string
}

// StartTreeSpinner starts a spinner in tree context
func StartTreeSpinner(message string, isLast bool) *TreeSpinner {
	if !IsTTY() {
		return &TreeSpinner{start: time.Now(), isLast: isLast}
	}

	return &TreeSpinner{spinner: startLiveSpinner(message), start: time.Now(), isLast: isLast}
}

// Success clears the spinner and prints "✓ message · 0.4s".
func (ts *TreeSpinner) Success(message string) { ts.finish(MarkOK, message, true) }

// Fail clears the spinner and prints "✗ message".
func (ts *TreeSpinner) Fail(message string) { ts.finish(MarkFail, message, false) }

// Warn clears the spinner and prints "! message · 0.4s".
func (ts *TreeSpinner) Warn(message string) { ts.finish(MarkWarn, message, true) }

// Stop clears the spinner without printing anything.
func (ts *TreeSpinner) Stop() {
	if ts.spinner != nil {
		ts.spinner.Stop()
	}
}

// Started is when the spinner started, for timing the step it covers.
func (ts *TreeSpinner) Started() time.Time { return ts.start }

func (ts *TreeSpinner) finish(mark, message string, timed bool) {
	if ts.spinner != nil {
		ts.spinner.Stop()
	}
	if timed {
		message += elapsedNote(ts.start)
	}
	fmt.Printf("%s %s\n", StyledMark(mark), message)
}

func (ts *TreeSpinner) Update(message string) {
	message, ok := normalizeSpinnerUpdate(message, ts.lastMessage, ts.lastUpdate)
	if !ok {
		return
	}
	ts.lastMessage = message
	ts.lastUpdate = time.Now()

	if ts.spinner != nil {
		ts.spinner.UpdateText(message)
	}
}

func normalizeSpinnerUpdate(message, lastMessage string, lastUpdate time.Time) (string, bool) {
	msg := normalizeGitProgressMessage(strings.TrimSpace(message))
	if msg == "" {
		return "", false
	}
	if msg == lastMessage {
		return "", false
	}

	// Git progress can emit rapid \r updates (especially transfer rate).
	// Throttle those lines to reduce visible flicker.
	if isGitProgressMessage(msg) && !lastUpdate.IsZero() && time.Since(lastUpdate) < spinnerGitUpdateMinInterval {
		return "", false
	}

	return msg, true
}

func normalizeGitProgressMessage(message string) string {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return ""
	}

	// "remote: ..." chatter is common; keep message body only.
	if strings.HasPrefix(strings.ToLower(msg), "remote:") {
		msg = strings.TrimSpace(msg[len("remote:"):])
	}

	// Drop volatile transfer-rate suffix to avoid constant redraws:
	// e.g. "... 234.42 MiB | 15.94 MiB/s"
	if strings.Contains(msg, "|") && strings.Contains(msg, "%") {
		msg = strings.TrimSpace(strings.SplitN(msg, "|", 2)[0])
		msg = strings.TrimRight(msg, ", ")
	}

	// Normalize percentage progress to stage + percent only.
	// e.g. "Receiving objects: 69% (...)" -> "Receiving objects: 69%"
	if m := gitProgressPercentRegex.FindStringSubmatch(msg); len(m) == 3 {
		stage := strings.TrimSpace(m[1])
		pct := strings.TrimSpace(m[2])
		if stage != "" && pct != "" {
			msg = fmt.Sprintf("%s: %s", stage, pct)
		}
	}

	// Pad short messages to a fixed width so they overwrite residual
	// characters left by a previous longer message on the same line.
	if len(msg) < minProgressWidth {
		msg += strings.Repeat(" ", minProgressWidth-len(msg))
	}

	return msg
}

func isGitProgressMessage(message string) bool {
	return strings.Contains(message, "%") && strings.Contains(message, ":")
}

// StepItem prints a step with label and value (legacy, use StepStart/Continue/End)
func StepItem(label, value string) {
	if IsTTY() {
		fmt.Printf("%s %-10s %s\n", pterm.Yellow(StepArrow), pterm.White(label), value)
	} else {
		fmt.Printf("%s %-10s %s\n", StepArrow, label, value)
	}
}

// StepDone prints a finished item: "✓ label  value".
func StepDone(label, value string) { Row(MarkOK, label, value, RowWidth(label)) }

// StepFail prints a failed item: "✗ label  value".
func StepFail(label, value string) { Row(MarkFail, label, value, RowWidth(label)) }

// StepSkip prints a skipped item: "! label  value".
func StepSkip(label, value string) { Row(MarkWarn, label, value, RowWidth(label)) }

// SkillBoxCompact prints a skill found in a repository with its location.
func SkillBoxCompact(name, location string) {
	if location == "." {
		location = "root"
	}
	if location == "" {
		fmt.Printf("  %s\n", name)
		return
	}
	fmt.Printf("  %s  %s\n", name, DimText(location))
}

// FormatPhaseHeader returns a formatted phase label string without printing.
// Suitable for use with ProgressBar.SetHeader().
func FormatPhaseHeader(current, total int, format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	if IsTTY() {
		return fmt.Sprintf("%s %s", pterm.Cyan(fmt.Sprintf("[%d/%d]", current, total)), msg)
	}
	return fmt.Sprintf("[%d/%d] %s", current, total, msg)
}

// PhaseHeader prints a phase label like "[1/3] Pulling 5 tracked repos..."
// Used by batch operations to indicate progress across distinct phases.
func PhaseHeader(current, total int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if IsTTY() {
		fmt.Printf("\n%s %s\n", pterm.Cyan(fmt.Sprintf("[%d/%d]", current, total)), msg)
	} else {
		fmt.Printf("\n[%d/%d] %s\n", current, total, msg)
	}
}

// SectionLabel starts a block of results with a bold name.
func SectionLabel(label string) {
	Section(label)
}
