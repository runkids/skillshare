//go:build windows

package sync

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/utils"
)

func TestCreateLinkThroughNestedJunctionUsesAbsoluteTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	physical := filepath.Join(root, "physical", "nested")
	for _, dir := range []string{source, physical} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(root, "alias")
	if err := utils.CreateJunction(alias, filepath.Dir(physical)); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(alias, "nested")
	if relativeLinkPathSupported(target) || shouldUseRelative(root, source, target) {
		t.Fatal("a junction ancestor must select absolute-link mode")
	}
	link := filepath.Join(target, "skill")
	if err := createLink(link, source, true, source); err != nil {
		t.Fatal(err)
	}
	raw, err := os.Readlink(link)
	if err != nil || !filepath.IsAbs(raw) {
		t.Fatalf("link target = %q, %v; want absolute path", raw, err)
	}
	want, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, filepath.Join(physical, "skill")} {
		got, err := os.Stat(path)
		if err != nil || !os.SameFile(got, want) {
			t.Fatalf("link is not reachable through %q: %v", path, err)
		}
	}
}
