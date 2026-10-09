package utils

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fmSnapshot is what every frontmatter entry point returns for one SKILL.md.
type fmSnapshot struct {
	SkillName string // ParseSkillName
	Field     string // ParseFrontmatterField(name)
	Fields    string // ParseFrontmatterFields(name)
	List      string // ParseFrontmatterList(targets), comma-joined
	ListBytes string // ParseFrontmatterListFromBytes(targets), comma-joined
	Map       string // ParseFrontmatterMap: name, or "error: ..."
	Body      string // ReadSkillBody
	Rewrite   string // RewriteFrontmatterList(targets=[x]), which runs splitFrontmatterAndBody
}

// writeSkill writes content as SKILL.md in a new directory.
func writeSkill(t *testing.T, content []byte) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

func snapshotFrontmatter(t *testing.T, content string) fmSnapshot {
	t.Helper()
	dir, path := writeSkill(t, []byte(content))

	var s fmSnapshot
	name, err := ParseSkillName(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.SkillName = name
	s.Field = ParseFrontmatterField(path, "name")
	s.Fields = ParseFrontmatterFields(path, []string{"name"})["name"]
	s.List = strings.Join(ParseFrontmatterList(path, "targets"), ",")
	s.ListBytes = strings.Join(ParseFrontmatterListFromBytes([]byte(content), "targets"), ",")
	if fm, err := ParseFrontmatterMap([]byte(content)); err != nil {
		s.Map = "error: " + err.Error()
	} else {
		s.Map, _ = fm["name"].(string)
	}
	s.Body = ReadSkillBody(path)
	if out, err := RewriteFrontmatterList([]byte(content), "targets", []string{"x"}); err != nil {
		s.Rewrite = "error: " + err.Error()
	} else {
		s.Rewrite = string(out)
	}
	return s
}

// The entry points locate the --- delimiters under different rules (issue #449), so the
// same file can give different answers. These cases pin each entry point's current answer
// on the inputs where the rules differ; they are not a statement of what is right.
func TestFrontmatterEntryPoints_DelimiterRules(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    fmSnapshot
	}{
		{
			name:    "plain",
			content: "---\nname: a\ntargets: [t]\n---\nbody\n",
			want:    fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "t", ListBytes: "t", Map: "a", Body: "body", Rewrite: "---\nname: a\ntargets:\n  - x\n---\nbody\n"},
		},
		{
			name:    "no frontmatter",
			content: "# Title\n\nbody\n",
			want:    fmSnapshot{SkillName: "", Field: "", Fields: "", List: "", ListBytes: "", Map: "error: no frontmatter at the start", Body: "# Title\n\nbody", Rewrite: "---\ntargets:\n  - x\n---\n# Title\n\nbody\n"},
		},
		{
			name:    "indented opening delimiter",
			content: "  ---\nname: a\ntargets: [t]\n---\nbody\n",
			want:    fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "t", ListBytes: "t", Map: "error: no frontmatter at the start", Body: "body", Rewrite: "---\ntargets:\n  - x\n---\n  ---\nname: a\ntargets: [t]\n---\nbody\n"},
		},
		{
			name:    "indented closing delimiter",
			content: "---\nname: a\ntargets: [t]\n  ---\nbody\n",
			want:    fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "t", ListBytes: "t", Map: "error: unclosed frontmatter: no closing ---", Body: "body", Rewrite: "---\ntargets:\n  - x\n---\n---\nname: a\ntargets: [t]\n  ---\nbody\n"},
		},
		{
			name:    "blank line before the opening delimiter",
			content: "\n---\nname: a\ntargets: [t]\n---\nbody\n",
			want:    fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "t", ListBytes: "t", Map: "error: no frontmatter at the start", Body: "body", Rewrite: "---\nname: a\ntargets:\n  - x\n---\nbody\n"},
		},
		{
			name:    "text before the opening delimiter",
			content: "# Title\n---\nname: a\ntargets: [t]\n---\nbody\n",
			want:    fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "t", ListBytes: "t", Map: "error: no frontmatter at the start", Body: "# Title\n---\nname: a\ntargets: [t]\n---\nbody", Rewrite: "---\ntargets:\n  - x\n---\n# Title\n---\nname: a\ntargets: [t]\n---\nbody\n"},
		},
		{
			name:    "first line only starts with dashes",
			content: "--- title\n---\nname: a\ntargets: [t]\n---\nbody\n",
			want:    fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "t", ListBytes: "t", Map: "error: no frontmatter at the start", Body: "body", Rewrite: "---\nname: a\ntargets:\n  - x\n---\nbody\n"},
		},
		{
			name:    "unclosed block",
			content: "---\nname: a\ntargets: [t]\nbody: text\n",
			want:    fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "t", ListBytes: "t", Map: "error: unclosed frontmatter: no closing ---", Body: "", Rewrite: "---\ntargets:\n  - x\n---\n---\nname: a\ntargets: [t]\nbody: text\n"},
		},
		{
			name:    "trailing spaces after the delimiters",
			content: "--- \nname: a\ntargets: [t]\n---\t \nbody\n",
			want:    fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "t", ListBytes: "t", Map: "error: no frontmatter at the start", Body: "body", Rewrite: "---\nname: a\ntargets:\n  - x\n---\nbody\n"},
		},
		{
			name:    "CRLF line endings",
			content: "---\r\nname: a\r\ntargets: [t]\r\n---\r\nbody\r\nmore\r\n",
			want:    fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "t", ListBytes: "t", Map: "a", Body: "body\nmore", Rewrite: "---\ntargets:\n  - x\n---\n---\r\nname: a\r\ntargets: [t]\r\n---\r\nbody\r\nmore\r\n"},
		},
		{
			name:    "BOM",
			content: "\xef\xbb\xbf---\nname: a\ntargets: [t]\n---\nbody\n",
			want:    fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "t", ListBytes: "t", Map: "a", Body: "body", Rewrite: "\ufeff---\nname: a\ntargets:\n  - x\n---\nbody\n"},
		},
		{
			name:    "BOM without frontmatter",
			content: "\xef\xbb\xbf# Title\n\nbody\n",
			want:    fmSnapshot{SkillName: "", Field: "", Fields: "", List: "", ListBytes: "", Map: "error: no frontmatter at the start", Body: "\ufeff# Title\n\nbody", Rewrite: "\ufeff---\ntargets:\n  - x\n---\n# Title\n\nbody\n"},
		},
		{
			name:    "empty block",
			content: "---\n---\nbody\n",
			want:    fmSnapshot{SkillName: "", Field: "", Fields: "", List: "", ListBytes: "", Map: "error: no frontmatter", Body: "body", Rewrite: "---\ntargets:\n  - x\n---\nbody\n"},
		},
		{
			name:    "blank block",
			content: "---\n\n---\nbody\n",
			want:    fmSnapshot{SkillName: "", Field: "", Fields: "", List: "", ListBytes: "", Map: "error: no frontmatter", Body: "body", Rewrite: "---\ntargets:\n  - x\n---\nbody\n"},
		},
		{
			name:    "delimiters inside the body",
			content: "---\nname: a\ntargets: [t]\n---\nbody\n---\nname: b\ntargets: [u]\n---\ntail\n",
			want:    fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "t", ListBytes: "t", Map: "a", Body: "body\n---\nname: b\ntargets: [u]\n---\ntail", Rewrite: "---\nname: a\ntargets:\n  - x\n---\nbody\n---\nname: b\ntargets: [u]\n---\ntail\n"},
		},
		{
			name:    "indented delimiter inside a block scalar",
			content: "---\nname: a\ndescription: |\n  ---\ntargets: [t]\n---\nbody\n",
			want:    fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "", ListBytes: "", Map: "a", Body: "targets: [t]\n---\nbody", Rewrite: "---\ndescription: |\n  ---\nname: a\ntargets:\n  - x\n---\nbody\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := snapshotFrontmatter(t, tt.content)
			if got != tt.want {
				t.Errorf("entry points changed:\n got %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

// The readers that take a path used to read through a bufio.Scanner, which stops without
// an error at the first line of bufio.MaxScanTokenSize bytes or more. The readers that
// take bytes have no such cap.
func TestFrontmatterEntryPoints_OverlongLine(t *testing.T) {
	longest := strings.Repeat("x", bufio.MaxScanTokenSize-1) // still read
	overlong := longest + "x"                                // stops the path readers

	t.Run("in the frontmatter", func(t *testing.T) {
		fits := snapshotFrontmatter(t, "---\ndescription: "+longest[:len(longest)-len("description: ")]+"\nname: a\ntargets: [t]\n---\nbody\n")
		if fits.SkillName != "a" || fits.Field != "a" || fits.Fields != "a" || fits.List != "t" || fits.Body != "body" {
			t.Errorf("a line one byte under the cap must be read: %q %q %q %q %q", fits.SkillName, fits.Field, fits.Fields, fits.List, fits.Body)
		}

		content := "---\nlicense: MIT\ndescription: " + overlong[:len(overlong)-len("description: ")] + "\nname: a\ntargets: [t]\n---\nbody\n"
		_, path := writeSkill(t, []byte(content))
		if got := ParseFrontmatterFields(path, []string{"license"})["license"]; got != "MIT" {
			t.Errorf("the lines before the overlong one must still be read, got license %q", got)
		}

		got := snapshotFrontmatter(t, content)
		got.Rewrite = "" // carries the long line; covered by the delimiter cases
		want := fmSnapshot{SkillName: "", Field: "", Fields: "", List: "", ListBytes: "t", Map: "a", Body: "", Rewrite: ""}
		if got != want {
			t.Errorf("entry points changed:\n got %#v\nwant %#v", got, want)
		}
	})

	t.Run("in the body", func(t *testing.T) {
		got := snapshotFrontmatter(t, "---\nname: a\ntargets: [t]\n---\nfirst\n"+overlong+"\nlast\n")
		got.Rewrite = ""
		want := fmSnapshot{SkillName: "a", Field: "a", Fields: "a", List: "t", ListBytes: "t", Map: "a", Body: "first", Rewrite: ""}
		if got != want {
			t.Errorf("entry points changed:\n got %#v\nwant %#v", got, want)
		}
	})

	t.Run("before any delimiter", func(t *testing.T) {
		content := overlong + "\n---\nname: a\ntargets: [t]\n---\nbody\n"
		got := snapshotFrontmatter(t, content)
		if got.Body != strings.TrimSpace(content) {
			t.Errorf("ReadSkillBody must return the whole file, got %d bytes", len(got.Body))
		}
		got.Body, got.Rewrite = "", ""
		want := fmSnapshot{SkillName: "", Field: "", Fields: "", List: "", ListBytes: "t", Map: "error: no frontmatter at the start", Body: "", Rewrite: ""}
		if got != want {
			t.Errorf("entry points changed:\n got %#v\nwant %#v", got, want)
		}
	})
}

// A path that opens but cannot be read (a directory) is "no frontmatter", not an error.
func TestFrontmatterEntryPoints_UnreadablePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}

	if name, err := ParseSkillName(dir); name != "" || err != nil {
		t.Errorf("ParseSkillName = %q, %v; want empty name and no error", name, err)
	}
	if got := ParseFrontmatterField(path, "name"); got != "" {
		t.Errorf("ParseFrontmatterField = %q", got)
	}
	if got := ParseFrontmatterFields(path, []string{"name"}); len(got) != 0 {
		t.Errorf("ParseFrontmatterFields = %v", got)
	}
	if got := ParseFrontmatterList(path, "targets"); got != nil {
		t.Errorf("ParseFrontmatterList = %v", got)
	}
	if got := ReadSkillBody(path); got != "" {
		t.Errorf("ReadSkillBody = %q", got)
	}
}

func TestParseSkillName_MissingFile(t *testing.T) {
	if _, err := ParseSkillName(t.TempDir()); !os.IsNotExist(err) {
		t.Errorf("err = %v, want not-exist", err)
	}
}

// CRLF inside values: the path readers dropped the \r per line, the bytes reader hands
// it to the YAML decoder. Both must give the same values.
func TestFrontmatterEntryPoints_CRLFValues(t *testing.T) {
	content := "---\r\nname: a\r\ndescription: |\r\n  one\r\n  two\r\nsummary: >-\r\n  three\r\n  four\r\nquoted: \"five\r\n  six\"\r\ntargets:\r\n  - t\r\n  - u\r\n---\r\nbody\r\n"
	_, path := writeSkill(t, []byte(content))

	got := ParseFrontmatterFields(path, []string{"description", "summary", "quoted"})
	want := map[string]string{"description": "one\ntwo\n", "summary": "three four", "quoted": "five six"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("ParseFrontmatterFields[%s] = %q, want %q", k, got[k], v)
		}
	}
	if got := ParseFrontmatterField(path, "description"); got != "one two" {
		t.Errorf("ParseFrontmatterField = %q", got)
	}
	if got := strings.Join(ParseFrontmatterList(path, "targets"), ","); got != "t,u" {
		t.Errorf("ParseFrontmatterList = %q", got)
	}
	if got := strings.Join(ParseFrontmatterListFromBytes([]byte(content), "targets"), ","); got != "t,u" {
		t.Errorf("ParseFrontmatterListFromBytes = %q", got)
	}
}

