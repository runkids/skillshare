package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFoldHomePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("UserHomeDir unavailable")
	}
	sep := string(filepath.Separator)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"already-tilde", "~/foo", "~/foo"},
		{"exact-home", home, "~"},
		{"home-child", home + sep + "foo", "~" + sep + "foo"},
		{"home-deep", home + sep + ".config" + sep + "skillshare", "~" + sep + ".config" + sep + "skillshare"},
		{"sibling-not-home", home + "_other" + sep + "foo", home + "_other" + sep + "foo"},
		{"unrelated-abs", "/opt/unrelated/foo", "/opt/unrelated/foo"},
		{"relative", "foo/bar", "foo/bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FoldHomePath(tt.in)
			if got != tt.want {
				t.Errorf("FoldHomePath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFoldHomePath_WindowsCaseInsensitive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only case-fold behavior")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("UserHomeDir unavailable")
	}
	// Pretend the path comes back with different casing (common on Windows).
	mixed := strings.ToLower(home) + string(filepath.Separator) + "Foo"
	got := FoldHomePath(mixed)
	if !strings.HasPrefix(got, "~") {
		t.Errorf("FoldHomePath(%q) = %q, expected fold under mixed case", mixed, got)
	}
}

func TestPathWithin(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "skills")

	for _, tc := range []struct {
		name string
		path string
		want bool
	}{
		{name: "root", path: root, want: true},
		{name: "child", path: filepath.Join(root, "demo"), want: true},
		{name: "nested child", path: filepath.Join(root, "team", "demo"), want: true},
		{name: "sibling prefix", path: filepath.Join(parent, "skills-backup", "demo"), want: false},
		{name: "parent", path: parent, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := PathWithin(tc.path, root); got != tc.want {
				t.Fatalf("PathWithin(%q, %q) = %v, want %v", tc.path, root, got, tc.want)
			}
		})
	}
}
