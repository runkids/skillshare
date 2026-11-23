package utils

import (
	"bufio"
	"bytes"
	"os"
	"strings"
)

// utf8BOM is the byte order mark that Windows editors write at the start of a file.
var utf8BOM = []byte("\xef\xbb\xbf")

// SplitBOM cuts a leading UTF-8 BOM off s, so a caller that rewrites the file can put it back.
func SplitBOM(s string) (bom, rest string) {
	rest = strings.TrimPrefix(s, string(utf8BOM))
	return s[:len(s)-len(rest)], rest
}

// TrimBOM returns s without a leading UTF-8 BOM.
func TrimBOM(s string) string {
	_, rest := SplitBOM(s)
	return rest
}

// frontmatterPolicy is the rule one reader uses to find the --- delimiters. The readers
// do not agree on it (issue #449), so each difference is a field here and each reader
// names its policy below.
type frontmatterPolicy struct {
	// dashGuard gives up unless the content, after leading whitespace, starts with ---.
	// The delimiter itself may still be a later line.
	dashGuard bool
	// firstLine requires the opening delimiter to be the first line. Otherwise the first
	// delimiter line anywhere in the content opens the block.
	firstLine         bool
	leadingBlanksOnly bool // line editors allow only blank lines before an opening delimiter
	// crlf accepts a CR only when it is followed by LF.
	crlf bool
	// delim reports whether a line, without its "\n", is a delimiter.
	delim func(line []byte) bool
}

var (
	// Flag and scalar line editors accept leading blank lines, but not prose.
	lineEditorBlock = frontmatterPolicy{leadingBlanksOnly: true, delim: func(line []byte) bool {
		return string(bytes.TrimRight(line, " \t\r")) == "---"
	}}

	// strictBlock is the Agent Skills format: exact delimiters, starting on line one.
	strictBlock = frontmatterPolicy{firstLine: true, delim: delimExact}

	// lenientBlock takes the first two lines that are --- after trimming all whitespace,
	// wherever they are. ParseSkillName, ParseFrontmatterList, ParseFrontmatterListFromBytes,
	// ParseFrontmatterFields, ParseFrontmatterField.
	lenientBlock = frontmatterPolicy{delim: delimTrimmed}

	// bodyBlock is lenientBlock for content that starts with ---. ReadSkillBody.
	bodyBlock = frontmatterPolicy{dashGuard: true, delim: delimTrimmed}

	// rewriteBlock allows trailing spaces and tabs and CRLF but no indent, so that an
	// indented --- inside a YAML block scalar stays content. splitFrontmatterAndBody.
	rewriteBlock = frontmatterPolicy{dashGuard: true, crlf: true, delim: delimColumn0}
)

func delimExact(line []byte) bool   { return string(bytes.TrimRight(line, "\r")) == "---" }
func delimTrimmed(line []byte) bool { return string(bytes.TrimSpace(line)) == "---" }
func delimColumn0(line []byte) bool { return string(bytes.TrimRight(line, " \t")) == "---" }

// frontmatterBlock is where the frontmatter sits in the content. raw and body point into
// the content; nothing is copied.
type frontmatterBlock struct {
	open                bool   // an opening delimiter was found
	openLine, closeLine int    // zero-based physical lines when present
	closed              bool   // and a closing one
	raw                 []byte // the lines between the delimiters; up to the end of the content when unclosed
	body                []byte // everything after the closing delimiter line; nil when unclosed
}

// withoutLastNewline returns raw without the newline before the closing delimiter, which
// is what the readers that used to split the content into lines decode. A block scalar
// on the last line loses its final newline that way; ParseFrontmatterMap decodes raw and
// keeps it.
func (b frontmatterBlock) withoutLastNewline() []byte {
	if b.closed {
		return bytes.TrimSuffix(b.raw, []byte("\n"))
	}
	return b.raw
}

// lineKind is where one line sits relative to the frontmatter block.
type lineKind int

