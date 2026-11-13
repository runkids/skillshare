package extraops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestRemove_FailureAfterTargetCleanupGuidesSync(t *testing.T) {
	for _, phase := range []string{"target", "source"} {
		t.Run(phase, func(t *testing.T) {
			req := removalFixture(t)
			output := filepath.Join(filepath.Dir(req.ExtrasRoot), "target-output.md")
			if err := os.WriteFile(output, []byte("synced output"), 0644); err != nil {
				t.Fatal(err)
			}
			oldRestore, oldRemove := restoreFileTargets, removeExtraDir
			t.Cleanup(func() { restoreFileTargets, removeExtraDir = oldRestore, oldRemove })
			fault := errors.New("synthetic failure after cleanup")
			restoreFileTargets = func(config.ExtraConfig, string, func(string) string) (int, error) {
				if err := os.Remove(output); err != nil {
					return 0, err
				}
				if phase == "target" {
					return 1, fault
				}
				return 1, nil
			}
			if phase == "source" {
				removeExtraDir = func(string) error { return fault }
			}
			_, err := Remove(req)
			if !errors.Is(err, fault) || !strings.Contains(err.Error(), "sync extras") {
				t.Errorf("failure lacks actionable resync guidance: %v", err)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("fixture did not clean a target: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(req.SourceDir, "rule.md"))
			if err != nil || string(data) != "rules" {
				t.Fatalf("source not restored: %s %v", data, err)
			}
			entries, err := req.Load()
			if err != nil || len(entries) != 2 || entries[0].Name != "rules" {
				t.Fatal(fmt.Sprintf("records not restored: %+v %v", entries, err))
			}
		})
	}
}
