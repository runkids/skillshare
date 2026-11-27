// Package contextrouter measures an agents-context-router setup: the kernel
// file and the topics a folder's wiki/ai-context.json declares, in the bytes
// the router's own check prints. It reads; it never runs the router script.
//
// The size rule mirrors skills/agents-context-router/scripts/ai-context.py at
// ReferenceCommit. Functions that mirror one of its functions name it.
package contextrouter

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// normalizeNewlines turns \r\n and lone \r into \n, as Python's text mode does
// before the router counts anything. ok is false when b is not valid UTF-8.
func normalizeNewlines(b []byte) (string, bool) {
	if !utf8.Valid(b) {
		return "", false
	}
	s := string(b)
	if !strings.Contains(s, "\r") {
		return s, true
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n"), true
}

// pyIsSpace is Python's str.isspace for one rune: Unicode whitespace plus the
// four separators 0x1C-0x1F that Go's unicode.IsSpace leaves out.
func pyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// pyTrim is Python's str.strip() with no argument.
func pyTrim(s string) string {
	return strings.TrimFunc(s, pyIsSpace)
}

// frontmatterEnd mirrors frontmatter_end: the line after a closed leading
// frontmatter envelope, or 0 when there is none.
func frontmatterEnd(lines []string) int {
	if len(lines) == 0 || strings.TrimRight(strings.TrimLeft(lines[0], "\ufeff"), " \t\r") != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if end := strings.TrimRight(lines[i], " \t\r"); end == "---" || end == "..." {
			return i + 1
		}
	}
	return 0
}

// fenceMark mirrors FENCE: up to three whitespace characters, then a run of at
// least three backticks or tildes. It returns the run.
func fenceMark(line string) string {
	rest := line
	for i := 0; i < 3; i++ {
		r, size := utf8.DecodeRuneInString(rest)
		if size == 0 || !pyIsSpace(r) {
			break
		}
		rest = rest[size:]
	}
	if rest == "" || (rest[0] != '`' && rest[0] != '~') {
		return ""
	}
	n := 0
	for n < len(rest) && rest[n] == rest[0] {
		n++
	}
	if n < 3 {
		return ""
	}
	return rest[:n]
}

// headingLine mirrors HEAD: ^(#{1,6})\s+(.+?)\s*#*\s*$ with Python's \s.
func headingLine(line string) (level int, text string, ok bool) {
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level < 1 || level > 6 {
		return 0, "", false
	}
	rest := line[level:]
	body := strings.TrimLeftFunc(rest, pyIsSpace)
	if len(body) == len(rest) || body == "" {
		return 0, "", false
	}
	// The lazy group keeps at least one character; the tail gives up trailing
	// whitespace, then a run of #, then the whitespace before it.
	_, size := utf8.DecodeRuneInString(body)
	tail := body[size:]
	tail = strings.TrimRightFunc(tail, pyIsSpace)
	tail = strings.TrimRight(tail, "#")
	tail = strings.TrimRightFunc(tail, pyIsSpace)
	return level, body[:size] + tail, true
}

type heading struct {
	line, level int
	text        string
}

// headings mirrors headings: every heading outside fenced code, from line start.
func headings(lines []string, start int) []heading {
	var out []heading
	fence := ""
	for i, ln := range lines {
		if i < start {
			continue
		}
		if mark := fenceMark(ln); mark != "" {
			if fence == "" {
				fence = mark
			} else if strings.HasPrefix(mark, fence) {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if level, text, ok := headingLine(ln); ok {
			out = append(out, heading{i, level, text})
		}
	}
	return out
}

type sectionOpts struct {
	Heading     *string
	Strip       bool // drop a leading frontmatter envelope
	Frontmatter bool // skip the envelope when looking for headings
}

// section mirrors section: one source's body. hits is how many headings
// matched; anything but 1 means the source cannot be loaded. A whole-file
// source always reports 1.
func section(text string, o sectionOpts) (body string, hits int) {
	lines := strings.Split(text, "\n")
	if o.Heading == nil {
		from := 0
		if o.Strip {
			from = frontmatterEnd(lines)
		}
		return pyTrim(strings.Join(lines[from:], "\n")), 1
	}
	from := 0
	if o.Strip || o.Frontmatter {
		from = frontmatterEnd(lines)
	}
	hs := headings(lines, from)
	start, level := -1, 0
	for _, h := range hs {
		if h.text == *o.Heading {
			hits++
			start, level = h.line, h.level
		}
	}
	if hits != 1 {
		return "", hits
	}
	end := len(lines)
	for _, h := range hs {
		if h.line > start && h.level <= level {
			end = h.line
			break
		}
	}
	return pyTrim(strings.Join(lines[start:end], "\n")), 1
}
