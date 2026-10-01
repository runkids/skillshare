package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
)

func TestHandleListBackups_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	// Isolate backup directory via XDG_DATA_HOME to avoid reading real system backups
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))
	req := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Backups []any `json:"backups"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Backups) != 0 {
		t.Errorf("expected 0 backups, got %d", len(resp.Backups))
	}
}

func TestHandleListBackups_TotalIsSumOfSizes(t *testing.T) {
	s, _ := newTestServer(t)
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))
	seedSnapshot(t, s.backupDir(), "2026-01-01_00-00-00", "claude", "a.md", "12345")
	seedSnapshot(t, s.backupDir(), "2026-01-02_00-00-00", "claude", "b.md", "1234567")

	rr := serveJSON(t, s, http.MethodGet, "/api/backups", "")
	var resp struct {
		Backups []struct {
			SizeBytes int64 `json:"sizeBytes"`
		} `json:"backups"`
		TotalSizeBytes int64 `json:"totalSizeBytes"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v: %s", err, rr.Body.String())
	}
	if len(resp.Backups) != 2 || resp.TotalSizeBytes != 12 {
		t.Fatalf("want 2 backups totalling 12 bytes, got %d totalling %d", len(resp.Backups), resp.TotalSizeBytes)
	}
}

func TestHandleListBackups_EntriesReportFilesAndSize(t *testing.T) {
	s, _ := newTestServer(t)
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))
	seedSnapshot(t, s.backupDir(), "2026-01-01_00-00-00", "claude", "a.md", "12345")
	seedSnapshot(t, s.backupDir(), "2026-01-01_00-00-00", "claude", "b.md", "12")
	seedSnapshot(t, s.backupDir(), "2026-01-01_00-00-00", "claude-agents", "c.md", "1")

	rr := serveJSON(t, s, http.MethodGet, "/api/backups", "")
	var resp struct {
		Backups []struct {
			Entries []struct {
				Name      string `json:"name"`
				SizeBytes int64  `json:"sizeBytes"`
				Files     int    `json:"files"`
			} `json:"entries"`
		} `json:"backups"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v: %s", err, rr.Body.String())
	}
	got := map[string][2]int64{}
	for _, e := range resp.Backups[0].Entries {
		got[e.Name] = [2]int64{int64(e.Files), e.SizeBytes}
	}
	want := map[string][2]int64{"claude": {2, 7}, "claude-agents": {1, 1}}
	if len(got) != 2 || got["claude"] != want["claude"] || got["claude-agents"] != want["claude-agents"] {
		t.Fatalf("entries = %v, want %v", got, want)
	}
}

// seedSnapshot writes <backupDir>/<ts>/<entry>/<file> with content.
func seedSnapshot(t *testing.T, backupDir, ts, entry, file, content string) {
	t.Helper()
	dir := filepath.Join(backupDir, ts, entry)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func serveJSON(t *testing.T, s *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	return rr
}

// Regression: agents snapshots ("<target>-agents") were looked up as targets
// and failed with "target not found".
func TestHandleRestore_AgentsSnapshotRestoresAgentsDir(t *testing.T) {
	skillsPath := filepath.Join(t.TempDir(), "claude-skills")
	agentPath := filepath.Join(t.TempDir(), "claude-agents")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": skillsPath})
	raw := "source: " + sourceDir + "\nmode: merge\ntargets:\n  claude:\n    skills:\n      path: " + skillsPath +
		"\n    agents:\n      path: " + agentPath + "\n"
	if err := os.WriteFile(os.Getenv("SKILLSHARE_CONFIG"), []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	ts := "2024-01-15_14-30-45"
	seedSnapshot(t, backup.BackupDir(), ts, "claude-agents", "helper.md", "# helper")

	rr := serveJSON(t, s, http.MethodPost, "/api/restore/validate", `{"timestamp":"`+ts+`","target":"claude-agents"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"valid":true`) {
		t.Fatalf("validate: %d %s", rr.Code, rr.Body.String())
	}
	rr = serveJSON(t, s, http.MethodPost, "/api/restore", `{"timestamp":"`+ts+`","target":"claude-agents","force":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", rr.Code, rr.Body.String())
	}
	if data, err := os.ReadFile(filepath.Join(agentPath, "helper.md")); err != nil || string(data) != "# helper" {
		t.Fatalf("agent not restored into agents dir: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(skillsPath, "helper.md")); !os.IsNotExist(err) {
		t.Fatal("agents snapshot was restored into the skills dir")
	}
}

func TestHandleDeleteBackup(t *testing.T) {
	s, _ := newTestServer(t)
	ts := "2024-01-15_14-30-45"
	seedSnapshot(t, backup.BackupDir(), ts, "claude", "SKILL.md", "")

	if rr := serveJSON(t, s, http.MethodDelete, "/api/backups/"+ts, ""); rr.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(backup.BackupDir(), ts)); !os.IsNotExist(err) {
		t.Fatal("backup folder still exists")
	}
	if rr := serveJSON(t, s, http.MethodDelete, "/api/backups/"+ts, ""); rr.Code != http.StatusNotFound {
		t.Fatalf("second delete: %d, want 404", rr.Code)
	}
	if rr := serveJSON(t, s, http.MethodDelete, "/api/backups/..", ""); rr.Code == http.StatusOK {
		t.Fatal("a non-timestamp name must be refused")
	}
	if rr := serveJSON(t, s, http.MethodDelete, "/api/backups/2024-01-15_14-30-4x", ""); rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid timestamp: %d, want 400", rr.Code)
	}
}

