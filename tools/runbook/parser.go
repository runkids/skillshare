package main

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Runbook is the fully parsed representation of an E2E runbook Markdown file.
type Runbook struct {
	Meta  RunbookMeta
	Steps []Step
}

// parserState tracks the state machine position.
type parserState int

const (
	stateScanning  parserState = iota // outside any step
	stateInStep                       // inside a step, before/between code blocks
	stateInCode                       // inside a fenced code block
	stateInExpected                   // collecting expected bullet items
)

// stepHeadingRe matches step headings in multiple formats:
//   - ## Step 0: Title
//   - ### Step 1: Title
//   - ### 1. Title
//   - ### 1b. Title
//   - ### 1. Step 1: Title (double labeling)
var stepHeadingRe = regexp.MustCompile(
	`^#{2,3}\s+(?:Step\s+)?(\d+)[b-z]?[\.:]\s*(?:Step\s+\d+:\s*)?(.+)`,
)

// codeFenceOpenRe matches opening code fences like ```bash, ```sh, etc.
var codeFenceOpenRe = regexp.MustCompile("^```(\\w*)\\s*$")

// codeFenceCloseRe matches closing code fences.
var codeFenceCloseRe = regexp.MustCompile("^```\\s*$")

// heredocStartRe detects heredoc start: <<EOF, <<'EOF', <<"EOF", <<-EOF, etc.
// Captures the delimiter word (without quotes or leading dash).
var heredocStartRe = regexp.MustCompile(`<<-?\s*['"]?(\w+)['"]?`)

// sectionHeadingRe matches ## level headings for metadata sections.
var sectionHeadingRe = regexp.MustCompile(`^##\s+(.+)`)

// bulletRe matches list items (- or *).
var bulletRe = regexp.MustCompile(`^\s*[-*]\s+(.+)`)

