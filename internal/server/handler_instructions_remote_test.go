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
}
