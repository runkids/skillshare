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

func TestDiffProfileDisambiguation(t *testing.T) {
	for _, scope := range []string{"global", "project"} {
		for _, scenario := range []string{"standard/include/collision", "standard/exclude/collision", "prefixed/include/collision", "prefixed/exclude/collision", "standard/include/variant", "standard/exclude/variant", "prefixed/include/variant", "prefixed/exclude/variant"} {
			t.Run(scope+"/"+scenario, func(t *testing.T) {
				parts := strings.Split(scenario, "/")
				naming, filter, mode := parts[0], parts[1], "merge"
				if naming == "prefixed" {
					mode = "copy"
				}
				target := t.TempDir()
				t.Setenv("HOME", t.TempDir())
				for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
					t.Setenv(key, "")
				}
				s, source := newTestServerWithTargets(t, map[string]string{"claude": target})
				path := config.ConfigPath()
				if scope == "project" {
					var root string
					s, root = newProjectTargetServer(t, []config.ProjectTargetEntry{{Name: "claude", Skills: &config.ResourceTargetConfig{Path: target, Mode: mode}}})
					source, path = s.cfg.Source, config.ProjectConfigPath(root)
				}
				selected, rejected, want := "a/review", "b/review", "review"
				if parts[2] == "variant" {
					selected, rejected = "_repo/skills/claude/review", "_repo/skills/claude-code/review"
					if naming == "prefixed" {
						want = "repo-review"
					}
				}
				for _, rel := range []string{selected, rejected} {
					addSkill(t, filepath.Join(source, filepath.Dir(rel)), "review")
				}
				pattern := strings.ReplaceAll(selected, "/", "__")
				if filter == "exclude" {
					pattern = strings.ReplaceAll(rejected, "/", "__")
				}
				text := fmt.Sprintf("source: %s\nmode: %s\ntargets:\n  claude:\n    skills:\n      path: %s\n      target_naming: %s\n      %s: [%s]\n", source, mode, target, naming, filter, pattern)
				if scope == "project" {
					text = fmt.Sprintf("targets:\n  - name: claude\n    skills:\n      path: %s\n      mode: %s\n      target_naming: %s\n      %s: [%s]\n", target, mode, naming, filter, pattern)
				}
				if err := os.WriteFile(path, []byte(text+"profiles:\n  empty:\n    skills:\n      include: []\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := s.reloadConfig(); err != nil {
					t.Fatal(err)
				}
				skills, err := ssync.DiscoverSourceSkills(source)
				if err != nil {
					t.Fatal(err)
				}
				publication := ssync.SyncSkillTarget(ssync.SkillTarget{Name: "claude", Target: s.cloneTargets()["claude"], Mode: mode}, skills, ssync.SkillRunOptions{Source: source})
				if publication.Err != nil || publication.PruneErr != nil || len(publication.Linked) != 1 || publication.Linked[0] != want {
					t.Fatalf("managed publication: %+v", publication)
				}
				manifest, err := ssync.ReadManifest(target)
				if err != nil || len(manifest.Managed) != 1 || manifest.Managed[want] == "" {
					t.Fatalf("expected one real managed target entry: %+v, %v", manifest, err)
				}
				statePath, _, err := config.ProfilePaths(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "empty"}); err != nil {
					t.Fatal(err)
				}
				if err := s.reloadConfig(); err != nil {
					t.Fatal(err)
				}
				for _, route := range []string{"/api/diff", "/api/diff/stream"} {
					t.Run(route, func(t *testing.T) {
						rr := httptest.NewRecorder()
						s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, route, nil))
						payload := rr.Body.String()
						if strings.HasSuffix(route, "/stream") {
							payload = ""
							doneEvents := 0
							for _, event := range strings.Split(rr.Body.String(), "\n\n") {
								if strings.HasPrefix(event, "event: done\n") {
									payload = strings.TrimPrefix(event, "event: done\ndata: ")
									doneEvents++
								}
							}
							if doneEvents != 1 || !strings.HasPrefix(rr.Header().Get("Content-Type"), "text/event-stream") {
								t.Fatal("expected one SSE completion", rr.Body.String())
							}
						}
						var out struct {
							Diffs []diffTarget `json:"diffs"`
						}
						if err := json.Unmarshal([]byte(payload), &out); err != nil || rr.Code != http.StatusOK || len(out.Diffs) != 1 || len(out.Diffs[0].Items) != 1 {
							t.Fatal(rr.Body.String(), err)
						}
						item, diff := out.Diffs[0].Items[0], out.Diffs[0]
						if diff.Target != "claude" || diff.SkippedCount != 0 || diff.CollisionCount != 0 || diff.NamingSkippedCount != 0 || item.Skill != want || item.Action != "prune" || item.Reason != "not in profile empty" {
							t.Fatalf("filtered managed removal lost identity/action/count/reason: %+v", diff)
						}
					})
				}
			})
		}
	}
}
