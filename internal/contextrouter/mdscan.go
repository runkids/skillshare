package contextrouter

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
)

// This file is a port of the Markdown scanners in the router's ai-context.py
// at Reference: the block scanner that masks code, HTML and reference
// definitions (markdown_text) and the inline scanner that finds real links
// (link_destinations in span mode). compactPathLinks rewrites only the links
// these scanners recognize, so the port follows the reference function for
// function rather than a Markdown specification. testdata/compact.json holds
// what the reference returns; a difference there is a bug here.
//
// Offsets are bytes. Every character the scanners test is ASCII, so stepping
// through a multi-byte character byte by byte never matches anything. Lengths
// the reference compares with a limit are counted in characters.

// pySpace is Python's \s for str patterns; RE2's \s is ASCII only.
const pySpace = `[\s\x0b\x1c-\x1f\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

const (
	titlePattern   = `(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|\((?:\\.|[^)\\])*\))`
	htmlAttribute  = `[A-Za-z_:][A-Za-z0-9_.:-]*(?:[ \t\r\n]*=[ \t\r\n]*(?:[^ \t\r\n"'=<>` + "`" + `]+|'[^']*'|"[^"]*"))?`
	htmlTagPattern = `</[A-Za-z][A-Za-z0-9-]*[ \t\r\n]*>|<[A-Za-z][A-Za-z0-9-]*(?:[ \t\r\n]+` + htmlAttribute + `)*[ \t\r\n]*/?>`
	punctuation    = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
)

var (
	linkFenceRe = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	blockHTMLRe = regexp.MustCompile(`(?i)^ {0,3}</?(?:address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h[1-6]|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|noframes|ol|optgroup|option|p|param|search|section|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)(?:` + pySpace + `|/?>|$)`)
	thematicRe  = regexp.MustCompile(`^ {0,3}(?:(?:\*[ \t]*){3,}|(?:-[ \t]*){3,}|(?:_[ \t]*){3,})\z`)
	listMarkRe  = regexp.MustCompile(`^ {0,3}(?:[-+*]|[0-9]{1,9}[.)])(?:[ \t]+|$)`)
	listHeadRe  = regexp.MustCompile(`^ {0,3}(?:[-+*]|[0-9]{1,9}[.)])`)
	orderedRe   = regexp.MustCompile(`^([0-9]{1,9})[.)]`)
	blockHeadRe = regexp.MustCompile(`^ {0,3}#{1,6}(?:[ \t]|$)`)
	setextRe    = regexp.MustCompile(`^ {0,3}(?:=+|-+)[ \t]*\z`)
	rawStartRe  = regexp.MustCompile(`(?i)^ {0,3}<(pre|script|style|textarea)(?:` + pySpace + `|>|$)`)
	declStartRe = regexp.MustCompile(`^ {0,3}(?:<!--|<\?|<![A-Z]|<!\[CDATA\[)`)
	htmlTagFull = regexp.MustCompile(`^(?:` + htmlTagPattern + `)\z`)
	quoteRunRe  = regexp.MustCompile(`^(?: {0,3}>[ \t]?)+`)
	quoteOneRe  = regexp.MustCompile(`^ {0,3}>[ \t]?`)
	quoteGapRe  = regexp.MustCompile(`^((?: {0,3}>[ \t]?)+)([ \t]+)>`)
	leadQuoteRe = regexp.MustCompile(`^[ \t]*>`)
	blankLineRe = regexp.MustCompile(`\n[ \t]*\n`)
	inlineEndRe = regexp.MustCompile(`^(?:` + pySpace + `+` + titlePattern + `)?` + pySpace + `*\)`)
	autolinkRe  = regexp.MustCompile("^<(?:[A-Za-z][A-Za-z0-9+.-]{1,31}:[^<>\\x00-\\x20]*|[A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Za-z0-9.-]+)>")
	// The reference spells the comment as <!--(?:(?!-->)[\s\S])*-->, which is the lazy form.
	inlineHTMLRe = regexp.MustCompile(`^(?:<!--[\s\S]*?-->|<\?[\s\S]*?\?>|<![A-Z][\s\S]*?>|<!\[CDATA\[[\s\S]*?\]\]>|` + htmlTagPattern + `)`)
	defStartRe   = regexp.MustCompile(`^ {0,3}\[((?:\\.|[^\]\\])+)\]:`)
	// The reference's (?=[ \t\n]) lookahead: the gap before a title is not empty.
	defEndRe     = regexp.MustCompile(`^(?:(?:[ \t]+(?:\n[ \t]*)?|\n[ \t]*)` + titlePattern + `)?[ \t]*(?:\n|$)`)
	escapedAnyRe = regexp.MustCompile(`\\.`)
	labelSpaceRe = regexp.MustCompile(`[ \t\r\n]+`)
	rawCloseRes  = map[string]*regexp.Regexp{}
)

