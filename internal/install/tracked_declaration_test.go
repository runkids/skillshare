package install

import (
	"reflect"
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
