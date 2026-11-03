//go:build (linux && (amd64 || arm64)) || (darwin && arm64) || (windows && (amd64 || arm64))

// Hegel bundles native engines for these platforms. The deterministic lifecycle
// regressions remain available on every platform supported by this package.
package plugin

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hegel.dev/go/hegel"
)

// A removal proof permits exactly its unchanged link, never another link or a
// proof for an absent link. Generate OMP module keys, including scoped names.
func TestOMPPreviewRuntimeProofConfinedToOwnedLink(t *testing.T) {
	root := t.TempDir()
	hegel.Test(t, func(ht *hegel.T) {
		module := hegel.Draw(ht, hegel.FromRegex(`(?:@[a-zA-Z0-9][a-zA-Z0-9._~-]*/)?[a-zA-Z0-9][a-zA-Z0-9._~-]*`, true))
		ht.Assume(len(module) <= 214) // OMP's native module-name limit.
		present := hegel.Draw(ht, hegel.Booleans())
		exactPath := hegel.Draw(ht, hegel.Booleans())
		sameTarget := hegel.Draw(ht, hegel.Booleans())
		relative := hegel.Draw(ht, hegel.Booleans())
		dir, err := os.MkdirTemp(root, "case-")
		if err != nil {
			ht.Fatal(err)
		}
		defer os.RemoveAll(dir)
		link := filepath.Join(dir, "node_modules", filepath.FromSlash(module))
		if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
			ht.Fatal(err)
		}
		target := filepath.Join(dir, "retained-cache")
		if relative {
			target, err = filepath.Rel(filepath.Dir(link), target)
			if err != nil {
				ht.Fatal(err)
			}
		}
		if err := os.Symlink(target, link); err != nil {
			ht.Fatal(err)
		}
		proof := &ompRemoval{link: link, target: target, linkPresent: present}
		if !exactPath {
			proof.link = filepath.Join(dir, "other-runtime")
		}
		if !sameTarget {
			proof.target = target + "-other"
		}
		err = ompPreviewRuntime(link, proof)
		if (err == nil) != (present && exactPath && sameTarget) {
			ht.Fatalf("wrong runtime ownership decision: present=%t exact=%t target=%t err=%v", present, exactPath, sameTarget, err)
		}
		if got, err := os.Readlink(link); err != nil || got != target {
			ht.Fatalf("preview changed runtime link: %q %v", got, err)
		}
	}, hegel.WithTestCases(2000), hegel.WithDatabase(""))
}

func TestOMPPreviewRuntimeRejectsLinkedAncestors(t *testing.T) {
	root := t.TempDir()
	hegel.Test(t, func(ht *hegel.T) {
		module := hegel.Draw(ht, hegel.FromRegex(`[a-zA-Z0-9][a-zA-Z0-9._~-]*`, true))
		ht.Assume(len(module) <= 214)
		depth := min(hegel.Draw(ht, hegel.Integers(0, math.MaxInt)), 8) // Bound filesystem materialization.
		dir, err := os.MkdirTemp(root, "case-")
		if err != nil {
			ht.Fatal(err)
		}
		defer os.RemoveAll(dir)
		parent := filepath.Join(dir, "actual", strings.Repeat("nested/", depth))
		if err := os.MkdirAll(parent, 0755); err != nil {
			ht.Fatal(err)
		}
		target := filepath.Join(dir, "cache")
		if err := os.Symlink(target, filepath.Join(parent, module)); err != nil {
			ht.Fatal(err)
		}
		alias := filepath.Join(dir, "alias")
		if err := os.Symlink(filepath.Join(dir, "actual"), alias); err != nil {
			ht.Fatal(err)
		}
		link := filepath.Join(alias, strings.Repeat("nested/", depth), module)
		proof := &ompRemoval{link: link, target: target, linkPresent: true}
		if err := ompPreviewRuntime(link, proof); err == nil {
			ht.Fatal("removal proof waived a linked ancestor")
		}
	}, hegel.WithTestCases(2000), hegel.WithDatabase(""))
}

func TestOMPPreviewRuntimeRejectsChangedLinkType(t *testing.T) {
	root := t.TempDir()
	hegel.Test(t, func(ht *hegel.T) {
		module := hegel.Draw(ht, hegel.FromRegex(`[a-zA-Z0-9][a-zA-Z0-9._~-]*`, true))
		ht.Assume(len(module) <= 214)
		directory := hegel.Draw(ht, hegel.Booleans())
		dir, err := os.MkdirTemp(root, "case-")
		if err != nil {
			ht.Fatal(err)
		}
		defer os.RemoveAll(dir)
		link := filepath.Join(dir, module)
		proof := &ompRemoval{link: link, target: filepath.Join(dir, "cache"), linkPresent: true}
		if directory {
			err = os.Mkdir(link, 0755)
		} else {
			err = os.WriteFile(link, []byte("KEEP"), 0600)
		}
		if err != nil {
			ht.Fatal(err)
		}
		if err := ompPreviewRuntime(link, proof); err == nil {
			ht.Fatal("removal proof waived a real runtime path")
		}
	}, hegel.WithTestCases(2000), hegel.WithDatabase(""))
}
