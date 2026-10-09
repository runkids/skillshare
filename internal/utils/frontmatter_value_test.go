package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetFrontmatterValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "existing value is replaced in place, everything else untouched",
			in:   "---\n# keep this comment\nname: prototype\ndescription: d\n---\n# Body\nname: not frontmatter\n",
			want: "---\n# keep this comment\nname: emil-design-prototype\ndescription: d\n---\n# Body\nname: not frontmatter\n",
		},
		{
			name: "quoted value and trailing comment are replaced",
			in:   "---\nname: \"prototype\" # upstream\n---\nBody",
			want: "---\nname: emil-design-prototype\n---\nBody",
		},
		{
			name: "nested key of the same name is not the field",
			in:   "---\nmetadata:\n  name: prototype\n---\nBody",
			want: "---\nname: emil-design-prototype\nmetadata:\n  name: prototype\n---\nBody",
		},
		{
			name: "CRLF line endings are kept",
			in:   "---\r\nname: prototype\r\ndescription: d\r\n---\r\nBody\r\n",
			want: "---\r\nname: emil-design-prototype\r\ndescription: d\r\n---\r\nBody\r\n",
		},
		{
			name: "quoted key is the same field",
			in:   "---\n\"name\": prototype\n---\nBody",
			want: "---\n\"name\": emil-design-prototype\n---\nBody",
		},
		{
			name: "space before the colon is the same field",
			in:   "---\nname : prototype\n---\nBody",
			want: "---\nname : emil-design-prototype\n---\nBody",
		},
		{
			name: "block scalar is replaced whole, comment before the next key kept",
			in:   "---\nname: >-\n  proto\n\n  type\n# next\ndescription: d\n---\nBody",
			want: "---\nname: emil-design-prototype\n# next\ndescription: d\n---\nBody",
		},
		{
			name: "flow-style mapping keeps its other fields",
			in:   "---\n{name: prototype, description: text}\n---\nBody",
			want: "---\n{name: emil-design-prototype, description: text}\n---\nBody",
		},
		{
			name: "flow-style mapping with name last keeps the fields before it",
			in:   "---\n{description: text, name: prototype}\n---\nBody",
			want: "---\n{description: text, name: emil-design-prototype}\n---\nBody",
		},
		{
			name: "indented root mapping keeps its indent",
			in:   "---\n  name: prototype\n  description: d\n---\nBody",
			want: "---\n  name: emil-design-prototype\n  description: d\n---\nBody",
		},
		{
			name: "indented root mapping without name gets it at the same indent",
			in:   "---\n  description: d\n---\nBody",
			want: "---\n  name: emil-design-prototype\n  description: d\n---\nBody",
		},
		{
			name: "inherited name is set inside the mapping, before a document end marker",
			in:   "---\ndefaults: &d {name: prototype}\n<<: *d\n...\n---\nBody",
			want: "---\nname: emil-design-prototype\ndefaults: &d {name: prototype}\n<<: *d\n...\n---\nBody",
		},
		{
			name: "document end marker with a comment, and indented comments after it",
			in:   "---\n# head\ndefaults: &d {name: prototype}\n<<: *d\n... # end\n  # trailing\n---\nBody",
			want: "---\n# head\nname: emil-design-prototype\ndefaults: &d {name: prototype}\n<<: *d\n... # end\n  # trailing\n---\nBody",
		},
		{
			name: "anchor on the name is kept for its aliases",
			in:   "---\nname: &n prototype\ndescription: *n\n---\nBody",
			want: "---\nname: &n emil-design-prototype\ndescription: *n\n---\nBody",
		},
		{
			name: "anchor on the name is kept in a flow mapping",
			in:   "---\n{name: &n prototype, description: *n}\n---\nBody",
			want: "---\n{name: &n emil-design-prototype, description: *n}\n---\nBody",
		},
		{
			name: "anchor on the name key is kept",
			in:   "---\n&f name: prototype\ndescription: *f\n---\nBody",
			want: "---\n&f name: emil-design-prototype\ndescription: *f\n---\nBody",
		},
		{
			name: "explicit key keeps its key line",
			in:   "---\n? name\n: prototype\ndescription: d\n---\nBody",
			want: "---\n? name\n: emil-design-prototype\ndescription: d\n---\nBody",
		},
		{
			name: "inherited name under an explicit first key gets the root indent",
			in:   "---\n? defaults\n: &d {name: prototype}\n<<: *d\n---\nBody",
			want: "---\nname: emil-design-prototype\n? defaults\n: &d {name: prototype}\n<<: *d\n---\nBody",
		},
		{
			name: "no frontmatter gets one",
			in:   "# Just a body\n",
			want: "---\nname: emil-design-prototype\n---\n# Just a body\n",
		},
		{
			name: "BOM is kept and only the name value changes",
			in:   "\ufeff---\r\nname: prototype\r\ndescription: d\r\n---\r\nBody\r\n",
			want: "\ufeff---\r\nname: emil-design-prototype\r\ndescription: d\r\n---\r\nBody\r\n",
		},
		{
			name: "BOM without frontmatter stays first, the new block follows it",
			in:   "\ufeff# Just a body\n",
			want: "\ufeff---\nname: emil-design-prototype\n---\n# Just a body\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "SKILL.md")
			if err := os.WriteFile(path, []byte(tt.in), 0644); err != nil {
				t.Fatal(err)
			}
			if err := SetFrontmatterValue(path, "name", "emil-design-prototype"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, _ := os.ReadFile(path)
			if string(got) != tt.want {
				t.Errorf("content mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}
