package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallRemoteExtra_PinnedWebTreeSelection(t *testing.T) {
	for _, sparse := range []bool{false, true} {
		for _, evidence := range []string{"unique", "zero", "multiple", "legacy-absent"} {
			t.Run(fmt.Sprintf("sparse=%t/%s", sparse, evidence), func(t *testing.T) {
				req, seed := pinnedWebTreeFixture(t, sparse, evidence == "multiple")
				lock, err := LoadLock(req.LockDir)
				if err != nil {
					t.Fatal(err)
				}
				pin := lock.Extras[req.Name]
				if len(pin.TreeHash) != 40 || pin.Source != req.RawSource || len(pin.Commit) != 40 {
					t.Fatalf("real producer pin fixture: %+v", pin)
				}
				if evidence == "zero" {
					pin.TreeHash, err = gitOutput(seed, "rev-parse", "HEAD:outside")
					if err != nil {
						t.Fatal(err)
					}
				} else if evidence == "legacy-absent" {
					pin.TreeHash = ""
				}
				lock.Extras[req.Name] = pin
				other := req
				other.ProjectRoot = t.TempDir()
				other.LockDir = filepath.Join(other.ProjectRoot, ".skillshare")
				other.ExtrasRoot = filepath.Join(other.LockDir, "extras")
				other.ConfigPath = filepath.Join(other.ProjectRoot, "config.yaml")
				if err := os.MkdirAll(other.LockDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := lock.Save(other.LockDir); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(other.ConfigPath, []byte("unchanged config"), 0644); err != nil {
					t.Fatal(err)
				}
				beforeLock, err := os.ReadFile(filepath.Join(other.LockDir, LockFileName))
				if err != nil {
					t.Fatal(err)
				}
				other.Source, err = ParseSource(other.RawSource)
				if err != nil {
					t.Fatal(err)
				}
				parsedSubdir := other.Source.Subdir
				other.Branch = ""
				saves := 0
				other.WriteConfig = func() error { saves++; return nil }
				other.RevertConfig = func() error { return nil }
				result, err := InstallRemoteExtra(other)
				if evidence == "unique" {
					if err != nil || result.Commit != pin.Commit || result.Branch != "main" || result.Remote != pin.Source {
						t.Fatalf("unique pinned selection: %+v %v", result, err)
					}
					if data, err := os.ReadFile(filepath.Join(other.ExtrasRoot, other.Name, "rule.md")); err != nil || string(data) != "pinned correct" {
						t.Errorf("unique tree selected wrong directory: %q %v", data, err)
					}
					store, _, err := ReadMetadata(other.ExtrasRoot)
					if err != nil || store.GetExact(other.Name) == nil || store.GetExact(other.Name).Subdir != "rules" || store.GetExact(other.Name).TreeHash != pin.TreeHash {
						t.Errorf("unique selection lost provenance: %+v %v", store, err)
					}
				} else {
					if err == nil || !strings.Contains(strings.ToLower(err.Error()), "selection") || !strings.Contains(err.Error(), "--update") {
						t.Errorf("unknown/ambiguous pinned selection needs actionable refusal: result=%+v err=%v", result, err)
					}
					if saves != 0 {
						t.Errorf("refusal saved config %d times", saves)
					}
					if entries, err := os.ReadDir(other.ExtrasRoot); err != nil || len(entries) != 1 || entries[0].Name() != ".skillshare-extras.lock" {
						t.Errorf("refusal changed managed participants: %+v %v", entries, err)
					}
					if data, err := os.ReadFile(other.ConfigPath); err != nil || string(data) != "unchanged config" {
						t.Errorf("refusal changed config: %q %v", data, err)
					}
					if data, err := os.ReadFile(filepath.Join(other.LockDir, LockFileName)); err != nil || string(data) != string(beforeLock) {
						t.Errorf("refusal changed pin: %q %v", data, err)
					}
					if _, err := os.Lstat(filepath.Join(other.LockDir, ".gitignore")); !os.IsNotExist(err) {
						t.Errorf("refusal changed managed ignore: %v", err)
					}
				}
				if other.Source.Raw != req.RawSource || other.Source.Subdir != parsedSubdir {
					t.Errorf("selection rewrote request identity: %+v", other.Source)
				}
			})
		}
	}
}
