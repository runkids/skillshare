package utils

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestToggleFrontmatterFlag(t *testing.T) {
	const key = "disable-model-invocation"
	tests := []struct {
		name   string
		in     string
		want   string
		wantOn bool
	}{
		{
			name:   "absent key is appended, everything else untouched",
			in:     "---\nname: s\n# keep this comment\ndescription: d\n---\n# Body\n",
			want:   "---\nname: s\n# keep this comment\ndescription: d\ndisable-model-invocation: true\n---\n# Body\n",
			wantOn: true,
		},
		{
			name:   "true is removed so the file returns to pristine",
			in:     "---\nname: s\ndisable-model-invocation: true\ndescription: d\n---\n# Body\n",
			want:   "---\nname: s\ndescription: d\n---\n# Body\n",
			wantOn: false,
		},
		{
			name:   "explicit false flips to true in place",
			in:     "---\ndisable-model-invocation: false # set by hand\nname: s\n---\nBody",
			want:   "---\ndisable-model-invocation: true\nname: s\n---\nBody",
			wantOn: true,
		},
		{
			name:   "nested key of the same name is not the flag",
			in:     "---\nmetadata:\n  disable-model-invocation: true\n---\nBody",
			want:   "---\nmetadata:\n  disable-model-invocation: true\ndisable-model-invocation: true\n---\nBody",
			wantOn: true,
		},
		{
			name:   "CRLF line endings are kept",
			in:     "---\r\nname: s\r\n---\r\nBody\r\n",
			want:   "---\r\nname: s\r\ndisable-model-invocation: true\r\n---\r\nBody\r\n",
			wantOn: true,
		},
		{
			name:   "BOM is kept in front of the block",
			in:     "\ufeff---\nname: s\n---\nBody\n",
			want:   "\ufeff---\nname: s\ndisable-model-invocation: true\n---\nBody\n",
			wantOn: true,
		},
		{
			name:   "no frontmatter gets one",
			in:     "# Just a body\n",
			want:   "---\ndisable-model-invocation: true\n---\n# Just a body\n",
			wantOn: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "SKILL.md")
			if err := os.WriteFile(path, []byte(tt.in), 0644); err != nil {
				t.Fatal(err)
			}
			on, err := ToggleFrontmatterFlag(path, key)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if on != tt.wantOn {
				t.Errorf("on = %v, want %v", on, tt.wantOn)
			}
			got, _ := os.ReadFile(path)
			if string(got) != tt.want {
				t.Errorf("content mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestToggleFrontmatterFlag_TwiceIsIdentity(t *testing.T) {
	orig := "---\nname: s\n# comment\nmetadata:\n  targets: [claude]\n---\n# Body\n"
	path := filepath.Join(t.TempDir(), "SKILL.md")
	os.WriteFile(path, []byte(orig), 0644)

	ToggleFrontmatterFlag(path, "disable-model-invocation")
	ToggleFrontmatterFlag(path, "disable-model-invocation")

	got, _ := os.ReadFile(path)
	if string(got) != orig {
		t.Errorf("two toggles should restore the file\n got: %q\nwant: %q", got, orig)
	}
}

func TestToggleFrontmatterFlag_FlowRoot(t *testing.T) {
	const key = "disable-model-invocation"
	flag := key + ": true"
	meta := "metadata: {generated: {by: tool}, sources: [{actor: 'team:finance'}], x-acme: {nested: [1, 2]}, " + key + ": true}"
	cases := []struct {
		name, in, on, off string
		firstOn, bomCRLF  bool
	}{
		{"nested punctuation", "{name: demo, description: 'a }, " + flag + "', " + meta + "}", "{" + flag + ", name: demo, description: 'a }, " + flag + "', " + meta + "}", "{name: demo, description: 'a }, " + flag + "', " + meta + "}", true, false},
		{"BOM CRLF comments", "{\n  name: demo, # keep\n  description: \"comma, brace } and \\\"quote\\\"\",\n  " + meta + "\n} # tail", "{\n  " + flag + ", name: demo, # keep\n  description: \"comma, brace } and \\\"quote\\\"\",\n  " + meta + "\n} # tail", "{\n  name: demo, # keep\n  description: \"comma, brace } and \\\"quote\\\"\",\n  " + meta + "\n} # tail", true, true},
		{"empty anchored", "&root {  } # empty", "&root {" + flag + "  } # empty", "&root {  } # empty", true, false},
		{"unicode and json keys", "{\"name\": \"日本語😀\", \"description\": \"é\"}", "{" + flag + ", \"name\": \"日本語😀\", \"description\": \"é\"}", "{\"name\": \"日本語😀\", \"description\": \"é\"}", true, false},
		{"first true", "{" + flag + ", name: demo}", "{name: demo}", "{" + flag + ", name: demo}", false, false},
		{"only trailing", "{" + flag + ",}", "{" + key + ": false,}", "{" + flag + ",}", false, false},
		{"comment after separator", "{\n  " + flag + ",\n  # keep metadata note\n  name: demo\n}", "{\n  " + key + ": false,\n  # keep metadata note\n  name: demo\n}", "{\n  " + flag + ",\n  # keep metadata note\n  name: demo\n}", false, false},
		{"comment before comma", "{name: demo, " + flag + " # keep\n, x: 1}", "{name: demo, " + key + ": false # keep\n, x: 1}", "{name: demo, " + flag + " # keep\n, x: 1}", false, false},
		{"quoted false comment", "{name: demo, \"" + key + "\": 'false' # keep\n}", "{name: demo, \"" + key + "\": true # keep\n}", "{name: demo, \"" + key + "\": false # keep\n}", true, false},
		{"plain false first", "{" + key + ": false, name: demo}", "{" + flag + ", name: demo}", "{name: demo}", true, false},
		{"apostrophe and quoted punctuation", "{name: demo, description: its user's \"quoted\" value, " + flag + ", x: '}, foo'}", "{name: demo, description: its user's \"quoted\" value, " + key + ": false, x: '}, foo'}", "{name: demo, description: its user's \"quoted\" value, " + flag + ", x: '}, foo'}", false, false},
		{"last CRLF comment", "{\n name: demo, # keep\n " + flag + "\n}", "{\n name: demo, # keep\n " + key + ": false\n}", "{\n name: demo, # keep\n " + flag + "\n}", false, true},
		{"json false last", "{\n \"name\": \"demo\",\n \"" + key + "\": false\n}", "{\n \"name\": \"demo\",\n \"" + key + "\": true\n}", "{\n \"name\": \"demo\",\n \"" + key + "\": false\n}", true, false},
		{"alias", "{name: &f false, " + key + ": *f}", "", "", false, false},
		{"escaped false", "{name: demo, " + key + ": \"f\\u0061lse\"}", "", "", false, false},
		{"mixed YAML line breaks before flag", "{description: \"a\r\nb\rc\u0085d\u2028e\u2029f\", name: demo, " + flag + "}", "{description: \"a\r\nb\rc\u0085d\u2028e\u2029f\", name: demo, " + key + ": false}", "{description: \"a\r\nb\rc\u0085d\u2028e\u2029f\", name: demo, " + flag + "}", false, false},
	}
	wrap := func(s string) string { return "---\n" + s + "\n---\nBody\n" }
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapCase := func(s string) string {
				result := wrap(s)
				if tc.bomCRLF {
					result = "\ufeff" + strings.ReplaceAll(result, "\n", "\r\n")
				}
				return result
			}
			orig := wrapCase(tc.in)
			path := filepath.Join(t.TempDir(), "SKILL.md")
			if err := os.WriteFile(path, []byte(orig), 0644); err != nil {
				t.Fatal(err)
			}
			before, err := ParseFrontmatterMap([]byte(orig))
			if err != nil {
				t.Fatal(err)
			}
			for n, want := range []string{tc.on, tc.off} {
				on, err := ToggleFrontmatterFlag(path, key)
				data, rerr := os.ReadFile(path)
				if rerr != nil {
					t.Fatal(rerr)
				}
				if want == "" {
					if err == nil || string(data) != orig {
						t.Fatalf("expected refusal, on=%v err=%v got=%q", on, err, data)
					}
					return
				}
				if err != nil || on != (tc.firstOn != (n == 1)) {
					t.Fatalf("toggle %d: on=%v err=%v", n+1, on, err)
				}
				if string(data) != wrapCase(want) {
					t.Fatalf("toggle %d: got %q want %q", n+1, data, wrapCase(want))
				}
				after, err := ParseFrontmatterMap(data)
				if err != nil {
					t.Fatal(err)
				}
				delete(after, key)
				delete(before, key)
				if !reflect.DeepEqual(after, before) {
					t.Fatalf("other fields changed: %v -> %v", before, after)
				}
			}
		})
	}
}
