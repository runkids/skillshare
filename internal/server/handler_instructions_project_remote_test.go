package server

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"skillshare/internal/config"
)

func TestProjectSharedMutators_RemoteRootLockAndReload(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, action := range []string{"content", "location-add", "location-mode", "location-remove"} {
			t.Run(fmt.Sprintf("remote=%t/%s", remote, action), func(t *testing.T) {
				s, root := newTestProjectServerWithExtras(t, nil)
				createShared(t, s, "team")
				if action == "location-mode" || action == "location-remove" {
					addLocation(t, s, "team", `{"path":"./docs","mode":"copy"}`)
				}
				if remote {
					s.projectCfg.Extras[0].Remote, s.projectCfg.Extras[0].Branch = "example-org/ai-hub/instructions", "before"
					if err := s.projectCfg.Save(root); err != nil {
						t.Fatal(err)
					}
				}
				calls, releases := 0, 0
				old := lockRemoteExtraRoot
				lockRemoteExtraRoot = func(locked string) (func(), error) {
					calls++
					if locked != s.projectCfg.EffectiveExtrasSource(root) {
						t.Errorf("wrong root %s", locked)
					}
					fresh, err := config.LoadProjectReadOnly(root)
					if err != nil {
						return nil, err
					}
					fresh.Extras[0].Branch = "published"
					if err := fresh.Save(root); err != nil {
						return nil, err
					}
					return func() { releases++ }, nil
				}
				t.Cleanup(func() { lockRemoteExtraRoot = old })
				method, path, body := http.MethodPut, "/api/instructions/team/content", `{"content":"# Updated\n"}`
				switch action {
				case "location-add":
					method, path, body = http.MethodPost, "/api/instructions/team/locations", `{"path":"./docs","mode":"copy"}`
				case "location-mode":
					path, body = "/api/instructions/team/locations/mode", `{"path":"docs","mode":"append"}`
				case "location-remove":
					method, path, body = http.MethodDelete, "/api/instructions/team/locations?path="+url.QueryEscape("docs"), ""
				}
				rr := instructionsRequest(t, s, method, path, body)
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
				if remote && s.projectCfg.Extras[0].Branch != "published" {
					t.Error("project writer retained stale declaration")
				}
			})
		}
	}
}
