package contextrouter

import "testing"

// Compaction prints exactly what the reference prints, on every corpus case.
func TestCompactCorpus(t *testing.T) {
	changed := 0
	for _, c := range loadCompactCases(t) {
		if got := compactPathLinks(c.Input, c.Source); got != c.Output {
			t.Errorf("%s:\n got  %q\n want %q\n input %q", c.Name, got, c.Output, c.Input)
		}
		if c.Output != c.Input {
			changed++
		}
	}
	if changed < 50 {
		t.Fatalf("only %d corpus cases compact anything", changed)
	}
}

func TestPyJoin(t *testing.T) {
	// posixpath.join drops the base for an absolute part; path.Join would not.
	if got := pyJoin("wiki", "/x"); got != "/x" {
		t.Fatalf("got %q", got)
	}
	if got := compactPathLinks("[wiki/x](/x)", "wiki/guide.md"); got != "[wiki/x](/x)" {
		t.Fatalf("absolute destination compacted: %q", got)
	}
}
