package contextrouter

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"
)

// stopper lets a long scan give up when the report's time is spent or the
// request is gone. The scanners are linear on the shapes known to stress them;
// this is the bound for the ones nobody has thought of.
type stopper struct {
	ctx      context.Context
	deadline time.Time
	calls    int
	stopped  bool
}

// now reports whether to stop. It looks at the clock once in 1024 calls.
func (s *stopper) now() bool {
	if s.stopped {
		return true
	}
	if s.calls++; s.calls&1023 != 0 {
		return false
	}
	if (!s.deadline.IsZero() && time.Now().After(s.deadline)) || (s.ctx != nil && s.ctx.Err() != nil) {
		s.stopped = true
	}
	return s.stopped
}

// regionIndex answers, in constant time, the questions the inline scanner asks
// again at every link in one region: where a destination ends, where a title
// closes, whether an HTML comment ever closes. The reference rescans the rest
// of the region each time, which is quadratic on a page of unclosed links; the
// answers here are the same ones, computed once.
type regionIndex struct {
	text       string
	begin, end int

	// For destinations, by offset from begin. Escapes are a backslash and a
	// punctuation character, as destination reads them.
	depth []int32 // unescaped '(' minus ')' before this offset
	drop  []int32 // the first offset past this one where depth is lower, or -1
	space []int32 // the first whitespace character at or after this offset; len(text) of the region when none

	// For titles, by offset from begin, one table per closing delimiter. An
	// escape is a backslash and any character but a newline, as TITLE reads it.
	title [3][]int32 // the delimiter's offset, or -1 when the title cannot close

	// noNeedle[s] is an offset from which s does not occur again.
	noNeedle map[string]int
}

func (x *regionIndex) buildDestinations() {
	n := x.end - x.begin
	region := x.text[x.begin:x.end]
	x.depth = make([]int32, n+1)
	x.drop = make([]int32, n+1)
	x.space = make([]int32, n+1)
	isSpace := make([]bool, n+1)
	for i := 0; i < n; {
		size, change := 1, int32(0)
		switch c := region[i]; {
		case c == '\\' && i+1 < n && isPunctuation(region[i+1]):
			size = 2
		case c == '(':
			change = 1
		case c == ')':
			change = -1
		case c >= utf8.RuneSelf:
			var r rune
			r, size = utf8.DecodeRuneInString(region[i:])
			isSpace[i] = pyIsSpace(r)
		default:
			isSpace[i] = pyIsSpace(rune(c))
		}
		for k := 1; k <= size; k++ {
			x.depth[i+k] = x.depth[i] + change
		}
		i += size
	}
	x.space[n] = int32(n)
	var lower []int32 // offsets whose depth is lower than everything after them so far
	for i := n; i >= 0; i-- {
		if i < n {
			x.space[i] = x.space[i+1]
			if isSpace[i] {
				x.space[i] = int32(i)
			}
		}
		for len(lower) > 0 && x.depth[lower[len(lower)-1]] >= x.depth[i] {
			lower = lower[:len(lower)-1]
		}
		x.drop[i] = -1
		if len(lower) > 0 {
			x.drop[i] = lower[len(lower)-1]
		}
		lower = append(lower, int32(i))
	}
}

// destination is destination(text, start, end) for the region, returning only
// where the destination stops. start follows a '(' that is not escaped, so it
// is a token boundary in the tables.
func (x *regionIndex) destination(start int) (stop int, ok bool) {
	for start < x.end && strings.IndexByte(" \t\r\n", x.text[start]) >= 0 {
		start++
	}
	if start < len(x.text) && x.text[start] == '<' {
		// An angle destination ends at the next '<', '>' or newline, and every
		// one starts with a '<', so these scans do not overlap.
		_, stop, ok = destination(x.text, start, x.end)
		return stop, ok
	}
	if x.depth == nil {
		x.buildDestinations()
	}
	at := start - x.begin
	n := x.end - x.begin
	space := int(x.space[at])
	if lower := int(x.drop[at]); lower >= 0 && lower-1 < space {
		return x.begin + lower - 1, true // a ')' that closes nothing opened inside the destination
	}
	if x.depth[min(space, n)] != x.depth[at] {
		return 0, false // whitespace or the end inside parentheses
	}
	return x.begin + space, true
}

