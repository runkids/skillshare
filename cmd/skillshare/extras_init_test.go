package main

import (
	"strings"
	"testing"
)

func TestValidateExtrasInit(t *testing.T) {
	target := func(mode string, flatten bool, as string) []extrasInitTarget {
		return []extrasInitTarget{{path: "/tmp/t", mode: mode, flatten: flatten, as: as}}
	}
	cases := []struct {
		name string
		opts extrasInitOptions
		want string // "" = valid
	}{
		{"single file with as", extrasInitOptions{name: "p", file: "system.md", targets: target("", false, "APPEND.md")}, ""},
		{"single file import", extrasInitOptions{name: "p", file: "system.md", targets: target("import", false, "")}, ""},
		{"as without file", extrasInitOptions{name: "p", targets: target("", false, "APPEND.md")}, "--as requires --file"},
		{"import without file", extrasInitOptions{name: "p", targets: target("import", false, "")}, "import mode requires a single-file extra"},
		{"flatten with file", extrasInitOptions{name: "p", file: "system.md", targets: target("", true, "")}, "flatten cannot be used with a single-file extra"},
		{"file is a path", extrasInitOptions{name: "p", file: "a/system.md", targets: target("", false, "")}, "must be a plain filename"},
		{"as is a path", extrasInitOptions{name: "p", file: "system.md", targets: target("", false, "../x.md")}, "must be a plain filename"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateExtrasInit(tc.opts)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
