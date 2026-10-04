package install

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"skillshare/internal/sourcefs"
)

func TestSwapStagedIntoSource(t *testing.T) {
	source := t.TempDir()
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "SKILL.md"), []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(source, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	stage := func() string {
		dir := filepath.Join(t.TempDir(), "skill")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("staged"), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	for _, dest := range []string{
		filepath.Join(source, "linked"),          // the skill is a link
		filepath.Join(source, "linked", "child"), // the skill is below one
	} {
		if err := swapStagedIntoSource(source, stage(), dest); !errors.Is(err, sourcefs.ErrLink) {
			t.Errorf("swap into %s: got %v, want ErrLink", dest, err)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(external, "SKILL.md")); string(data) != "external" {
		t.Fatalf("external skill changed: %q", data)
	}
	if entries, _ := os.ReadDir(external); len(entries) != 1 {
		t.Fatalf("external tree changed: %v", entries)
	}

	if err := swapStagedIntoSource(source, stage(), filepath.Join(source, "plain")); err != nil {
		t.Fatalf("swap into a real directory: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(source, "plain", "SKILL.md")); string(data) != "staged" {
		t.Fatalf("plain skill = %q, want staged content", data)
	}
}

// linkedSource builds a skills source with linked, a link to an external
// tree that holds one skill, and returns the source and the external tree.
func linkedSource(t *testing.T) (string, string) {
	t.Helper()
	source, external := t.TempDir(), t.TempDir()
	createLocalSkillSource(t, external, "outside")
	if err := os.Symlink(external, filepath.Join(source, "linked")); err != nil {
		t.Fatal(err)
	}
	return source, external
}

// treeContents records every path and file content below dir.
func treeContents(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if d.IsDir() {
			out[rel] = "<dir>"
			return nil
		}
		data, err := os.ReadFile(p)
		out[rel] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestInstallLocal_RefusesDestinationThroughLink(t *testing.T) {
	source, external := linkedSource(t)
	before := treeContents(t, external)
	skill := createLocalSkillSource(t, t.TempDir(), "new-skill")
	src := &Source{Type: SourceTypeLocalPath, Raw: skill, Path: skill, Name: "new-skill"}

	_, err := Install(src, filepath.Join(source, "linked", "new-skill"), InstallOptions{SourceDir: source, SkipAudit: true})
	if !errors.Is(err, sourcefs.ErrLink) {
		t.Fatalf("install below a link: got %v, want ErrLink", err)
	}
	if after := treeContents(t, external); !reflect.DeepEqual(after, before) {
		t.Fatalf("external tree changed: %v -> %v", before, after)
	}
}

func TestMetadataSave_RefusesLinkedMetadataFile(t *testing.T) {
	source, external := t.TempDir(), t.TempDir()
	shared := filepath.Join(external, "shared.json")
	if err := os.WriteFile(shared, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, filepath.Join(source, MetadataFileName)); err != nil {
		t.Fatal(err)
	}

	store := NewMetadataStore()
	store.Set("demo", &MetadataEntry{Source: "github.com/o/r"})
	if err := store.Save(source); !errors.Is(err, sourcefs.ErrLink) {
		t.Fatalf("save onto a linked .metadata.json: got %v, want ErrLink", err)
	}
	if data, _ := os.ReadFile(shared); string(data) != "{}" {
		t.Fatalf("link target changed: %q", data)
	}
}
