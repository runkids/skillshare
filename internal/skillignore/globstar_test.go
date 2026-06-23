package skillignore

import (
	"strings"
	"testing"
	"time"
)

// RepeatedGlobstarDoesNotBacktrackExponentially ensures that patterns with
// multiple consecutive ** segments complete in polynomial time.
//
// Repeated globstar patterns used to trigger exponential recursive
// backtracking. This regression test uses a generous timeout only to
// guard against the previous hang; it is not intended as a benchmark.
func TestRepeatedGlobstarDoesNotBacktrackExponentially(t *testing.T) {
	pattern := "**/**/**/**/0"
	m := Compile([]string{pattern})

	segs := make([]string, 150)
	for i := range segs {
		segs[i] = "1"
	}
	path := strings.Join(segs, "/")

	ch := make(chan bool, 1)
	go func() {
		ch <- m.Match(path, false)
	}()
	select {
	case matched := <-ch:
		if matched {
			t.Error("expected false: path has no segment '0'")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Match with multiple ** segments hung: exponential backtracking regression")
	}
}

// TestGlobstarMatchingSemantics guards the core ** matching behavior
// to ensure memoization did not alter results. Expected values are
// derived from the current implementation, not from a gitignore spec.
func TestGlobstarMatchingSemantics(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		path     string
		isDir    bool
		want     bool
	}{
		{
			name:     "single globstar matches zero segments",
			patterns: []string{"**/foo"},
			path:     "foo",
			isDir:    false,
			want:     true,
		},
		{
			name:     "single globstar matches multiple segments",
			patterns: []string{"**/foo"},
			path:     "a/b/foo",
			isDir:    false,
			want:     true,
		},
		{
			name:     "repeated globstar matches zero segments",
			patterns: []string{"**/**/foo"},
			path:     "foo",
			isDir:    false,
			want:     true,
		},
		{
			name:     "repeated globstar matches nested path",
			patterns: []string{"**/**/foo"},
			path:     "a/b/foo",
			isDir:    false,
			want:     true,
		},
		{
			name:     "triple globstar matches shallow path",
			patterns: []string{"**/**/**/z"},
			path:     "a/z",
			isDir:    false,
			want:     true,
		},
		{
			name:     "triple globstar matches deep path",
			patterns: []string{"**/**/**/z"},
			path:     "a/b/z",
			isDir:    false,
			want:     true,
		},
		{
			name:     "quad globstar matches deep path",
			patterns: []string{"**/**/**/**/z"},
			path:     "a/b/c/z",
			isDir:    false,
			want:     true,
		},
		{
			name:     "repeated globstar returns false when suffix missing",
			patterns: []string{"**/**/missing"},
			path:     "a/b/foo",
			isDir:    false,
			want:     false,
		},
		{
			name:     "anchored repeated globstar matches zero middle segments",
			patterns: []string{"a/**/**/z"},
			path:     "a/z",
			isDir:    false,
			want:     true,
		},
		{
			name:     "anchored repeated globstar matches deep middle",
			patterns: []string{"a/**/**/z"},
			path:     "a/b/c/z",
			isDir:    false,
			want:     true,
		},
		{
			name:     "anchored repeated globstar no match wrong prefix",
			patterns: []string{"a/**/**/z"},
			path:     "b/c/z",
			isDir:    false,
			want:     false,
		},
		{
			name:     "quad globstar positive match",
			patterns: []string{"**/**/**/**/0"},
			path:     "1/2/3/4/0",
			isDir:    false,
			want:     true,
		},
		{
			name:     "trailing repeated globstar",
			patterns: []string{"foo/**/**"},
			path:     "foo/a/b",
			isDir:    false,
			want:     true,
		},
		{
			name:     "globstar matches directory isDir true",
			patterns: []string{"**/foo"},
			path:     "a/foo",
			isDir:    true,
			want:     true,
		},
		{
			name:     "globstar no match directory isDir true",
			patterns: []string{"**/missing"},
			path:     "a/foo",
			isDir:    true,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Compile(tt.patterns)
			got := m.Match(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("Compile(%q).Match(%q, %v) = %v, want %v",
					tt.patterns, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}
