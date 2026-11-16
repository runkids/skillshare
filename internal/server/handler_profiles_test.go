package server

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/profile"
	"skillshare/internal/profileapply"
	ssync "skillshare/internal/sync"
)

type profileRouteFixture struct {
	server                     *Server
	source, path, target, root string
	scope                      profileapply.Scope
	roots                      []string
}

type profileRouteReply struct {
	profileapply.Result
	Error  string            `json:"error"`
	Code   string            `json:"error_code"`
	Params map[string]string `json:"error_params"`
}

func profilePost(t *testing.T, f profileRouteFixture, body string, status int) profileRouteReply {
	t.Helper()
	rr := projectRequest(t, f.server, http.MethodPost, "/api/profiles/active", body)
	var out profileRouteReply
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || rr.Code != status {
		t.Fatalf("profile POST: %d %s, want %d: %v", rr.Code, rr.Body.String(), status, err)
	}
	return out
}

func profileFixtureWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}

func profileFixtureTarget(t *testing.T, f profileRouteFixture, name, kind, mode, extra string) string {
	t.Helper()
	path := filepath.Join(f.roots[0], name)
	data, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if kind == "agents" {
		row := fmt.Sprintf("    agents:\n      path: %s\n      mode: %s\n    skills:\n", path, mode)
		profileFixtureWrite(t, f.path, strings.Replace(string(data), "    skills:\n", row, 1))
		return path
	}
	row := fmt.Sprintf("  %s:\n    %s:\n      path: %s\n      mode: %s\n%s", name, kind, path, mode, extra)
	if f.scope.ProjectRoot != "" {
		row = fmt.Sprintf("  - name: %s\n    %s:\n      path: %s\n      mode: %s\n%s", name, kind, path, mode, extra)
	}
	profileFixtureWrite(t, f.path, strings.Replace(string(data), "profiles:\n", row+"profiles:\n", 1))
	return path
}

func TestProfilesAPI_Refusals(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, tc := range []struct {
			body string
			code string
		}{
			{`{"name":"empty"}`, "profile_empty"},
			{`{"name":"missing"}`, "profile_unavailable"},
			{`{"name":"broken"}`, "profile_unavailable"},
			{`{"name":""}`, "profile_refused"},
		} {
			t.Run(fmt.Sprintf("project=%v/%s", project, tc.body), func(t *testing.T) {
				f := newProfileRouteFixture(t, project)
				if _, err := profileapply.Apply(context.Background(), profileapply.Request{Scope: f.scope, Name: "all"}); err != nil {
					t.Fatal(err)
				}
				before := profileRouteTree(t, f.roots)
				out := profilePost(t, f, tc.body, http.StatusConflict)
				if out.Code != tc.code || out.Error == "" || !maps.Equal(before, profileRouteTree(t, f.roots)) {
					t.Fatalf("refusal lost code/message or changed retained state: %+v", out)
				}
			})
		}
	}
}

func TestProfilesAPI_LockedAndInvalidJSON(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newProfileRouteFixture(t, project)
			for _, body := range []string{`{`, `{}`, `{"name":12}`} {
				profilePost(t, f, body, http.StatusBadRequest)
			}
			_, lockPath, err := config.ProfilePaths(f.path)
			if err != nil {
				t.Fatal(err)
			}
			unlock, err := profile.Lock(context.Background(), lockPath)
			if err != nil {
				t.Fatal(err)
			}
			defer unlock()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			rr := httptest.NewRecorder()
			f.server.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/profiles/active", strings.NewReader(`{"name":"work"}`)).WithContext(ctx))
			var out profileRouteReply
			if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || rr.Code != http.StatusConflict || out.Code != "profile_locked" {
				t.Fatalf("lock refusal: %d %s %v", rr.Code, rr.Body.String(), err)
			}
		})
	}
}

func TestProfilesAPI_ApplyEmptyAndNone(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newProfileRouteFixture(t, project)
			for i, body := range []string{`{"name":"work"}`, `{"name":"empty","allowEmpty":true}`, `{"name":null}`} {
				out := profilePost(t, f, body, http.StatusOK)
				want := []string{"work", "empty", ""}[i]
				v, err := profileapply.Load(f.scope)
				if err != nil || !out.Complete || out.Selection.Active != want || out.Selection.Pending || v.State.Active != want || v.State.Pending {
					t.Fatalf("apply/settle: %+v %v", out, err)
				}
				entries, err := oplog.Read(f.path, oplog.OpsFile, 100)
				if err != nil || len(entries) != i+1 || entries[0].Command != "profile use" {
					t.Fatalf("one shared oplog per switch: %+v %v", entries, err)
				}
			}
			for _, name := range []string{"alpha", "beta", "team__review"} {
				if _, err := os.Stat(filepath.Join(f.target, name, "SKILL.md")); err != nil {
					t.Fatalf("null did not restore pass-through selection: %s: %v", name, err)
				}
			}
		})
	}
}

