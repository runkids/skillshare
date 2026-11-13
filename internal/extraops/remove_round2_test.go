package extraops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

func TestRemove_PreservesSourceEditedDuringSave(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, added := range []bool{false, true} {
			t.Run(fmt.Sprintf("project=%t/added=%t", project, added), func(t *testing.T) {
				req := removalFixture(t)
				if project {
					req.ProjectRoot = filepath.Dir(req.ExtrasRoot)
					cfg := &config.ProjectConfig{Extras: req.Extras}
					if err := cfg.Save(req.ProjectRoot); err != nil {
						t.Fatal(err)
					}
					req.Load = func() ([]config.ExtraConfig, error) {
						fresh, err := config.LoadProjectReadOnly(req.ProjectRoot)
						if err != nil {
							return nil, err
						}
						return fresh.Extras, nil
					}
					req.Save = func(remaining []config.ExtraConfig) error {
						fresh, err := config.LoadProjectReadOnly(req.ProjectRoot)
						if err != nil {
							return err
						}
						fresh.Extras = remaining
						return fresh.Save(req.ProjectRoot)
					}
					lock := &install.Lock{Version: 1, Extras: map[string]install.LockEntry{
						"rules": {Source: "example-org/ai-hub", Commit: strings.Repeat("a", 40)},
					}}
					if err := lock.Save(filepath.Join(req.ProjectRoot, ".skillshare")); err != nil {
						t.Fatal(err)
					}
					if err := install.UpdateGitIgnore(req.ProjectRoot, "extras/rules/"); err != nil {
						t.Fatal(err)
					}
				}
				file := filepath.Join(req.SourceDir, "rule.md")
				if added {
					file = filepath.Join(req.SourceDir, "new-note.md")
				}
				originalSave := req.Save
				req.Save = func(remaining []config.ExtraConfig) error {
					if err := originalSave(remaining); err != nil {
						return err
					}
					// A supported save callback occurs after the initial hash/backup.
					// The editor does not participate in the remote-extra lock.
					return os.WriteFile(file, []byte("observed later edit"), 0644)
				}
				result, err := Remove(req)
				if err != nil {
					t.Fatal(err)
				}
				if !result.KeptEditedDir || result.DeletedDir || result.MissingDir {
					t.Errorf("edited source not retained: %+v", result)
				}
				if data, err := os.ReadFile(file); err != nil || string(data) != "observed later edit" {
					t.Errorf("later edit lost: %q %v", data, err)
				}
				entries, err := req.Load()
				if err != nil || len(entries) != 1 || entries[0].Name != "other" {
					t.Errorf("declaration removal: %+v %v", entries, err)
				}
				store, _, err := install.ReadMetadata(req.ExtrasRoot)
				if err != nil || store.GetExact("rules") != nil {
					t.Errorf("metadata removal: %+v %v", store, err)
				}
				if project {
					lock, err := install.LoadLock(filepath.Join(req.ProjectRoot, ".skillshare"))
					if err != nil || lock.ExtraCommitFor("rules", "example-org/ai-hub") != "" {
						t.Errorf("pin removal: %+v %v", lock, err)
					}
					ignore, err := os.ReadFile(filepath.Join(req.ProjectRoot, ".gitignore"))
					if err != nil || !strings.Contains(string(ignore), "extras/rules/") {
						t.Errorf("edited-source ignore removed: %q %v", ignore, err)
					}
				}
			})
		}
	}
}
