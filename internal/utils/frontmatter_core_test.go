package utils

import (
	"encoding/json"
	"testing"
)

func TestParseFrontmatterMap_CoreScalarsMatchServedYAML(t *testing.T) {
	content := []byte("---\nname: fixture\ndescription: |\n  Keep newline.\ndate: 2026-11-14\nnumber: 012\nexplicit: !!int 0123\nstrings: [0b10, 1_000, -0o12, 0XFF]\nmetadata:\n  date: 2026-11-14T12:34:56Z\n  list: [007, 0o12, 0x10, true, null, '012', !!str 012]\n  nested: {count: 00012, exponent: 1e3}\n---\nbody\n")
	fm, err := ParseFrontmatterMap(content)
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(fm)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"date":"2026-11-14","description":"Keep newline.\n","explicit":123,"metadata":{"date":"2026-11-14T12:34:56Z","list":[7,10,16,true,null,"012","012"],"nested":{"count":12,"exponent":1000}},"name":"fixture","number":12,"strings":["0b10","1_000","-0o12","0XFF"]}`
	if string(got) != want {
		t.Errorf("frontmatter JSON = %s\nwant = %s", got, want)
	}
}

func TestParseFrontmatterMap_CoreAnchorsAndRejections(t *testing.T) {
	fm, err := ParseFrontmatterMap([]byte("---\nmetadata: &meta {date: 2026-11-14, count: 012}\ncopy: *meta\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(fm)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"copy":{"count":12,"date":"2026-11-14"},"metadata":{"count":12,"date":"2026-11-14"}}`
	if string(got) != want {
		t.Errorf("alias JSON = %s, want %s", got, want)
	}
	for _, raw := range []string{"value: &self [*self]", "value: .nan", "metadata: {1: value}", "value: [unclosed", "name: one\nname: two"} {
		if _, err := ParseFrontmatterMap([]byte("---\n" + raw + "\n---\n")); err == nil {
			t.Errorf("accepted invalid or non-JSON frontmatter %q", raw)
		}
	}
}