func init() {
	for _, tag := range []string{"pre", "script", "style", "textarea"} {
		rawCloseRes[tag] = regexp.MustCompile(`(?i)</` + tag + pySpace + `*>`)
	}
}

func isPunctuation(c byte) bool { return strings.IndexByte(punctuation, c) >= 0 }

func pyTrimLeft(s string) string { return strings.TrimLeftFunc(s, pyIsSpace) }

func isBlank(s string) bool { return pyTrim(s) == "" }

// blank masks a line: every byte but a newline becomes a space, so offsets hold.
func blank(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c != '\n' {
			out[i] = ' '
		}
	}
	return string(out)
}

// expandTabs is str.expandtabs(4) for text without newlines.
func expandTabs(s string) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var out strings.Builder
	column := 0
	for _, r := range s {
		if r == '\t' {
			n := 4 - column%4
			out.WriteString(strings.Repeat(" ", n))
			column += n
			continue
		}
		out.WriteRune(r)
		column++
	}
	return out.String()
}

// containerLeadEnd mirrors CONTAINER_LEAD: the leading run of spaces, tabs,
// quote markers and list markers that are followed by a space, a tab or the end.
func containerLeadEnd(line string) int {
	pos := 0
	for pos < len(line) {
		c := line[pos]
		if c == ' ' || c == '\t' || c == '>' {
			pos++
			continue
		}
		end := -1
		if c == '-' || c == '+' || c == '*' {
			end = pos + 1
		} else {
			digits := pos
			for digits < len(line) && digits-pos < 9 && line[digits] >= '0' && line[digits] <= '9' {
				digits++
			}
			if digits > pos && digits < len(line) && (line[digits] == '.' || line[digits] == ')') {
				end = digits + 1
			}
		}
		if end < 0 || (end < len(line) && line[end] != ' ' && line[end] != '\t') {
			break
		}
		pos = end
	}
	return pos
}

// expandLead expands tabs among a line's leading container markers.
func expandLead(line string) string {
	end := containerLeadEnd(line)
	return expandTabs(line[:end]) + line[end:]
}

// destination mirrors destination: an angle-wrapped or balanced bare link destination.
func destination(text string, start, end int) (dest string, stop int, ok bool) {
	for start < end && strings.IndexByte(" \t\r\n", text[start]) >= 0 {
		start++
	}
	angle := start < len(text) && text[start] == '<'
	pos, depth := start, 0
	if angle {
		pos++
	}
	begin := pos
	for pos < end {
		c := text[pos]
		if c == '\\' && pos+1 < end && isPunctuation(text[pos+1]) {
			pos += 2
			continue
		}
		size := 1
		if angle {
			if c == '>' {
				return text[begin:pos], pos + 1, true
			}
			if c == '<' || c == '\n' {
				break
			}
		} else {
			r, n := utf8.DecodeRuneInString(text[pos:])
			size = n
			if pyIsSpace(r) || (c == ')' && depth == 0) {
				if depth == 0 {
					return text[begin:pos], pos, true
				}
				return "", 0, false
			}
			if c == '(' {
				depth++
			} else if c == ')' {
				depth--
			}
		}
		pos += size
	}
	if !angle && depth == 0 {
		return text[begin:pos], pos, true
	}
	return "", 0, false
}

type definition struct {
	label string
	count int // lines consumed
}

// referenceDefinition mirrors reference_definition.
func referenceDefinition(text string) *definition {
	match := defStartRe.FindStringSubmatchIndex(text)
	if match == nil || !validLabel(text[match[2]:match[3]]) {
		return nil
	}
	dest, stop, ok := destination(text, match[1], len(text))
	// Only <> may be empty; a definition needs a bare destination.
	if !ok || (dest == "" && !strings.HasSuffix(text[:stop], ">")) || blankLineRe.MatchString(text[:stop]) {
		return nil
	}
	tail := defEndRe.FindStringIndex(text[stop:])
	if tail == nil {
		return nil
	}
	end := stop + tail[1]
	count := strings.Count(text[:end], "\n")
	if end == 0 || text[end-1] != '\n' {
		count++
	}
	return &definition{label: text[match[2]:match[3]], count: count}
}

