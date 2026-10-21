package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetachSkillsTarget_DryRun(t *testing.T) {
	for _, mode := range []string{"whole-folder", "merge", "copy"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			target := filepath.Join(root, "tool", "skills")
			writeDetachFile(t, filepath.Join(source, "alpha", "SKILL.md"), "source")
			writeDetachFile(t, filepath.Join(root, "tool", "agents", "helper.md"), "agent")
			want := DetachReport{Removed: []string{target}}
			if mode == "whole-folder" {
				if err := os.Symlink(source, target); err != nil {
					t.Fatal(err)
				}
			} else {
				manifest := filepath.Join(target, ManifestFile)
				alpha := filepath.Join(target, "alpha")
				local := filepath.Join(target, "notes.txt")
				writeDetachFile(t, manifest, "manifest")
				writeDetachFile(t, local, "local")
				want = DetachReport{Removed: []string{manifest}, Retained: []string{local}}
				if mode == "merge" {
					external := filepath.Join(target, "external")
					outside := filepath.Join(root, "source-other", "alpha")
					writeDetachFile(t, filepath.Join(outside, "SKILL.md"), "external")
					if err := os.Symlink(outside, external); err != nil {
						t.Fatal(err)
					}
					want.Retained = []string{external, local}
					if err := os.Symlink(filepath.Join(source, "alpha"), alpha); err != nil {
						t.Fatal(err)
					}
					missing := filepath.Join(target, "missing")
					if err := os.Symlink(filepath.Join(source, "missing"), missing); err != nil {
						t.Fatal(err)
					}
					want.Removed = append(want.Removed, alpha, missing)
					want.CopiedBack = []string{alpha}
				} else {
					writeDetachFile(t, filepath.Join(alpha, "SKILL.md"), "edited copy")
					want.Retained = []string{alpha, local}
				}
			}
			before := snapshotDetachTree(t, root)
			preview, err := DetachSkillsTarget(target, DetachOptions{Source: source, DryRun: true})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(preview, want) {
				t.Errorf("preview = %+v, want %+v", preview, want)
			}
			if after := snapshotDetachTree(t, root); !reflect.DeepEqual(before, after) {
				t.Errorf("dry-run changed source/target/agents tree: before=%v after=%v", before, after)
			}
			applied, err := DetachSkillsTarget(target, DetachOptions{Source: source})
			if err != nil || !reflect.DeepEqual(applied, preview) {
				t.Errorf("apply = %+v, %v; want preview %+v", applied, err, preview)
			}
		})
	}
}

func TestDetachSkillsTarget_CopyFailureReportsRemovedLink(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	writeDetachFile(t, filepath.Join(source, "alpha", "SKILL.md"), "source")
	writeDetachFile(t, filepath.Join(target, ManifestFile), "manifest")
	// Existing copy behavior fails when it tries to open a dangling nested link.
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(source, "alpha", "broken")); err != nil {
		t.Fatal(err)
	}
	alpha := filepath.Join(target, "alpha")
	if err := os.Symlink(filepath.Join(source, "alpha"), alpha); err != nil {
		t.Fatal(err)
	}
	report, err := DetachSkillsTarget(target, DetachOptions{Source: source})
	if err == nil {
		t.Fatal("expected copy failure")
	}
	want := DetachReport{Removed: []string{filepath.Join(target, ManifestFile), alpha}}
	if !reflect.DeepEqual(report, want) {
		t.Errorf("completed work = %+v, want %+v", report, want)
	}
	if info, statErr := os.Lstat(alpha); statErr != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("removed link must not be reported as restored: %v", statErr)
	}
}

func TestDetachSkillsTarget_MissingAndPlainFile(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(fmt.Sprintf("dryRun=%v", dryRun), func(t *testing.T) {
			root := t.TempDir()
			opts := DetachOptions{Source: filepath.Join(root, "source"), DryRun: dryRun}
			missing := filepath.Join(root, "missing")
			if report, err := DetachSkillsTarget(missing, opts); err != nil || !reflect.DeepEqual(report, DetachReport{}) {
				t.Fatalf("missing target = %+v, %v", report, err)
			}
			file := filepath.Join(root, "file")
			writeDetachFile(t, file, "local")
			report, err := DetachSkillsTarget(file, opts)
			if err != nil || !reflect.DeepEqual(report, DetachReport{Retained: []string{file}}) {
				t.Fatalf("plain file = %+v, %v", report, err)
			}
			assertDetachContent(t, file, "local")
		})
	}
}

