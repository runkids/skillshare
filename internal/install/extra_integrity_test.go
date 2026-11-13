package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/utils"
)

func TestExtraLocallyModified_FilesAndModes(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
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
	})
	t.Run("TestExtraLocallyModified_UnknownAndUnsafe", func(t *testing.T) {
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
	})
	t.Run("TestExtraLocallyModified_OSMetadataIsStillPartOfExtra", func(t *testing.T) {
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
	})
	t.Run("TestExtraLocallyModified_KnownEmptyVersusMissingHashes", func(t *testing.T) {
		for _, known := range []bool{false, true} {
			name := "unknown"
			if known {
				name = "known-empty"
			}
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				dir := filepath.Join(root, "rules")
				if err := os.Mkdir(dir, 0755); err != nil {
					t.Fatal(err)
				}
				entry := &MetadataEntry{Kind: "extra", Source: "example-org/ai-hub"}
				if known {
					entry.FileHashes = map[string]string{}
				}
				store := NewMetadataStore()
				store.Set("rules", entry)
				if err := store.Save(root); err != nil {
					t.Fatal(err)
				}
				loaded, _, err := ReadMetadata(root)
				if err != nil {
					t.Fatal(err)
				}
				if got := loaded.GetExact("rules"); got == nil || (got.FileHashes != nil) != known {
					t.Errorf("empty versus absent provenance lost on roundtrip: %+v", got)
				}
				modified, _, err := ExtraLocallyModified(root, "rules")
				if err != nil || modified == known {
					t.Errorf("empty directory: modified=%t known=%t err=%v", modified, known, err)
				}
				if err := os.WriteFile(filepath.Join(dir, "new-note.md"), []byte("user content"), 0644); err != nil {
					t.Fatal(err)
				}
				if modified, _, err := ExtraLocallyModified(root, "rules"); err != nil || !modified {
					t.Errorf("new user file not protected: modified=%t err=%v", modified, err)
				}
			})
		}
	})
}

func TestInspectRemoteExtra_InterruptedBeforeMissing(t *testing.T) {
	t.Run("local classification", func(t *testing.T) {
		for _, state := range []string{"intact", "modified", "missing", "interrupted", "error"} {
			t.Run(state, func(t *testing.T) {
				root := t.TempDir()
				source := filepath.Join(root, "rules")
				if err := os.Mkdir(source, 0755); err != nil {
					t.Fatal(err)
				}
				file := filepath.Join(source, "rule.md")
				if err := os.WriteFile(file, []byte("original"), 0644); err != nil {
					t.Fatal(err)
				}
				hashes, err := extraTreeHashes(source)
				if err != nil {
					t.Fatal(err)
				}
				store := NewMetadataStore()
				store.Set("rules", &MetadataEntry{Kind: "extra", Source: "example-org/ai-hub", Commit: strings.Repeat("a", 40), FileHashes: hashes})
				if err := store.Save(root); err != nil {
					t.Fatal(err)
				}
				switch state {
				case "modified":
					if err := os.WriteFile(file, []byte("edited"), 0644); err != nil {
						t.Fatal(err)
					}
				case "missing":
					if err := os.RemoveAll(source); err != nil {
						t.Fatal(err)
					}
				case "interrupted":
					if err := os.RemoveAll(source); err != nil {
						t.Fatal(err)
					}
					stage := filepath.Join(root, extraStagingDir)
					if err := os.Mkdir(stage, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(stage, "publishing-rules.json"), []byte("opaque corruption observation"), 0600); err != nil {
						t.Fatal(err)
					}
				case "error":
					if err := os.WriteFile(filepath.Join(root, MetadataFileName), []byte("{invalid"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				got := InspectRemoteExtra(root, "rules")
				if got.State() != state || got.Name != "rules" || (state != "error" && got.Commit != strings.Repeat("a", 40)) {
					t.Fatalf("classification: %+v state=%s", got, got.State())
				}
				if (got.Err != nil) != (state == "error") {
					t.Fatalf("classification error: %+v", got)
				}
			})
		}
	})
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
