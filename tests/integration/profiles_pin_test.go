//go:build !online

package integration

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

// CI builds the immutable pre-profile source with the same Makefile target as
// HEAD. Comparing actual binaries avoids inventing baseline output or silently
// accepting new JSON fields in a hand-written subset assertion.
func TestProfiles_NoProfilePin(t *testing.T) {
	baseline := os.Getenv("SKILLSHARE_PROFILE_BASELINE_BINARY")
	if baseline == "" {
		t.Skip("requires the pinned baseline binary built by the integration CI job")
	}
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"merge", "copy"} {
			t.Run(fmt.Sprintf("%v/%s", project, mode), func(t *testing.T) {
				type snapshot struct {
					JSON []any
					Config string
					Tree map[string]string
				}
				capture := func(binary string) snapshot {
					f := newProfileFixture(t, project, mode)
					if binary != "" { f.sb.BinaryPath = binary }
					text := strings.Split(f.sb.ReadFile(f.config), "profiles:\n")[0]
					if err := os.WriteFile(f.config, []byte(text), 0644); err != nil { t.Fatal(err) }
					s := snapshot{Tree: make(map[string]string)}
					for _, args := range [][]string{{"sync", "--json"}, {"status", "--json"}, {"diff", "--json"}, {"sync", "--json"}} {
						result := f.run(args...)
						result.AssertSuccess(t)
						var value map[string]any
						if err := json.Unmarshal([]byte(strings.ReplaceAll(result.Stdout, f.sb.Root, "<sandbox>")), &value); err != nil { t.Fatal(err) }
						// Elapsed wall time is the only nondeterministic JSON field.
						if _, ok := value["duration"]; ok { value["duration"] = "<duration>" }
						s.JSON = append(s.JSON, value)
					}
					s.Config = strings.ReplaceAll(f.sb.ReadFile(f.config), f.sb.Root, "<sandbox>")
					err := filepath.WalkDir(f.target, func(path string, entry fs.DirEntry, err error) error {
						if err != nil { return err }
						rel, err := filepath.Rel(f.target, path)
						if err != nil { return err }
						info, err := entry.Info()
						if err != nil { return err }
						value := info.Mode().String()
						if info.Mode()&os.ModeSymlink != 0 {
							link, err := os.Readlink(path)
							if err != nil { return err }
							value += ":"+link
						} else if !entry.IsDir() {
							data, err := os.ReadFile(path)
							if err != nil { return err }
							if entry.Name() == ssync.ManifestFile {
								var manifest map[string]any
								if err := json.Unmarshal(data, &manifest); err != nil { return err }
								// outputs is the declared bookkeeping addition. Both
								// remaining fields contain filesystem/wall timestamps.
								delete(manifest, "outputs")
								delete(manifest, "updated_at")
								if mtimes, ok := manifest["mtimes"].(map[string]any); ok { for name := range mtimes { mtimes[name] = float64(0) } }
								data, err = json.Marshal(manifest)
								if err != nil { return err }
							}
							value += ":"+string(data)
						}
						s.Tree[rel] = strings.ReplaceAll(value, f.sb.Root, "<sandbox>")
						return nil
					})
					if err != nil { t.Fatal(err) }
					_, lock, err := config.ProfilePaths(f.config)
					if err != nil { t.Fatal(err) }
					if _, err := os.Lstat(lock); !os.IsNotExist(err) { t.Fatal("no-profile command created a lock", err) }
					return s
				}
				before, after := capture(baseline), capture("")
				if !reflect.DeepEqual(before, after) { t.Fatalf("no-profile baseline changed:\nbaseline=%+v\nHEAD=%+v", before, after) }
			})
		}
	}
}
