//go:build !online

package integration

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestProjectAdoption_OnlyMovedEntries(t *testing.T) {
	for _, move := range []bool{false, true} {
		name := "stale record without a move"
		if move {
			name = "real move with unrelated installs"
		}
		t.Run(name, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			root := sb.SetupProjectDir("claude")
			dir := filepath.Join(root, "skillshare")
			if err := os.Rename(filepath.Join(root, ".skillshare"), dir); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(dir, "skills")
			// The same source also holds global installs not declared by this project.
			sb.WriteConfig("source: " + source + "\n")
			manifest := "# Keep this comment when nothing moved.\ntargets:\n  - claude\nskills:\n  - name: demo\n    source: github.com/user/repo/demo\n  - name: local\n"
			sb.WriteFile(filepath.Join(dir, "config.yaml"), manifest)
			ignore := "# User rule\nprivate/\n# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\nlogs/\nskills/demo/\n# END SKILLSHARE MANAGED\n"
			sb.WriteFile(filepath.Join(dir, ".gitignore"), ignore)
			pin := install.LockEntry{Source: "github.com/user/repo/demo", Commit: strings.Repeat("a", 40)}
			unrelatedPin := install.LockEntry{Source: "github.com/user/repo/other", Commit: strings.Repeat("c", 40)}
			lock := &install.Lock{Skills: map[string]install.LockEntry{"demo": pin, "other": unrelatedPin}}
			if err := lock.Save(dir); err != nil {
				t.Fatal(err)
			}
			store := install.NewMetadataStore()
			for _, skill := range []string{"demo", "undeclared", "unpinned", "gone"} {
				path := filepath.Join(source, skill)
				sb.WriteFile(filepath.Join(path, "SKILL.md"), "---\nname: "+skill+"\n---\n# "+skill)
				hashes, err := install.ComputeFileHashes(path)
				if err != nil {
					t.Fatal(err)
				}
				commit := strings.Repeat("b", 40)
				if skill == "unpinned" {
					commit = ""
				}
				store.Set(skill, &install.MetadataEntry{Source: "github.com/user/repo/" + skill, Commit: commit, FileHashes: hashes})
			}
			store.AuditAccepted = map[string][]string{"demo": {"accepted-key"}, "gone": {"gone-key"}}
			if err := store.Save(source); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(filepath.Join(source, "gone")); err != nil {
				t.Fatal(err)
			}
			if move {
				if err := os.MkdirAll(filepath.Join(source, "grp"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(source, "demo"), filepath.Join(source, "grp", "demo")); err != nil {
					t.Fatal(err)
				}
			}
			before := map[string][]byte{}
			for _, file := range []string{"config.yaml", ".gitignore", install.LockFileName} {
				data, err := os.ReadFile(filepath.Join(dir, file))
				if err != nil {
					t.Fatal(err)
				}
				before[file] = data
			}

			sb.RunCLIInDir(root, "sync", "-p").AssertSuccess(t)

			if !move {
				for file, want := range before {
					got, err := os.ReadFile(filepath.Join(dir, file))
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(got, want) {
						t.Errorf("%s changed without a move:\n%s", file, got)
					}
				}
			} else {
				cfg, err := config.LoadProject(root)
				if err != nil {
					t.Fatal(err)
				}
				want := []config.SkillEntry{{Name: "demo", Source: pin.Source, Group: "grp"}, {Name: "local"}}
				if !reflect.DeepEqual(cfg.Skills, want) {
					t.Errorf("skills = %+v, want %+v", cfg.Skills, want)
				}
				gotIgnore := sb.ReadFile(filepath.Join(dir, ".gitignore"))
				if gotIgnore != strings.Replace(ignore, "skills/demo/", "skills/grp/demo/", 1) {
					t.Errorf("unexpected ignore rules:\n%s", gotIgnore)
				}
				gotLock, err := install.LoadLock(dir)
				if err != nil {
					t.Fatal(err)
				}
				wantPins := map[string]install.LockEntry{"grp/demo": pin, "other": unrelatedPin}
				if !reflect.DeepEqual(gotLock.Skills, wantPins) {
					t.Errorf("pins = %+v, want %+v", gotLock.Skills, wantPins)
				}
			}
			gotStore, err := install.LoadMetadata(source)
			if err != nil {
				t.Fatal(err)
			}
			if !gotStore.Has("gone") || len(gotStore.AuditAccepted["gone"]) != 1 {
				t.Error("sync pruned the stale record or its audit acceptances")
			}
			if move && (gotStore.Has("demo") || !gotStore.Has("grp/demo") || len(gotStore.AuditAccepted["grp/demo"]) != 1) {
				t.Error("sync did not carry the moved record and audit acceptances")
			}
		})
	}
}
