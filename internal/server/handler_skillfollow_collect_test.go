package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/trash"
)

// followedTeam declares team in source and, when live, links it to an external
// tree holding one skill. It returns the external directory.
func followedTeam(t *testing.T, source string, live bool) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(source, ".skillfollow"), []byte("team\n"), 0644); err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	addSkill(t, external, "a")
	if live {
		if err := os.Symlink(external, filepath.Join(source, "team")); err != nil {
			t.Fatal(err)
		}
	}
	return external
}

func assertTeamUntouched(t *testing.T, source, external string, live bool) {
	t.Helper()
	info, err := os.Lstat(filepath.Join(source, "team"))
	if !live {
		if !os.IsNotExist(err) {
			t.Errorf("request created the declared entry: %v", err)
		}
		return
	}
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the declared link was replaced: %v %v", info, err)
	}
	if entries, _ := os.ReadDir(external); len(entries) != 1 {
		t.Errorf("external tree changed: %v", entries)
	}
}

func TestHandleCollect_RefusesDeclaredEntry(t *testing.T) {
	for _, live := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "live"}[live], func(t *testing.T) {
			tgtPath := filepath.Join(t.TempDir(), "claude-skills")
			s, source := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
			external := followedTeam(t, source, live)
			addSkill(t, tgtPath, "team")
			addSkill(t, tgtPath, "keep")

			body := `{"skills":[{"name":"team","targetName":"claude"},{"name":"keep","targetName":"claude"}],"force":true}`
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(body)))
			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
			}
			var resp struct {
				Pulled []string          `json:"pulled"`
				Failed map[string]string `json:"failed"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if want := filepath.Join(source, "team") + " is a link; edit its target directly"; !strings.Contains(resp.Failed["team"], want) {
				t.Errorf("failed[team] = %q, want %q", resp.Failed["team"], want)
			}
			if !slices.Equal(resp.Pulled, []string{"keep"}) {
				t.Errorf("pulled = %v, want [keep]", resp.Pulled)
			}
			assertTeamUntouched(t, source, external, live)
		})
	}
}

func TestHandleRestoreTrash_RefusesDeclaredEntry(t *testing.T) {
	for _, live := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "live"}[live], func(t *testing.T) {
			s, source := newTestServer(t)
			external := followedTeam(t, source, live)
			trashed := filepath.Join(t.TempDir(), "team")
			addSkill(t, filepath.Dir(trashed), "team")
			if _, err := trash.MoveToTrash(trashed, "team", s.trashBase()); err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/trash/team/restore", nil))
			if rr.Code != http.StatusConflict {
				t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), "is a link; edit its target directly") {
				t.Errorf("missing link error: %s", rr.Body.String())
			}
			if trash.FindByName(s.trashBase(), "team") == nil {
				t.Error("trash entry should stay after a refused restore")
			}
			assertTeamUntouched(t, source, external, live)
		})
	}
}

// An unreadable declaration may hide any entry, so dashboard writes that could
// create one are refused with the read error, like an unreadable update.
func TestHandleWrites_RefuseUnreadableDeclaration(t *testing.T) {
	local := filepath.Join(t.TempDir(), "team")
	addSkill(t, filepath.Dir(local), "team")
	for _, tc := range []struct {
		name, method, path, body string
		status                   int
		setup                    func(t *testing.T, s *Server, target string)
	}{
		{name: "create", method: http.MethodPost, path: "/api/resources", body: `{"name":"team","pattern":"none"}`, status: http.StatusInternalServerError},
		{name: "create-into", method: http.MethodPost, path: "/api/resources", body: `{"name":"skill","into":"team","pattern":"none"}`, status: http.StatusInternalServerError},
		{name: "install", method: http.MethodPost, path: "/api/install", body: `{"source":"` + local + `","skipAudit":true}`, status: http.StatusInternalServerError},
		{name: "install-into", method: http.MethodPost, path: "/api/install", body: `{"source":"` + local + `","into":"team","skipAudit":true}`, status: http.StatusInternalServerError},
		{name: "collect", method: http.MethodPost, path: "/api/collect", body: `{"skills":[{"name":"team","targetName":"claude"}],"force":true}`, status: http.StatusOK,
			setup: func(t *testing.T, _ *Server, target string) { addSkill(t, target, "team") }},
		{name: "restore", method: http.MethodPost, path: "/api/trash/team/restore", status: http.StatusInternalServerError,
			setup: func(t *testing.T, s *Server, _ string) {
				trashed := filepath.Join(t.TempDir(), "team")
				addSkill(t, filepath.Dir(trashed), "team")
				if _, err := trash.MoveToTrash(trashed, "team", s.trashBase()); err != nil {
					t.Fatal(err)
				}
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "claude-skills")
			s, source := newTestServerWithTargets(t, map[string]string{"claude": target})
			if err := os.Mkdir(filepath.Join(source, ".skillfollow"), 0755); err != nil {
				t.Fatal(err)
			}
			if tc.setup != nil {
				tc.setup(t, s, target)
			}
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if rr.Code != tc.status || !strings.Contains(rr.Body.String(), "read skillfollow declaration") {
				t.Errorf("got %d %s, want %d with the declaration read error", rr.Code, rr.Body.String(), tc.status)
			}
			if _, err := os.Lstat(filepath.Join(source, "team")); !os.IsNotExist(err) {
				t.Errorf("request created a possibly declared entry: %v", err)
			}
		})
	}
}