// indentPrefix mirrors indent_prefix: characters consumed and columns covered
// by leading spaces and tabs, stopping once minimum columns are covered
// (minimum < 0 means no limit).
func indentPrefix(text string, minimum, start int) (chars, columns int) {
	pos, column := 0, start
	for pos < len(text) && (text[pos] == ' ' || text[pos] == '\t') && (minimum < 0 || column-start < minimum) {
		if text[pos] == ' ' {
			column++
		} else {
			column += 4 - column%4
		}
		pos++
	}
	return pos, column - start
}

func indentColumns(text string) int {
	_, columns := indentPrefix(text, -1, 0)
	return columns
}

// listMarkerEnd is the end of LIST_MARKER at the start of content, or -1.
func listMarkerEnd(content string) int {
	if loc := listMarkRe.FindStringIndex(content); loc != nil {
		return loc[1]
	}
	return -1
}

// listPadding mirrors list_padding.
func listPadding(content string, markerEnd, base int) (end, listIndent, baseColumn, extra int) {
	start := listHeadRe.FindStringIndex(content)[1]
	_, columns := indentPrefix(content[start:markerEnd], -1, base+start)
	if columns <= 4 {
		return markerEnd, base + start + max(columns, 1), base + start + columns, 0
	}
	_, first := indentPrefix(content[start:start+1], -1, base+start)
	return start + 1, base + start + 1, base + start + first, first - 1
}

// listInterrupts mirrors list_interrupts.
func listInterrupts(content string, markerEnd int) bool {
	if isBlank(content[markerEnd:]) {
		return false
	}
	ordered := orderedRe.FindStringSubmatch(pyTrimLeft(content))
	return ordered == nil || strings.TrimLeft(ordered[1], "0") == "1"
}

// startsBlock is the test title_continuations applies: a line that starts a new block.
func startsBlock(line string) bool {
	if blockHeadRe.MatchString(line) || thematicRe.MatchString(line) || listMarkRe.MatchString(line) || setextRe.MatchString(line) {
		return true
	}
	if fence := linkFenceRe.FindStringSubmatchIndex(line); fence != nil {
		if line[fence[2]] == '~' || !strings.Contains(line[fence[1]:], "`") {
			return true
		}
	}
	return blockHTMLRe.MatchString(line) || rawStartRe.MatchString(line) || declStartRe.MatchString(line)
}

// titleClosed mirrors title_closed.
func titleClosed(line string, closing byte) bool {
	line = strings.TrimRight(line, " \t")
	if line == "" || line[len(line)-1] != closing {
		return false
	}
	prefix := line[:len(line)-1]
	return (len(prefix)-len(strings.TrimRight(prefix, "\\")))%2 == 0
}

func titleOpener(s string) (closing byte, ok bool) {
	if s == "" {
		return 0, false
	}
	switch s[0] {
	case '"', '\'':
		return s[0], true
	case '(':
		return ')', true
	}
	return 0, false
}