// A block scalar on the last frontmatter line: the readers that rebuilt the block from
// lines dropped its final newline, ParseFrontmatterMap decodes the block as written.
func TestFrontmatterEntryPoints_BlockScalarOnLastLine(t *testing.T) {
	content := "---\nname: a\ndescription: |\n  text\n---\nbody\n"
	_, path := writeSkill(t, []byte(content))

	if got := ParseFrontmatterFields(path, []string{"description"})["description"]; got != "text" {
		t.Errorf("ParseFrontmatterFields = %q, want %q", got, "text")
	}
	fm, err := ParseFrontmatterMap([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	if got := fm["description"]; got != "text\n" {
		t.Errorf("ParseFrontmatterMap = %q, want %q", got, "text\n")
	}
}

// The path readers drop one \r per line before decoding, so \r\r\n is a single line break
// for them and two for the bytes reader.
func TestFrontmatterEntryPoints_DoubleCarriageReturn(t *testing.T) {
	content := "---\r\r\ntargets:\r\r\n  - |\r\r\n    t\r\r\n    u\r\r\n---\r\r\nbody\r\r\n"
	_, path := writeSkill(t, []byte(content))

	if got := ParseFrontmatterList(path, "targets"); len(got) != 1 || got[0] != "t\nu\n" {
		t.Errorf("ParseFrontmatterList = %q", got)
	}
	if got := ParseFrontmatterListFromBytes([]byte(content), "targets"); len(got) != 1 || got[0] != "\nt\n\nu\n" {
		t.Errorf("ParseFrontmatterListFromBytes = %q", got)
	}
	if got := ReadSkillBody(path); got != "body" {
		t.Errorf("ReadSkillBody = %q", got)
	}
}

// The readers that take a path keep only the frontmatter block: neither the body after
// it nor the text of a file without one costs them memory.
func TestFrontmatterEntryPoints_PathReadersKeepOnlyTheBlock(t *testing.T) {
	filler := bytes.Repeat([]byte("body\n"), 1<<19)
	tests := []struct {
		name    string
		content []byte
		want    string
	}{
		{"large body after the block", append([]byte("---\nname: a\ntargets: [a]\n---\n"), filler...), "a"},
		{"large file without frontmatter", filler, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, path := writeSkill(t, tt.content)

			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			name, err := ParseSkillName(dir)
			field := ParseFrontmatterField(path, "name")
			fields := ParseFrontmatterFields(path, []string{"name"})["name"]
			list := strings.Join(ParseFrontmatterList(path, "targets"), ",")
			runtime.ReadMemStats(&after)

			if err != nil || name != tt.want || field != tt.want || fields != tt.want || list != tt.want {
				t.Fatalf("got %q, %v, %q, %q, %q; want %q from each", name, err, field, fields, list, tt.want)
			}
			if got := after.TotalAlloc - before.TotalAlloc; got > 1<<20 {
				t.Errorf("the readers allocated %d bytes, want under 1 MiB", got)
			}
		})
	}
}

// ParseSkillName and ParseFrontmatterField stop at the line they look for, so a large
// frontmatter block after it, closed or not, costs them no memory.
func TestFrontmatterEntryPoints_KeyReadersStopAtTheKey(t *testing.T) {
	filler := bytes.Repeat([]byte("k: v\n"), 1<<19)
	tests := []struct {
		name    string
		content []byte
	}{
		{"closed block", append(append([]byte("---\nname: a\n"), filler...), "---\nbody\n"...)},
		{"unclosed block", append([]byte("---\nname: a\n"), filler...)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, path := writeSkill(t, tt.content)

			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			name, err := ParseSkillName(dir)
			field := ParseFrontmatterField(path, "name")
			runtime.ReadMemStats(&after)

			if err != nil || name != "a" || field != "a" {
				t.Fatalf("got %q, %v, %q; want %q from each", name, err, field, "a")
			}
			if got := after.TotalAlloc - before.TotalAlloc; got > 1<<20 {
				t.Errorf("the readers allocated %d bytes, want under 1 MiB", got)
			}
		})
	}
}

func TestParseSkillName_ValidYAMLForms(t *testing.T) {
	for _, fm := range []string{
		`"name": prototype`,
		`name : prototype`,
		`name: prototype # upstream`,
		`{name: prototype, description: d}`,
		"metadata:\n  name: other\nname: prototype",
		"name: >-\n  prototype\ndescription: d",
		"base: &n prototype\nname: *n",
		"defaults: &d {name: prototype}\n<<: *d",
		"? name\n: prototype",
		"name: \"proto\\\n  type\"",
		"name: prototype\nbad: [unclosed", // not YAML: the name: line is still read
	} {
		dir, _ := writeSkill(t, []byte("---\n"+fm+"\n---\nbody\n"))
		if got, err := ParseSkillName(dir); err != nil || got != "prototype" {
			t.Errorf("%q: ParseSkillName = %q, %v; want prototype", fm, got, err)
		}
	}
}
