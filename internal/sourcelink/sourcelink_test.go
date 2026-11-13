package sourcelink

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/trash"
	"skillshare/internal/utils"
)

func fixture(t *testing.T) (source, checkout string) {
	t.Helper()
	source = filepath.Join(t.TempDir(), "skills")
	checkout = filepath.Join(t.TempDir(), "dev-skills")
	for _, dir := range []string{source, filepath.Join(checkout, "foo")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return source, checkout
}

func TestCreateLinksDefaultName(t *testing.T) {
	source, checkout := fixture(t)
	res, err := Create(source, nil, checkout, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != filepath.Join(source, "_dev-skills") || (res.Kind != "symlink" && (runtime.GOOS != "windows" || res.Kind != "junction")) || res.Warning == "" {
		t.Fatalf("got %+v", res)
	}
	if !utils.IsSymlinkOrJunction(res.Path) {
		t.Fatal("created entry is not a link")
	}
	if target, err := utils.ResolveLinkTarget(res.Path); err != nil || target != checkout {
		t.Fatalf("link target %q, %v; want %s", target, err, checkout)
	}
}

func TestCreateCheckoutHasNoWarning(t *testing.T) {
	source, checkout := fixture(t)
	if err := os.Mkdir(filepath.Join(checkout, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Create(source, nil, checkout, "_dev")
	if err != nil || res.Warning != "" {
		t.Fatalf("got %+v, %v", res, err)
	}
}

func TestCreateRefuses(t *testing.T) {
	source, checkout := fixture(t)
	if err := os.Mkdir(filepath.Join(source, "_taken"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, target, link, want string
		targets                  []string
	}{
		{name: "existing name", target: checkout, link: "_taken", want: "already exists"},
		{name: "nested name", target: checkout, link: "a/b", want: "not a first-level name"},
		{name: "source", target: source, link: "_self", want: "target is the source or a parent of it"},
		{name: "sync target", target: checkout, link: "_dev", targets: []string{checkout}, want: "target overlaps sync target"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Create(source, tc.targets, tc.target, tc.link)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Create = %v, want %q", err, tc.want)
			}
			if tc.link != "_taken" {
				if _, err := os.Lstat(filepath.Join(source, tc.link)); !os.IsNotExist(err) {
					t.Fatalf("refused link was created: %v", err)
				}
			}
		})
	}
}

func TestRemoveMovesOnlyTheLink(t *testing.T) {
	source, checkout := fixture(t)
	// Relative link text, as a user's `ln -s` may leave it.
	rel, err := filepath.Rel(source, checkout)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rel, filepath.Join(source, "_dev")); err != nil {
		t.Fatal(err)
	}
	trashDir := t.TempDir()
	trashPath, err := Remove(source, trashDir, "_dev/")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(source, "_dev")); !os.IsNotExist(err) {
		t.Fatalf("link still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(checkout, "foo")); err != nil {
		t.Fatalf("target touched: %v", err)
	}
	if entry := trash.FindByName(trashDir, "_dev"); entry == nil || entry.LinkTarget == "" || entry.Path != trashPath {
		t.Fatalf("trash entry %+v, want the link", entry)
	}
}

func TestRemoveRefusesDirectory(t *testing.T) {
	source, _ := fixture(t)
	if err := os.Mkdir(filepath.Join(source, "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	if path, err := Remove(source, t.TempDir(), "local"); path != "" || err == nil || !strings.Contains(err.Error(), "is not a link") {
		t.Fatalf("Remove = %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "local")); err != nil {
		t.Fatal(err)
	}
}
