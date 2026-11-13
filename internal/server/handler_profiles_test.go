package server

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/profile"
	"skillshare/internal/profileapply"
)

type profileRouteFixture struct {
	server                    *Server
	source, path, target, root string
	scope                     profileapply.Scope
	roots                     []string
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
					} else if out.Selection.Previous != "all" || !slices.Equal(out.Targets[0].Removed, []string{"beta", "team__review"}) {
						t.Fatalf("existing target preview: %s", rr.Body.String())
					}
				}
			})
		}
	}
}