func TestProfilesAPI_ReplansOnApply(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newProfileRouteFixture(t, project)
			profilePost(t, f, `{"name":"all"}`, http.StatusOK)
			preview := profilePost(t, f, `{"name":"work","dryRun":true}`, http.StatusOK)
			if len(preview.Blocked) != 0 || len(preview.Targets) != 1 || !slices.Contains(preview.Targets[0].Removed, "beta") {
				t.Fatal("preview did not plan clean departure")
			}
			edited := filepath.Join(f.target, "beta", "SKILL.md")
			profileFixtureWrite(t, edited, "authored after preview")
			before := profileRouteTree(t, f.roots)
			refused := profilePost(t, f, `{"name":"work"}`, http.StatusConflict)
			var blocked []ssync.EntryChange
			if err := json.Unmarshal([]byte(refused.Params["blocked"]), &blocked); err != nil || refused.Code != "profile_blocked" || len(blocked) != 1 || blocked[0].Target != "custom" || blocked[0].Entry != "beta" || blocked[0].Reason == "" || !maps.Equal(before, profileRouteTree(t, f.roots)) {
				t.Fatalf("stale preview was trusted or blocked params lost: %+v %v", refused, err)
			}
			out := profilePost(t, f, `{"name":"work","discardEdits":true}`, http.StatusOK)
			if !out.Complete || out.Selection.Pending || len(out.Snapshot) != 1 || out.Snapshot[0].Restore == "" || out.Snapshot[0].Retention == "" {
				t.Fatalf("discard missing restorable snapshot: %+v", out)
			}
			saved, err := os.ReadFile(filepath.Join(out.Snapshot[0].Path, "beta", "SKILL.md"))
			if err != nil || string(saved) != "authored after preview" {
				t.Fatal("snapshot did not capture edited bytes", err)
			}
			if _, err := os.Lstat(edited); !os.IsNotExist(err) {
				t.Fatal("confirmed departure not removed", err)
			}
			stamp := filepath.Base(filepath.Dir(out.Snapshot[0].Path))
			body, err := json.Marshal(map[string]any{"timestamp": stamp, "target": "custom", "force": true})
			if err != nil {
				t.Fatal(err)
			}
			rr := projectRequest(t, f.server, http.MethodPost, "/api/restore", string(body))
			restored, err := os.ReadFile(edited)
			if rr.Code != http.StatusOK || err != nil || string(restored) != "authored after preview" {
				t.Fatalf("actual restore: %d %s %v", rr.Code, rr.Body.String(), err)
			}
		})
	}
}

func TestProfilesAPI_IncompleteAndScopeIsolation(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newProfileRouteFixture(t, project)
			protected := []string{f.source}
			view, err := profileapply.Load(f.scope)
			if err != nil {
				t.Fatal(err)
			}
			agentsSource := view.Config.EffectiveAgentsSource()
			profileFixtureWrite(t, filepath.Join(agentsSource, "guide.md"), "# source agent")
			protected = append(protected, agentsSource)
			for _, tc := range []struct{ name, kind, mode, extra string }{
				{"off", "skills", "copy", "      enabled: false\n"},
				{"whole", "skills", "symlink", ""},
				{"agents", "agents", "copy", ""},
			} {
				path := profileFixtureTarget(t, f, tc.name, tc.kind, tc.mode, tc.extra)
				profileFixtureWrite(t, filepath.Join(path, "sentinel"), tc.name)
				protected = append(protected, path)
			}
			if !project {
				root := filepath.Join(f.roots[0], "registered")
				addProject(t, f.server, root)
				protected = append(protected, root)
			}
			bad := profileFixtureTarget(t, f, "bad", "skills", "copy", "")
			if err := os.Symlink(filepath.Join(f.roots[0], "missing-parent", "dest"), bad); err != nil {
				t.Fatal(err)
			}
			before := profileRouteTree(t, protected)
			out := profilePost(t, f, `{"name":"work"}`, http.StatusOK)
			v, err := profileapply.Load(f.scope)
			if err != nil || out.Complete || !out.Selection.Pending || v.State.Active != "work" || !v.State.Pending || !slices.Equal(out.SymlinkTargets, []string{"whole"}) {
				t.Fatalf("incomplete transport lost pending/scope: %+v %v", out, err)
			}
			states := make(map[string]string)
			for _, row := range out.Targets {
				states[row.Name] = row.State
			}
			if len(states) != 2 || states["custom"] != "applied" || states["bad"] != "failed" || !maps.Equal(before, profileRouteTree(t, protected)) {
				t.Fatalf("switch expanded beyond own skills or lost partial work: %+v", out)
			}
			if _, err := os.Stat(filepath.Join(f.target, "alpha", "SKILL.md")); err != nil {
				t.Fatal("healthy target did not apply", err)
			}
		})
	}
}

