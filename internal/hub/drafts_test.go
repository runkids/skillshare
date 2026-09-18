package hub

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDraftLifecycle(t *testing.T) {
	store := DraftStore{Dir: filepath.Join(t.TempDir(), "hub-drafts")}
	d, err := store.Create(Draft{Name: "Team", Entries: []DraftEntry{{ID: "one", Data: map[string]json.RawMessage{"name": json.RawMessage(`"Review"`), "source": json.RawMessage(`"acme/skills"`)}}}})
	if err != nil {
		t.Fatal(err)
	}
	old := d
	d.Name = "Changed"
	d, err = store.Save(d)
	if err != nil || d.Revision == old.Revision {
		t.Fatalf("save: %v %#v", err, d)
	}
	if _, err = store.Save(old); !errors.Is(err, ErrDraftConflict) {
		t.Fatalf("stale write: %v", err)
	}
	if err = store.Delete(d.ID, old.Revision); !errors.Is(err, ErrDraftConflict) {
		t.Fatalf("stale delete: %v", err)
	}
	loaded, err := store.Get(d.ID)
	if err != nil || loaded.Name != "Changed" {
		t.Fatalf("reload: %v", err)
	}
	if _, err = store.Get("../outside"); err == nil {
		t.Fatal("accepted traversal")
	}
	if err = store.Delete(d.ID, d.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Get(d.ID); !os.IsNotExist(err) {
		t.Fatalf("delete: %v", err)
	}
}

func TestDraftImportExport(t *testing.T) {
	raw := []byte(`{"schemaVersion":1,"custom":{"keep":true},"skills":[{"name":"same","source":"acme/repo","skill":"nested/review","future":42},{"name":"same","source":"https://github.com/acme/other"}]}`)
	d, err := ImportDraft(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Entries) != 2 || d.Entries[0].ID == d.Entries[1].ID {
		t.Fatal("duplicate names lost")
	}
	data, err := ExportDraft(d)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]json.RawMessage
	if err = json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	var custom map[string]bool
	json.Unmarshal(out["custom"], &custom)
	if !custom["keep"] {
		t.Fatalf("unknown field lost: %s", data)
	}
	var skills []map[string]json.RawMessage
	json.Unmarshal(out["skills"], &skills)
	if string(skills[0]["skill"]) != `"nested/review"` || string(skills[0]["future"]) != "42" {
		t.Fatalf("entry fields lost: %s", data)
	}
}

func TestDraftLocalAndInvalidImport(t *testing.T) {
	d, err := ImportDraft([]byte(`{"schemaVersion":1,"sourcePath":"/author/skills","skills":[{"name":"local","source":"team/review"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ExportDraft(d); err == nil {
		t.Fatal("local source exported")
	}
	d.Entries[0].Data["source"] = json.RawMessage(`"acme/review"`)
	data, err := ExportDraft(d)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	json.Unmarshal(data, &out)
	if _, ok := out["sourcePath"]; ok {
		t.Fatal("author path leaked")
	}
	for _, input := range []string{`{"schemaVersion":2,"skills":[]}`, `{"schemaVersion":1,"skills":{}}`, `{"schemaVersion":1,"skills":[{"name":3}]}`, `{"schemaVersion":1,"skills":[{"name":"x","tags":"bad"}]}`} {
		if _, err := ImportDraft([]byte(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}

func TestDraftCredentialAndAudit(t *testing.T) {
	for _, source := range []string{"https://token@github.com/acme/repo", "https://github.com/acme/repo?token=secret", "ssh://git:secret@host/repo"} {
		if SourceProblem(source) != "credentials" {
			t.Errorf("credential source accepted: %s", source)
		}
	}
	store := DraftStore{Dir: filepath.Join(t.TempDir(), "hub-drafts")}
	d, _ := ImportDraft([]byte(`{"schemaVersion":1,"skills":[{"name":"x","source":"acme/one","riskScore":0,"riskLabel":"clean","auditedAt":"yesterday"}]}`))
	d, err := store.Create(d)
	if err != nil {
		t.Fatal(err)
	}
	d.Entries[0].Data["source"] = json.RawMessage(`"acme/two"`)
	d, err = store.Save(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Entries[0].Data["riskScore"]; ok {
		t.Fatal("stale audit retained")
	}
}

func TestDraftConcurrentSaveAndScope(t *testing.T) {
	store := DraftStore{Dir: filepath.Join(t.TempDir(), "hub-drafts")}
	d, err := store.Create(Draft{Name: "one"})
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for range 2 {
		go func() { _, err := store.Save(d); results <- err }()
	}
	successes, conflicts := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if errors.Is(err, ErrDraftConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	other := DraftStore{Dir: filepath.Join(t.TempDir(), "hub-drafts")}
	if _, err := other.Get(d.ID); !os.IsNotExist(err) {
		t.Fatalf("draft leaked between scopes: %v", err)
	}
}

func TestDraftRejectSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	os.Mkdir(target, 0700)
	os.Symlink(target, filepath.Join(root, "hub-drafts"))
	store := DraftStore{Dir: filepath.Join(root, "hub-drafts")}
	if _, err := store.Create(Draft{Name: "unsafe"}); err == nil {
		t.Fatal("followed symlink directory")
	}
}

func TestDraftCandidatesLocalPathsAndMetadata(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"team/local", "remote"} {
		dir := filepath.Join(root, path)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: review\ndescription: >-\n  Review changes\n  carefully.\ntags: [review, team]\n---\n# Review"), 0644)
	}
	os.WriteFile(filepath.Join(root, ".metadata.json"), []byte(`{"version":1,"entries":{"remote":{"source":"acme/repo","subdir":"skills/review"}}}`), 0644)
	entries, err := DraftCandidates(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries=%d", len(entries))
	}
	for _, entry := range entries {
		if field(entry.Data, "description") != "Review changes carefully." {
			t.Fatalf("missing folded description: %s", entry.Data["description"])
		}
		if entry.ID == "team/local" && SourceProblem(field(entry.Data, "source")) != "local_source" {
			t.Fatal("local path treated as GitHub repo")
		}
		if entry.ID == "remote" && field(entry.Data, "skill") != "skills/review" {
			t.Fatal("metadata selector lost")
		}
	}
}

func TestDraftCandidatesTrackedRepository(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "_team", "skills", "review")
	os.MkdirAll(dir, 0755)
	os.MkdirAll(filepath.Join(root, "_team", ".git"), 0755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: review\n---\nReview."), 0644)
	os.WriteFile(filepath.Join(root, ".metadata.json"), []byte(`{"version":1,"entries":{"_team":{"source":"acme/repo","tracked":true}}}`), 0644)
	entries, err := DraftCandidates(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("candidates: %v %#v", err, entries)
	}
	if field(entries[0].Data, "source") != "acme/repo" || field(entries[0].Data, "skill") != "skills/review" {
		t.Fatalf("tracked source lost: %#v", entries[0])
	}
}
