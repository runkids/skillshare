package contextrouter

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCleanRel(t *testing.T) {
	for in, want := range map[string]string{
		"wiki/a.md":      "wiki/a.md",
		"wiki\\a.md":     "wiki/a.md",
		"./wiki//a.md":   "wiki/a.md",
		"a/../wiki/a.md": "wiki/a.md",
	} {
		if got, ok := cleanRel(in); !ok || got != want {
			t.Errorf("%q: got %q %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "../x", "a/../../x", "/etc/passwd", "C:/x", "\\\\host\\share", "a\x00b"} {
		if got, ok := cleanRel(in); ok {
			t.Errorf("%q accepted as %q", in, got)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func openRoot(t *testing.T, dir string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

func TestReadRegular(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "root")
	// A sibling whose name starts with the root's: a prefix check would let it through.
	evil := filepath.Join(base, "root-evil")
	writeFile(t, filepath.Join(dir, "wiki", "a.md"), "a\r\nb")
	writeFile(t, filepath.Join(dir, "docs", "b.md"), "inside")
	writeFile(t, filepath.Join(dir, "big.md"), strings.Repeat("x", 11))
	writeFile(t, filepath.Join(dir, "bad.md"), "\xff")
	writeFile(t, filepath.Join(evil, "secret.md"), "secret")
	writeFile(t, filepath.Join(evil, "wiki", "a.md"), "secret")
	if err := os.Mkdir(filepath.Join(dir, "folder.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	root := openRoot(t, dir)

	for _, tc := range []struct{ rel, text, code string }{
		{"wiki/a.md", "a\nb", ""},
		{"nope.md", "", ErrMissing},
		{"folder.md", "", ErrNotFile},
		{"big.md", "", ErrTooLarge},
		{"bad.md", "", ErrInvalidUTF8},
	} {
		if text, code := readRegular(root, tc.rel, 10); text != tc.text || code != tc.code {
			t.Errorf("%s: got %q %q, want %q %q", tc.rel, text, code, tc.text, tc.code)
		}
	}

	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	if err := os.Symlink("../docs/b.md", filepath.Join(dir, "wiki", "in.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(evil, "secret.md"), filepath.Join(dir, "wiki", "out.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../root-evil/secret.md", filepath.Join(dir, "wiki", "rel-out.md")); err != nil {
		t.Fatal(err)
	}
	if text, code := readRegular(root, "wiki/in.md", 10); text != "inside" || code != "" {
		t.Errorf("symlink inside the folder: got %q %q", text, code)
	}
	for _, rel := range []string{"wiki/out.md", "wiki/rel-out.md"} {
		if text, code := readRegular(root, rel, 10); text != "" || code != ErrOutside {
			t.Errorf("%s: got %q %q, want outside", rel, text, code)
		}
	}

	// The wiki directory itself points out of the folder.
	linked := filepath.Join(base, "linked")
	if err := os.Mkdir(linked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(evil, "wiki"), filepath.Join(linked, "wiki")); err != nil {
		t.Fatal(err)
	}
	if text, code := readRegular(openRoot(t, linked), "wiki/a.md", 10); text != "" || code != ErrOutside {
		t.Errorf("parent symlink: got %q %q, want outside", text, code)
	}
}