func TestDetachSkillsTarget_CleanupFailureReportsCompletedWork(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	manifest, local := filepath.Join(target, ManifestFile), filepath.Join(target, "0-local")
	writeDetachFile(t, manifest, "manifest")
	writeDetachFile(t, local, "local")
	for _, name := range []string{"alpha", "beta"} {
		writeDetachFile(t, filepath.Join(source, name, "SKILL.md"), name)
		if err := os.Symlink(filepath.Join(source, name), filepath.Join(target, name)); err != nil {
			t.Fatal(err)
		}
	}
	original := removeTargetPath
	removeTargetPath = func(path string) error {
		if path == filepath.Join(target, "beta") {
			return errors.New("remove denied")
		}
		return original(path)
	}
	t.Cleanup(func() { removeTargetPath = original })
	report, err := DetachSkillsTarget(target, DetachOptions{Source: source})
	if err == nil {
		t.Fatal("expected cleanup failure")
	}
	want := DetachReport{
		Removed:    []string{manifest, filepath.Join(target, "alpha")},
		CopiedBack: []string{filepath.Join(target, "alpha")}, Retained: []string{local},
	}
	if !reflect.DeepEqual(report, want) {
		t.Errorf("completed work = %+v, want %+v", report, want)
	}
	assertDetachContent(t, filepath.Join(target, "alpha", "SKILL.md"), "alpha")
	if info, statErr := os.Lstat(filepath.Join(target, "beta")); statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("failed link should remain: %v", statErr)
	}
}

func snapshotDetachTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		value := fmt.Sprintf("%v/%d", info.Mode(), info.ModTime().UnixNano())
		if info.Mode()&os.ModeSymlink != 0 {
			dest, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += "/" + dest
		} else if !info.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += "/" + string(data)
		}
		result[path] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// These cases pin target removal's copy-back semantics, which differ from
// turning skills off (DetachSkills).
func TestDetachSkillsTarget_Parity(t *testing.T) {
	for _, mode := range []string{"whole-folder", "merge", "copy"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			target := filepath.Join(root, "tool", "skills")
			agent := filepath.Join(root, "tool", "agents", "helper.md")
			writeDetachFile(t, filepath.Join(source, "alpha", "nested", "SKILL.md"), "source content")
			writeDetachFile(t, agent, "agent content")
			if mode == "whole-folder" {
				if err := os.Symlink(source, target); err != nil {
					t.Fatal(err)
				}
			} else {
				writeDetachFile(t, filepath.Join(target, ".skillshare-manifest.json"), `{"managed":{"alpha":"symlink"}}`)
				writeDetachFile(t, filepath.Join(target, "local", "SKILL.md"), "local content")
				writeDetachFile(t, filepath.Join(target, "notes.txt"), "local notes")
				if mode == "merge" {
					if err := os.Symlink(filepath.Join(source, "alpha"), filepath.Join(target, "alpha")); err != nil {
						t.Fatal(err)
					}
					// A relative link to a missing source is removed without a copy.
					if err := os.Symlink(filepath.Join("..", "..", "source", "missing"), filepath.Join(target, "missing")); err != nil {
						t.Fatal(err)
					}
					outside := filepath.Join(root, "source-other", "external")
					writeDetachFile(t, filepath.Join(outside, "SKILL.md"), "external content")
					if err := os.Symlink(outside, filepath.Join(target, "external")); err != nil {
						t.Fatal(err)
					}
				} else {
					writeDetachFile(t, filepath.Join(target, "alpha", "nested", "SKILL.md"), "edited copy")
				}
			}

			if _, err := DetachSkillsTarget(target, DetachOptions{Source: source}); err != nil {
				t.Fatal(err)
			}
			assertDetachContent(t, agent, "agent content")
			assertDetachContent(t, filepath.Join(source, "alpha", "nested", "SKILL.md"), "source content")
			if mode == "whole-folder" {
				assertDetachAbsent(t, target)
				return
			}
			assertDetachAbsent(t, filepath.Join(target, ".skillshare-manifest.json"))
			assertDetachContent(t, filepath.Join(target, "local", "SKILL.md"), "local content")
			assertDetachContent(t, filepath.Join(target, "notes.txt"), "local notes")
			want := "edited copy"
			if mode == "merge" {
				want = "source content"
				assertDetachAbsent(t, filepath.Join(target, "missing"))
				if info, err := os.Lstat(filepath.Join(target, "external")); err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("external link changed: %v", err)
				}
			}
			assertDetachContent(t, filepath.Join(target, "alpha", "nested", "SKILL.md"), want)
			if info, err := os.Lstat(filepath.Join(target, "alpha")); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				t.Fatalf("alpha should be a real directory: %v", err)
			}
		})
	}
}

func writeDetachFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertDetachContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("%s: got %q, %v; want %q", path, got, err, want)
	}
}

func assertDetachAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s should be absent, got %v", path, err)
	}
}
