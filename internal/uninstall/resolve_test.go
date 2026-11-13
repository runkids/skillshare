package uninstall

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
)

func TestResolveName(t *testing.T) {
	for _, tc := range []struct {
		name, input, disk, want, wantErr              string
		project, recordOnly, ambiguous, fatal, noDisk bool
	}{
		{name: "exact occupied declaration", input: " ./_occupied/ ", want: "_occupied", fatal: true, noDisk: true},
		{name: "exact absent declaration", input: "_absent", want: "_absent", fatal: true, noDisk: true},
		{name: "disk before shorthand", input: "team", disk: "org/team", want: "org/team"},
		{name: "declaration shorthand", input: "team", want: "org/_team"},
		{name: "disk before ambiguous declarations", input: "team", disk: "org/team", want: "org/team", ambiguous: true},
		{name: "ambiguous declarations", input: "team", ambiguous: true, wantErr: "multiple tracked repositories match"},
		{name: "disk refusal", input: "team", fatal: true, wantErr: "disk refusal"},
		{name: "checkout", input: "_checkout", disk: "_checkout", want: "_checkout"},
		{name: "project occupant", input: "_occupied", project: true, disk: "_occupied", want: "_occupied"},
		{name: "project declaration absent", input: "team", project: true, wantErr: ErrNotDeclared.Error()},
		{name: "record only exact", input: "_occupied", recordOnly: true, want: "_occupied", noDisk: true},
		{name: "record only shorthand refused", input: "team", recordOnly: true, wantErr: ErrNotDeclared.Error(), noDisk: true},
		{name: "record only project refused", input: "_occupied", project: true, recordOnly: true, wantErr: ErrNotDeclared.Error(), noDisk: true},
		{name: "record only restored checkout", input: "_checkout", recordOnly: true, wantErr: ErrCheckoutRestored.Error(), noDisk: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := t.TempDir()
			store := install.NewMetadataStore()
			for _, rel := range []string{"_occupied", "_absent", "org/_team", "_checkout"} {
				store.Set(rel, &install.MetadataEntry{Source: "unreachable", Tracked: true})
			}
			if tc.ambiguous {
				store.Set("other/_team", &install.MetadataEntry{Source: "unreachable", Tracked: true})
			}
			for _, rel := range []string{"_occupied", "_checkout/.git"} {
				if err := os.MkdirAll(filepath.Join(src, rel), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if tc.project {
				store = nil
			}
			called := false
			got, err := ResolveName(src, store, tc.input, tc.recordOnly, func() (Item, error) {
				called = true
				if tc.fatal {
					return Item{}, errors.New("disk refusal")
				}
				if tc.disk == "" {
					return Item{}, nil
				}
				return Item{Name: tc.disk, Path: filepath.Join(src, tc.disk), Repo: tc.disk == "_checkout"}, nil
			})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("result=%+v err=%v", got, err)
				}
			} else if err != nil || got.Name != tc.want || got.Path != filepath.Join(src, filepath.FromSlash(tc.want)) || got.MetadataOnly != (tc.disk == "") || (got.MetadataOnly && (got.Repo || got.Gitignore != "")) {
				t.Fatalf("result=%+v err=%v", got, err)
			}
			if called == tc.noDisk {
				t.Fatalf("disk called=%t, expected=%t", called, !tc.noDisk)
			}
		})
	}
}
