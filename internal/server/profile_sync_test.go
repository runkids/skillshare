package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/profile"
	"skillshare/internal/profileapply"
	ssync "skillshare/internal/sync"
)

func TestWriteProfileError_Transport(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{"locked", fmt.Errorf("wrapped: %w", &profileapply.Refusal{Code: profileapply.RefusalLocked, Reason: "busy"}), "profile_locked"},
		{"empty", fmt.Errorf("wrapped: %w", &profileapply.Refusal{Code: profileapply.RefusalEmpty, Reason: "empty"}), "profile_empty"},
		{"blocked", fmt.Errorf("wrapped: %w", &profileapply.Refusal{Code: profileapply.RefusalBlocked, Reason: "blocked"}), "profile_blocked"},
		{"generic", &profileapply.Refusal{Code: profileapply.RefusalGeneric, Reason: "generic"}, "profile_refused"},
		{"zero", &profileapply.Refusal{Reason: "profile_locked"}, "profile_refused"},
		{"unknown", &profileapply.Refusal{Code: "future_code", Reason: "profile_empty"}, "profile_refused"},
		{"untyped", fmt.Errorf("wrapped: %w", profileapply.ErrRefused), "profile_refused"},
		{"unavailable", fmt.Errorf("wrapped: %w", profile.ErrProfileUnavailable), "profile_unavailable"},
		{"unavailable_priority", errors.Join(&profileapply.Refusal{Code: profileapply.RefusalLocked, Reason: "busy"}, profile.ErrProfileUnavailable), "profile_unavailable"},
		{"unrelated", errors.New("profile_locked"), ""},
		{"nil", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, params := range []map[string]string{nil, {"target": "custom", "details": "blocked: alpha"}} {
				rr := httptest.NewRecorder()
				wantParams := maps.Clone(params)
				handled := writeProfileError(rr, tc.err, params)
				if tc.code == "" {
					if handled || rr.Code != http.StatusOK || rr.Body.Len() != 0 || len(rr.Header()) != 0 {
						t.Fatalf("unrelated error changed caller response: %v %+v", handled, rr)
					}
					continue
				}
				var out struct {
					Error  string            `json:"error"`
					Code   string            `json:"error_code"`
					Params map[string]string `json:"error_params"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				if !handled || rr.Code != http.StatusConflict || out.Code != tc.code || out.Error != tc.err.Error() || !maps.Equal(out.Params, wantParams) || !maps.Equal(params, wantParams) || (out.Params == nil) != (params == nil) {
					t.Fatalf("transport changed status/code/message/params: %v %d %s", handled, rr.Code, rr.Body.String())
				}
				if rr.Header().Get("Content-Type") != "application/json" {
					t.Fatal("transport lost JSON content type")
				}
			}
		})
	}
}

func TestProfiles_APISyncRefusalTransport(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, scenario := range []string{"locked", "unavailable", "legacy_discard"} {
			t.Run(fmt.Sprintf("project=%v/%s", project, scenario), func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"} {
					t.Setenv(key, t.TempDir())
				}
				for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
					t.Setenv(key, "")
				}
				dest := t.TempDir()
				var s *Server
				var source, path, text string
				if project {
					var root string
					s, root = newProjectTargetServer(t, []config.ProjectTargetEntry{{Name: "custom", Skills: &config.ResourceTargetConfig{Path: dest, Mode: "copy"}}})
					source, path = filepath.Join(root, ".skillshare", "skills"), config.ProjectConfigPath(root)
					text = "targets:\n  - name: custom\n    skills:\n      path: " + dest + "\n      mode: copy\n"
				} else {
					s, source = newTestServerWithTargets(t, map[string]string{"custom": dest})
					path = config.ConfigPath()
					text = "source: " + source + "\nmode: copy\ntargets:\n  custom:\n    skills:\n      path: " + dest + "\n"
				}
				addSkill(t, source, "alpha")
				if scenario != "legacy_discard" {
					text += "profiles:\n  all: {}\n"
				}
				if err := os.WriteFile(path, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
				statePath, lockPath, err := config.ProfilePaths(path)
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "unavailable" {
					if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "missing"}); err != nil {
						t.Fatal(err)
					}
				}
				if err := s.reloadConfig(); err != nil {
					t.Fatal(err)
				}
				body := `{"kind":"skill"}`
				wantCode := "profile_locked"
				wantMessage := fmt.Sprintf("another sync or profile switch is running for this config (lock %s): context deadline exceeded", lockPath)
				if scenario == "legacy_discard" {
					body = `{"kind":"skill","discardEdits":true}`
					wantCode, wantMessage = "profile_refused", "discardEdits requires an active profile or pending switch"
				} else {
					unlock, err := profile.Lock(context.Background(), lockPath)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(unlock)
					if scenario == "unavailable" {
						// Availability wins even when the real profile lock is held.
						err := s.activeProfileError()
						if err == nil {
							t.Fatal("fixture did not make the active profile unavailable")
						}
						wantCode, wantMessage = "profile_unavailable", err.Error()
					}
				}
				rr := httptest.NewRecorder()
				s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body)))
				var out struct {
					Error  string            `json:"error"`
					Code   string            `json:"error_code"`
					Params map[string]string `json:"error_params"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				if rr.Code != http.StatusConflict || out.Code != wantCode || out.Error != wantMessage || out.Params != nil {
					t.Errorf("refusal transport: %d %s; want 409 %s: %s, no params", rr.Code, rr.Body.String(), wantCode, wantMessage)
				}
				entries, err := os.ReadDir(dest)
				if err != nil || len(entries) != 0 {
					t.Fatalf("refused sync touched target: %v %v", entries, err)
				}
			})
		}
	}
}

