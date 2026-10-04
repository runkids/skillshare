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