// ParseRunbook reads a Markdown runbook and extracts metadata and steps.
func ParseRunbook(r io.Reader) (*Runbook, error) {
	scanner := bufio.NewScanner(r)
	rb := &Runbook{}

	state := stateScanning
	var currentStep *Step
	var codeLines []string
	var codeLang string
	var descLines []string
	var metaSection string // "scope", "env", or ""
	var metaLines []string
	inOptional := false
	stopParsing := false
	var heredocDelim string // non-empty when inside a heredoc

	flushStep := func() {
		if currentStep != nil {
			if len(descLines) > 0 {
				currentStep.Description = strings.TrimSpace(strings.Join(descLines, "\n"))
			}
			rb.Steps = append(rb.Steps, *currentStep)
			currentStep = nil
		}
		descLines = nil
	}

	flushMeta := func() {
		if metaSection == "" {
			return
		}
		text := strings.TrimSpace(strings.Join(metaLines, "\n"))
		switch metaSection {
		case "scope":
			rb.Meta.Scope = text
		case "env":
			rb.Meta.Env = text
		}
		metaSection = ""
		metaLines = nil
	}

	appendCode := func() {
		block := strings.TrimRight(strings.Join(codeLines, "\n"), "\n")
		if currentStep == nil {
			return
		}
		if currentStep.Command == "" {
			currentStep.Command = block
			currentStep.Lang = codeLang
		} else {
			currentStep.Command += "\n---\n" + block
		}
		codeLines = nil
		codeLang = ""
	}

	for scanner.Scan() {
		line := scanner.Text()

		if stopParsing {
			break
		}

		// --- State: inside code block ---
		if state == stateInCode {
			// Track heredoc boundaries so embedded ``` inside heredocs
			// don't terminate the code block prematurely.
			if heredocDelim != "" {
				// Inside heredoc — check for closing delimiter.
				if strings.TrimSpace(line) == heredocDelim {
					heredocDelim = ""
				}
				codeLines = append(codeLines, line)
				continue
			}

			if codeFenceCloseRe.MatchString(line) {
				appendCode()
				state = stateInStep
			} else {
				// Check if this line starts a heredoc.
				if m := heredocStartRe.FindStringSubmatch(line); m != nil {
					heredocDelim = m[1]
				}
				codeLines = append(codeLines, line)
			}
			continue
		}

		// --- Check for title (# heading) ---
		if strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
			rb.Meta.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			// Strip inline markdown from title (backticks)
			rb.Meta.Title = stripInlineMarkdown(rb.Meta.Title)
			continue
		}

		// --- Check for ## section headings ---
		if m := sectionHeadingRe.FindStringSubmatch(line); m != nil {
			heading := strings.TrimSpace(m[1])
			headingLower := strings.ToLower(heading)

			// Stop at pass criteria
			if strings.Contains(headingLower, "pass") && (strings.Contains(headingLower, "criteria") || strings.Contains(headingLower, "fail")) {
				flushStep()
				stopParsing = true
				continue
			}

			// Optional section — skip until next ## heading
			if strings.HasPrefix(headingLower, "optional") {
				flushMeta()
				inOptional = true
				continue
			}

			// Steps container heading — just skip
			if headingLower == "steps" {
				flushMeta()
				inOptional = false
				continue
			}

			// Scope section
			if headingLower == "scope" {
				flushMeta()
				inOptional = false
				metaSection = "scope"
				metaLines = nil
				continue
			}

			// Environment section
			if headingLower == "environment" {
				flushMeta()
				inOptional = false
				metaSection = "env"
				metaLines = nil
				continue
			}

			// Check if it's a step heading at ## level (e.g., "## Step 0: ...")
			if sm := stepHeadingRe.FindStringSubmatch(line); sm != nil {
				flushMeta()
				flushStep()
				inOptional = false
				num, _ := strconv.Atoi(sm[1])
				currentStep = &Step{
					Number: num,
					Title:  strings.TrimSpace(sm[2]),
				}
				descLines = nil
				state = stateInStep
				continue
			}

			// Other ## headings — end optional, flush meta
			flushMeta()
			inOptional = false
			continue
		}

		// Skip content in optional sections
		if inOptional {
			continue
		}

		// --- Check for ### step headings ---
		if sm := stepHeadingRe.FindStringSubmatch(line); sm != nil {
			flushMeta()
			flushStep()
			num, _ := strconv.Atoi(sm[1])
			currentStep = &Step{
				Number: num,
				Title:  strings.TrimSpace(sm[2]),
			}
			descLines = nil
			state = stateInStep
			continue
		}

		// --- Collecting meta section content ---
		if metaSection != "" {
			metaLines = append(metaLines, line)
			continue
		}

		// --- State: in step ---
		if state == stateInStep || state == stateInExpected {
			// Check for code fence opening
			if cm := codeFenceOpenRe.FindStringSubmatch(line); cm != nil {
				if state == stateInExpected {
					state = stateInStep
				}
				codeLang = cm[1]
				if codeLang == "" {
					codeLang = "bash"
				}
				codeLines = nil
				state = stateInCode
				continue
			}

			// Check for Expected label
			if isExpectedLabel(line) {
				state = stateInExpected
				continue
			}

			// Collecting expected items
			if state == stateInExpected {
				if bm := bulletRe.FindStringSubmatch(line); bm != nil {
					item := stripInlineMarkdown(strings.TrimSpace(bm[1]))
					if currentStep != nil {
						currentStep.Expected = append(currentStep.Expected, item)
					}
					continue
				}
				// Non-bullet, non-empty lines end Expected collection
				// (but blank lines are allowed between items)
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				// Indented continuation of a bullet (e.g., "  - YAML includes:\n    - ...")
				// These are sub-items; we skip them to keep assertions flat.
				if strings.HasPrefix(line, "  ") && !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "*") {
					continue
				}
				state = stateInStep
				// Fall through to description collection
			}

			// Collect description lines (text between heading and first code block)
			if state == stateInStep {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && currentStep != nil && currentStep.Command == "" {
					descLines = append(descLines, line)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	// Flush any remaining step
	flushStep()

	return rb, nil
}

// isExpectedLabel detects "Expected:", "**Expected:**", "**Expected**:", etc.
func isExpectedLabel(s string) bool {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "*", "")
	s = strings.TrimRight(s, ":")
	s = strings.TrimSpace(s)
	return strings.EqualFold(s, "expected")
}

// stripInlineMarkdown removes backticks and bold markers from text.
func stripInlineMarkdown(s string) string {
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "**", "")
	return s
}