func TestProfiles_APIOrdinarySyncAgreement(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"copy", "merge"} {
			t.Run(fmt.Sprintf("%v/%s", project, mode), func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
					t.Setenv(key, "")
				}
				dest := t.TempDir()
				var s *Server
				var source, path, text string
				scope := profileapply.Scope{}
				if project {
					var root string
					s, root = newProjectTargetServer(t, []config.ProjectTargetEntry{{Name: "custom", Skills: &config.ResourceTargetConfig{Path: dest, Mode: mode}}})
					source, path = filepath.Join(root, ".skillshare", "skills"), config.ProjectConfigPath(root)
					scope.ProjectRoot = root
					text = "targets:\n  - name: custom\n    skills:\n      path: " + dest + "\n      mode: " + mode + "\n"
				} else {
					s, source = newTestServerWithTargets(t, map[string]string{"custom": dest})
					path = config.ConfigPath()
					text = "source: " + source + "\nmode: " + mode + "\ntargets:\n  custom:\n    skills:\n      path: " + dest + "\n"
				}
				scope.ConfigPath = path
				addSkill(t, source, "alpha")
				addSkill(t, source, "beta")
				text += "profiles:\n  a:\n    skills:\n      include: [alpha]\n"
				if err := os.WriteFile(path, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := profileapply.Apply(context.Background(), profileapply.Request{Scope: scope, Name: "a", NoSync: true}); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 2; i++ {
					rr := httptest.NewRecorder()
					s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"kind":"skill"}`)))
					if rr.Code != http.StatusOK {
						t.Fatalf("sync: %d %s", rr.Code, rr.Body.String())
					}
					var out struct {
						Profile profileapply.Info `json:"profile"`
					}
					if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || out.Profile.Pending || out.Profile.Selected != 1 {
						t.Fatalf("profile result: %s %v", rr.Body.String(), err)
					}
					if _, err := os.Stat(filepath.Join(dest, "alpha", "SKILL.md")); err != nil {
						t.Fatal(err)
					}
					if _, err := os.Lstat(filepath.Join(dest, "beta")); !os.IsNotExist(err) {
						t.Fatal("profile not applied")
					}
				}
				rr := httptest.NewRecorder()
				s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/sync-matrix", nil))
				if rr.Code != http.StatusOK {
					t.Fatalf("matrix: %d %s", rr.Code, rr.Body.String())
				}
				var rows struct {
					Entries []syncMatrixEntry `json:"entries"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &rows); err != nil {
					t.Fatal(err)
				}
				selected := 0
				for _, row := range rows.Entries {
					if row.Kind != "agent" && row.Status == "synced" {
						selected++
						if row.Skill != "alpha" {
							t.Fatalf("matrix deployed wrong selection: %s", rr.Body.String())
						}
					}
				}
				if selected != 1 {
					t.Fatalf("matrix disagrees with sync: %s", rr.Body.String())
				}
			})
		}
	}
}

