package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTrackedDeclarations(t *testing.T) {
	store := NewMetadataStore()
	for key, entry := range map[string]*MetadataEntry{
		"_team":          {Source: "https://example.com/team.git", Tracked: true},
		"org/_web":       {Source: "https://example.com/web.git", Tracked: true, Group: "org"},
		"_api":           {Source: "https://example.com/api.git", Tracked: true, Group: "backend"},
		"_team/_member":  {Source: "https://example.com/member.git", Tracked: true, Group: "_team"},
		"_child":         {Source: "https://example.com/child.git", Tracked: true, Group: "backend/_api"},
		"_plain":         {Source: "https://example.com/plain.git"},
		"_nosrc":         {Tracked: true},
		"legacy":         {Source: "https://example.com/legacy.git", Tracked: true},
		"_other/_member": {Source: "https://example.com/member.git", Tracked: true},
	} {
		store.Set(key, entry)
	}
	want := []string{"_api", "_other/_member", "_team", "org/_web"}
	if got := store.TrackedDeclarations(); !reflect.DeepEqual(got, want) {
		t.Fatalf("declarations = %v, want %v", got, want)
	}
}

func TestForgetTrackedRepo(t *testing.T) {
	for _, tc := range []struct{ rel, key, group string }{
		{"_team", "_team", ""}, {"org/_team", "org/_team", "org"}, {"org/_team", "_team", "org"},
	} {
		t.Run(tc.key+" in "+tc.group, func(t *testing.T) {
			rel := tc.rel
			store := NewMetadataStore()
			store.Set(tc.key, &MetadataEntry{Source: "https://example.invalid/team.git", Tracked: true, Group: tc.group})
			store.Set(rel+"/child", &MetadataEntry{Source: "child", Group: rel})
			store.Set("legacy", &MetadataEntry{Source: "child", Tracked: true, Group: strings.TrimPrefix(rel, "_")})
			store.Set("_team-other", &MetadataEntry{Source: "other", Tracked: true})
			store.AuditAccepted = map[string][]string{rel: {"root"}, rel + "/child/deep": {"child"}, "_team-other/child": {"keep"}}
			store.TargetOverrides = map[string][]string{rel + "/child": {"claude"}, "_team-other/child": {"codex"}}
			if !store.ForgetTrackedRepo("./" + rel + "/") {
				t.Fatal("declaration not forgotten")
			}
			if store.Has(tc.key) || store.Has(rel+"/child") || len(store.AuditAccepted) != 1 || len(store.TargetOverrides) != 1 || !store.Has("_team-other") {
				t.Fatalf("wrong cleanup: %+v", store)
			}
			if store.Has("legacy") {
				t.Fatal("legacy member survived")
			}
			before, _ := json.Marshal(store)
			if store.ForgetTrackedRepo(rel) || store.ForgetTrackedRepo("missing") {
				t.Fatal("reported removing an absent declaration")
			}
			after, _ := json.Marshal(store)
			if string(before) != string(after) {
				t.Fatal("no-op mutated metadata")
			}
		})
	}
}

