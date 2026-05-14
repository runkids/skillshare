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
		{"unrelated-abs", "/tmp/foo", "/tmp/foo"},
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
