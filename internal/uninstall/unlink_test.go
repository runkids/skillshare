package uninstall

import (
	"os"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/sourcelink"
)

func TestUnlink_MetadataModeAndReload(t *testing.T) {
	for _, mode := range []string{"global", "project", "invalid metadata"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			if _, err := sourcelink.Create(f.source, nil, t.TempDir(), "_team"); err != nil {
				t.Fatal(err)
			}
			f.opts.Store.Set("_team", &install.MetadataEntry{Source: "normalized"})
			if err := f.opts.Store.Save(f.source); err != nil {
				t.Fatal(err)
			}
			if mode == "invalid metadata" {
				if err := os.WriteFile(install.MetadataPath(f.source), []byte("invalid"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			result, err := Unlink(f.source, f.opts.TrashDir, "_team/", mode != "project")
			if err != nil || (result.MetadataErr != nil) != (mode == "invalid metadata") || result.ReloadErr != nil {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			switch mode {
			case "global":
				if result.Store == nil || result.Store.Has("_team") {
					t.Fatalf("fresh store not returned: %+v", result)
				}
			case "project":
				saved, err := install.LoadMetadata(f.source)
				if err != nil || result.Store != nil || !saved.Has("_team") {
					t.Fatalf("project metadata changed: %+v %v", result, err)
				}
			case "invalid metadata":
				if result.Store != nil {
					t.Fatal("failed load returned a store")
				}
			}
		})
	}
}