func TestProfiles_APISyncBacksUpBeforeRefresh(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
		t.Setenv(key, "")
	}
	s, source := newTestServerWithTargets(t, map[string]string{"custom": t.TempDir()})
	target := s.cfg.Targets["custom"]
	dest := target.SkillsConfig().Path
	addSkill(t, source, "alpha")
	path := config.ConfigPath()
	text := "source: " + source + "\nmode: copy\ntargets:\n  custom:\n    skills:\n      path: " + dest + "\nprofiles:\n  all: {}\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := profileapply.Apply(context.Background(), profileapply.Request{Scope: profileapply.Scope{ConfigPath: path}, Name: "all"}); err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(filepath.Join(dest, "alpha", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "alpha", "SKILL.md"), []byte("# changed source"), 0600); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"kind":"skill"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", rr.Code, rr.Body.String())
	}
	paths, err := filepath.Glob(filepath.Join(backup.BackupDir(), "*", "custom", "alpha", "SKILL.md"))
	if err != nil || len(paths) != 1 {
		t.Fatal("pre-sync backup missing", paths, err)
	}
	saved, err := os.ReadFile(paths[0])
	if err != nil || string(saved) != string(old) {
		t.Fatal("automatic backup captured refreshed content", err)
	}
	actual, err := os.ReadFile(filepath.Join(dest, "alpha", "SKILL.md"))
	if err != nil || string(actual) != "# changed source" {
		t.Fatalf("API sync did not refresh after backup: %q, %v", actual, err)
	}
}

func TestProfiles_APIDiscardUnchangedSource(t *testing.T) {
	for _, tc := range []struct {
		project, checksum, pending bool
	}{
		{},
		{checksum: true},
		{project: true},
		{project: true, checksum: true},
		{pending: true},
	} {
		t.Run(fmt.Sprintf("project=%v/checksum=%v/pending=%v", tc.project, tc.checksum, tc.pending), func(t *testing.T) {
			scenario, authored, active := "retained", "authored alpha", "all"
			if tc.pending {
				scenario, authored, active = "pending_none", "authored", ""
			}
			s, scope, dest, source := newProfileDiffFixture(t, tc.project, "copy", scenario)
			entry := filepath.Join(dest, "alpha", "SKILL.md")
			if tc.pending {
				if err := os.WriteFile(entry, []byte(authored), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(source, "alpha", "SKILL.md"), []byte("# new source"), 0600); err != nil {
					t.Fatal(err)
				}
				rr := httptest.NewRecorder()
				s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"kind":"skill"}`)))
				data, err := os.ReadFile(entry)
				if rr.Code != http.StatusOK || err != nil || string(data) != authored || !strings.Contains(rr.Body.String(), "kept_modified") {
					t.Fatalf("pending-none lost edit or report: %d %s, %q %v", rr.Code, rr.Body.String(), data, err)
				}
			}
			if tc.checksum {
				m, err := ssync.ReadManifest(dest)
				if err != nil {
					t.Fatal(err)
				}
				delete(m.Mtimes, "alpha")
				if err := ssync.WriteManifest(dest, m); err != nil {
					t.Fatal(err)
				}
			}
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"kind":"skill","discardEdits":true}`)))
			var out struct {
				Snapshot []profileapply.Snapshot
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || rr.Code != http.StatusOK || len(out.Snapshot) != 1 {
				t.Fatalf("discard: %d %s %v", rr.Code, rr.Body.String(), err)
			}
			saved, err := os.ReadFile(filepath.Join(out.Snapshot[0].Path, "alpha", "SKILL.md"))
			if err != nil || string(saved) != authored {
				t.Fatal("API snapshot lost authored bytes", err)
			}
			original, err := os.ReadFile(filepath.Join(source, "alpha", "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(entry)
			if err != nil || string(actual) != string(original) || (tc.pending && string(actual) != "# new source") {
				t.Fatalf("successful discard retained edit: %q %v", actual, err)
			}
			statePath, _, _ := config.ProfilePaths(scope.Path())
			state, err := profile.ReadState(statePath)
			if err != nil || state.Pending || state.Active != active {
				t.Fatal("API discard did not settle", state, err)
			}
		})
	}
}
