package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/profile"
	ssync "skillshare/internal/sync"
)

func TestDiffProfileReasons(t *testing.T) {
	for _, scenario := range []string{"copy/global", "merge/global", "copy/global/overlap", "merge/global/overlap", "copy/project/overlap", "merge/project/overlap"} {
		t.Run(scenario, func(t *testing.T) {
			mode, _, _ := strings.Cut(scenario, "/")
			target := t.TempDir()
			s, source := newTestServerWithTargets(t, map[string]string{"custom": target})
			path := config.ConfigPath()
			if strings.Contains(scenario, "/project") {
				var root string
				s, root = newProjectTargetServer(t, []config.ProjectTargetEntry{{Name: "custom", Skills: &config.ResourceTargetConfig{Path: target, Mode: mode}}})
				source, path = s.cfg.Source, config.ProjectConfigPath(root)
			}
			for _, name := range []string{"other", "target-only"} {
				addSkill(t, source, name)
				addSkill(t, target, name)
			}
			if err := ssync.WriteManifest(target, &ssync.Manifest{Managed: map[string]string{"other": "fixture", "target-only": "fixture"}}); err != nil {
				t.Fatal(err)
			}
			text := fmt.Sprintf("source: %s\nmode: %s\ntargets:\n  custom:\n    skills:\n      path: %s\n      exclude: [target-only]\nprofiles:\n  frontend:\n    skills:\n      include: [target-only]\n", source, mode, target)
			if s.IsProjectMode() {
				text = fmt.Sprintf("targets:\n  - name: custom\n    skills:\n      path: %s\n      mode: %s\n      exclude: [target-only]\nprofiles:\n  frontend:\n    skills:\n      include: [target-only]\n", target, mode)
			}
			if strings.HasSuffix(scenario, "/overlap") {
				text = strings.ReplaceAll(text, "exclude: [target-only]", "exclude: [target-only, other]")
			}
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			statePath, _, err := config.ProfilePaths(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "frontend"}); err != nil {
				t.Fatal(err)
			}
			if err := s.reloadConfig(); err != nil {
				t.Fatal(err)
			}
			for _, route := range []string{"/api/diff", "/api/diff/stream"} {
				rr := httptest.NewRecorder()
				s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, route, nil))
				payload := rr.Body.String()
				if strings.HasSuffix(route, "/stream") {
					payload = ""
					for _, event := range strings.Split(rr.Body.String(), "\n\n") {
						if strings.HasPrefix(event, "event: done\n") {
							payload = strings.TrimPrefix(event, "event: done\ndata: ")
						}
					}
				}
				var out struct {
					Diffs []diffTarget `json:"diffs"`
				}
				if err := json.Unmarshal([]byte(payload), &out); err != nil || rr.Code != http.StatusOK || len(out.Diffs) != 1 || len(out.Diffs[0].Items) != 2 {
					t.Fatal(route, rr.Body.String(), err)
				}
				for _, item := range out.Diffs[0].Items {
					if item.Action != "prune" || item.Skill == "other" && item.Reason != "not in profile frontend" || item.Skill == "target-only" && !strings.HasPrefix(item.Reason, "orphan") {
						t.Fatal(route, item)
					}
				}
			}
			// An unmanaged local folder retains its existing local-only row.
			if err := os.Remove(filepath.Join(target, ssync.ManifestFile)); err != nil {
				t.Fatal(err)
			}
			discovered, err := ssync.DiscoverSourceSkills(source)
			if err != nil {
				t.Fatal(err)
			}
			diff := s.computeTargetDiff("custom", s.cloneTargets()["custom"], discovered, mode, source, nil)
			for _, item := range diff.Items {
				if item.Skill == "other" && (item.Action != "local" || item.Reason != "local only") {
					t.Fatal("profile reason changed unmanaged operation", item)
				}
			}
		})
	}
}
