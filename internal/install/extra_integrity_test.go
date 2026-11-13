package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"skillshare/internal/utils"
)

func TestExtraLocallyModified_FilesAndModes(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "rules")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rule.md")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	hash, err := utils.FileHashFormatted(path)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMetadataStore()
	store.Set("rules", &MetadataEntry{Kind: "extra", Commit: "revision", FileHashes: map[string]string{"rule.md": hash}})
	if err := store.Save(root); err != nil {
		t.Fatal(err)
	}
	modified, _, err := ExtraLocallyModified(root, "rules")
	if err != nil || modified {
		t.Fatalf("unchanged: %v %v", modified, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatal(err)
		}
		modified, _, err = ExtraLocallyModified(root, "rules")
		if err != nil || modified {
			t.Fatalf("mode only: %v %v", modified, err)
		}
	}
	if err := os.WriteFile(path, []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}
	modified, _, err = ExtraLocallyModified(root, "rules")
	if err != nil || !modified {
		t.Fatalf("edited: %v %v", modified, err)
	}
}

func TestExtraLocallyModified_UnknownAndUnsafe(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "rules")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	store := NewMetadataStore()
	store.Set("rules", &MetadataEntry{Kind: "extra"})
	if err := store.Save(root); err != nil {
		t.Fatal(err)
	}
	modified, _, err := ExtraLocallyModified(root, "rules")
	if err != nil || !modified {
		t.Fatalf("missing hashes: %v %v", modified, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	sentinel := filepath.Join(t.TempDir(), "sentinel")
	if err := os.WriteFile(sentinel, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sentinel, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	store.Set("rules", &MetadataEntry{Kind: "extra", FileHashes: map[string]string{"link": "sha256:fake"}})
	if err := store.Save(root); err != nil {
		t.Fatal(err)
	}
	modified, _, err = ExtraLocallyModified(root, "rules")
	if err != nil || !modified {
		t.Fatalf("link: %v %v", modified, err)
	}
}

func TestExtraLocallyModified_OSMetadataIsStillPartOfExtra(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "rules")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, ".DS_Store")
	if err := os.WriteFile(file, []byte("tracked content"), 0644); err != nil {
		t.Fatal(err)
	}
	hashes, err := extraTreeHashes(dir)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMetadataStore()
	store.Set("rules", &MetadataEntry{Kind: "extra", FileHashes: hashes})
	if err := store.Save(root); err != nil {
		t.Fatal(err)
	}
	modified, _, err := ExtraLocallyModified(root, "rules")
	if err != nil || modified {
		t.Fatalf("unchanged: %v %v", modified, err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	modified, _, err = ExtraLocallyModified(root, "rules")
	if err != nil || !modified {
		t.Fatalf("removed: %v %v", modified, err)
	}
}

func TestInspectRemoteExtra_InterruptedBeforeMissing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".skillshare-staging")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "publishing-rules.json")
	if err := os.WriteFile(marker, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	got := InspectRemoteExtra(root, "rules")
	if !got.Interrupted || got.Err != nil {
		t.Fatalf("inspection: %+v", got)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	got = InspectRemoteExtra(root, "rules")
	if got.Interrupted || !got.Missing || got.Err != nil {
		t.Fatalf("inspection: %+v", got)
	}
}
