package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A removal proof permits exactly its unchanged link, never another link or a
// proof for an absent link, and preview never touches the link.
func TestOMPPreviewRuntimeProofConfinedToOwnedLink(t *testing.T) {
	tests := []struct {
		name        string
		module      string
		relative    bool
		absent      bool
		otherLink   bool
		otherTarget bool
		noProof     bool
		wantOK      bool
	}{
		{name: "owned link", module: "pkg", wantOK: true},
		{name: "owned relative link", module: "pkg", relative: true, wantOK: true},
		{name: "owned scoped link", module: "@scope/pkg.name~1", wantOK: true},
		{name: "proof for an absent link", module: "pkg", absent: true},
		{name: "proof for another link", module: "pkg", otherLink: true},
		{name: "proof for another target", module: "pkg", otherTarget: true},
		{name: "relative link, other target", module: "@scope/pkg", relative: true, otherTarget: true},
		{name: "no proof", module: "pkg", noProof: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			link := filepath.Join(dir, "node_modules", filepath.FromSlash(tt.module))
			if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(dir, "retained-cache")
			if tt.relative {
				rel, err := filepath.Rel(filepath.Dir(link), target)
				if err != nil {
					t.Fatal(err)
				}
				target = rel
			}
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			proof := &ompRemoval{link: link, target: target, linkPresent: !tt.absent}
			if tt.otherLink {
				proof.link = filepath.Join(dir, "other-runtime")
			}
			if tt.otherTarget {
				proof.target = target + "-other"
			}
			if tt.noProof {
				proof = nil
			}

			err := ompPreviewRuntime(link, proof)
			if (err == nil) != tt.wantOK {
				t.Errorf("ompPreviewRuntime() error = %v, want ok=%t", err, tt.wantOK)
			}
			if got, err := os.Readlink(link); err != nil || got != target {
				t.Errorf("preview changed runtime link: %q %v", got, err)
			}
		})
	}
}

func TestOMPPreviewRuntimeRejectsLinkedAncestors(t *testing.T) {
	for _, depth := range []int{0, 1, 3} {
		t.Run(strings.Repeat("nested/", depth)+"pkg", func(t *testing.T) {
			dir := t.TempDir()
			parent := filepath.Join(dir, "actual", strings.Repeat("nested/", depth))
			if err := os.MkdirAll(parent, 0755); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(dir, "cache")
			if err := os.Symlink(target, filepath.Join(parent, "pkg")); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(dir, "alias")
			if err := os.Symlink(filepath.Join(dir, "actual"), alias); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(alias, strings.Repeat("nested/", depth), "pkg")
			proof := &ompRemoval{link: link, target: target, linkPresent: true}
			if err := ompPreviewRuntime(link, proof); err == nil {
				t.Fatal("removal proof waived a linked ancestor")
			}
		})
	}
}

func TestOMPPreviewRuntimeRejectsChangedLinkType(t *testing.T) {
	tests := []struct {
		name   string
		create func(path string) error
	}{
		{name: "directory", create: func(path string) error { return os.Mkdir(path, 0755) }},
		{name: "regular file", create: func(path string) error { return os.WriteFile(path, []byte("KEEP"), 0600) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			link := filepath.Join(dir, "pkg")
			if err := tt.create(link); err != nil {
				t.Fatal(err)
			}
			proof := &ompRemoval{link: link, target: filepath.Join(dir, "cache"), linkPresent: true}
			if err := ompPreviewRuntime(link, proof); err == nil {
				t.Fatal("removal proof waived a real runtime path")
			}
		})
	}
}