func TestUnlinkedSnapshotMatchesRemoval(t *testing.T) {
	store := NewMetadataStore()
	for key, entry := range map[string]*MetadataEntry{
		"_team": {Source: "normalized"}, "_team/_nested": {Source: "nested", Tracked: true},
		"child":       {Source: "child", Group: "_team/sub"},
		"legacy":      {Source: "legacy", Tracked: true, Group: "team"},
		"team/member": {Source: "legacy", Tracked: true, Group: "team"},
		"team/_repo":  {Source: "foreign", Tracked: true, Group: "team"},
		"_other":      {Source: "foreign legacy", Tracked: true, Group: "team"},
		"team/plain":  {Source: "plain", Group: "team"}, "_team-other": {Source: "sibling"},
	} {
		store.Set(key, entry)
	}
	store.AuditAccepted = map[string][]string{"_team/orphan": {"orphan"}, "legacy": {"old"}, "team/member": {"old path"}, "team/_repo": {"keep"}, "team/_repo/child": {"keep child"}, "_team-other": {"sibling"}}
	store.SetTargetOverride("_team/orphan", nil)
	store.SetTargetOverride("team/_repo/child", []string{"codex"})
	saved := store.ExtractUnlinkedName("./_team/")
	if saved.Has("team/_repo") || saved.Has("_other") || len(saved.Entries) != 5 || len(saved.AuditAccepted) != 3 || len(saved.TargetOverrides) != 1 {
		t.Fatalf("snapshot over/under-captured: %+v", saved)
	}
	if !store.ForgetUnlinkedName("_team") || len(store.Entries) != 4 || len(store.AuditAccepted) != 3 || len(store.TargetOverrides) != 1 {
		t.Fatalf("wrong removal: %+v", store)
	}
	store.Merge(saved)
	if store.GetExact("_team/sub/child") == nil || !store.Has("legacy") || !store.Has("team/member") || !store.Has("team/_repo") || len(store.AuditAccepted) != 6 {
		t.Fatalf("restore lost records/acceptances: %+v", store)
	}
	if targets, ok := store.TargetOverrides["_team/orphan"]; !ok || targets == nil || len(targets) != 0 {
		t.Fatalf("explicit all-target override lost: %+v", store.TargetOverrides)
	}
	// Generic uninstall and declaration-only removal keep their legacy contract.
	if !store.ForgetTrackedRepo("team/_repo") || store.Has("team/_repo") {
		t.Fatal("declaration removal changed")
	}
	store.Set("_team", &MetadataEntry{Source: "repo", Tracked: true})
	if !store.ForgetTrackedRepo("_team") || store.Has("_other") {
		t.Fatal("legacy declaration-only matching changed")
	}
}

func TestUnlinkedSnapshotWithoutEntries(t *testing.T) {
	for _, kind := range []string{"empty", "acceptance", "override"} {
		t.Run(kind, func(t *testing.T) {
			store := NewMetadataStore()
			if kind == "acceptance" {
				store.AuditAccepted = map[string][]string{"_team/child": {"accepted"}}
			} else if kind == "override" {
				store.SetTargetOverride("_team/child", nil)
			}
			saved := store.ExtractUnlinkedName("_team")
			if saved.IsEmpty() != (kind == "empty") || store.ForgetUnlinkedName("_team") != (kind != "empty") || !store.IsEmpty() {
				t.Fatalf("snapshot/removal disagree: %+v %+v", saved, store)
			}
		})
	}
}

func TestFindTrackedDeclaration(t *testing.T) {
	src := t.TempDir()
	store := NewMetadataStore()
	for _, rel := range []string{"_absent", "org/_team", "_folder", "_link", "_checkout", "_checkout-link"} {
		store.Set(rel, &MetadataEntry{Source: "https://example.invalid/team.git", Tracked: true})
	}
	if err := os.MkdirAll(filepath.Join(src, "_folder"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "_checkout", ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"_link": t.TempDir(), "_checkout-link": filepath.Join(src, "_checkout")} {
		if err := os.Symlink(target, filepath.Join(src, name)); err != nil {
			t.Fatal(err)
		}
	}
	if got := DeclarationsWithoutCheckout(src, store); !reflect.DeepEqual(got, []string{"_absent", "_folder", "_link", "org/_team"}) {
		t.Fatalf("without checkout = %v", got)
	}
	for _, tc := range []struct {
		input, want string
		exact       bool
	}{
		{"./_absent/", "_absent", true}, {"team", "org/_team", false}, {"org/team", "org/_team", false},
		{"_folder", "_folder", true}, {"_link", "_link", true}, {"_checkout", "", false}, {"_checkout-link", "", false}, {"missing", "", false},
	} {
		name, exact, err := FindTrackedDeclaration(src, store, tc.input)
		if err != nil || name != tc.want || exact != tc.exact {
			t.Errorf("%q = %q,%v,%v", tc.input, name, exact, err)
		}
	}
	store.Set("other/_team", &MetadataEntry{Source: "https://example.invalid/other.git", Tracked: true})
	if _, _, err := FindTrackedDeclaration(src, store, "team"); err == nil || !strings.Contains(err.Error(), "multiple tracked repositories match") {
		t.Fatalf("ambiguity = %v", err)
	}
	if name, exact, err := FindTrackedDeclaration(src, store, "org/_team"); err != nil || !exact || name != "org/_team" {
		t.Fatalf("exact path = %q,%v,%v", name, exact, err)
	}
}