func newProfileRouteFixture(t *testing.T, project bool) profileRouteFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(home, "global.yaml"))
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"} {
		t.Setenv(key, filepath.Join(home, key))
	}
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
		t.Setenv(key, "")
	}
	f := profileRouteFixture{target: filepath.Join(home, "target")}
	var raw string
	if project {
		f.server, f.root = newProjectTargetServer(t, []config.ProjectTargetEntry{{Name: "custom", Skills: &config.ResourceTargetConfig{Path: f.target, Mode: "copy"}}})
		f.source, f.path = filepath.Join(f.root, ".skillshare", "skills"), config.ProjectConfigPath(f.root)
		f.scope.ProjectRoot = f.root
		raw = "targets:\n  - name: custom\n    skills:\n      path: " + f.target + "\n      mode: copy\n"
	} else {
		f.server, f.source = newTestServerWithTargets(t, map[string]string{"custom": f.target})
		f.root, f.path = filepath.Dir(f.source), config.ConfigPath()
		raw = "source: " + f.source + "\nmode: copy\ntargets:\n  custom:\n    skills:\n      path: " + f.target + "\n"
	}
	f.scope.ConfigPath = f.path
	f.roots = []string{home, f.root}
	for _, name := range []string{"alpha", "beta", "team/review", "ignored"} {
		addSkill(t, f.source, name)
	}
	if err := os.WriteFile(filepath.Join(f.source, ".skillignore"), []byte("ignored\n"), 0600); err != nil {
		t.Fatal(err)
	}
	raw += "profiles:\n  all: {}\n  broken:\n    future: true\n  empty:\n    skills:\n      include: []\n  work:\n    description: Focus\n    skills:\n      include: [alpha, missing-*]\n      exclude: [beta]\n"
	if err := os.WriteFile(f.path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	return f
}

