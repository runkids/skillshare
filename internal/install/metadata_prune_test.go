package install

import (
	"sort"
	"strings"
	"testing"
)

func TestRemoveByNames(t *testing.T) {
	tests := []struct {
		name    string
		entries map[string]*MetadataEntry
		remove  []string
		kept    []string
	}{
		{
			name:    "exact key",
			entries: map[string]*MetadataEntry{"foo": {}, "bar": {}},
			remove:  []string{"foo"},
			kept:    []string{"bar"},
		},
		{
			name:    "legacy basename key matched by its full path",
			entries: map[string]*MetadataEntry{"foo": {Group: "frontend"}},
			remove:  []string{"frontend/foo"},
		},
		{
			name: "group directory removes its members",
			entries: map[string]*MetadataEntry{
				"grp/a":     {Group: "grp"},
				"b":         {Group: "grp"},
				"grp/sub/c": {Group: "grp/sub"},
				"grp-two/d": {Group: "grp-two"},
			},
			remove: []string{"grp"},
			kept:   []string{"grp-two/d"},
		},
		{
			name: "tracked repo removes its entry and members",
			entries: map[string]*MetadataEntry{
				"_team":   {Tracked: true},
				"_team/a": {Group: "_team", Tracked: true},
				"b":       {Group: "_team", Tracked: true},
				"_team/c": {Tracked: true}, // full-path key without a group
			},
			remove: []string{"_team"},
		},
		{
			name:    "legacy tracked group without the underscore prefix",
			entries: map[string]*MetadataEntry{"team/a": {Group: "team", Tracked: true}},
			remove:  []string{"_team"},
		},
		{
			name: "repo installed into the legacy group stays",
			entries: map[string]*MetadataEntry{
				"_team":   {Tracked: true},
				"team/a":  {Group: "team", Tracked: true},
				"team/_b": {Group: "team", Tracked: true},
			},
			remove: []string{"_team"},
			kept:   []string{"team/_b"},
		},
		{
			name:    "untracked group that shares the repo's name stays",
			entries: map[string]*MetadataEntry{"team/a": {Group: "team"}},
			remove:  []string{"_team"},
			kept:    []string{"team/a"},
		},
		{
			name: "nested repo leaves a sibling with the same basename",
			entries: map[string]*MetadataEntry{
				"org/_team/a":  {Group: "org/_team", Tracked: true},
				"dept/_team/b": {Group: "dept/_team", Tracked: true},
				"team/c":       {Group: "team", Tracked: true},
			},
			remove: []string{"org/_team"},
			kept:   []string{"dept/_team/b", "team/c"},
		},
		{
			name:    "grouped skill with the same basename stays",
			entries: map[string]*MetadataEntry{"foo": {}, "frontend/foo": {Group: "frontend"}},
			remove:  []string{"foo"},
			kept:    []string{"frontend/foo"},
		},
		{
			name:    "legacy basename key of a grouped skill is not the top-level name",
			entries: map[string]*MetadataEntry{"foo": {Group: "frontend"}},
			remove:  []string{"foo"},
			kept:    []string{"foo"},
		},
		{
			name:    "top-level skill with the same basename stays",
			entries: map[string]*MetadataEntry{"foo": {}, "frontend/foo": {Group: "frontend"}},
			remove:  []string{"frontend/foo"},
			kept:    []string{"foo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMetadataStore()
			for key, entry := range tt.entries {
				store.Set(key, entry)
				store.SetTargetOverride(KeyToRelPath(key, entry), []string{"claude"})
			}
			names := map[string]bool{}
			for _, n := range tt.remove {
				names[n] = true
			}

			store.RemoveByNames(names)

			got := store.List()
			want := append([]string{}, tt.kept...)
			sort.Strings(want)
			// Target overrides follow the skill they belong to.
			for _, key := range want {
				if _, ok := store.TargetOverrides[KeyToRelPath(key, tt.entries[key])]; !ok {
					t.Errorf("override of kept entry %q was removed", key)
				}
			}
			for _, name := range tt.remove {
				for key := range store.TargetOverrides {
					if key == name || strings.HasPrefix(key, name+"/") {
						t.Errorf("override %q of removed %q survived", key, name)
					}
				}
			}
			if len(got) != len(want) {
				t.Fatalf("kept %v, want %v", got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("kept %v, want %v", got, want)
				}
			}
		})
	}
}
