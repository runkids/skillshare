package utils

import (
	"encoding/json"
	"testing"
)

func TestParseFrontmatterMap_CoreScalarsMatchServedYAML(t *testing.T) {
	for _, tc := range []struct{ name, content, want string }{
		{name: "scalars", content: "---\nname: fixture\ndescription: |\n  Keep newline.\ndate: 2026-11-14\nnumber: 012\nexplicit: !!int 0123\nstrings: [0b10, 1_000, -0o12, 0XFF]\nmetadata:\n  date: 2026-11-14T12:34:56Z\n  list: [007, 0o12, 0x10, true, null, '012', !!str 012]\n  nested: {count: 00012, exponent: 1e3}\n---\nbody\n", want: `{"date":"2026-11-14","description":"Keep newline.\n","explicit":123,"metadata":{"date":"2026-11-14T12:34:56Z","list":[7,10,16,true,null,"012","012"],"nested":{"count":12,"exponent":1000}},"name":"fixture","number":12,"strings":["0b10","1_000","-0o12","0XFF"]}`},
		{name: "anchors", content: "---\nmetadata: &meta {date: 2026-11-14, count: 012}\ncopy: *meta\n---\n", want: `{"copy":{"count":12,"date":"2026-11-14"},"metadata":{"count":12,"date":"2026-11-14"}}`},
		{name: "cyclic alias", content: "---\nvalue: &self [*self]\n---\n"},
		{name: "nonfinite number", content: "---\nvalue: .nan\n---\n"},
		{name: "nonstring key", content: "---\nmetadata: {1: value}\n---\n"},
		{name: "malformed YAML", content: "---\nvalue: [unclosed\n---\n"},
		{name: "duplicate key", content: "---\nname: one\nname: two\n---\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fm, err := ParseFrontmatterMap([]byte(tc.content))
			if tc.want == "" {
				if err == nil {
					t.Errorf("accepted invalid or non-JSON frontmatter %q", tc.content)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(fm)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("frontmatter JSON = %s\nwant = %s", got, tc.want)
			}
		})
	}
}
