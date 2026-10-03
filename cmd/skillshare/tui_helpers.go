package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"skillshare/internal/config"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
)

// shouldLaunchTUI determines whether to launch interactive TUI.
// Priority: --no-tui flag (force off) > config tui field > default (true).
// Pass cfg if already loaded; pass nil to auto-load best-effort.
func shouldLaunchTUI(noTUI bool, cfg *config.Config) bool {
	if noTUI {
		return false
	}
	if !ui.IsTTY() {
		return false
	}
	if cfg != nil {
		return cfg.IsTUIEnabled()
	}
	c, err := config.Load()
	if err != nil {
		return true // config missing/broken → default to TUI enabled
	}
	return c.IsTUIEnabled()
}

// wrapAndScroll hard-wraps content to width then applies vertical scrolling.
// Returns (visible content, scroll info). Scroll indicator is returned separately
// so it doesn't consume panel height — prevents bottom content from being clipped by MaxHeight.
func wrapAndScroll(content string, width, detailScroll, viewHeight int) (string, string) {
	content = hardWrapContent(content, width)
	return applyDetailScrollSplit(content, detailScroll, viewHeight)
}

// applyDetailScrollSplit applies scrolling and returns (visible content, scroll info).
func applyDetailScrollSplit(content string, detailScroll, viewHeight int) (string, string) {
	lines := strings.Split(content, "\n")

	maxDetailLines := viewHeight
	if maxDetailLines < 5 {
		maxDetailLines = 5
	}

	totalLines := len(lines)
	if totalLines <= maxDetailLines {
		return content, ""
	}

	maxScroll := totalLines - maxDetailLines
	offset := min(detailScroll, maxScroll)

	end := min(offset+maxDetailLines, totalLines)
	visible := lines[offset:end]

	scrollInfo := fmt.Sprintf("(%d/%d)", offset+1, maxScroll+1)
	return strings.Join(visible, "\n"), scrollInfo
}

// formatDurationShort returns a compact human-readable duration string.
// Covers: just now, minutes, hours, days, months, years.
func formatDurationShort(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		months := int(d.Hours() / 24 / 30)
		if months < 12 {
			return fmt.Sprintf("%dmo", months)
		}
		return fmt.Sprintf("%dy", months/12)
	}
}

// Minimum terminal width for horizontal split; below this use vertical layout.
const tuiMinSplitWidth = 80

// Minimum terminal width for the audit and log split panels, which need less
// room than tuiMinSplitWidth; below this they use a vertical layout.
const tuiNarrowSplitWidth = 70

// truncateStr truncates a string to maxLen, appending "..." if needed.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// newTUIFilterInput builds the "/ " filter input shared by all TUIs.
// Pass "" for no placeholder.
func newTUIFilterInput(placeholder string) textinput.Model {
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = theme.Accent()
	fi.Cursor.Style = theme.Accent()
	fi.Placeholder = placeholder
	return fi
}

// handleTUIFilterKey handles a key press while the filter input is focused.
// esc clears the filter and re-applies it; enter locks the filter in without
// re-applying; any other key goes to the input and re-applies only when the
// value changed.
func handleTUIFilterKey(msg tea.KeyMsg, filtering *bool, text *string, input *textinput.Model, apply func()) tea.Cmd {
	switch msg.String() {
	case "esc":
		*filtering = false
		*text = ""
		input.SetValue("")
		apply()
		return nil
	case "enter":
		*filtering = false
		return nil
	}
	var cmd tea.Cmd
	*input, cmd = input.Update(msg)
	if newVal := input.Value(); newVal != *text {
		*text = newVal
		apply()
	}
	return cmd
}
