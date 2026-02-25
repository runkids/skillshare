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

// displayWidth returns the visible width of a string (excluding ANSI codes, handling wide chars)
func displayWidth(s string) int {
	// Remove ANSI codes first, then calculate Unicode-aware width
	clean := ansiRegex.ReplaceAllString(s, "")
	return runewidth.StringWidth(clean)
}

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

	// Find max display width for consistent box width (excludes ANSI codes)
	maxLen := 0
	for _, line := range lines {
		w := displayWidth(line)
		if w > maxLen {
			maxLen = w
		}
	}

	// Pad lines to same display width
	content := ""
	for i, line := range lines {
		padded := line
		w := displayWidth(line)
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
	if !IsTTY() {
		fmt.Printf("%s\n%s\n", command, subtitle)
		return
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
	if !IsTTY() {
		fmt.Printf("... %s\n", message)
		return &Spinner{start: time.Now()}
	}

	s, _ := pterm.DefaultSpinner.
		WithRemoveWhenDone(true).
		Start(message)
	return &Spinner{spinner: s, start: time.Now()}
}

// StartSpinnerWithSteps starts a spinner that shows step progress
func StartSpinnerWithSteps(message string, totalSteps int) *Spinner {
	if !IsTTY() {
		fmt.Printf("... [1/%d] %s\n", totalSteps, message)
		return &Spinner{start: time.Now(), currentStep: 1, totalSteps: totalSteps}
	}

	stepPrefix := fmt.Sprintf("[1/%d] ", totalSteps)
	s, _ := pterm.DefaultSpinner.Start(stepPrefix + message)
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
			fmt.Printf("... [%d/%d] %s\n", s.currentStep, s.totalSteps, message)
		} else {
			fmt.Printf("... %s\n", message)
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
		s.spinner.Success(msg)
	} else {
		fmt.Printf("✓ %s\n", msg)
	}
}

// Fail stops spinner with failure (red)
func (s *Spinner) Fail(message string) {
	if s.spinner != nil {
		s.spinner.Fail(message)
	} else {
		fmt.Printf("✗ %s\n", message)
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
		s.spinner.Warning(msg)
	} else {
		fmt.Printf("! %s\n", msg)
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

	// Find max display width for consistent box width (excludes ANSI codes)
	maxLen := 0
	for _, line := range lines {
		w := displayWidth(line)
		if w > maxLen {
			maxLen = w
		}
	}

	// Pad lines to same display width
	content := ""
	for i, line := range lines {
		padded := line
		w := displayWidth(line)
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
		w := displayWidth(line)
		if w > maxLen {
			maxLen = w
		}
	}

	// Pad lines to same display width
	content := ""
	for i, line := range lines {
		padded := line
		w := displayWidth(line)
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

// Increment increments progress by 1.
func (p *ProgressBar) Increment() {
	if p.bar != nil {
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
	// Truncate long titles with ellipsis
	display := title
	if len(display) > maxWidth {
		display = display[:maxWidth-3] + "..."
	}
	// Pad to fixed width to prevent bar width from changing
	if len(display) < maxWidth {
		display += strings.Repeat(" ", maxWidth-len(display))
	}

	if p.bar != nil {
		p.bar.UpdateTitle(display)
	} else {
		fmt.Printf("  %s\n", strings.TrimRight(display, " "))
	}
}

// Stop stops the progress bar. It's safe to call even if the bar already
// stopped itself (pterm auto-stops when Current >= Total).
func (p *ProgressBar) Stop() {
	if p.bar != nil && p.bar.IsActive {
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

	// Build content lines
	lines := []string{
		"",
		fmt.Sprintf("  Version: %s -> %s", currentVersion, latestVersion),
		"",
		"  Run: skillshare upgrade",
		"",
	}

	// Find max display width for consistent box width (excludes ANSI codes)
	maxLen := 0
	for _, line := range lines {
		w := displayWidth(line)
		if w > maxLen {
			maxLen = w
		}
	}

	// Pad lines to same display width
	content := ""
	for i, line := range lines {
		padded := line
		w := displayWidth(line)
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

// SyncSummary prints a beautiful sync summary box
func SyncSummary(stats SyncStats) {
	if !IsTTY() {
		fmt.Printf("\n─── Sync Complete ───\n")
		fmt.Printf("  Targets: %d  Linked: %d  Local: %d  Updated: %d  Pruned: %d\n",
			stats.Targets, stats.Linked, stats.Local, stats.Updated, stats.Pruned)
		if stats.Duration > 0 {
			fmt.Printf("  Duration: %.1fs\n", stats.Duration.Seconds())
		}
		return
	}

	// Build stats line with colors
	statsLine := fmt.Sprintf(
		"  %s targets  %s linked  %s local  %s updated  %s pruned",
		pterm.Cyan(fmt.Sprint(stats.Targets)),
		pterm.Green(fmt.Sprint(stats.Linked)),
		pterm.Blue(fmt.Sprint(stats.Local)),
		pterm.Yellow(fmt.Sprint(stats.Updated)),
		pterm.Gray(fmt.Sprint(stats.Pruned)),
	)

	var durationLine string
	if stats.Duration > 0 {
		durationLine = fmt.Sprintf("  Completed in %s", pterm.Gray(fmt.Sprintf("%.1fs", stats.Duration.Seconds())))
	}

	// Build content with proper padding
	lines := []string{"", statsLine}
	if durationLine != "" {
		lines = append(lines, durationLine)
	}
	lines = append(lines, "")

	// Find max display width
	maxLen := 0
	for _, line := range lines {
		w := displayWidth(line)
		if w > maxLen {
			maxLen = w
		}
	}

	// Pad lines
	var content strings.Builder
	for i, line := range lines {
		padded := line
		w := displayWidth(line)
		if w < maxLen {
			padded = line + strings.Repeat(" ", maxLen-w)
		}
		content.WriteString(padded)
		if i < len(lines)-1 {
			content.WriteString("\n")
		}
	}

	box := pterm.DefaultBox.
		WithTitle(pterm.Green("✓ Sync Complete")).
		WithBoxStyle(pterm.NewStyle(pterm.FgGreen))
	box.Println(content.String())
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