func TestHandleDeleteAllBackups(t *testing.T) {
	s, _ := newTestServer(t)
	for _, ts := range []string{"2024-01-15_14-30-45", "2024-01-16_09-00-00"} {
		seedSnapshot(t, backup.BackupDir(), ts, "claude", "SKILL.md", "")
	}

	rr := serveJSON(t, s, http.MethodDelete, "/api/backups", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"removed":2`) {
		t.Fatalf("delete all: %d %s", rr.Code, rr.Body.String())
	}
	if left, _ := backup.ListInDir(backup.BackupDir()); len(left) != 0 {
		t.Fatalf("want no backups left, got %d", len(left))
	}
}

func TestHandleBackups_RetentionFollowsConfig(t *testing.T) {
	s, _ := newTestServer(t)
	now := time.Now()
	for i := range 3 {
		seedSnapshot(t, backup.BackupDir(), now.Add(-time.Duration(i)*time.Hour).Format("2006-01-02_15-04-05"), "claude", "SKILL.md", "")
	}

	if rr := serveJSON(t, s, http.MethodPatch, "/api/config", `{"backupMaxCount":-1}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("negative limit: %d, want 400", rr.Code)
	}
	if rr := serveJSON(t, s, http.MethodPatch, "/api/config", `{"backupMaxCount":1,"backupMaxSizeMB":0}`); rr.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rr.Code, rr.Body.String())
	}
	if rr := serveJSON(t, s, http.MethodGet, "/api/backups", ""); !strings.Contains(rr.Body.String(), `"retention":{"maxAgeDays":30,"maxCount":1,"maxSizeMB":0}`) {
		t.Fatalf("list: %s", rr.Body.String())
	}
	if rr := serveJSON(t, s, http.MethodPost, "/api/backup/cleanup", ""); !strings.Contains(rr.Body.String(), `"removed":2`) {
		t.Fatalf("cleanup: %d %s", rr.Code, rr.Body.String())
	}
}

func TestHandleBackups_ProjectModeUsesProjectBackupDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))
	s, projectRoot := newTestProjectServerWithExtras(t, nil)
	projectCfg := &config.ProjectConfig{Targets: []config.ProjectTargetEntry{{
		Name:   "claude",
		Skills: &config.ResourceTargetConfig{Path: ".claude/skills"},
		Agents: &config.ResourceTargetConfig{Path: ".claude/agents"},
	}}}
	if err := projectCfg.Save(projectRoot); err != nil {
		t.Fatal(err)
	}
	agentPath := filepath.Join(projectRoot, ".claude", "agents")
	os.MkdirAll(agentPath, 0755)
	os.WriteFile(filepath.Join(agentPath, "helper.md"), []byte("# helper"), 0644)

	if rr := serveJSON(t, s, http.MethodPost, "/api/backup", `{}`); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "claude-agents") {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	rr := serveJSON(t, s, http.MethodGet, "/api/backups", "")
	var resp struct {
		Backups []struct {
			Timestamp string   `json:"timestamp"`
			Path      string   `json:"path"`
			Targets   []string `json:"targets"`
		} `json:"backups"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Backups) != 1 || !strings.HasPrefix(resp.Backups[0].Path, backup.ProjectBackupDir(projectRoot)) {
		t.Fatalf("backups = %+v", resp.Backups)
	}
	if entries, _ := os.ReadDir(backup.BackupDir()); len(entries) != 0 {
		t.Fatal("project backup was written to the global backup dir")
	}

	os.WriteFile(filepath.Join(agentPath, "helper.md"), []byte("changed"), 0644)
	body := `{"timestamp":"` + resp.Backups[0].Timestamp + `","target":"claude-agents","force":true}`
	if rr := serveJSON(t, s, http.MethodPost, "/api/restore", body); rr.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", rr.Code, rr.Body.String())
	}
	if data, _ := os.ReadFile(filepath.Join(agentPath, "helper.md")); string(data) != "# helper" {
		t.Fatalf("restored content = %q", data)
	}
	if rr := serveJSON(t, s, http.MethodDelete, "/api/backups/"+resp.Backups[0].Timestamp, ""); rr.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rr.Code, rr.Body.String())
	}
}
