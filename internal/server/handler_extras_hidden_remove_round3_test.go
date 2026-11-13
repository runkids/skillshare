package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	syncpkg "skillshare/internal/sync"
	"skillshare/internal/utils"
)

func TestExtrasDelete_HiddenOwnedLinksAPI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX symlink fixture; no native Windows assertion")
	}
	for _, project := range []bool{false, true} {
		for _, remote := range []bool{false, true} {
			t.Run(fmt.Sprintf("project=%t/remote=%t", project, remote), func(t *testing.T) {
				target := t.TempDir()
				entry := config.ExtraConfig{Name: "rules", Targets: []config.ExtraTargetConfig{{Path: target, Mode: "merge"}}}
				if remote {
					entry.Remote = "example-org/ai-hub"
				}
				var s *Server
				if project {
					s, _ = newTestProjectServerWithExtras(t, []config.ExtraConfig{entry})
				} else {
					s, _ = newTestServerWithExtras(t, []config.ExtraConfig{entry}, "")
				}
				source := s.extrasSourceDir(entry)
				paths := []string{".rules.md", ".nested/.inside.md", ".nested/visible.md"}
				hashes := map[string]string{}
				for _, name := range paths {
					path := filepath.Join(source, name)
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("owned fixture"), 0644); err != nil {
						t.Fatal(err)
					}
					hash, err := utils.FileHashFormatted(path)
					if err != nil {
						t.Fatal(err)
					}
					hashes[filepath.ToSlash(name)] = hash
				}
				if remote {
					store := install.NewMetadataStore()
					store.Set("rules", &install.MetadataEntry{Kind: "extra", Source: entry.Remote, Commit: strings.Repeat("a", 40), FileHashes: hashes})
					if err := store.Save(filepath.Dir(source)); err != nil {
						t.Fatal(err)
					}
				}
				result, err := syncpkg.SyncExtraTarget(entry, entry.Targets[0], source, target, "merge", nil, syncpkg.ExtraSyncOptions{})
				if err != nil || len(result.Errors) != 0 {
					t.Fatalf("supported hidden sync fixture: %+v %v", result, err)
				}
				for _, name := range paths {
					if info, err := os.Lstat(filepath.Join(target, name)); err != nil || info.Mode()&os.ModeSymlink == 0 {
						t.Fatalf("hidden sync did not create owned link %s: %v", name, err)
					}
				}
				foreign := filepath.Join(target, ".foreign.md")
				if err := os.WriteFile(foreign, []byte("keep foreign"), 0644); err != nil {
					t.Fatal(err)
				}
				rr := serveExtras(s, http.MethodDelete, "/api/extras/rules", `{}`)
				if rr.Code != http.StatusOK {
					t.Fatalf("remove: %d %s", rr.Code, rr.Body.String())
				}
				for _, name := range paths {
					_, err := os.Lstat(filepath.Join(target, name))
					if remote && !os.IsNotExist(err) || !remote && err != nil {
						t.Errorf("hidden cleanup remote=%t path=%s: %v", remote, name, err)
					}
				}
				if body, err := os.ReadFile(foreign); err != nil || string(body) != "keep foreign" {
					t.Errorf("foreign hidden content changed: %q %v", body, err)
				}
			})
		}
	}
}