// definitionCandidate mirrors definition_candidate.
func definitionCandidate(lines []string, index int, content string, depth, listIndent int) *definition {
	candidate := []string{content}
	// following is the reference's generator: the lines that continue this block.
	next, done := index+1, false
	following := func() (string, bool) {
		if done || next >= len(lines) {
			done = true
			return "", false
		}
		line, _, _, _ := containerPrefix(lines[next], listIndent, nil, false)
		next++
		end := 0
		if loc := quoteRunRe.FindStringIndex(line); loc != nil {
			end = loc[1]
		}
		text := line[end:]
		if strings.Count(line[:end], ">") != depth || isBlank(text) {
			done = true
			return "", false
		}
		chars, columns := indentPrefix(text, listIndent, 0)
		if columns < listIndent {
			done = true
			return "", false
		}
		return text[chars:], true
	}
	// continuation is title_continuations over following: it ends at a line that starts a block.
	stopped := false
	continuation := func() (string, bool) {
		if stopped {
			return "", false
		}
		line, ok := following()
		if !ok || startsBlock(line) {
			stopped = true
			return "", false
		}
		return line, true
	}

	text := content
	// A label may span lines, but CommonMark limits it to 999 characters.
	for !defStartRe.MatchString(text) && utf8.RuneCountInString(text) <= 1002 {
		if !strings.HasPrefix(pyTrimLeft(text), "[") || strings.Contains(escapedAnyRe.ReplaceAllString(text, ""), "]") {
			return nil
		}
		line, ok := following()
		if !ok {
			return nil
		}
		candidate = append(candidate, line)
		text = strings.Join(candidate, "\n")
	}
	match := defStartRe.FindStringSubmatchIndex(text)
	if match == nil || !validLabel(text[match[2]:match[3]]) {
		return nil
	}
	if isBlank(text[match[1]:]) {
		if line, ok := following(); ok {
			candidate = append(candidate, line)
			text = strings.Join(candidate, "\n")
		}
	}
	found := referenceDefinition(text)
	if found == nil {
		tail := ""
		if _, stop, ok := destination(text, match[1], len(text)); ok {
			tail = pyTrimLeft(text[stop:])
		}
		closing, ok := titleOpener(tail)
		if !ok {
			return nil
		}
		for {
			line, ok := continuation()
			if !ok {
				break
			}
			candidate = append(candidate, line)
			if titleClosed(line, closing) {
				break
			}
		}
		return referenceDefinition(strings.Join(candidate, "\n"))
	}
	// A title can start on the following line and span more than one line.
	if line, ok := continuation(); ok {
		if closing, opens := titleOpener(pyTrimLeft(line)); opens {
			candidate = append(candidate, line)
			fragment := pyTrimLeft(line)[1:]
			for !titleClosed(fragment, closing) {
				line, ok := continuation()
				if !ok {
					break
				}
				candidate = append(candidate, line)
				fragment = line
			}
			if extended := referenceDefinition(strings.Join(candidate, "\n")); extended != nil {
				return extended
			}
		}
	}
	return found
}

// listItem is an open list item: its content indent and the quote depth it opened at.
type listItem struct{ width, quoteAt int }

// marker is a list marker on a line: how many quote markers precede it, and its content indent.
type marker struct{ quotes, width int }

func keepLists(lists []listItem, keep func(listItem) bool) []listItem {
	out := make([]listItem, 0, len(lists))
	for _, item := range lists {
		if keep(item) {
			out = append(out, item)
		}
	}
	return out
}

// leadingQuote mirrors leading_quote. hasOpen distinguishes the reference's
// open_lists=None from an empty list.
func leadingQuote(line string, listIndent int, openLists []listItem, hasOpen bool) (pos int, quotes []string, indent int) {
	if hasOpen {
		ends := []int{0}
		for {
			loc := quoteOneRe.FindStringIndex(line[ends[len(ends)-1]:])
			if loc == nil {
				break
			}
			ends = append(ends, ends[len(ends)-1]+loc[1])
		}
		// Try the deepest split first: k outer quotes, then a list's spaces, then another '>'.
		for depth := len(ends) - 1; depth > 0; depth-- {
			rest := line[ends[depth]:]
			gap := 0
			for gap < len(rest) && (rest[gap] == ' ' || rest[gap] == '\t') {
				gap++
			}
			if gap == 0 || gap >= len(rest) || rest[gap] != '>' {
				continue
			}
			inside, found := 0, false
			for _, item := range openLists {
				if item.quoteAt == depth {
					inside, found = item.width, true
				}
			}
			if found && indentColumns(rest[:gap]) >= inside {
				return ends[depth], []string{line[:ends[depth]]}, inside
			}
		}
		return 0, nil, listIndent
	}
	if match := quoteGapRe.FindStringSubmatchIndex(line); match != nil && listIndent != 0 &&
		indentColumns(line[match[4]:match[5]]) >= listIndent {
		return match[3], []string{line[match[2]:match[3]]}, listIndent
	}
	return 0, nil, listIndent
}

