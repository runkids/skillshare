package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStageFetchedTree_ContainedLinkAndModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks and Unix executable modes")
	}
	root := t.TempDir()
	selected := filepath.Join(root, "rules")
	if err := os.Mkdir(selected, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"run": "data", "text": "#!/bin/sh\n"} {
		if err := os.WriteFile(filepath.Join(selected, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(selected, "run"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("run", filepath.Join(selected, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(selected, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "stage")
	if err := stageFetchedTree(root, "rules", dst); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, content string
		exec          bool
	}{{"run", "data", true}, {"alias", "data", true}, {"text", "#!/bin/sh\n", false}} {
		path := filepath.Join(dst, tc.name)
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() || (info.Mode()&0111 != 0) != tc.exec || string(data) != tc.content {
			t.Errorf("%s: %s %q", tc.name, info.Mode(), data)
		}
	}
	if _, err := os.Lstat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git copied: %v", err)
	}
}

func TestStageFetchedTree_Refusals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires symlinks")
	}
	for _, kind := range []string{"absolute", "escape", "dangling", "directory", "selected", "ancestor", "chain-escape"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			sentinel := filepath.Join(t.TempDir(), "sentinel")
			if err := os.WriteFile(sentinel, []byte("private sentinel"), 0644); err != nil {
				t.Fatal(err)
			}
			selected := filepath.Join(root, "a", "rules")
			if err := os.MkdirAll(selected, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(selected, "safe"), []byte("safe"), 0644); err != nil {
				t.Fatal(err)
			}
			var target, link string
			switch kind {
			case "absolute":
				target, link = sentinel, filepath.Join(selected, "bad")
			case "escape":
				target, link = "../../outside", filepath.Join(selected, "bad")
			case "dangling":
				target, link = "missing", filepath.Join(selected, "bad")
			case "directory":
				target, link = ".", filepath.Join(selected, "bad")
			case "selected":
				if err := os.RemoveAll(selected); err != nil {
					t.Fatal(err)
				}
				target, link = ".", selected
			case "ancestor":
				if err := os.RemoveAll(filepath.Join(root, "a")); err != nil {
					t.Fatal(err)
				}
				target, link = ".", filepath.Join(root, "a")
			case "chain-escape":
				if err := os.Symlink(sentinel, filepath.Join(selected, "outside")); err != nil {
					t.Fatal(err)
				}
				target, link = "outside", filepath.Join(selected, "bad")
			}
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			dst := filepath.Join(t.TempDir(), "stage")
			err := stageFetchedTree(root, filepath.Join("a", "rules"), dst)
			if err == nil {
				t.Fatal("unsafe tree accepted")
			}
			if _, err := os.Stat(dst); !os.IsNotExist(err) {
				t.Fatalf("wrote before validating entire tree: %v", err)
			}
			if data, err := os.ReadFile(sentinel); err != nil || !strings.Contains(string(data), "sentinel") {
				t.Fatal("sentinel changed")
			}
		})
	}
}

func TestStageFetchedTree_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if err := stageFetchedTree(root, "../outside", filepath.Join(t.TempDir(), "stage")); err == nil {
		t.Fatal("accepted traversal")
	}
}

func TestStageFetchedTree_DoesNotOverwriteDestination(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("incoming"), 0644); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	target := filepath.Join(dst, "file")
	if err := os.WriteFile(target, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := stageFetchedTree(root, "", dst); err == nil {
		t.Fatal("accepted existing destination")
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "existing" {
		t.Fatalf("destination changed: %q, %v", data, err)
	}
}

func TestStageFetchedTree_RejectsLinkIntoGitMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires symlink")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("metadata"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(".git", "config"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := stageFetchedTree(root, "", filepath.Join(t.TempDir(), "stage")); err == nil {
		t.Fatal("accepted link into excluded metadata")
	}
}

func TestStageFetchedTree_RejectsSelectingGitMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := stageFetchedTree(root, ".git", filepath.Join(t.TempDir(), "stage")); err == nil {
		t.Fatal("selected git metadata")
	}
}