// Capture directories as well as bytes and links, so a preview cannot create
// a lock, backup directory or other retained path unnoticed.
func profileRouteTree(t *testing.T, roots []string) map[string]string {
	t.Helper()
	out := make(map[string]string)
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			value := info.Mode().String()
			if info.Mode()&os.ModeSymlink != 0 {
				link, err := os.Readlink(path)
				if err != nil {
					return err
				}
				value += ":" + link
			} else if !entry.IsDir() {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				value += ":" + string(data)
			}
			out[path] = value
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestProfilesAPI_List(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, active := range []string{"", "work", "missing", "broken"} {
			t.Run(fmt.Sprintf("project=%v/active=%s", project, active), func(t *testing.T) {
				f := newProfileRouteFixture(t, project)
				statePath, _, err := config.ProfilePaths(f.path)
				if err != nil {
					t.Fatal(err)
				}
				if active != "" {
					if err := profile.WriteState(statePath, profile.State{Version: 1, Active: active, Pending: true}); err != nil {
						t.Fatal(err)
					}
				}
				before := profileRouteTree(t, f.roots)
				rr := projectRequest(t, f.server, http.MethodGet, "/api/profiles", "")
				if !maps.Equal(before, profileRouteTree(t, f.roots)) {
					t.Error("profile list changed retained files or directories")
				}
				if rr.Code != http.StatusOK {
					t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
				}
				var out struct {
					Active   string `json:"active"`
					State    string `json:"state"`
					Pending  *bool  `json:"pending"`
					Profiles []struct {
						Name        string          `json:"name"`
						Description string          `json:"description"`
						Skills      json.RawMessage `json:"skills"`
						Selected    int             `json:"selected"`
						Unmatched   []string        `json:"unmatched"`
						Usable      bool            `json:"usable"`
						Reason      string          `json:"reason"`
					} `json:"profiles"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				wantState := "active"
				if active == "" {
					wantState = "none"
				} else if active == "missing" || active == "broken" {
					wantState = "unavailable"
				}
				if out.Active != active || out.State != wantState || out.Pending == nil || *out.Pending != (active != "") || len(out.Profiles) != 4 {
					t.Fatalf("list lost state/catalog: %s", rr.Body.String())
				}
				seen := make(map[string]bool)
				for _, row := range out.Profiles {
					if seen[row.Name] {
						t.Fatalf("duplicate profile %q", row.Name)
					}
					seen[row.Name] = true
					var selection *profile.SkillSelection
					if err := json.Unmarshal(row.Skills, &selection); err != nil {
						t.Fatalf("profile %s must include skills as an object or null: %v", row.Name, err)
					}
					switch row.Name {
					case "all":
						if !row.Usable || string(row.Skills) != "null" || row.Selected != 3 {
							t.Errorf("pass-through must be null and select enabled inventory: %+v", row)
						}
					case "empty":
						if !row.Usable || selection == nil || len(selection.Include) != 0 || row.Selected != 0 {
							t.Errorf("explicit empty lost its presence: %+v", row)
						}
					case "work":
						if !row.Usable || row.Description != "Focus" || row.Selected != 1 || selection == nil || !slices.Equal(selection.Include, []string{"alpha", "missing-*"}) || !slices.Equal(selection.Exclude, []string{"beta"}) || !slices.Equal(row.Unmatched, []string{"missing-*"}) {
							t.Errorf("desired selection/patterns lost: %+v", row)
						}
					case "broken":
						if row.Usable || row.Reason == "" || row.Selected != 0 {
							t.Errorf("unusable profile hidden or treated as usable: %+v", row)
						}
					default:
						t.Errorf("unexpected profile %q", row.Name)
					}
				}
			})
		}
	}
}

func TestProfiles_ReadOnlyRoutes(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, route := range []string{"list", "preview", "fresh-preview"} {
			t.Run(fmt.Sprintf("project=%v/%s", project, route), func(t *testing.T) {
				f := newProfileRouteFixture(t, project)
				if route != "fresh-preview" {
					if _, err := profileapply.Apply(context.Background(), profileapply.Request{Scope: f.scope, Name: "all"}); err != nil {
						t.Fatal(err)
					}
				}
				if project {
					data, err := os.ReadFile(f.path)
					if err != nil {
						t.Fatal(err)
					}
					legacy := strings.Replace(string(data), "    skills:\n      path: "+f.target+"\n      mode: copy\n", "    path: "+f.target+"\n    mode: copy\n", 1)
					if legacy == string(data) {
						t.Fatal("fixture failed to create legacy project target")
					}
					if err := os.WriteFile(f.path, []byte(legacy), 0600); err != nil {
						t.Fatal(err)
					}
				}
				before := profileRouteTree(t, f.roots)
				method, path, body := http.MethodGet, "/api/profiles", ""
				if route != "list" {
					method, path, body = http.MethodPost, "/api/profiles/active", `{"name":"work","dryRun":true}`
				}
				rr := projectRequest(t, f.server, method, path, body)
				if !maps.Equal(before, profileRouteTree(t, f.roots)) {
					t.Error("read/preview changed config, state, manifests, source, targets or retained paths")
				}
				if rr.Code != http.StatusOK {
					t.Fatalf("%s: %d %s", route, rr.Code, rr.Body.String())
				}
				if route != "list" {
					var fields map[string]json.RawMessage
					if err := json.Unmarshal(rr.Body.Bytes(), &fields); err != nil {
						t.Fatal(err)
					}
					for _, key := range []string{"profile", "complete", "targets", "blocked", "symlink_targets", "snapshot"} {
						if _, ok := fields[key]; !ok {
							t.Errorf("preview missing required field %q: %s", key, rr.Body.String())
						}
					}
					var out profileapply.Result
					if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
						t.Fatal(err)
					}
					if out.Selection.Active != "work" || out.Selection.Pending || len(out.Targets) != 1 || out.Targets[0].Name != "custom" {
						t.Fatalf("preview did not describe candidate removal without applying: %s", rr.Body.String())
					}
					if route == "fresh-preview" {
						if out.Selection.Previous != "" || !slices.Equal(out.Targets[0].Added, []string{"alpha"}) || len(out.Targets[0].Removed) != 0 {
							t.Fatalf("fresh preview: %s", rr.Body.String())
						}
					} else if out.Selection.Previous != "all" || !slices.Equal(slices.Sorted(slices.Values(out.Targets[0].Removed)), []string{"beta", "team__review"}) {
						t.Fatalf("existing target preview: %s", rr.Body.String())
					}
				}
			})
		}
	}
}
