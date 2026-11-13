package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProfileEntry_DiscardSeparatesBinarySecondEdit(t *testing.T) {
	_, entry, manifest := fingerprintFixture(t)
	clean, err := CopyOutputFingerprint(entry)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Outputs = map[string]string{"repo-demo": clean}
	file := filepath.Join(entry, "a")
	if err := os.WriteFile(file, []byte("x\x00b\x00y"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := CopyOutputFingerprint(entry)
	if err != nil {
		t.Fatal(err)
	}
	grant := []DiscardEntry{{Target: "custom", Dir: filepath.Dir(entry), Entry: "repo-demo", Fingerprint: first}}
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry, "b"), []byte("y"), 0600); err != nil {
		t.Fatal(err)
	}
	got, allowed := entryMutationAllowed("custom", EntryInput{Name: "repo-demo", Path: entry, Manifest: manifest}, grant)
	if allowed || got.Class != EntryModifiedCopy || got.Fingerprint == first {
		t.Fatalf("first edit's grant authorized different second edit: %+v allowed=%v", got, allowed)
	}
}

func TestProfileEntry_Classes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(target, "demo")
	m := &Manifest{Managed: map[string]string{}, Outputs: map[string]string{}}
	classify := func() EntryClassification {
		t.Helper()
		return ClassifyEntry(EntryInput{Name: "demo", Path: entry, SourceRoot: source, Manifest: m})
	}
	if got := classify(); got.Class != EntryAbsent {
		t.Fatalf("absent: %+v", got)
	}
	if err := os.Mkdir(entry, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry, "SKILL.md"), []byte("clean"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := classify(); got.Class != EntryUnmanaged {
		t.Fatalf("unrecorded folder: %+v", got)
	}
	m.Managed["demo"] = manifestSymlink
	if got := classify(); got.Class != EntryUnmanaged {
		t.Fatalf("folder recorded as link: %+v", got)
	}
	sum, err := CopyOutputFingerprint(entry)
	if err != nil {
		t.Fatal(err)
	}
	m.Managed["demo"], m.Outputs["demo"] = "source", sum
	if got := classify(); got.Class != EntryCleanCopy {
		t.Fatalf("clean copy: %+v", got)
	}
	if err := os.WriteFile(filepath.Join(entry, "SKILL.md"), []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := classify(); got.Class != EntryModifiedCopy || got.Fingerprint == sum {
		t.Fatalf("edited copy: %+v", got)
	}
	if err := os.RemoveAll(entry); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(source, "missing"), entry); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	m.Managed["demo"] = manifestSymlink
	if got := classify(); got.Class != EntryManagedLink {
		t.Fatalf("broken internal link: %+v", got)
	}
	delete(m.Managed, "demo")
	if got := classify(); got.Class != EntryUnmanaged {
		t.Fatalf("unrecorded link: %+v", got)
	}
	if err := os.Remove(entry); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside"), entry); err != nil {
		t.Fatal(err)
	}
	m.Managed["demo"] = manifestSymlink
	if got := classify(); got.Class != EntryUnmanaged {
		t.Fatalf("external link: %+v", got)
	}
	if err := os.Remove(entry); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte("local file"), 0644); err != nil {
		t.Fatal(err)
	}
	m.Managed["demo"] = "recorded-copy"
	if got := classify(); got.Class != EntryUnmanaged {
		t.Fatalf("file recorded as copy: %+v", got)
	}
}

