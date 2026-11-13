package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestProjectTargetInstructions_ReloadPathAfterRemoteLock(t *testing.T) {
	s, root := newTestProjectServerWithExtras(t, []config.ExtraConfig{{
		Name: "rules", Remote: "example-org/ai-hub/rules", Targets: []config.ExtraTargetConfig{},
	}})
	s.projectCfg.Targets = []config.ProjectTargetEntry{{Name: "myagent",
		Skills:       &config.ResourceTargetConfig{Path: ".myagent/skills"},
		Instructions: &config.TargetInstructionsConfig{Path: ".myagent/before.md"},
	}}
	if err := s.projectCfg.Save(root); err != nil {
		t.Fatal(err)
	}
	if err := s.reloadConfig(); err != nil {
		t.Fatal(err)
	}
	before, after := filepath.Join(root, ".myagent/before.md"), filepath.Join(root, ".myagent/published.md")
	if err := os.MkdirAll(filepath.Dir(before), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(before, []byte("# Original\n"), 0644); err != nil {
		t.Fatal(err)
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
		fresh.Targets[0].Instructions.Path = ".myagent/published.md"
		if err := fresh.Save(root); err != nil {
			return nil, err
		}
		return func() { releases++ }, nil
	}
	t.Cleanup(func() { lockRemoteExtraRoot = old })
	rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/myagent/instructions", `{"content":"# Updated\n"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	if calls != 1 || releases != 1 {
		t.Errorf("locks/releases=%d/%d want=1/1", calls, releases)
	}
	if data, err := os.ReadFile(after); err != nil || string(data) != "# Updated\n" {
		t.Errorf("published instruction path not used: %q %v", data, err)
	}
	if data, err := os.ReadFile(before); err != nil || string(data) != "# Original\n" {
		t.Errorf("stale instruction path changed: %q %v", data, err)
	}
	lockRemoteExtraRoot = old
	t.Run("interrupted-associated-file", func(t *testing.T) {
		for _, project := range []bool{false, true} {
			for _, mode := range []string{"copy", "symlink"} {
				for _, action := range []string{"edit", "detach", "guidance"} {
					if project && action != "edit" {
						continue
					}
					t.Run(fmt.Sprintf("project=%t/%s/%s", project, mode, action), func(t *testing.T) {
						if mode == "symlink" && runtime.GOOS == "windows" {
							t.Skip("requires file symlink privilege; copy rows remain platform-neutral")
						}
						var s *Server
						var target string
						if project {
							var root string
							s, root = newProjectSharedServer(t)
							addLocation(t, s, "team", `{"path":".","mode":"`+mode+`"}`)
							target = filepath.Join(root, "AGENTS.md")
						} else {
							var home string
							s, home = newInstructionsServer(t, "claude")
							target = writeHome(t, home, ".claude/CLAUDE.md", "own file\n")
							createShared(t, s, "team")
							rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude"],"extras":["team"]}`)
							if rr.Code != http.StatusOK {
								t.Fatalf("actual attachment: %d %s", rr.Code, rr.Body)
							}
							rr = instructionsRequest(t, s, http.MethodPut, "/api/instructions/team/targets/claude/mode", `{"mode":"`+mode+`"}`)
							if rr.Code != http.StatusOK {
								t.Fatalf("actual delivery mode: %d %s", rr.Code, rr.Body)
							}
						}
						extra := &(*s.sharedExtras())[0]
						extra.Remote, extra.Branch = "file:///fixture-interrupted-team", "main"
						if err := s.saveAndReloadConfig(); err != nil {
							t.Fatal(err)
						}
						path, method, body := "/api/targets/claude/instructions", http.MethodPut, `{"content":"must not write\n"}`
						if project {
							path = "/api/instructions/project"
						} else if action == "detach" {
							path, method, body = "/api/instructions/assign", http.MethodPost, `{"targets":["claude"],"extras":[]}`
						} else if action == "guidance" {
							plan := planGuidanceFor(t, s, `["claude"]`)
							if len(plan.Changes) == 0 {
								t.Fatal("no supported guidance write planned")
							}
							path, method, body = "/api/extras/memory/guidance/apply", http.MethodPost, `{"targets":["claude"],"token":"`+plan.Token+`"}`
						}
						source := filepath.Join(s.extrasSourceDir(*extra), extra.File)
						marker := filepath.Join(filepath.Dir(filepath.Dir(source)), ".skillshare-staging", "publishing-team.json")
						markerBytes, err := json.Marshal(map[string]any{"version": 1, "name": "team", "remote": extra.Remote, "branch": "main", "commit": strings.Repeat("a", 40), "had_previous": false, "old": nil})
						if err != nil {
							t.Fatal(err)
						}
						if err := os.MkdirAll(filepath.Dir(marker), 0700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(marker, markerBytes, 0600); err != nil {
							t.Fatal(err)
						}
						configBefore, err := os.ReadFile(s.configPath())
						if err != nil {
							t.Fatal(err)
						}
						if data, err := os.ReadFile(target); err != nil || string(data) != "team\n" {
							t.Fatalf("actual delivered target missing: %q %v", data, err)
						}
						rr := instructionsRequest(t, s, method, path, body)
						if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "interrupted") {
							t.Errorf("affected %s admitted: %d %s", action, rr.Code, rr.Body)
						}
						for _, file := range []string{source, target} {
							if data, err := os.ReadFile(file); err != nil || string(data) != "team\n" {
								t.Errorf("affected participant changed: %s %q %v", file, data, err)
							}
						}
						if info, err := os.Lstat(target); err != nil || (info.Mode()&os.ModeSymlink != 0) != (mode == "symlink") {
							t.Errorf("delivery type changed: %v %v", info, err)
						}
						if data, err := os.ReadFile(marker); err != nil || string(data) != string(markerBytes) {
							t.Errorf("recovery evidence changed: %q %v", data, err)
						}
						if data, err := os.ReadFile(s.configPath()); err != nil || string(data) != string(configBefore) {
							t.Errorf("affected assignment changed: %q %v", data, err)
						}
					})
				}
			}
		}
	})
	t.Run("copy-location-participants", func(t *testing.T) {
		for _, project := range []bool{false, true} {
			for _, action := range []string{"content", "guidance"} {
				if project && action == "guidance" {
					continue
				}
				for _, state := range []string{"local", "remote-clean", "remote-interrupted"} {
					t.Run(fmt.Sprintf("project=%t/%s/%s", project, action, state), func(t *testing.T) {
						var s *Server
						var own string
						if project {
							s, _ = newProjectSharedServer(t)
						} else {
							var home string
							s, home = newInstructionsServer(t, "claude")
							own = writeHome(t, home, ".claude/CLAUDE.md", "own file\n")
							createShared(t, s, "team")
						}
						createShared(t, s, "rules")
						if action == "guidance" {
							rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude"],"extras":["team"]}`)
							if rr.Code != http.StatusOK {
								t.Fatalf("actual guidance attachment: %d %s", rr.Code, rr.Body)
							}
						}
						rules, _ := s.sharedExtra("rules")
						dir := s.extrasSourceDir(rules)
						stored := dir
						if project {
							var err error
							stored, err = filepath.Rel(s.projectRoot, dir)
							if err != nil {
								t.Fatal(err)
							}
							stored = filepath.ToSlash(stored)
						}
						location, err := json.Marshal(map[string]string{"path": stored, "mode": "copy"})
						if err != nil {
							t.Fatal(err)
						}
						addLocation(t, s, "team", string(location))
						if state != "local" {
							for i := range *s.sharedExtras() {
								extra := &(*s.sharedExtras())[i]
								if extra.Name == "rules" {
									extra.Remote, extra.Branch = "file:///fixture-copy-rules", "main"
								}
							}
							if err := s.saveAndReloadConfig(); err != nil {
								t.Fatal(err)
							}
						}
						team, _ := s.sharedExtra("team")
						source := filepath.Join(s.extrasSourceDir(team), team.File)
						destination := filepath.Join(dir, rules.File)
						if data, err := os.ReadFile(destination); err != nil || string(data) != "team\n" {
							t.Fatalf("copy producer did not deliver destination: %q %v", data, err)
						}
						method, path, body := http.MethodPut, "/api/instructions/team/content", `{"content":"updated through copy\n"}`
						if action == "guidance" {
							plan := planGuidanceFor(t, s, `["claude"]`)
							if len(plan.Changes) != 1 || plan.Changes[0].Path != source {
								t.Fatalf("guidance must write local shared source: %+v", plan.Changes)
							}
							method, path, body = http.MethodPost, "/api/extras/memory/guidance/apply", `{"targets":["claude"],"token":"`+plan.Token+`"}`
						}
						marker := filepath.Join(filepath.Dir(dir), ".skillshare-staging", "publishing-rules.json")
						var markerBytes []byte
						if state == "remote-interrupted" {
							markerBytes, err = json.Marshal(map[string]any{"version": 1, "name": "rules", "remote": "file:///fixture-copy-rules", "branch": "main", "commit": strings.Repeat("a", 40), "had_previous": false, "old": nil})
							if err != nil {
								t.Fatal(err)
							}
							if err := os.MkdirAll(filepath.Dir(marker), 0700); err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(marker, markerBytes, 0600); err != nil {
								t.Fatal(err)
							}
						}
						configBefore, err := os.ReadFile(s.configPath())
						if err != nil {
							t.Fatal(err)
						}
						var ownBefore []byte
						if !project {
							ownBefore, err = os.ReadFile(own)
							if err != nil {
								t.Fatal(err)
							}
						}
						rr := instructionsRequest(t, s, method, path, body)
						blocked := state == "remote-interrupted"
						want := http.StatusOK
						if blocked {
							want = http.StatusConflict
						}
						if rr.Code != want || blocked && !strings.Contains(rr.Body.String(), "interrupted") {
							t.Errorf("copy participant: %d %s want %d", rr.Code, rr.Body, want)
						}
						wantContent := "updated through copy\n"
						if blocked {
							wantContent = "team\n"
						} else if action == "guidance" {
							data, err := os.ReadFile(source)
							if err != nil || !strings.Contains(string(data), "<!-- skillshare:memory") {
								t.Fatalf("guidance block not written: %q %v", data, err)
							}
							wantContent = string(data)
						}
						for _, file := range []string{source, destination} {
							if data, err := os.ReadFile(file); err != nil || string(data) != wantContent {
								t.Errorf("copy source/destination bytes: %s %q %v", file, data, err)
							}
						}
						if data, err := os.ReadFile(s.configPath()); err != nil || string(data) != string(configBefore) {
							t.Errorf("copy participant declaration changed: %q %v", data, err)
						}
						if blocked {
							if data, err := os.ReadFile(marker); err != nil || string(data) != string(markerBytes) {
								t.Errorf("copy participant marker changed: %q %v", data, err)
							}
							if !project {
								if data, err := os.ReadFile(own); err != nil || string(data) != string(ownBefore) {
									t.Errorf("blocked guidance/target bytes changed: %q %v", data, err)
								}
							}
						}
					})
				}
			}
		}
	})
}
