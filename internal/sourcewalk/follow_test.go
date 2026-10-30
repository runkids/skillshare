package sourcewalk

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// followFixture builds a source with a first-level link _dev to a checkout
// named differently, which holds a nested link that must stay a link.
func followFixture(t *testing.T) (root, checkout string) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "skills")
	write(t, filepath.Join(root, "a", "SKILL.md"))
	write(t, filepath.Join(root, "z"))
	checkout = filepath.Join(t.TempDir(), "checkout")
	write(t, filepath.Join(checkout, "foo", "SKILL.md"))
	other := t.TempDir()
	write(t, filepath.Join(other, "f"))
	symlink(t, other, filepath.Join(checkout, "foo", "nested"))
	symlink(t, checkout, filepath.Join(root, "_dev"))
	return root, checkout
}

func walkEntries(t *testing.T, root string, opts Options, skip string) (walk, walkDir []string) {
	t.Helper()
	// Keep passing the caller's root so Walk's own root resolution stays covered.
	base := resolvedPath(t, root)
	label := func(path string, dir bool, mode fs.FileMode) string {
		rel, _ := filepath.Rel(base, path)
		kind := "file"
		switch {
		case dir:
			kind = "dir"
		case mode&fs.ModeSymlink != 0:
			kind = "link"
		}
		return filepath.ToSlash(rel) + ":" + kind
	}
	err := Walk(root, opts, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		walk = append(walk, label(path, info.IsDir(), info.Mode()))
		if filepath.Base(path) != info.Name() {
			t.Fatalf("info name %q for %s", info.Name(), path)
		}
		if filepath.Base(path) == skip {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = WalkDir(root, opts, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		walkDir = append(walkDir, label(path, d.IsDir(), d.Type()))
		if filepath.Base(path) != d.Name() {
			t.Fatalf("entry name %q for %s", d.Name(), path)
		}
		if filepath.Base(path) == skip {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return walk, walkDir
}

func TestFollowFirstLevelLink(t *testing.T) {
	root, _ := followFixture(t)
	follow := NewFollow(root, nil)
	walk, walkDir := walkEntries(t, root, Options{Follow: follow}, "")
	want := []string{".:dir", "_dev:dir", "_dev/foo:dir", "_dev/foo/SKILL.md:file", "_dev/foo/nested:link", "a:dir", "a/SKILL.md:file", "z:file"}
	if !reflect.DeepEqual(walk, want) || !reflect.DeepEqual(walkDir, want) {
		t.Fatalf("Walk %v\nWalkDir %v\nwant %v", walk, walkDir, want)
	}
	entries, err := ReadDir(root, Options{Follow: follow})
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Name() != "_dev" || !entries[0].IsDir() {
		t.Fatalf("ReadDir entry %v", entries[0])
	}
	if info, err := entries[0].Info(); err != nil || info.Name() != "_dev" || !info.IsDir() {
		t.Fatalf("ReadDir info %v, %v", info, err)
	}
	if got := follow.Skipped(); len(got) != 0 {
		t.Fatalf("skipped %v", got)
	}
}

func TestFollowSkipDirOnLink(t *testing.T) {
	root, _ := followFixture(t)
	walk, walkDir := walkEntries(t, root, Options{Follow: NewFollow(root, nil)}, "_dev")
	want := []string{".:dir", "_dev:dir", "a:dir", "a/SKILL.md:file", "z:file"}
	if !reflect.DeepEqual(walk, want) || !reflect.DeepEqual(walkDir, want) {
		t.Fatalf("Walk %v\nWalkDir %v\nwant %v", walk, walkDir, want)
	}
}

func TestFollowDisabledKeepsLinks(t *testing.T) {
	root, _ := followFixture(t)
	walk, walkDir := walkEntries(t, root, Options{}, "")
	want := []string{".:dir", "_dev:link", "a:dir", "a/SKILL.md:file", "z:file"}
	if !reflect.DeepEqual(walk, want) || !reflect.DeepEqual(walkDir, want) {
		t.Fatalf("Walk %v\nWalkDir %v\nwant %v", walk, walkDir, want)
	}
	entries, err := ReadDir(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Type()&fs.ModeSymlink == 0 {
		t.Fatalf("ReadDir entry %v", entries[0])
	}
}

func TestFollowThroughLinkedRoot(t *testing.T) {
	root, _ := followFixture(t)
	alias := filepath.Join(t.TempDir(), "alias")
	symlink(t, root, alias)
	var got []string
	err := Walk(alias, Options{Follow: NewFollow(alias, nil)}, func(path string, info os.FileInfo, err error) error {
		rel, _ := filepath.Rel(resolvedPath(t, root), path)
		got = append(got, filepath.ToSlash(rel))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[:3], []string{".", "_dev", "_dev/foo"}) {
		t.Fatalf("got %v", got)
	}
}

func TestFollowGuards(t *testing.T) {
	for _, tc := range []struct {
		name        string
		link        func(root, checkout string) string // link target
		targets     func(root, checkout string) []string
		unavailable bool
	}{
		{name: "missing", link: func(_, c string) string { return filepath.Join(c, "gone") }, unavailable: true},
		{name: "root", link: func(r, _ string) string { return r }},
		{name: "ancestor", link: func(r, _ string) string { return filepath.Dir(r) }},
		{name: "target equal", link: func(_, c string) string { return c }, targets: func(_, c string) []string { return []string{c} }},
		{name: "target inside", link: func(_, c string) string { return c }, targets: func(_, c string) []string { return []string{filepath.Join(c, "not-yet", "skills")} }},
		{name: "target contains", link: func(_, c string) string { return filepath.Join(c, "foo") }, targets: func(_, c string) []string { return []string{c} }},
		// A link into a sync target that is not created yet is an overlap, not a
		// missing target: it must not hold back pruning.
		{name: "target missing but overlapping", link: func(_, c string) string { return filepath.Join(c, "not-yet", "skills") }, targets: func(_, c string) []string { return []string{filepath.Join(c, "not-yet")} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, checkout := followFixture(t)
			if err := os.Remove(filepath.Join(root, "_dev")); err != nil {
				t.Fatal(err)
			}
			symlink(t, tc.link(root, checkout), filepath.Join(root, "_dev"))
			var targets []string
			if tc.targets != nil {
				targets = tc.targets(root, checkout)
			}
			follow := NewFollow(root, targets)
			walk, walkDir := walkEntries(t, root, Options{Follow: follow}, "")
			want := []string{".:dir", "_dev:link", "a:dir", "a/SKILL.md:file", "z:file"}
			if !reflect.DeepEqual(walk, want) || !reflect.DeepEqual(walkDir, want) {
				t.Fatalf("Walk %v\nWalkDir %v\nwant %v", walk, walkDir, want)
			}
			if _, err := ReadDir(root, Options{Follow: follow}); err != nil {
				t.Fatal(err)
			}
			skipped := follow.Skipped()
			if len(skipped) != 1 || skipped[0].Name != "_dev" || skipped[0].Reason == "" {
				t.Fatalf("want one warning per operation, got %v", skipped)
			}
			if follow.Incomplete() != tc.unavailable {
				t.Fatalf("incomplete = %v, want %v (%v)", follow.Incomplete(), tc.unavailable, skipped)
			}
		})
	}
}

func TestFollowFileLinkStaysEntry(t *testing.T) {
	root, checkout := followFixture(t)
	symlink(t, filepath.Join(checkout, "foo", "SKILL.md"), filepath.Join(root, "shared.md"))
	follow := NewFollow(root, nil)
	walk, _ := walkEntries(t, root, Options{Follow: follow}, "")
	if walk[len(walk)-2] != "shared.md:link" || len(follow.Skipped()) != 0 {
		t.Fatalf("walk %v, skipped %v", walk, follow.Skipped())
	}
}

func TestFollowSiblingTargetIsNotOverlap(t *testing.T) {
	root, checkout := followFixture(t)
	follow := NewFollow(root, []string{checkout + "-copy"})
	walk, _ := walkEntries(t, root, Options{Follow: follow}, "")
	if len(walk) != 8 || len(follow.Skipped()) != 0 {
		t.Fatalf("walk %v, skipped %v", walk, follow.Skipped())
	}
}

func TestFollowAllow(t *testing.T) {
	root, checkout := followFixture(t)
	for _, tc := range []struct {
		name, target, want string
		targets            []string
	}{
		{name: "checkout", target: checkout},
		{name: "root", target: root, want: "target is the source or a parent of it"},
		{name: "ancestor", target: filepath.Dir(root), want: "target is the source or a parent of it"},
		{name: "inside source", target: filepath.Join(root, "a"), want: "target is inside the source"},
		{name: "overlaps target", target: checkout, targets: []string{filepath.Join(checkout, "foo")}, want: "target overlaps sync target"},
		{name: "missing", target: filepath.Join(checkout, "gone"), want: "target is missing"},
		{name: "file", target: filepath.Join(checkout, "foo", "SKILL.md"), want: "target is not a directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := NewFollow(root, tc.targets).Allow(tc.target)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Allow: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Allow = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestFollowWalkErrors(t *testing.T) {
	for _, walker := range []string{"Walk", "WalkDir"} {
		for _, failure := range []string{"unreadable-subdirectory", "missing-target", "vanished-child"} {
			t.Run(walker+"/"+failure, func(t *testing.T) {
				source, target := resolvedPath(t, t.TempDir()), resolvedPath(t, t.TempDir())
				write(t, filepath.Join(source, "healthy", "SKILL.md"))
				write(t, filepath.Join(target, "a", "SKILL.md"))
				write(t, filepath.Join(target, "z", "SKILL.md"))
				link := filepath.Join(source, "linked")
				symlink(t, target, link)
				if failure == "unreadable-subdirectory" {
					dir := filepath.Join(target, "z")
					if err := os.Chmod(dir, 0000); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { os.Chmod(dir, 0755) })
					if _, err := os.ReadDir(dir); !os.IsPermission(err) {
						t.Skip("requires directory read permissions to be enforced")
					}
				}
				follow := NewFollow(source, nil)
				healthy, followedHealthy, errorsSeen := false, false, 0
				visit := func(path string, err error) error {
					if err != nil {
						errorsSeen++
						return nil
					}
					if path == filepath.Join(source, "healthy", "SKILL.md") {
						healthy = true
					}
					if path == filepath.Join(link, "a", "SKILL.md") {
						followedHealthy = true
					}
					if failure == "missing-target" && path == link {
						if err := os.Rename(target, target+".away"); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { os.Rename(target+".away", target) })
					}
					if failure == "vanished-child" && path == filepath.Join(link, "a") {
						if err := os.Rename(filepath.Join(target, "z"), filepath.Join(target, "gone")); err != nil {
							t.Fatal(err)
						}
					}
					return nil
				}
				var err error
				if walker == "Walk" {
					err = Walk(source, Options{Follow: follow}, func(path string, _ os.FileInfo, err error) error { return visit(path, err) })
				} else {
					err = WalkDir(source, Options{Follow: follow}, func(path string, _ fs.DirEntry, err error) error { return visit(path, err) })
				}
				if err != nil || !healthy || errorsSeen == 0 || (failure != "missing-target" && !followedHealthy) {
					t.Fatalf("err=%v healthy=%v followedHealthy=%v errors=%d", err, healthy, followedHealthy, errorsSeen)
				}
				skipped := follow.Skipped()
				if failure == "vanished-child" {
					if follow.Incomplete() || len(skipped) != 0 {
						t.Fatalf("vanished child made inventory incomplete: %+v", skipped)
					}
				} else {
					wantReason := "target is not readable"
					if failure == "missing-target" {
						wantReason = "target is missing"
					}
					if !follow.Incomplete() || len(skipped) != 1 || skipped[0].Name != "linked" || !skipped[0].Unavailable || skipped[0].Reason != wantReason {
						t.Fatalf("walk failure not recorded: incomplete=%v skipped=%+v", follow.Incomplete(), skipped)
					}
				}
			})
		}
	}
}