// containerPrefix mirrors container_prefix: quote markers are put before list
// prefixes, keeping the line's length.
func containerPrefix(line string, listIndent int, openLists []listItem, hasOpen bool) (out string, nested bool, markers []marker, quoteEnd int) {
	pos, quotes, inner := leadingQuote(line, listIndent, openLists, hasOpen)
	if len(quotes) > 0 { // Spaces after the leading quotes belong to the list inside them.
		listIndent = inner
	}
	var lists []string
	sawList, sawQuote, indentation := false, false, 0
	for pos < len(line) {
		start := pos
		chars, _ := indentPrefix(line[pos:], -1, 0)
		pos += chars
		space := line[start:pos]
		indentation += indentColumns(space)
		if pos < len(line) && line[pos] == '>' && !(listIndent != 0 && !sawList && !sawQuote && indentation >= listIndent+4) {
			nested = nested || sawList || (listIndent > 0 && indentation >= listIndent)
			sawQuote = true
			end := pos + 1
			if end < len(line) && (line[end] == ' ' || line[end] == '\t') {
				end++
			}
			keep := len(space)
			if listIndent != 0 && len(lists) == 0 && !sawList {
				keep, _ = indentPrefix(space, listIndent, 0)
			}
			quotes = append(quotes, space[keep:]+line[pos:end]) // Columns past the list indent stay before the marker.
			lists = append(lists, space[:keep])
			pos = end
			continue
		}
		markerEnd := listMarkerEnd(line[pos:])
		if markerEnd < 0 || (!sawList && len(quotes) == 0 && indentColumns(space) >= 4) {
			pos = start
			break
		}
		end, width, _, extra := listPadding(line[pos:], markerEnd, len(expandTabs(line[:pos])))
		lists = append(lists, space+line[pos:pos+end])
		markers = append(markers, marker{len(quotes), width})
		sawList, pos = true, pos+end
		if extra != 0 || end < markerEnd {
			break
		}
	}
	quoteText := strings.Join(quotes, "")
	if nested {
		return quoteText + strings.Join(lists, "") + line[pos:], true, markers, len(quoteText)
	}
	return line, false, markers, len(quoteText)
}

func openIndent(lists []listItem) int {
	if len(lists) == 0 {
		return 0
	}
	return lists[len(lists)-1].width
}

// quotePrefixEnd is the end of exactly n quote markers at the start of line.
func quotePrefixEnd(line string, n int) int {
	pos := 0
	for i := 0; i < n; i++ {
		loc := quoteOneRe.FindStringIndex(line[pos:])
		if loc == nil {
			break
		}
		pos += loc[1]
	}
	return pos
}

// region is a run of text the inline scanner reads as one block.
type region struct{ begin, end, line int }

type eligibleLine struct {
	index        int
	starts, ends bool
}

// blockRegions mirrors block_regions: adjacent eligible lines of one paragraph.
func blockRegions(result []string, eligible []eligibleLine) []region {
	offsets := make([]int, len(result)+1)
	for i, ln := range result {
		offsets[i+1] = offsets[i] + len(ln) + 1
	}
	var regions []region
	previous, ended := -2, true
	for _, e := range eligible {
		end := offsets[e.index] + len(result[e.index])
		if e.index == previous+1 && !e.starts && !ended {
			regions[len(regions)-1].end = end
		} else {
			regions = append(regions, region{offsets[e.index], end, e.index + 1})
		}
		previous, ended = e.index, e.ends
	}
	return regions
}

