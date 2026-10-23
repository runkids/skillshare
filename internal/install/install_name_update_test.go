package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExistingSkillInstallName(t *testing.T) {
	for _, tt := range []struct {
		name      string
		keys      []string
		group     string
		into      string
		source    string
		want      string
		wantError bool
	}{
		{name: "top-level", keys: []string{"skills"}, source: "owner/repo/skills", want: "skills"},
		{name: "grouped", keys: []string{"extractors/skills"}, into: "extractors", source: "owner/repo/skills", want: "skills"},
		{name: "legacy-group-key", keys: []string{"skills"}, group: "extractors", into: "extractors", source: "owner/repo/skills", want: "skills"},
		{name: "different-source", keys: []string{"skills"}, source: "other/repo/skills", want: "pdf"},
		{name: "different-group", keys: []string{"extractors/skills"}, source: "owner/repo/skills", want: "pdf"},
		{name: "ambiguous", keys: []string{"first", "second"}, source: "owner/repo/skills", wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			store := NewMetadataStore()
			for _, key := range tt.keys {
				entry := &MetadataEntry{Source: tt.source, Group: tt.group}
				store.Set(key, entry)
				dir := filepath.Join(root, KeyToRelPath(key, entry))
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.Save(root); err != nil {
				t.Fatal(err)
			}
			got, err := ExistingSkillInstallName(root, &Source{Raw: "owner/repo"}, SkillInfo{Name: "pdf", Path: "skills"}, InstallOptions{Update: true, Into: tt.into})
			if (err != nil) != tt.wantError || (!tt.wantError && got != tt.want) {
				t.Fatalf("got name %q, error %v; want name %q, error %v", got, err, tt.want, tt.wantError)
			}
		})
	}
}
