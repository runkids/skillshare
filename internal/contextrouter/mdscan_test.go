package contextrouter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type compactCase struct {
	Name   string   `json:"name"`
	Source string   `json:"source"`
	Input  string   `json:"input"`
	Spans  [][2]int `json:"spans"`
	Output string   `json:"output"`
}

func loadCompactCases(t *testing.T) []compactCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "compact.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Reference string        `json:"reference"`
		Cases     []compactCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if file.Reference != Reference {
		t.Fatalf("corpus comes from %s, the port follows %s", file.Reference, Reference)
	}
	return file.Cases
}

// The scanners find exactly the links the reference finds, on every corpus case.
func TestLinkSpansCorpus(t *testing.T) {
	for _, c := range loadCompactCases(t) {
		text, spans, _ := linkSpans(strings.Split(c.Input, "\n"), nil)
		got := [][2]int{}
		for _, s := range spans {
			got = append(got, [2]int{s.start, s.end})
		}
		want := c.Spans
		if want == nil {
			want = [][2]int{}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v\ninput: %q\nmasked: %q", c.Name, got, want, c.Input, text)
		}
	}
}

func TestCorpusIsNotTrivial(t *testing.T) {
	cases := loadCompactCases(t)
	withSpans := 0
	for _, c := range cases {
		if len(c.Spans) > 0 {
			withSpans++
		}
	}
	if len(cases) < 300 || withSpans < 50 {
		t.Fatalf("corpus has %d cases, %d with links", len(cases), withSpans)
	}
}
