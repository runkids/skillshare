package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/memory"
)

func TestMemoryMutators_RemoteRootLockAndReload(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, remote := range []bool{false, true} {
			for _, action := range []string{"init", "write", "delete", "index", "move"} {
				t.Run(fmt.Sprintf("project=%t/remote=%t/%s", project, remote, action), func(t *testing.T) {
					extra := config.ExtraConfig{Name: "memory", Targets: []config.ExtraTargetConfig{}}
					if remote {
						extra.Remote, extra.Branch = "example-org/ai-hub/memory", "before"
					}
					var s *Server
					if project {
						s, _ = newTestProjectServerWithExtras(t, []config.ExtraConfig{extra})
					} else {
						s, _ = newTestServerWithExtras(t, []config.ExtraConfig{extra}, "")
					}
					root, err := s.memoryRoot()
					if err != nil {
						t.Fatal(err)
					}
					if err := memory.Init(root); err != nil {
						t.Fatal(err)
					}
					note, err := memory.Write(root, "note.md", "# Fixture\n", "")
					if err != nil {
						t.Fatal(err)
					}
					index, err := memory.Read(root, "INDEX.md")
					if err != nil {
						t.Fatal(err)
					}
					calls, releases := 0, 0
					old := lockRemoteExtraRoot
					lockRemoteExtraRoot = func(locked string) (func(), error) {
						calls++
						if locked != filepath.Dir(root) {
							t.Errorf("wrong root %s", locked)
						}
						if s.IsProjectMode() {
							fresh, err := config.LoadProjectReadOnly(s.projectRoot)
							if err != nil {
								return nil, err
							}
							fresh.Extras[0].Branch = "published"
							if err := fresh.Save(s.projectRoot); err != nil {
								return nil, err
							}
						} else {
							fresh, err := config.LoadReadOnly()
							if err != nil {
								return nil, err
							}
							fresh.Extras[0].Branch = "published"
							if err := fresh.Save(); err != nil {
								return nil, err
							}
						}
						return func() { releases++ }, nil
					}
					t.Cleanup(func() { lockRemoteExtraRoot = old })
					method, path := http.MethodPost, "/api/extras/memory/init"
					body := map[string]string{}
					switch action {
					case "write":
						method, path = http.MethodPut, "/api/extras/memory/notes/content"
						body = map[string]string{"path": "note.md", "content": "# Updated\n", "version": note.Version}
					case "delete":
						method, path = http.MethodDelete, "/api/extras/memory/notes/content"
						body = map[string]string{"path": "note.md", "version": note.Version}
					case "index":
						method, path = http.MethodPut, "/api/extras/memory/index"
						body = map[string]string{"path": "note.md", "version": index.Version}
					case "move":
						path = "/api/extras/memory/move"
						body = map[string]string{"path": "note.md", "new_path": "moved.md", "version": note.Version}
					}
					data, err := json.Marshal(body)
					if err != nil {
						t.Fatal(err)
					}
					rr := serveJSON(t, s, method, path, string(data))
					if rr.Code != http.StatusOK {
						t.Fatalf("%d %s", rr.Code, rr.Body)
					}
					want := 0
					if remote {
						want = 1
					}
					if calls != want || releases != want {
						t.Errorf("locks/releases=%d/%d want=%d", calls, releases, want)
					}
					if remote && s.extrasConfig()[0].Branch != "published" {
						t.Error("writer retained pre-lock config")
					}
				})
			}
		}
	}
}
