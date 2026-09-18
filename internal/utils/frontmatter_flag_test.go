package utils

import (
	"os"
	"path/filepath"
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