// markdownText mirrors markdown_text: lines (leading tabs already expanded)
// with inactive Markdown masked, the labels of reference definitions, and the
// regions where inline syntax is live.
func markdownText(lines []string) (text string, labels []string, regions []region) {
	result := make([]string, 0, len(lines))
	var eligible []eligibleLine
	consumed := 0
	owner := struct{ depth, indent int }{}
	fence := ""
	rawOn := false
	var rawClose func(string) bool // nil while rawOn: the block ends at a blank line
	paragraph, quoteDepth := false, 0
	var lists []listItem // open list items, innermost last
	for lineIndex, ln := range lines {
		if lineIndex < consumed {
			result = append(result, blank(ln))
			continue
		}
		column := indentColumns(ln)
		if leadQuoteRe.MatchString(ln) {
			outerOpen := false
			for _, item := range lists {
				if item.quoteAt == 0 && item.width > column {
					outerOpen = true
				}
			}
			if outerOpen {
				// A quote marker left of an outer list item closes that item and everything inside it.
				lists = keepLists(lists, func(item listItem) bool { return item.quoteAt == 0 && item.width <= column })
				if owner.indent > column {
					fence, rawOn = "", false
				}
				if quoteDepth != 0 { // The quote inside the closed item ends; this line starts another.
					paragraph, quoteDepth = false, 0
				}
			}
		}
		outer := 0 // Only lists opened outside any quote sit ahead of the quote markers.
		for _, item := range lists {
			if item.quoteAt == 0 {
				outer = item.width
			}
		}
		source := ln
		ln, nested, markers, quoteEnd := containerPrefix(ln, outer, lists, true)
		// The regex would read a moved list indent as the optional space after a bare '>'.
		prefix := 0
		if nested {
			prefix = quoteEnd
		} else if loc := quoteRunRe.FindStringIndex(ln); loc != nil {
			prefix = loc[1]
		}
		depth := strings.Count(ln[:prefix], ">")
		if fence != "" || rawOn {
			block := source // A '>' inside a code block outside quotes is literal.
			if owner.depth != 0 {
				block = ln
			}
			inside := -1
			if depth >= owner.depth {
				if owner.depth == depth {
					inside = prefix
				} else {
					inside = quotePrefixEnd(block, owner.depth)
				}
			}
			rest := ""
			if inside >= 0 {
				rest = block[inside:]
			}
			// A list marker ahead of the quote starts a new item, which ends a block owned by the old quote.
			newItem := nested && len(markers) > 0 && markers[0].quotes < owner.depth
			if inside >= 0 && !newItem && !(owner.indent != 0 && !isBlank(rest) && indentColumns(rest) < owner.indent) {
				ln, prefix, depth = block, inside, owner.depth
			} else {
				fence, rawOn, paragraph = "", false, false
			}
		}
		content := ln[prefix:]
		indentation := indentColumns(content)
		baseColumn, extraIndent := 0, 0
		reopened := nested && len(markers) > 0 && markers[0].quotes < depth // An item opened before its quote.
		if reopened && !(fence != "" || rawOn) {
			paragraph, quoteDepth = false, markers[0].quotes // The new item's quote is a new quote.
		}
		var lazy []listItem
		hasLazy := false
		if paragraph && depth < quoteDepth && !isBlank(content) {
			// A lazy line keeps the quote open but sits outside the lists opened inside it.
			lazy, hasLazy = lists, true
			lists = keepLists(lists, func(item listItem) bool { return item.quoteAt <= depth })
		} else if depth != quoteDepth {
			if !nested {
				// Leaving a quote keeps the items around it; entering one closes the items it interrupts.
				keep := quoteDepth - 1
				if depth < quoteDepth {
					keep = depth
				}
				lists = keepLists(lists, func(item listItem) bool { return item.quoteAt <= keep })
			}
			paragraph, quoteDepth = false, depth
		}
		sibling := indentation < openIndent(lists) || hasLazy
		if sibling && !isBlank(content) {
			if !hasLazy && paragraph {
				lazy, hasLazy = lists, true // Restored if the line continues the paragraph.
			}
			lists = keepLists(lists, func(item listItem) bool { return item.width <= indentation })
		}
		listIndent := openIndent(lists)
		if listIndent != 0 {
			chars, columns := indentPrefix(content, listIndent, 0)
			if columns >= listIndent {
				prefix += chars
				baseColumn, extraIndent = columns, columns-listIndent
				content = ln[prefix:]
			}
		}
		// A marker moved after the quotes can join the quoted text into a false thematic break.
		thematic := thematicRe.MatchString(content) && !reopened
		markerEnd := -1
		if !thematic {
			markerEnd = listMarkerEnd(content)
		}
		for markerEnd >= 0 && !(fence != "" || rawOn) && (!paragraph || sibling || listInterrupts(content, markerEnd)) {
			var end int
			end, listIndent, baseColumn, extraIndent = listPadding(content, markerEnd, baseColumn)
			quoteAt, width := depth, listIndent
			if len(markers) > 0 {
				quoteAt, width = markers[0].quotes, markers[0].width
				markers = markers[1:]
			}
			if quoteAt == 0 { // Reordering moves a quote's padding after an enclosing marker; use the source indent.
				listIndent = width
			}
			lists = append(lists[:len(lists):len(lists)], listItem{listIndent, quoteAt})
			prefix += end
			content = ln[prefix:]
			paragraph = false
			thematic = thematicRe.MatchString(content)
			_, columns := indentPrefix(content, -1, baseColumn)
			if thematic || columns+extraIndent >= 4 {
				markerEnd = -1
			} else {
				markerEnd = listMarkerEnd(content)
			}
		}
		_, columns := indentPrefix(content, -1, baseColumn)
		indent := columns + extraIndent
		mark, markEnd := "", 0
		if loc := linkFenceRe.FindStringSubmatchIndex(content); loc != nil {
			mark, markEnd = content[loc[2]:loc[3]], loc[1]
		}
		if fence != "" {
			result = append(result, blank(ln))
			if mark != "" && strings.HasPrefix(mark, fence) && isBlank(content[markEnd:]) {
				fence = ""
			}
			paragraph = false
			continue
		}
		if rawOn {
			result = append(result, blank(ln))
			if (rawClose == nil && isBlank(content)) || (rawClose != nil && rawClose(content)) {
				rawOn = false
			}
			continue
		}
		if indent >= 4 && !paragraph {
			result = append(result, blank(ln))
			continue
		}
		html := rawStartRe.FindStringSubmatch(content)
		switch {
		case mark != "" && (mark[0] == '~' || !strings.Contains(content[markEnd:], "`")):
			fence = mark
			owner.depth, owner.indent = depth, listIndent
			result = append(result, blank(ln))
			paragraph = false
		case html != nil:
			closer := rawCloseRes[strings.ToLower(html[1])]
			rawOn, rawClose = !closer.MatchString(content), closer.MatchString
			owner.depth, owner.indent = depth, listIndent
			result = append(result, blank(ln))
			paragraph = false
		case declStartRe.MatchString(content):
			needle := ">"
			switch start := pyTrimLeft(content); {
			case strings.HasPrefix(start, "<!--"):
				needle = "-->"
			case strings.HasPrefix(start, "<?"):
				needle = "?>"
			case strings.HasPrefix(start, "<![CDATA["):
				needle = "]]>"
			}
			rawOn, rawClose = !strings.Contains(content, needle), func(s string) bool { return strings.Contains(s, needle) }
			owner.depth, owner.indent = depth, listIndent
			result = append(result, blank(ln))
			paragraph = false
		case blockHTMLRe.MatchString(content) || (!paragraph && htmlTagFull.MatchString(pyTrim(content))):
			rawOn, rawClose = true, nil // Ordinary HTML blocks end at a blank line.
			owner.depth, owner.indent = depth, listIndent
			result = append(result, blank(ln))
			paragraph = false
		default:
			if !paragraph && strings.HasPrefix(pyTrimLeft(content), "[") {
				if found := definitionCandidate(lines, lineIndex, content, depth, listIndent); found != nil {
					labels = append(labels, found.label)
					consumed = lineIndex + found.count
					result = append(result, blank(ln))
					continue
				}
			}
			result = append(result, strings.Repeat(" ", prefix)+content)
			boundary := thematic || blockHeadRe.MatchString(content) || (paragraph && !hasLazy && setextRe.MatchString(content))
			if hasLazy && paragraph && !boundary {
				lists = lazy
			}
			if !isBlank(content) {
				eligible = append(eligible, eligibleLine{lineIndex, !paragraph || boundary, boundary})
			}
			paragraph = !isBlank(content) && !boundary
		}
	}
	return strings.Join(result, "\n"), labels, blockRegions(result, eligible)
}

