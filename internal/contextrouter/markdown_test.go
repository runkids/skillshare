package contextrouter

import (
	"strings"
	"testing"
)

func TestNormalizeNewlines(t *testing.T) {
	got, ok := normalizeNewlines([]byte("a\r\nb\rc\n"))
	if !ok || got != "a\nb\nc\n" {
		t.Fatalf("got %q %v", got, ok)
	}
	if _, ok := normalizeNewlines([]byte("\xff")); ok {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestFrontmatterEnd(t *testing.T) {
	for _, tc := range []struct {
		text string
		want int
	}{
		{"---\na: 1\n---\nx", 3},
		{"\ufeff---  \na\n...\t\t\nx", 3},
		{"---\nno closer", 0},
		{" ---\na\n---", 0},
		{"", 0},
	} {
		if got := frontmatterEnd(strings.Split(tc.text, "\n")); got != tc.want {
			t.Errorf("%q: got %d, want %d", tc.text, got, tc.want)
		}
	}
}

func TestSection(t *testing.T) {
	h := func(s string) *string { return &s }
	for _, tc := range []struct {
		name, text string
		o          sectionOpts
		want       string
		hits       int
	}{
		{"strip", "---\na\n---\n\n# T\nbody\n", sectionOpts{Strip: true}, "# T\nbody", 1},
		{"frontmatter alone keeps the envelope", "---\na\n---\n# T\n", sectionOpts{Frontmatter: true}, "---\na\n---\n# T", 1},
		{"python trims 0x1c", "x\x1c\n", sectionOpts{}, "x", 1},
		{"children included, stops at equal level", "# A\n1\n## B\n2\n# C\n3", sectionOpts{Heading: h("A")}, "# A\n1\n## B\n2", 1},
		{"closing hashes", "## B ##  \nx\n## C", sectionOpts{Heading: h("B")}, "## B ##  \nx", 1},
		{"fenced heading ignored", "```\n# A\n```\n# A\nx", sectionOpts{Heading: h("A")}, "# A\nx", 1},
		{"inner fence of another kind does not close", "~~~~\n```\n# A\n~~~~\n", sectionOpts{Heading: h("A")}, "", 0},
		{"duplicate", "# A\n# A", sectionOpts{Heading: h("A")}, "", 2},
		{"strip hides envelope heading", "---\n# A\n---\n# A\nx", sectionOpts{Heading: h("A"), Strip: true}, "# A\nx", 1},
		{"envelope heading seen without options", "---\n# A\n---\n# A\nx", sectionOpts{Heading: h("A")}, "", 2},
		{"unicode space after hashes", "#\u00a0A\nx", sectionOpts{Heading: h("A")}, "#\u00a0A\nx", 1},
		{"no space is not a heading", "#A\nx", sectionOpts{Heading: h("A")}, "", 0},
		{"seven hashes", "####### A", sectionOpts{Heading: h("A")}, "", 0},
	} {
		got, hits := section(tc.text, tc.o)
		if got != tc.want || hits != tc.hits {
			t.Errorf("%s: got %q/%d, want %q/%d", tc.name, got, hits, tc.want, tc.hits)
		}
	}
}