func TestProfileEntry_DiscardRequiresCurrentFingerprint(t *testing.T) {
	entry := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(entry, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(entry, "SKILL.md")
	if err := os.WriteFile(file, []byte("clean"), 0644); err != nil {
		t.Fatal(err)
	}
	clean, err := CopyOutputFingerprint(entry)
	if err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Managed: map[string]string{"demo": "source"}, Outputs: map[string]string{"demo": clean}}
	if err := os.WriteFile(file, []byte("first edit"), 0644); err != nil {
		t.Fatal(err)
	}
	input := EntryInput{Name: "demo", Path: entry, Manifest: m}
	first := ClassifyEntry(input)
	discard := []DiscardEntry{{Target: "claude", Dir: filepath.Dir(entry), Entry: "demo", Fingerprint: first.Fingerprint}}
	if _, allowed := entryMutationAllowed("claude", input, discard); !allowed {
		t.Fatal("snapshotted edit was not permitted")
	}
	for _, field := range []string{"target", "dir", "empty_dir", "relative_dir", "entry", "fingerprint", "other_location"} {
		t.Run(field, func(t *testing.T) {
			grant, candidate := discard[0], input
			switch field {
			case "target":
				grant.Target = "other"
			case "dir":
				grant.Dir = t.TempDir()
			case "empty_dir":
				grant.Dir = ""
			case "relative_dir":
				grant.Dir = "."
			case "entry":
				grant.Entry = "other"
			case "fingerprint":
				grant.Fingerprint = "stale"
			case "other_location":
				candidate.Path = filepath.Join(t.TempDir(), "demo")
				if err := os.MkdirAll(candidate.Path, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(candidate.Path, "SKILL.md"), []byte("first edit"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if _, allowed := entryMutationAllowed("claude", candidate, []DiscardEntry{grant}); allowed {
				t.Fatalf("%s widened snapshot authorization", field)
			}
		})
	}
	encoded, err := json.Marshal(EntryChange{Target: "claude", Dir: "private-directory", Entry: "demo"})
	if err != nil || strings.Contains(string(encoded), "private-directory") {
		t.Fatalf("internal directory leaked into JSON: %s, %v", encoded, err)
	}
	if err := os.WriteFile(file, []byte("later edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, allowed := entryMutationAllowed("claude", input, discard); allowed {
		t.Fatal("later edit was permitted by stale snapshot")
	}
	delete(m.Managed, "demo")
	if _, allowed := entryMutationAllowed("claude", input, discard); allowed {
		t.Fatal("discard authorized an unmanaged folder")
	}
}

func TestProfileEntry_LegacyRewriteWithoutSourceFailsClosed(t *testing.T) {
	source, entry, manifest := fingerprintFixture(t)
	data, err := os.ReadFile(filepath.Join(source, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry, "SKILL.md"), data, 0644); err != nil {
		t.Fatal(err)
	}
	got := ClassifyEntry(EntryInput{Name: "repo-demo", Path: entry, Manifest: manifest})
	if got.Class != EntryModifiedCopy {
		t.Fatalf("legacy rewritten entry without source classified as %+v", got)
	}
}

func TestProfileEntry_CorruptManifestDoesNotClaimFolder(t *testing.T) {
	target := t.TempDir()
	entry := filepath.Join(target, "demo")
	if err := os.MkdirAll(entry, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, ManifestFile), []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest, err := ReadManifest(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := ClassifyEntry(EntryInput{Name: "demo", Path: entry, Manifest: manifest}); got.Class != EntryUnmanaged {
		t.Fatalf("corrupt manifest claimed folder: %+v", got)
	}
}

func TestProfileEntry_UnreadableCopyFailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("file permissions cannot exercise read failure")
	}
	entry := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(entry, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(entry, "SKILL.md")
	if err := os.WriteFile(file, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	sum, err := CopyOutputFingerprint(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(file, 0644)
	manifest := &Manifest{Managed: map[string]string{"demo": "source"}, Outputs: map[string]string{"demo": sum}}
	input := EntryInput{Name: "demo", Path: entry, Manifest: manifest}
	got, allowed := entryMutationAllowed("claude", input, []DiscardEntry{{Target: "claude", Dir: filepath.Dir(entry), Entry: "demo", Fingerprint: sum}})
	if got.Class != EntryModifiedCopy || allowed {
		t.Fatalf("unreadable copy was permitted: %+v, %v", got, allowed)
	}
}
