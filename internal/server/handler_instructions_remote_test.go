package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestSharedInstructionMutators_RemoteRootLockAndReload(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, action := range []string{"content", "convert", "assign", "restore", "mode", "collect", "location-add", "location-mode", "location-remove", "guidance", "setup", "setup-delete", "create"} {
			t.Run(fmt.Sprintf("remote=%t/%s", remote, action), func(t *testing.T) {
				s, home := newInstructionsServer(t, "claude", "codex")
				writeHome(t, home, ".claude/CLAUDE.md", "# Personal\nBe brief.\n")
				writeHome(t, home, ".codex/AGENTS.md", "# Codex\n")
				createShared(t, s, "team")
				if action == "restore" || action == "mode" || action == "collect" || action == "guidance" || action == "setup" {
					rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude"],"extras":["team"]}`)
					if rr.Code != http.StatusOK {
						t.Fatalf("attach: %d %s", rr.Code, rr.Body)
					}
				}
				location := filepath.Join(home, "location")
				if action == "location-mode" || action == "location-remove" {
					body, _ := json.Marshal(map[string]string{"path": location, "mode": "copy"})
					addLocation(t, s, "team", string(body))
				}
				if action == "setup-delete" {
					rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/codex/instructions/setup", `{"path":"~/.codex/AGENTS.md"}`)
					if rr.Code != http.StatusOK {
						t.Fatalf("setup: %d %s", rr.Code, rr.Body)
					}
				}
				if remote {
					s.cfg.Extras[0].Remote, s.cfg.Extras[0].Branch = "example-org/ai-hub/instructions", "before"
					if err := s.cfg.Save(); err != nil {
						t.Fatal(err)
					}
				}
				method, path, body := http.MethodPut, "/api/instructions/team/content", `{"content":"# Updated shared\n"}`
				switch action {
				case "convert":
					method, path, body = http.MethodPost, "/api/targets/claude/instructions/convert", `{"method":"import","share_into":"team","apply":true}`
				case "assign":
					method, path, body = http.MethodPost, "/api/instructions/assign", `{"targets":["claude"],"extras":["team"]}`
				case "restore":
					method, path, body = http.MethodPost, "/api/instructions/team/restore", `{"target":"claude"}`
				case "mode":
					path, body = "/api/instructions/team/targets/claude/mode", `{"mode":"copy"}`
				case "collect":
					target := filepath.Join(home, ".claude", "CLAUDE.md")
					if err := os.Remove(target); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(target, []byte("# Collected\n"), 0644); err != nil {
						t.Fatal(err)
					}
					method, path, body = http.MethodPost, "/api/instructions/team/resolve", `{"target":"claude","action":"collect"}`
				case "location-add":
					method, path = http.MethodPost, "/api/instructions/team/locations"
					data, _ := json.Marshal(map[string]string{"path": location, "mode": "copy"})
					body = string(data)
				case "location-mode":
					path = "/api/instructions/team/locations/mode"
					data, _ := json.Marshal(map[string]string{"path": location, "mode": "append"})
					body = string(data)
				case "location-remove":
					method, path, body = http.MethodDelete, "/api/instructions/team/locations?path="+url.QueryEscape(location), ""
				case "guidance":
					plan := planGuidanceFor(t, s, `["claude"]`)
					if len(plan.Changes) == 0 {
						t.Fatal("no actual guidance change planned")
					}
					method, path, body = http.MethodPost, "/api/extras/memory/guidance/apply", `{"targets":["claude"],"token":"`+plan.Token+`"}`
				case "setup":
					path, body = "/api/targets/claude/instructions/setup", `{"path":"~/.claude/CLAUDE.md","import":true}`
				case "setup-delete":
					method, path, body = http.MethodDelete, "/api/targets/codex/instructions/setup", ""
				case "create":
					method, path, body = http.MethodPost, "/api/instructions", `{"name":"other","content":"# Other\n"}`
				}
				calls, releases := 0, 0
				old := lockRemoteExtraRoot
				lockRemoteExtraRoot = func(root string) (func(), error) {
					calls++
					if root != s.cfg.EffectiveExtrasSource() {
						t.Errorf("wrong root: %s", root)
					}
					fresh, err := config.LoadReadOnly()
					if err != nil {
						return nil, err
					}
					fresh.Extras[0].Branch = "published"
					if err := fresh.Save(); err != nil {
						return nil, err
					}
					return func() { releases++ }, nil
				}
				t.Cleanup(func() { lockRemoteExtraRoot = old })
				rr := instructionsRequest(t, s, method, path, body)
				wantStatus := http.StatusOK
				if remote && action == "guidance" {
					// The plan token includes the declaration. Publication while
					// waiting must invalidate it rather than apply a stale plan.
					wantStatus = http.StatusConflict
				}
				if rr.Code != wantStatus {
					t.Fatalf("mutation: %d %s", rr.Code, rr.Body)
				}
				want := 0
				if remote {
					want = 1
				}
				if calls != want || releases != want {
					t.Errorf("locks/releases=%d/%d want=%d", calls, releases, want)
				}
				if remote {
					fresh, err := config.LoadReadOnly()
					if err != nil {
						t.Fatal(err)
					}
					if fresh.Extras[0].Branch != "published" || s.cfg.Extras[0].Branch != "published" {
						t.Error("writer kept or saved pre-lock declaration")
					}
				}
				if action == "guidance" {
					extra, _ := s.sharedExtra("team")
					data, err := os.ReadFile(filepath.Join(s.extrasSourceDir(extra), extra.File))
					if err != nil || strings.Contains(string(data), "<!-- skillshare:memory") == remote {
						t.Errorf("guidance not written: %q %v", data, err)
					}
				}
			})
		}
	}
	t.Run("interrupted-unrelated-extra", func(t *testing.T) {
		for _, project := range []bool{false, true} {
			for _, state := range []string{"local", "remote-clean", "remote-interrupted"} {
				for _, action := range []string{"unrelated-shared", "affected-shared", "project-file"} {
					if action == "project-file" && !project {
						continue
					}
					t.Run(fmt.Sprintf("project=%t/%s/%s", project, state, action), func(t *testing.T) {
						extra := config.ExtraConfig{Name: "rules", File: "AGENTS.md", Targets: []config.ExtraTargetConfig{}}
						if state != "local" {
							extra.Remote, extra.Branch = "example-org/ai-hub/rules", "main"
						}
						var s *Server
						if project {
							s, _ = newTestProjectServerWithExtras(t, []config.ExtraConfig{extra})
						} else {
							s, _ = newTestServerWithExtras(t, []config.ExtraConfig{extra}, "")
						}
						createShared(t, s, "team")
						source := s.extrasSourceDir(extra)
						file := filepath.Join(source, extra.File)
						if err := os.WriteFile(file, []byte("remote candidate\n"), 0644); err != nil {
							t.Fatal(err)
						}
						marker := filepath.Join(filepath.Dir(source), ".skillshare-staging", "publishing-rules.json")
						var markerBytes []byte
						if state == "remote-interrupted" {
							var err error
							markerBytes, err = json.Marshal(map[string]any{"version": 1, "name": extra.Name, "remote": extra.Remote, "branch": extra.Branch, "commit": strings.Repeat("a", 40), "had_previous": false, "old": nil})
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
						before, err := os.ReadFile(s.configPath())
						if err != nil {
							t.Fatal(err)
						}
						path := "/api/instructions/team/content"
						written := filepath.Join(s.extrasSourceDir(config.ExtraConfig{Name: "team"}), "AGENTS.md")
						if action == "affected-shared" {
							path, written = "/api/instructions/rules/content", file
						} else if action == "project-file" {
							path, written = "/api/instructions/project", filepath.Join(s.projectRoot, "AGENTS.md")
						}
						calls, releases := 0, 0
						old := lockRemoteExtraRoot
						lockRemoteExtraRoot = func(root string) (func(), error) {
							calls++
							unlock, err := old(root)
							return func() {
								releases++
								if unlock != nil {
									unlock()
								}
							}, err
						}
						t.Cleanup(func() { lockRemoteExtraRoot = old })
						rr := instructionsRequest(t, s, http.MethodPut, path, `{"content":"updated unrelated file\n"}`)
						blocked := state == "remote-interrupted" && action == "affected-shared"
						want := http.StatusOK
						if blocked {
							want = http.StatusConflict
						}
						if rr.Code != want || blocked && !strings.Contains(rr.Body.String(), "interrupted") {
							t.Errorf("write: %d %s; want %d", rr.Code, rr.Body, want)
						}
						if !blocked {
							if data, err := os.ReadFile(written); err != nil || string(data) != "updated unrelated file\n" {
								t.Errorf("requested write missing: %q %v", data, err)
							}
						}
						if blocked || action != "affected-shared" {
							if data, err := os.ReadFile(file); err != nil || string(data) != "remote candidate\n" {
								t.Errorf("unrelated/blocked write changed remote bytes: %q %v", data, err)
							}
						}
						wantLocks := 1
						if state == "local" {
							wantLocks = 0
						}
						if calls != wantLocks || releases != wantLocks {
							t.Errorf("root serialization lost: %d/%d want %d", calls, releases, wantLocks)
						}
						if data, err := os.ReadFile(s.configPath()); err != nil || string(data) != string(before) {
							t.Errorf("content edit changed declaration: %q %v", data, err)
						}
						if markerBytes != nil {
							if data, err := os.ReadFile(marker); err != nil || string(data) != string(markerBytes) {
								t.Errorf("edit consumed unrelated recovery evidence: %q %v", data, err)
							}
						}
					})
				}
			}
		}
	})
}
