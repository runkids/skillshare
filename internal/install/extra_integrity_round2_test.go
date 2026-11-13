package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtraLocallyModified_KnownEmptyVersusMissingHashes(t *testing.T) {
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
}
