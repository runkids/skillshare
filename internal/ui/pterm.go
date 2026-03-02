package ui

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/pterm/pterm"
)

// ansiRegex matches ANSI escape sequences
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

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

// DisplayWidth returns the visible width of a string (excluding ANSI codes, handling wide chars)
func DisplayWidth(s string) int {
	// Remove ANSI codes first, then calculate Unicode-aware width
	clean := ansiRegex.ReplaceAllString(s, "")
	return runewidth.StringWidth(clean)
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

// isProgressTTY returns true if ProgressWriter is a terminal.
func isProgressTTY() bool { return fileTTY(ProgressWriter) }

// Box prints content in a styled box
func Box(title string, lines ...string) {
	BoxWithMinWidth(title, 0, lines...)
}

// BoxWithMinWidth prints a titled box, enforcing a minimum content width.
// Use this to align multiple boxes to the same width.
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

	// Find max display width for consistent box width (excludes ANSI codes)
	maxLen := minWidth
	for _, line := range lines {
		w := DisplayWidth(line)
		if w > maxLen {
			maxLen = w
		}
	}

	// Pad lines to same display width
	content := ""
	for i, line := range lines {
		padded := line
		w := DisplayWidth(line)
		if w < maxLen {
			padded = line + strings.Repeat(" ", maxLen-w)
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
	HeaderBoxWithMinWidth(command, subtitle, 0)
}

// HeaderBoxWithMinWidth prints command header box with a minimum content width.
func HeaderBoxWithMinWidth(command, subtitle string, minWidth int) {
	if !IsTTY() {
		fmt.Printf("%s\n%s\n", command, subtitle)
		return
	}

	// Pad subtitle lines to at least minWidth.
	if minWidth > 0 {
		lines := strings.Split(subtitle, "\n")
		maxLen := minWidth
		for _, line := range lines {
			if w := DisplayWidth(line); w > maxLen {
				maxLen = w
			}
		}
		var padded []string
		for _, line := range lines {
			w := DisplayWidth(line)
			if w < maxLen {
				line = line + strings.Repeat(" ", maxLen-w)
			}
			padded = append(padded, line)
		}
		subtitle = strings.Join(padded, "\n")
	}

	box := pterm.DefaultBox.
		WithTitle(pterm.Cyan(command)).
		WithTitleTopLeft()
	box.Println(subtitle)
}

// Spinner wraps pterm spinner with step tracking
type Spinner struct {
	spinner     *pterm.SpinnerPrinter
	start       time.Time
	currentStep int
	totalSteps  int
	stepPrefix  string
	lastUpdate  time.Time
	lastMessage string
}

// StartSpinner starts a spinner with message
func StartSpinner(message string) *Spinner {
	if !isProgressTTY() {
		fmt.Fprintf(ProgressWriter, "... %s\n", message)
		return &Spinner{start: time.Now()}
	}

	s, _ := pterm.DefaultSpinner.
		WithRemoveWhenDone(true).
		WithWriter(ProgressWriter).
		Start(message)
	return &Spinner{spinner: s, start: time.Now()}
}

// StartSpinnerWithSteps starts a spinner that shows step progress
func StartSpinnerWithSteps(message string, totalSteps int) *Spinner {
	if !isProgressTTY() {
		fmt.Fprintf(ProgressWriter, "... [1/%d] %s\n", totalSteps, message)
		return &Spinner{start: time.Now(), currentStep: 1, totalSteps: totalSteps}
	}

	stepPrefix := fmt.Sprintf("[1/%d] ", totalSteps)
	s, _ := pterm.DefaultSpinner.
		WithRemoveWhenDone(true).
		WithWriter(ProgressWriter).
		Start(stepPrefix + message)
	return &Spinner{
		spinner:     s,
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
	} else {
		if s.totalSteps > 0 {
			fmt.Fprintf(ProgressWriter, "... [%d/%d] %s\n", s.currentStep, s.totalSteps, message)
		} else {
			fmt.Fprintf(ProgressWriter, "... %s\n", message)
		}
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
	elapsed := time.Since(s.start)
	msg := message
	if elapsed.Seconds() >= 0.05 {
		msg = fmt.Sprintf("%s (%.1fs)", message, elapsed.Seconds())
	}
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
	elapsed := time.Since(s.start)
	msg := message
	if elapsed.Seconds() >= 0.05 {
		msg = fmt.Sprintf("%s (%.1fs)", message, elapsed.Seconds())
	}
	if s.spinner != nil {
		s.spinner.Stop() //nolint:errcheck
		fmt.Fprintf(ProgressWriter, "%s %s\n", pterm.Yellow("!"), msg)
	} else {
		fmt.Fprintf(ProgressWriter, "! %s\n", msg)
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

// WarningBox prints warning in a box
func WarningBox(title string, lines ...string) {
	if !IsTTY() {
		fmt.Printf("! %s\n", title)
		for _, line := range lines {
			fmt.Printf("  %s\n", line)
		}
		return
	}

	// Find max display width for consistent box width (excludes ANSI codes)
	maxLen := 0
	for _, line := range lines {
		w := DisplayWidth(line)
		if w > maxLen {
			maxLen = w
		}
	}

	// Pad lines to same display width
	content := ""
	for i, line := range lines {
		padded := line
		w := DisplayWidth(line)
		if w < maxLen {
			padded = line + strings.Repeat(" ", maxLen-w)
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

	// Find max display width for consistent box width (excludes ANSI codes)
	maxLen := 0
	for _, line := range lines {
		w := DisplayWidth(line)
		if w > maxLen {
			maxLen = w
		}
	}

	// Pad lines to same display width
	content := ""
	for i, line := range lines {
		padded := line
		w := DisplayWidth(line)
		if w < maxLen {
			padded = line + strings.Repeat(" ", maxLen-w)
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
	if !isProgressTTY() {
		fmt.Fprintf(ProgressWriter, "%s (0/%d)\n", title, total)
		return &ProgressBar{total: total}
	}

	bar, _ := pterm.DefaultProgressbar.
		WithTotal(total).
		WithTitle(title).
		WithWriter(ProgressWriter).
		Start()
	return &ProgressBar{bar: bar, total: total}
}

// Increment increments progress by 1.
// When the bar reaches 100%, the title is replaced with "Done" so the
// final render does not freeze on the last item name.
func (p *ProgressBar) Increment() {
	if p.bar != nil {
		if p.bar.Current+1 >= p.bar.Total {
			p.UpdateTitle("Done")
		}
		p.bar.Increment()
	}
}

// Add increments progress by n.
func (p *ProgressBar) Add(n int) {
	if p.bar != nil {
		p.bar.Add(n)
	}
}

// UpdateTitle updates progress bar title. The title is padded or
// truncated to a fixed width so that the bar width stays stable.
func (p *ProgressBar) UpdateTitle(title string) {
	const maxWidth = 40
	display := title
	w := runewidth.StringWidth(display)
	if w > maxWidth {
		display = runewidth.Truncate(display, maxWidth, "...")
		w = runewidth.StringWidth(display)
	}
	if w < maxWidth {
		display += strings.Repeat(" ", maxWidth-w)
	}

	if p.bar != nil {
		p.bar.UpdateTitle(display)
	} else {
		fmt.Fprintf(ProgressWriter, "  %s\n", strings.TrimRight(display, " "))
	}
}

// Stop stops the progress bar. It's safe to call even if the bar already
// stopped itself (pterm auto-stops when Current >= Total).
func (p *ProgressBar) Stop() {
	if p.bar != nil && p.bar.IsActive {
		p.bar.Stop()
	}
}

// UpdateNotification prints a colorful update notification.
// upgradeCmd is the command the user should run (e.g. "brew upgrade skillshare"
// or "skillshare upgrade").
func UpdateNotification(currentVersion, latestVersion, upgradeCmd string) {
	if !IsTTY() {
		fmt.Printf("\n! Update available: %s -> %s\n", currentVersion, latestVersion)
		fmt.Printf("  Run '%s' to update\n", upgradeCmd)
		return
	}

	fmt.Println()

	// Build content lines
	lines := []string{
		"",
		fmt.Sprintf("  Version: %s -> %s", currentVersion, latestVersion),
		"",
		fmt.Sprintf("  Run: %s", upgradeCmd),
		"",
	}

	// Find max display width for consistent box width (excludes ANSI codes)
	maxLen := 0
	for _, line := range lines {
		w := DisplayWidth(line)
		if w > maxLen {
			maxLen = w
		}
	}

	// Pad lines to same display width
	content := ""
	for i, line := range lines {
		padded := line
		w := DisplayWidth(line)
		if w < maxLen {
			padded = line + strings.Repeat(" ", maxLen-w)
		}
		content += padded
		if i < len(lines)-1 {
			content += "\n"
		}
	}

	box := pterm.DefaultBox.
		WithTitle(pterm.Yellow("Update Available")).
		WithBoxStyle(pterm.NewStyle(pterm.FgYellow))
	box.Println(content)
}

// SyncSummary prints a sync summary line.
func SyncSummary(stats SyncStats) {
	OperationSummary("Sync", stats.Duration,
		Metric{Label: "targets", Count: stats.Targets, HighlightColor: pterm.Cyan},
		Metric{Label: "linked", Count: stats.Linked, HighlightColor: pterm.Green},
		Metric{Label: "local", Count: stats.Local, HighlightColor: pterm.Blue},
		Metric{Label: "updated", Count: stats.Updated, HighlightColor: pterm.Yellow},
		Metric{Label: "pruned", Count: stats.Pruned, HighlightColor: pterm.Yellow},
	)
}

// SyncStats holds statistics for sync summary
type SyncStats struct {
	Targets  int
	Linked   int
	Local    int
	Updated  int
	Pruned   int
	Duration time.Duration
}

// UpdateSummary prints an update summary line matching SyncSummary style.
func UpdateSummary(stats UpdateStats) {
	OperationSummary("Update", stats.Duration,
		Metric{Label: "updated", Count: stats.Updated, HighlightColor: pterm.Green},
		Metric{Label: "skipped", Count: stats.Skipped, HighlightColor: pterm.Yellow},
		Metric{Label: "pruned", Count: stats.Pruned, HighlightColor: pterm.Yellow},
	)
	if stats.SecurityFailed > 0 {
		Warning("Blocked: %d repo(s) by security audit", stats.SecurityFailed)
	}
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
	for _, m := range metrics {
		colorFn := pterm.Gray
		if m.Count > 0 && m.HighlightColor != nil {
			colorFn = m.HighlightColor
		}
		parts = append(parts, colorFn(fmt.Sprint(m.Count))+" "+m.Label)
	}
	line := strings.Join(parts, "  ")
	if duration > 0 {
		line += "  " + pterm.Gray(fmt.Sprintf("(%.1fs)", duration.Seconds()))
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
		fmt.Printf("  %s %-20s %s\n", style.Sprint(statusIcon), name, pterm.Gray(detail))
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
	StepBranch = "├"
	StepCorner = "└"
)

// TreeLine returns the tree continuation character (│) with TTY coloring
func TreeLine() string {
	if IsTTY() {
		return pterm.Gray(StepLine)
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

// StepStart prints the first step (with arrow)
func StepStart(label, value string) {
	if IsTTY() {
		fmt.Printf("%s  %-10s  %s\n", pterm.Yellow(StepArrow), pterm.LightCyan(label), pterm.Bold.Sprint(value))
	} else {
		fmt.Printf("%s  %s  %s\n", StepArrow, label, value)
	}
}

// StepContinue prints a middle step (with branch)
func StepContinue(label, value string) {
	if IsTTY() {
		fmt.Printf("%s\n", pterm.Gray(StepLine))
		fmt.Printf("%s %-10s  %s\n", pterm.Gray(StepBranch+"─"), pterm.Gray(label), pterm.White(value))
	} else {
		fmt.Printf("%s\n", StepLine)
		fmt.Printf("%s─ %s  %s\n", StepBranch, label, value)
	}
}

// StepResult prints the result as the final node of the tree
func StepResult(status, message string, duration time.Duration) {
	var icon string
	var style pterm.Style
	switch status {
	case "success":
		icon = StepCheck
		style = *pterm.NewStyle(pterm.FgGreen, pterm.Bold)
	case "error":
		icon = StepCross
		style = *pterm.NewStyle(pterm.FgRed, pterm.Bold)
	default:
		icon = "→"
		style = *pterm.NewStyle(pterm.FgYellow, pterm.Bold)
	}

	timeStr := ""
	if duration > 0 {
		timeStr = pterm.Gray(fmt.Sprintf(" (%.1fs)", duration.Seconds()))
	}

	if IsTTY() {
		fmt.Printf("%s\n", pterm.Gray(StepLine))
		fmt.Printf("%s %s %s  %s%s\n", pterm.Gray(StepCorner+"─"), style.Sprint(icon), style.Sprint(strings.ToUpper(status)), message, timeStr)
	} else {
		fmt.Printf("%s\n", StepLine)
		fmt.Printf("%s─ %s %s  %s%s\n", StepCorner, icon, strings.ToUpper(status), message, timeStr)
	}
}

// StepEnd prints the last step (with corner)
func StepEnd(label, value string) {
	if IsTTY() {
		fmt.Printf("%s\n", pterm.Gray(StepLine))
		fmt.Printf("%s %s  %s\n", pterm.Gray(StepCorner+"─"), pterm.White(label), value)
	} else {
		fmt.Printf("%s\n", StepLine)
		fmt.Printf("%s─ %s  %s\n", StepCorner, label, value)
	}
}

// TreeSpinner is a spinner that fits into tree structure
type TreeSpinner struct {
	spinner     *pterm.SpinnerPrinter
	start       time.Time
	isLast      bool
	lastUpdate  time.Time
	lastMessage string
}

// StartTreeSpinner starts a spinner in tree context
func StartTreeSpinner(message string, isLast bool) *TreeSpinner {
	prefix := StepBranch + "─"
	if isLast {
		prefix = StepCorner + "─"
	}

	if !IsTTY() {
		fmt.Printf("%s\n", StepLine)
		fmt.Printf("%s %s\n", prefix, message)
		return &TreeSpinner{start: time.Now(), isLast: isLast}
	}

	fmt.Printf("%s\n", pterm.Gray(StepLine))

	// Custom spinner with tree prefix
	s, _ := pterm.DefaultSpinner.
		WithRemoveWhenDone(true).
		Start(message)

	return &TreeSpinner{spinner: s, start: time.Now(), isLast: isLast}
}

// Success completes the tree spinner with success
func (ts *TreeSpinner) Success(message string) {
	elapsed := time.Since(ts.start)

	prefix := StepBranch + "─"
	if ts.isLast {
		prefix = StepCorner + "─"
	}

	if ts.spinner != nil {
		ts.spinner.Stop()
	}

	if IsTTY() {
		fmt.Printf("%s %s  %s\n", pterm.Gray(prefix), pterm.Green(message), pterm.Gray(fmt.Sprintf("(%.1fs)", elapsed.Seconds())))
	} else {
		fmt.Printf("%s %s (%.1fs)\n", prefix, message, elapsed.Seconds())
	}
}

// Fail completes the tree spinner with failure
func (ts *TreeSpinner) Fail(message string) {
	prefix := StepBranch + "─"
	if ts.isLast {
		prefix = StepCorner + "─"
	}

	if ts.spinner != nil {
		ts.spinner.Stop()
	}

	if IsTTY() {
		fmt.Printf("%s %s\n", pterm.Gray(prefix), pterm.Red(message))
	} else {
		fmt.Printf("%s %s\n", prefix, message)
	}
}

// Warn completes the tree spinner with a warning
func (ts *TreeSpinner) Warn(message string) {
	elapsed := time.Since(ts.start)

	prefix := StepBranch + "─"
	if ts.isLast {
		prefix = StepCorner + "─"
	}

	if ts.spinner != nil {
		ts.spinner.Stop()
	}

	if IsTTY() {
		fmt.Printf("%s %s  %s\n", pterm.Gray(prefix), pterm.Yellow(message), pterm.Gray(fmt.Sprintf("(%.1fs)", elapsed.Seconds())))
	} else {
		fmt.Printf("%s %s (%.1fs)\n", prefix, message, elapsed.Seconds())
	}
}

// Update updates the tree spinner text while running.
func (ts *TreeSpinner) Update(message string) {
	message, ok := normalizeSpinnerUpdate(message, ts.lastMessage, ts.lastUpdate)
	if !ok {
		return
	}
	ts.lastMessage = message
	ts.lastUpdate = time.Now()

	if ts.spinner != nil {
		ts.spinner.UpdateText(message)
		return
	}
	fmt.Printf("... %s\n", message)
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

// StepDone prints a completed step
func StepDone(label, value string) {
	if IsTTY() {
		fmt.Printf("%s %-10s %s\n", pterm.Green(StepCheck), pterm.White(label), value)
	} else {
		fmt.Printf("%s %-10s %s\n", StepCheck, label, value)
	}
}

// StepFail prints a failed step
func StepFail(label, value string) {
	if IsTTY() {
		styledLabel := pterm.White(label)
		if ansiRegex.MatchString(label) {
			styledLabel = label
		}
		fmt.Printf("%s %-10s %s\n", pterm.Red(StepCross), styledLabel, value)
	} else {
		fmt.Printf("%s %-10s %s\n", StepCross, label, value)
	}
}

// StepSkip prints a skipped step (yellow ⊘)
func StepSkip(label, value string) {
	if IsTTY() {
		fmt.Printf("%s %-10s %s\n", pterm.Yellow(StepSkipCh), pterm.White(label), value)
	} else {
		fmt.Printf("%s %-10s %s\n", StepSkipCh, label, value)
	}
}

// SkillBox prints a skill summary in a modern, compact card
func SkillBox(name, description, location string) {
	if !IsTTY() {
		fmt.Printf("\n── %s ──\n", name)
		if description != "" {
			fmt.Printf("  %s\n", description)
		}
		fmt.Printf("  Location: %s\n", location)
		return
	}

	var content strings.Builder

	if description != "" {
		wrapped := wrapText(description, 50)
		for _, line := range wrapped {
			content.WriteString(pterm.White("  " + line + "\n"))
		}
		content.WriteString("\n")
	} else {
		content.WriteString(pterm.Italic.Sprint("  Ready to use!\n\n"))
	}

	loc := location
	if loc == "" || loc == "." {
		loc = "(root)"
	}
	content.WriteString(pterm.Gray("  Location:  ") + pterm.LightCyan("skills/"+loc))

	pterm.DefaultBox.
		WithTitle(pterm.LightCyan(pterm.Bold.Sprint(name))).
		WithTitleTopLeft().
		WithBoxStyle(pterm.NewStyle(pterm.FgCyan)).
		WithPadding(1).
		Println(content.String())
}

// SkillBoxCompact prints a compact skill box (for multiple skills)
func SkillBoxCompact(name, location string) {
	loc := location
	if loc == "." {
		loc = "root"
	}

	if IsTTY() {
		if loc == "" {
			fmt.Printf("  %s %s\n", pterm.Cyan(StepBullet), pterm.White(name))
			return
		}
		fmt.Printf("  %s %s %s\n", pterm.Cyan(StepBullet), pterm.White(name), pterm.Gray("("+loc+")"))
	} else {
		if loc == "" {
			fmt.Printf("  %s %s\n", StepBullet, name)
			return
		}
		fmt.Printf("  %s %s (%s)\n", StepBullet, name, loc)
	}
}

// SkillDisplay holds skill info for display
type SkillDisplay struct {
	Name        string
	Description string
	Path        string
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

// SectionLabel prints a dim section label for visual grouping in batch output.
// Only used when the result set is large enough to benefit from sections (>10 items).
func SectionLabel(label string) {
	if IsTTY() {
		fmt.Printf("\n  %s\n", pterm.Gray(label))
	} else {
		fmt.Printf("\n- %s\n", label)
	}
}

// wrapText wraps text to specified width
func wrapText(text string, width int) []string {
	if len(text) <= width {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	currentLine := ""

	for _, word := range words {
		if currentLine == "" {
			currentLine = word
		} else if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}