// validLabel mirrors valid_label.
func validLabel(value string) bool {
	if strings.Trim(value, " \t\r\n") == "" || utf8.RuneCountInString(value) > 999 || blankLineRe.MatchString(value) {
		return false
	}
	for pos := 0; pos < len(value); pos++ {
		if value[pos] == '\\' && pos+1 < len(value) && isPunctuation(value[pos+1]) {
			pos++
			continue
		}
		if value[pos] == '[' || value[pos] == ']' {
			return false
		}
	}
	return true
}

// referenceLabel mirrors reference_label: the [label] that starts at start.
func referenceLabel(text string, start, end int) (value string, stop int, ok bool) {
	pos, count := start+1, 1
	for pos < end && count <= 1000 {
		c := text[pos]
		if c == '\\' && pos+1 < end && isPunctuation(text[pos+1]) {
			pos += 2
			count += 2
			continue
		}
		if c == ']' {
			value = text[start+1 : pos]
			if value == "" || validLabel(value) {
				return value, pos + 1, true
			}
			return "", 0, false
		}
		if c == '[' {
			break
		}
		_, size := utf8.DecodeRuneInString(text[pos:])
		pos += size
		count++
	}
	return "", 0, false
}

var labelFold = cases.Fold()

func normalizeLabel(value string) string {
	return labelFold.String(labelSpaceRe.ReplaceAllString(strings.Trim(value, " \t\r\n"), " "))
}

