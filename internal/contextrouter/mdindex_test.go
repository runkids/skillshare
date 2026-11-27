package contextrouter

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The patterns the reference matches where regionIndex answers from its tables.
var (
	referenceInlineEndRe  = regexp.MustCompile(`^(?:` + pySpace + `+` + titlePattern + `)?` + pySpace + `*\)`)
	referenceInlineHTMLRe = regexp.MustCompile(`^(?:<!--[\s\S]*?-->|<\?[\s\S]*?\?>|<![A-Z][\s\S]*?>|<!\[CDATA\[[\s\S]*?\]\]>|` + htmlTagPattern + `)`)
)

// The tables give the answers the reference's rescans give, at every position
// the inline scanner can ask from.
func TestRegionIndexMatchesRescans(t *testing.T) {
	tokens := []string{"(", ")", "(", ")", "\\", "\\\\", " ", "\t", "\n", "\"", "'", "<", ">", "a", "é", " ",
		"](", "<!--", "-->", "<?", "?>", "<!A", "<![CDATA[", "]]>", "<a b='c'>", "</a>", "x y"}
	rng := rand.New(rand.NewSource(19))
	asked := 0
	for round := 0; round < 20000; round++ {
		var b strings.Builder
		for n := rng.Intn(24); n > 0; n-- {
			b.WriteString(tokens[rng.Intn(len(tokens))])
		}
		text := "pad " + b.String() + " pad"
		begin, end := 2, len(text)-2 // a region inside the text, as a paragraph is
		index := regionIndex{text: text, begin: begin, end: end}
		for p := begin; p <= end; p++ {
			// A destination starts after "](": the scanner never asks from anywhere else.
			if p-2 >= begin && text[p-2:p] == "](" {
				_, wantStop, wantOK := destination(text, p, end)
				if stop, ok := index.destination(p); ok != wantOK || (ok && stop != wantStop) {
					t.Fatalf("destination(%q, %d): got %d %v, want %d %v", text[begin:end], p-begin, stop-begin, ok, wantStop-begin, wantOK)
				}
				asked++
			}
			want := referenceInlineEndRe.FindStringIndex(text[p:end])
			if stop, ok := index.inlineEnd(p); ok != (want != nil) || (ok && stop != p+want[1]) {
				t.Fatalf("inlineEnd(%q, %d): got %d %v, want %v", text[begin:end], p-begin, stop-begin, ok, want)
			}
			if p < end && text[p] == '<' {
				want := referenceInlineHTMLRe.FindStringIndex(text[p:end])
				if stop, ok := index.inlineHTML(p); ok != (want != nil) || (ok && stop != p+want[1]) {
					t.Fatalf("inlineHTML(%q, %d): got %d %v, want %v", text[begin:end], p-begin, stop-begin, ok, want)
				}
			}
		}
	}
	if asked < 5000 {
		t.Fatalf("only %d destinations compared", asked)
	}
}

// hostileBodies are a megabyte each of the shapes that made the scanner, or the
// reference it mirrors, quadratic: every link rescanning the rest of the page,
// every list marker re-reading its line, every line walking deep nesting.
func hostileBodies(n int) map[string]string {
	fill := func(unit string) string { return strings.Repeat(unit, n/len(unit)+1)[:n] }
	half := n / 2
	return map[string]string{
		"unclosed links":           fill("[a]("),
		"unclosed parentheses":     fill("[a](("),
		"unclosed paren titles":    fill("[a](x ("),
		"unclosed quoted titles":   fill("[a](x \""),
		"list markers on one line": strings.Repeat("- ", half) + "x",
		"blank lines under lists":  strings.Repeat("- ", half/2) + "x" + strings.Repeat("\n", half),
		"quotes under lists":       strings.Repeat("- ", half/2) + "x" + strings.Repeat("\n>", half/2),
		"lists inside a quote":     "> " + strings.Repeat("- ", half/2) + "x" + strings.Repeat("\n>", half/2),
		"nested brackets":          strings.Repeat("[", half) + strings.Repeat("]", half),
		"links inside image text":  strings.Repeat("![", half/2) + fill("[docs/run.md](../docs/run.md)")[:half],
		"unclosed comments":        fill("<!--"),
		"unclosed declarations":    fill("<!A"),
		"backticks":                fill("`a"),
		"many definitions":         fill("[r]: x\n"),
		"many real links":          fill("[docs/run.md](../docs/run.md)\n"),
	}
}

func TestCompactStaysLinearOnHostileInput(t *testing.T) {
	if testing.Short() {
		t.Skip("scans a megabyte per shape")
	}
	const size = 1 << 20 // the largest file a report reads
	for name, body := range hostileBodies(size) {
		start := time.Now()
		if _, ok := compactPathLinks(body, "wiki/guide.md", nil); !ok {
			t.Errorf("%s: stopped without a stopper", name)
		}
		// Each takes a fraction of a second; quadratic work at this size takes minutes.
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("%s: %v for %d bytes", name, elapsed, size)
		}
	}
}

func TestStopperHaltsAScan(t *testing.T) {
	body := strings.Repeat("[docs/run.md](../docs/run.md)\n", 2000)
	if _, ok := compactPathLinks(body, "wiki/guide.md", &stopper{deadline: time.Now().Add(-time.Second)}); ok {
		t.Fatal("a scan past its deadline ran to the end")
	}
	if out, ok := compactPathLinks(body, "wiki/guide.md", &stopper{deadline: time.Now().Add(time.Hour)}); !ok || !strings.HasPrefix(out, "`docs/run.md`\n") {
		t.Fatal("a scan within its deadline did not finish")
	}
}
