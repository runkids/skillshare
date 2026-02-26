package ui

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"atomicgo.dev/cursor"
	"github.com/pterm/pterm"
)

// ProgressBar is a thread-safe progress bar with optional MaxWidth.
// It replaces pterm's ProgressbarPrinter to avoid race conditions
// from concurrent Increment() calls and to support width constraints.
type ProgressBar struct {
	current  atomic.Int64
	total    int
	title    string
	maxWidth int // 0 = terminal width
	start    time.Time
	isTTY    bool

	mu      sync.Mutex
	stopped bool
}

// StartProgress starts a progress bar. maxWidth 0 means full terminal width.
func StartProgress(title string, total int) *ProgressBar {
	return StartProgressWithMaxWidth(title, total, 0)
}

// StartProgressWithMaxWidth starts a progress bar constrained to maxWidth columns.
func StartProgressWithMaxWidth(title string, total int, maxWidth int) *ProgressBar {
	if !IsTTY() {
		fmt.Printf("%s (0/%d)\n", title, total)
		return &ProgressBar{total: total, title: title}
	}

	cursor.Hide()
	p := &ProgressBar{
		total:    total,
		title:    title,
		maxWidth: maxWidth,
		start:    time.Now(),
		isTTY:    true,
	}
	p.render()
	return p
}

// Increment increments progress by 1. Safe for concurrent use.
func (p *ProgressBar) Increment() {
	p.current.Add(1)
	if p.isTTY {
		p.render()
	}
}

// Add increments progress by n. Safe for concurrent use.
func (p *ProgressBar) Add(n int) {
	p.current.Add(int64(n))
	if p.isTTY {
		p.render()
	}
}

// UpdateTitle updates the progress bar title.
func (p *ProgressBar) UpdateTitle(title string) {
	const maxTitleWidth = 40
	display := title
	if len(display) > maxTitleWidth {
		display = display[:maxTitleWidth-3] + "..."
	}

	p.mu.Lock()
	p.title = display
	p.mu.Unlock()

	if p.isTTY {
		p.render()
	} else {
		fmt.Printf("  %s\n", strings.TrimRight(display, " "))
	}
}

// Stop stops the progress bar, rendering the final state with N/N.
func (p *ProgressBar) Stop() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true
	p.mu.Unlock()

	if !p.isTTY {
		return
	}

	// Force to total and render final frame.
	p.current.Store(int64(p.total))
	p.render()
	fmt.Println()
	cursor.Show()
}

// render draws the progress bar on the current line using \r.
func (p *ProgressBar) render() {
	cur := int(p.current.Load())
	if cur > p.total {
		cur = p.total
	}
	total := p.total
	if total == 0 {
		return
	}

	p.mu.Lock()
	title := p.title
	p.mu.Unlock()

	width := pterm.GetTerminalWidth()
	if p.maxWidth > 0 && p.maxWidth < width {
		width = p.maxWidth
	}

	// Format: title [cur/total] ████░░░░ pct% | elapsed
	pct := int(math.Round(float64(cur) / float64(total) * 100))
	elapsed := time.Since(p.start).Round(time.Second)

	padding := 1 + int(math.Log10(float64(total)))
	counter := fmt.Sprintf("[%0*d/%d]", padding, cur, total)
	pctStr := fmt.Sprintf("%3d%%", pct)
	timeStr := fmt.Sprintf("| %s", elapsed)

	// Colorize parts.
	titleC := Cyan + title + Reset
	counterC := Gray + "[" + White + fmt.Sprintf("%0*d", padding, cur) + Gray + "/" + White + fmt.Sprintf("%d", total) + Gray + "]" + Reset
	pctC := pctColor(pct) + pctStr + Reset

	// Calculate bar width: total - title - counter - pct - time - spaces.
	fixedLen := len(title) + 1 + len(counter) + 1 + 1 + len(pctStr) + 1 + len(timeStr)
	barWidth := width - fixedLen
	if barWidth < 5 {
		barWidth = 5
	}

	filled := barWidth * cur / total
	empty := barWidth - filled
	barC := Cyan + strings.Repeat("█", filled) + Gray + strings.Repeat("░", empty) + Reset

	line := fmt.Sprintf("%s %s %s %s %s", titleC, counterC, barC, pctC, timeStr)

	fmt.Printf("\r%-*s", width, line)
}

// pctColor returns a color code fading from red (0%) to green (100%).
func pctColor(pct int) string {
	if pct >= 100 {
		return Green
	}
	if pct >= 50 {
		return Yellow
	}
	return Red
}
