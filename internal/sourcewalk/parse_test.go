package sourcewalk

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseDeclarations(t *testing.T) {
	root := t.TempDir()
	invalid := []string{".", "..", "a/b", `a\b`, "/absolute", "C:", "C:relative", `\\server\share`, "a/../b", "*", "a?", "[ab]", "!a", "a{b,c}"}
	base := "  # comment\n\n alpha \nalpha\n" + strings.Join(invalid, "\n")
	if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte(base), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillfollow.local"), []byte("alpha\nbeta\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := readDeclarations(root)
	if !reflect.DeepEqual(got.names, []string{"alpha", "beta"}) || !got.local || !got.active {
		t.Fatalf("%+v", got)
	}
	if len(got.warnings) != len(invalid) {
		t.Fatalf("warnings: %v", got.warnings)
	}
	for _, warning := range got.warnings {
		if !strings.Contains(warning, ".skillfollow:") {
			t.Fatal(warning)
		}
	}
}

func TestParseMissingDeclarations(t *testing.T) {
	got := readDeclarations(t.TempDir())
	if got.active || got.local || len(got.names) != 0 || len(got.warnings) != 0 {
		t.Fatalf("%+v", got)
	}
}
