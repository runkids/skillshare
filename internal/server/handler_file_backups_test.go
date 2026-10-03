package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	syncpkg "skillshare/internal/sync"
)

// seedFileHistory backs up path's current content with reason, then writes
// next as the file's new content.
func seedFileHistory(t *testing.T, path, content, reason, next string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := syncpkg.BackupFile(path, reason); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(next), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestFileBackupsAPI_ListVersionsRestore(t *testing.T) {
	s, _ := newTestServer(t)
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	seedFileHistory(t, path, "# before edit\n", syncpkg.BackupReasonEdit, "# now\n")

	rr := serveJSON(t, s, http.MethodGet, "/api/file-backups", "")
	var list struct {
		Files []struct {
			Path     string `json:"path"`
			Versions int    `json:"versions"`
			Latest   string `json:"latest"`
		} `json:"files"`
	}
	json.Unmarshal(rr.Body.Bytes(), &list)
	if rr.Code != http.StatusOK || len(list.Files) != 1 || list.Files[0].Path != path || list.Files[0].Versions != 1 || list.Files[0].Latest == "" {
		t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
	}

	q := "?path=" + url.QueryEscape(path)
	rr = serveJSON(t, s, http.MethodGet, "/api/file-backups/versions"+q, "")
	var versions struct {
		Current  struct{ Exists bool } `json:"current"`
		Versions []struct {
			ID      string `json:"id"`
			Kind    string `json:"kind"`
			Reason  string `json:"reason"`
			Preview string `json:"preview"`
		} `json:"versions"`
	}
	json.Unmarshal(rr.Body.Bytes(), &versions)
	if rr.Code != http.StatusOK || !versions.Current.Exists || len(versions.Versions) != 1 {
		t.Fatalf("versions: %d %s", rr.Code, rr.Body.String())
	}
	v := versions.Versions[0]
	if v.Kind != "history" || v.Reason != "edit" || v.Preview != "# before edit" {
		t.Fatalf("version = %+v", v)
	}

	rr = serveJSON(t, s, http.MethodGet, "/api/file-backups/version"+q+"&id="+url.QueryEscape(v.ID), "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"content":"# before edit\n"`) || !strings.Contains(rr.Body.String(), `"current":"# now\n"`) {
		t.Fatalf("version: %d %s", rr.Code, rr.Body.String())
	}

	body, _ := json.Marshal(map[string]string{"path": path, "id": v.ID})
	rr = serveJSON(t, s, http.MethodPost, "/api/file-backups/restore", string(body))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"backup_id"`) {
		t.Fatalf("restore: %d %s", rr.Code, rr.Body.String())
	}
	if data, _ := os.ReadFile(path); string(data) != "# before edit\n" {
		t.Fatalf("content = %q", data)
	}
}

func TestFileBackupsAPI_Errors(t *testing.T) {
	s, _ := newTestServer(t)
	tmp := t.TempDir()
	path := filepath.Join(tmp, "CLAUDE.md")
	seedFileHistory(t, path, "old", syncpkg.BackupReasonEdit, "now")
	shared := filepath.Join(tmp, "AGENTS.md")
	os.WriteFile(shared, []byte("shared"), 0644)
	os.Remove(path)
	if err := os.Symlink(shared, path); err != nil {
		t.Fatal(err)
	}
	versions, _ := syncpkg.FileBackupVersions(path)

	body, _ := json.Marshal(map[string]string{"path": path, "id": versions[0].ID})
	rr := serveJSON(t, s, http.MethodPost, "/api/file-backups/restore", string(body))
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), `"file_backup_is_link"`) {
		t.Fatalf("link: %d %s", rr.Code, rr.Body.String())
	}

	rr = serveJSON(t, s, http.MethodGet, "/api/file-backups/versions?path="+url.QueryEscape(filepath.Join(tmp, "other.md")), "")
	if rr.Code != http.StatusNotFound || !strings.Contains(rr.Body.String(), `"file_backup_not_found"`) {
		t.Fatalf("unknown path: %d %s", rr.Code, rr.Body.String())
	}
	rr = serveJSON(t, s, http.MethodGet, "/api/file-backups/version?path="+url.QueryEscape(path)+"&id=../../etc", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("bad id: %d %s", rr.Code, rr.Body.String())
	}
}

func TestFileBackupsAPI_ProjectModeHidesOutsideFiles(t *testing.T) {
	s, projectRoot := newTestProjectServerWithExtras(t, nil)
	inside := filepath.Join(projectRoot, "CLAUDE.md")
	outside := filepath.Join(t.TempDir(), "CLAUDE.md")
	seedFileHistory(t, inside, "in", syncpkg.BackupReasonShim, "in2")
	seedFileHistory(t, outside, "out", syncpkg.BackupReasonEdit, "out2")

	rr := serveJSON(t, s, http.MethodGet, "/api/file-backups", "")
	if !strings.Contains(rr.Body.String(), inside) || strings.Contains(rr.Body.String(), outside) {
		t.Fatalf("list: %s", rr.Body.String())
	}
	rr = serveJSON(t, s, http.MethodGet, "/api/file-backups/versions?path="+url.QueryEscape(outside), "")
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `"file_backup_outside_project"`) {
		t.Fatalf("outside: %d %s", rr.Code, rr.Body.String())
	}
}

func TestFileBackupsAPI_ProjectExternalMemorySource(t *testing.T) {
	root := filepath.Join(t.TempDir(), "memory")
	s, _ := newTestProjectServerWithExtras(t, []config.ExtraConfig{{Name: "memory"}})
	s.projectCfg.Sources.Extras = filepath.Dir(root)
	if err := s.saveConfig(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "wiki", "note.md")
	outside := filepath.Join(filepath.Dir(root), "unrelated.md")
	seedFileHistory(t, path, "old memory", syncpkg.BackupReasonEdit, "current memory")
	seedFileHistory(t, outside, "old unrelated", syncpkg.BackupReasonEdit, "current unrelated")
	rr := serveJSON(t, s, http.MethodGet, "/api/file-backups", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), path) || strings.Contains(rr.Body.String(), outside) {
		t.Fatalf("list: %d %s", rr.Code, rr.Body)
	}
	rr = serveJSON(t, s, http.MethodGet, "/api/file-backups/versions?path="+url.QueryEscape(path), "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "old memory") {
		t.Fatalf("versions: %d %s", rr.Code, rr.Body)
	}
	versions, err := syncpkg.FileBackupVersions(path)
	if err != nil || len(versions) != 1 {
		t.Fatalf("versions: %+v, %v", versions, err)
	}
	body, _ := json.Marshal(map[string]string{"path": path, "id": versions[0].ID})
	rr = serveJSON(t, s, http.MethodPost, "/api/file-backups/restore", string(body))
	if rr.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", rr.Code, rr.Body)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "old memory" {
		t.Fatalf("content: %q, %v", data, err)
	}
	rr = serveJSON(t, s, http.MethodGet, "/api/file-backups/versions?path="+url.QueryEscape(outside), "")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("outside: %d %s", rr.Code, rr.Body)
	}
}

func TestFileBackupsAPI_ProjectUnconfiguredExternalMemorySource(t *testing.T) {
	s, projectRoot := newTestProjectServerWithExtras(t, nil)
	s.projectCfg.Sources.Extras = t.TempDir()
	if err := s.saveConfig(); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(projectRoot, "AGENTS.md")
	path := filepath.Join(s.projectCfg.Sources.Extras, "memory", "wiki", "note.md")
	seedFileHistory(t, inside, "old rules", syncpkg.BackupReasonEdit, "current rules")
	seedFileHistory(t, path, "old external", syncpkg.BackupReasonEdit, "current external")
	versions, err := syncpkg.FileBackupVersions(path)
	if err != nil || len(versions) != 1 {
		t.Fatalf("versions: %+v, %v", versions, err)
	}
	rr := serveJSON(t, s, http.MethodGet, "/api/file-backups", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), inside) || strings.Contains(rr.Body.String(), path) {
		t.Fatalf("list: %d %s", rr.Code, rr.Body)
	}
	q := "?path=" + url.QueryEscape(path)
	for _, route := range []string{"/api/file-backups/versions" + q, "/api/file-backups/version" + q + "&id=" + url.QueryEscape(versions[0].ID)} {
		rr = serveJSON(t, s, http.MethodGet, route, "")
		if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `"file_backup_outside_project"`) {
			t.Errorf("%s: %d %s", route, rr.Code, rr.Body)
		}
	}
	body, _ := json.Marshal(map[string]string{"path": path, "id": versions[0].ID})
	rr = serveJSON(t, s, http.MethodPost, "/api/file-backups/restore", string(body))
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `"file_backup_outside_project"`) {
		t.Errorf("restore: %d %s", rr.Code, rr.Body)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "current external" {
		t.Fatalf("external content: %q, %v", data, err)
	}
}