var titleClosers = [3]byte{'"', '\'', ')'}

func (x *regionIndex) buildTitles() {
	n := x.end - x.begin
	region := x.text[x.begin:x.end]
	for kind := range x.title {
		x.title[kind] = make([]int32, n+1)
		x.title[kind][n] = -1
	}
	// Token starts, so the tables can be filled from the right.
	starts := make([]int32, 0, n)
	for i := 0; i < n; {
		starts = append(starts, int32(i))
		if region[i] == '\\' && i+1 < n && region[i+1] != '\n' {
			_, size := utf8.DecodeRuneInString(region[i+1:])
			i += 1 + size
			continue
		}
		i++
	}
	next := n
	for k := len(starts) - 1; k >= 0; k-- {
		i := int(starts[k])
		for kind, closer := range titleClosers {
			value := x.title[kind][next]
			switch c := region[i]; {
			case c == '\\' && next == i+1:
				value = -1 // a backslash before a newline or the end: nothing in a title matches it
			case c == closer:
				value = int32(i)
			}
			for j := i; j < next; j++ {
				x.title[kind][j] = value
			}
		}
		next = i
	}
}

// inlineEnd is INLINE_END matched at p: an optional title, then the closing ')'.
func (x *regionIndex) inlineEnd(p int) (stop int, ok bool) {
	skipSpace := func(q int) int {
		for q < x.end {
			r, size := utf8.DecodeRuneInString(x.text[q:x.end])
			if !pyIsSpace(r) {
				break
			}
			q += size
		}
		return q
	}
	q := skipSpace(p)
	if q >= x.end {
		return 0, false
	}
	if x.text[q] == ')' {
		return q + 1, true
	}
	kind := strings.IndexByte(`"'(`, x.text[q])
	if q == p || kind < 0 {
		return 0, false
	}
	if x.title[0] == nil {
		x.buildTitles()
	}
	closer := x.title[kind][q+1-x.begin]
	if closer < 0 {
		return 0, false
	}
	if q = skipSpace(x.begin + int(closer) + 1); q < x.end && x.text[q] == ')' {
		return q + 1, true
	}
	return 0, false
}

// find returns the end of the first needle at or after from.
func (x *regionIndex) find(needle string, from int) (stop int, ok bool) {
	if from > x.end {
		return 0, false
	}
	if none, known := x.noNeedle[needle]; known && from >= none {
		return 0, false
	}
	i := strings.Index(x.text[from:x.end], needle)
	if i < 0 {
		if x.noNeedle == nil {
			x.noNeedle = map[string]int{}
		}
		x.noNeedle[needle] = from
		return 0, false
	}
	return from + i + len(needle), true
}

// inlineHTML is INLINE_HTML matched at pos: a comment, a processing
// instruction, a declaration, a CDATA section or a tag.
func (x *regionIndex) inlineHTML(pos int) (stop int, ok bool) {
	rest := x.text[pos:x.end]
	switch {
	case strings.HasPrefix(rest, "<!--"):
		return x.find("-->", pos+4)
	case strings.HasPrefix(rest, "<?"):
		return x.find("?>", pos+2)
	case len(rest) >= 3 && rest[1] == '!' && rest[2] >= 'A' && rest[2] <= 'Z':
		return x.find(">", pos+3)
	case strings.HasPrefix(rest, "<![CDATA["):
		return x.find("]]>", pos+9)
	}
	if loc := inlineTagRe.FindStringIndex(rest); loc != nil {
		return pos + loc[1], true
	}
	return 0, false
}
