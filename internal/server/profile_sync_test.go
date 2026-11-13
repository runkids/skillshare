package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/profileapply"
)

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
					text = "targets:\n  - name: custom\n    skills:\n      path: "+dest+"\n      mode: "+mode+"\n"
				} else {
					s, source = newTestServerWithTargets(t, map[string]string{"custom": dest})
					path = config.ConfigPath()
					text = "source: "+source+"\nmode: "+mode+"\ntargets:\n  custom:\n    skills:\n      path: "+dest+"\n"
				}
				scope.ConfigPath = path
				addSkill(t, source, "alpha")
				addSkill(t, source, "beta")
				text += "profiles:\n  a:\n    skills:\n      include: [alpha]\n"
				if err := os.WriteFile(path, []byte(text), 0600); err != nil { t.Fatal(err) }
				if _, err := profileapply.Apply(context.Background(), profileapply.Request{Scope: scope, Name: "a", NoSync: true}); err != nil { t.Fatal(err) }
				for i := 0; i < 2; i++ {
					rr := httptest.NewRecorder()
					s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"kind":"skill"}`)))
					if rr.Code != http.StatusOK { t.Fatalf("sync: %d %s", rr.Code, rr.Body.String()) }
					var out struct { Profile profileapply.Info `json:"profile"` }
					if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || out.Profile.Pending || out.Profile.Selected != 1 { t.Fatalf("profile result: %s %v", rr.Body.String(), err) }
					if _, err := os.Stat(filepath.Join(dest, "alpha", "SKILL.md")); err != nil { t.Fatal(err) }
					if _, err := os.Lstat(filepath.Join(dest, "beta")); !os.IsNotExist(err) { t.Fatal("profile not applied") }
				}
				rr := httptest.NewRecorder()
				s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/sync-matrix", nil))
				if rr.Code != http.StatusOK { t.Fatalf("matrix: %d %s", rr.Code, rr.Body.String()) }
				var rows struct { Entries []syncMatrixEntry `json:"entries"` }
				if err := json.Unmarshal(rr.Body.Bytes(), &rows); err != nil { t.Fatal(err) }
				selected := 0
				for _, row := range rows.Entries {
					if row.Kind != "agent" && row.Status == "synced" {
						selected++
						if row.Skill != "alpha" { t.Fatalf("matrix deployed wrong selection: %s", rr.Body.String()) }
					}
				}
				if selected != 1 { t.Fatalf("matrix disagrees with sync: %s", rr.Body.String()) }
			})
		}
	}
}

func TestProfiles_APIDiscardAndPendingNone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} { t.Setenv(key, "") }
	dest := t.TempDir()
	s, source := newTestServerWithTargets(t, map[string]string{"custom": dest})
	addSkill(t, source, "alpha")
	path := config.ConfigPath()
	text := "source: "+source+"\nmode: copy\ntargets:\n  custom:\n    skills:\n      path: "+dest+"\nprofiles:\n  all: {}\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil { t.Fatal(err) }
	scope := profileapply.Scope{ConfigPath: path}
	if _, err := profileapply.Apply(context.Background(), profileapply.Request{Scope: scope, Name: "all"}); err != nil { t.Fatal(err) }
	if _, err := profileapply.Apply(context.Background(), profileapply.Request{Scope: scope, None: true, NoSync: true}); err != nil { t.Fatal(err) }
	edited := filepath.Join(dest, "alpha", "SKILL.md")
	if err := os.WriteFile(edited, []byte("authored"), 0600); err != nil { t.Fatal(err) }
	// Change source to require refresh during the pending clear-to-none retry.
	if err := os.WriteFile(filepath.Join(source, "alpha", "SKILL.md"), []byte("# new source"), 0600); err != nil { t.Fatal(err) }
	for _, discard := range []bool{false, true} {
		body := `{"kind":"skill"}`
		if discard { body = `{"kind":"skill","discardEdits":true}` }
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body)))
		if rr.Code != http.StatusOK { t.Fatalf("sync: %d %s", rr.Code, rr.Body.String()) }
		if !discard { bytes, _ := os.ReadFile(edited); if string(bytes) != "authored" || !strings.Contains(rr.Body.String(), "kept_modified") { t.Fatal("pending-none lost edit or report") } } else if !strings.Contains(rr.Body.String(), "snapshot") { t.Fatal("discard lacks snapshot") }
	}
}

func TestProfiles_APISyncBacksUpBeforeRefresh(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} { t.Setenv(key, "") }
	s, source := newTestServerWithTargets(t, map[string]string{"custom": t.TempDir()})
	dest := s.cfg.Targets["custom"].SkillsConfig().Path
	addSkill(t, source, "alpha")
	path := config.ConfigPath()
	text := "source: "+source+"\nmode: copy\ntargets:\n  custom:\n    skills:\n      path: "+dest+"\nprofiles:\n  all: {}\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil { t.Fatal(err) }
	if _, err := profileapply.Apply(context.Background(), profileapply.Request{Scope: profileapply.Scope{ConfigPath: path}, Name: "all"}); err != nil { t.Fatal(err) }
	old, err := os.ReadFile(filepath.Join(dest, "alpha", "SKILL.md"))
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(source, "alpha", "SKILL.md"), []byte("# changed source"), 0600); err != nil { t.Fatal(err) }
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"kind":"skill"}`)))
	if rr.Code != http.StatusOK { t.Fatalf("sync: %d %s", rr.Code, rr.Body.String()) }
	paths, err := filepath.Glob(filepath.Join(backup.BackupDir(), "*", "custom", "alpha", "SKILL.md"))
	if err != nil || len(paths) != 1 { t.Fatal("pre-sync backup missing", paths, err) }
	saved, err := os.ReadFile(paths[0])
	if err != nil || string(saved) != string(old) { t.Fatal("automatic backup captured refreshed content", err) }
}