// span is an inline link the scanners recognize: [text](destination), by byte
// offsets into the scanned text, on a 1-based line.
type span struct{ line, start, end int }

type opener struct {
	opening, line int
	image         bool
	generation    int
}

// linkSpans mirrors link_destinations(lines, spans=True): the inline links
// outside images, in source order. text is the lines with their leading tabs
// expanded, which is what the offsets refer to.
func linkSpans(lines []string) (text string, spans []span) {
	expanded := make([]string, len(lines))
	for i, line := range lines {
		expanded[i] = expandLead(line)
	}
	masked, labels, regions := markdownText(expanded)
	definitions := map[string]bool{}
	for _, label := range labels {
		definitions[normalizeLabel(label)] = true
	}
	text = masked
	for _, reg := range regions {
		begin, end, line := reg.begin, reg.end, reg.line
		// Index exact backtick-run closers without assigning token ownership yet.
		closers := map[int]int{} // position of a possible opener -> start of its closing run, or -1
		latest := map[int]int{}
		var runs [][2]int
		for pos := begin; pos < end; pos++ {
			if text[pos] != '`' {
				continue
			}
			start := pos
			for pos < end && text[pos] == '`' {
				pos++
			}
			runs = append(runs, [2]int{start, pos})
		}
		for i := len(runs) - 1; i >= 0; i-- {
			start, stop := runs[i][0], runs[i][1]
			size := stop - start
			// An escaped first backtick may leave a shorter opening run.
			for width := 1; width <= size; width++ {
				if at, ok := latest[width]; ok {
					closers[stop-width] = at
				} else {
					closers[stop-width] = -1
				}
			}
			latest[size] = start
		}
		var stack []opener
		generation := 0
		for pos := begin; pos < end; {
			c, stop := text[pos], pos+1
			switch {
			case c == '\\' && pos+1 < end && isPunctuation(text[pos+1]):
				stop = pos + 2
			case c == '`':
				runEnd := pos
				for runEnd < end && text[runEnd] == '`' {
					runEnd++
				}
				stop = runEnd
				if closeAt, ok := closers[pos]; ok && closeAt >= 0 {
					stop = closeAt + runEnd - pos
				}
			case c == '<':
				if loc := autolinkRe.FindStringIndex(text[pos:end]); loc != nil {
					stop = pos + loc[1]
				} else if loc := inlineHTMLRe.FindStringIndex(text[pos:end]); loc != nil {
					stop = pos + loc[1]
				}
			case c == '[' || (c == '!' && pos+1 < len(text) && text[pos+1] == '['):
				image := c == '!'
				opening := pos
				if image {
					opening++
				}
				stack = append(stack, opener{opening, line, image, generation})
				stop = opening + 1
			case c == ']' && len(stack) > 0:
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if !top.image && top.generation != generation {
					break
				}
				inline, reference := false, false
				if stop < end && text[stop] == '(' {
					if _, destEnd, ok := destination(text, stop+1, end); ok {
						if tail := inlineEndRe.FindStringIndex(text[destEnd:end]); tail != nil {
							inline, stop = true, destEnd+tail[1]
						}
					}
				}
				if !inline {
					label, labelEnd, hasLabel := "", 0, false
					if stop < len(text) && text[stop] == '[' {
						label, labelEnd, hasLabel = referenceLabel(text, stop, end)
					}
					own := ""
					if utf8.RuneCountInString(text[top.opening:pos]) <= 1000 {
						own = text[top.opening+1 : pos]
					}
					value := own
					if hasLabel && label != "" {
						value = label
					}
					if validLabel(value) && definitions[normalizeLabel(value)] {
						reference = true
						if hasLabel {
							stop = labelEnd
						}
					}
				}
				if inline || reference {
					if !top.image {
						generation++ // Earlier link openers become inactive; images remain active.
					}
					if inline && !top.image {
						insideImage := false
						for _, item := range stack {
							insideImage = insideImage || item.image
						}
						if !insideImage {
							spans = append(spans, span{top.line, top.opening, stop})
						}
					}
				}
			}
			line += strings.Count(text[pos:stop], "\n")
			pos = stop
		}
	}
	return text, spans
}