const (
	lineBefore  lineKind = iota // before the opening delimiter
	lineNoBlock                 // the policy rules out a block: stop
	lineOpen                    // the opening delimiter
	lineInside                  // a line of the block
	lineClose                   // the closing delimiter
)

// blockWalk applies a policy's line rule to one line after another. It is the single
// place that decides which line opens and which closes the block.
type blockWalk struct {
	p    frontmatterPolicy
	open bool
}

// next classifies line, given without its "\n".
func (w *blockWalk) next(line []byte) lineKind {
	isDelim := w.p.delim(line)
	switch {
	case w.open && isDelim:
		return lineClose
	case w.open:
		return lineInside
	case isDelim:
		w.open = true
		return lineOpen
	case w.p.firstLine || (w.p.leadingBlanksOnly && len(bytes.TrimSpace(line)) != 0):
		return lineNoBlock
	}
	return lineBefore
}

// locateFrontmatter finds the frontmatter block under the given policy. It walks the
// lines only as far as the closing delimiter and allocates nothing, so a large body
// costs no memory. Whether an unclosed block counts is the caller's decision.
func locateFrontmatter(content []byte, p frontmatterPolicy) frontmatterBlock {
	content = bytes.TrimPrefix(content, utf8BOM)
	if p.dashGuard && !bytes.HasPrefix(bytes.TrimSpace(content), []byte("---")) {
		return frontmatterBlock{}
	}

	w := blockWalk{p: p}
	start, pos, lineNo, openLine := -1, 0, 0, -1
	for line := range bytes.Lines(content) {
		next := pos + len(line)
		lineText := bytes.TrimSuffix(line, []byte("\n"))
		if p.crlf && bytes.HasSuffix(line, []byte("\r\n")) {
			lineText = bytes.TrimSuffix(lineText, []byte("\r"))
		}
		switch w.next(lineText) {
		case lineClose:
			return frontmatterBlock{open: true, closed: true, openLine: openLine, closeLine: lineNo, raw: content[start:pos], body: content[next:]}
		case lineOpen:
			start, openLine = next, lineNo
		case lineNoBlock:
			return frontmatterBlock{}
		}
		pos = next
		lineNo++
	}
	if start < 0 {
		return frontmatterBlock{}
	}
	return frontmatterBlock{open: true, openLine: openLine, raw: content[start:]}
}

// scanLenientBlock calls fn with each line of a file's lenientBlock, without the
// delimiters, until the block ends or fn returns false. It is how the readers that take a
// path read: line by line through a default bufio.Scanner (see scanLines), keeping
// nothing, so they can stop at the key they want. A failed read ends the lines early;
// only a failed open is an error. The line passed to fn is only valid during the call.
func scanLenientBlock(path string, fn func(line []byte) bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := blockWalk{p: lenientBlock}
	scanner := bufio.NewScanner(f)
	for first := true; scanner.Scan(); first = false {
		line := scanner.Bytes()
		if first {
			line = bytes.TrimPrefix(line, utf8BOM)
		}
		switch w.next(line) {
		case lineClose, lineNoBlock:
			return nil
		case lineInside:
			if !fn(line) {
				return nil
			}
		}
	}
	return nil
}

// readLenientBlock returns the lines of a file's lenientBlock joined with "\n", which is
// the YAML the list and fields readers have always decoded.
func readLenientBlock(path string) ([]byte, error) {
	var raw bytes.Buffer
	n := 0
	err := scanLenientBlock(path, func(line []byte) bool {
		if n++; n > 1 {
			raw.WriteByte('\n')
		}
		raw.Write(line)
		return true
	})
	return raw.Bytes(), err
}

// scanLines returns data as a default bufio.Scanner delivers it: the lines joined with
// "\n", one trailing "\r" dropped per line, ending without an error before the first line
// of bufio.MaxScanTokenSize bytes or more. The readers that take a path have always read
// this way; the ones that take bytes see the content as it is.
func scanLines(data []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(data))
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for n := 0; scanner.Scan(); n++ {
		if n > 0 {
			out.WriteByte('\n')
		}
		out.Write(scanner.Bytes())
	}
	return out.Bytes()
}
